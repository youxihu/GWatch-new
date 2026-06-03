package monitoring

import (
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	"github.com/youxihu/GWatch-new/internal/domain/monitoring"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	"github.com/youxihu/GWatch-new/internal/utils"
	"strconv"
	"time"
)

// findOrCreateAlertState 查找现有告警状态，若不存在则创建新状态
func (p *StatefulPolicy) findOrCreateAlertState(alertType entity.AlertType, severity string, currentThreshold, currentValue float64, pid, processName string, now time.Time, peekMode bool) (*monitoring.AlertState, error) {
	if p.alertStateStorage == nil {
		if alertType == entity.HTTPErr {
			logger.Warnf("HTTPErr告警需要Redis存储，但alertStateStorage为nil")
		}
		return nil, nil
	}

	existingState := p.findExistingState(alertType, processName)
	if existingState != nil {
		return p.updateExistingState(existingState, alertType, severity, currentThreshold, currentValue, pid, processName, now), nil
	}

	return p.createNewState(alertType, severity, currentThreshold, currentValue, pid, processName, now, peekMode)
}

// findExistingState 从Redis查找现有告警状态（CPU/内存优先按进程名，其他按告警类型）
func (p *StatefulPolicy) findExistingState(alertType entity.AlertType, processName string) *monitoring.AlertState {
	var existingState *monitoring.AlertState
	var err error

	if (alertType == entity.CPUHigh || alertType == entity.MemHigh) && processName != "" {
		logger.Debugf("[状态查找] %s 尝试通过进程名查找现有状态: %s", alertType, processName)
		existingState, err = p.alertStateStorage.GetAlertStateByProcessName(alertType, processName)
		if err != nil {
			logger.Warnf("通过进程名查找告警状态失败: %v", err)
		} else if existingState != nil {
			logger.Debugf("[状态查找] %s 通过进程名找到现有状态: EventID=%s", alertType, existingState.EventID)
		}
	}

	if existingState == nil && alertType != entity.CPUHigh && alertType != entity.MemHigh {
		logger.Debugf("[状态查找] %s 尝试通过告警类型查找现有状态", alertType)
		existingState, err = p.alertStateStorage.GetAlertStateByType(alertType)
		if err != nil {
			logger.Warnf("通过告警类型查找告警状态失败: %v", err)
		} else if existingState != nil {
			logger.Debugf("[状态查找] %s 通过告警类型找到现有状态: EventID=%s", alertType, existingState.EventID)
		}
	}

	return existingState
}

// updateExistingState 更新现有告警状态（等级变化、进程信息、磁盘扫描检查）
func (p *StatefulPolicy) updateExistingState(state *monitoring.AlertState, alertType entity.AlertType, severity string, currentThreshold, currentValue float64, pid, processName string, now time.Time) *monitoring.AlertState {
	if state.Severity != severity {
		logger.Debugf("%s 告警等级变化: %s -> %s", alertType, state.Severity, severity)
		state.Severity = severity
		state.Threshold = currentThreshold
		state.SeverityStartTime = now
	}

	p.updateProcessInfo(state, alertType, currentValue, pid, processName, now)
	p.cleanupStaleProcesses(state, 10*time.Minute)
	state.CurrentValue = currentValue
	p.checkDiskScanIfNeeded(alertType, state, now)

	return state
}

// updateProcessInfo 更新或添加进程信息到告警状态
func (p *StatefulPolicy) updateProcessInfo(state *monitoring.AlertState, alertType entity.AlertType, currentValue float64, pid, processName string, now time.Time) {
	cpuPercent, memPercent := p.getProcessMetrics(alertType, currentValue, pid)

	processInfo := monitoring.ProcessInfo{
		PID: pid, ProcessName: processName,
		FirstDetectedTime: now, LastSeenTime: now,
		CPUPercent: cpuPercent, MemPercent: memPercent,
	}

	for i, proc := range state.Processes {
		if proc.PID == pid {
			processInfo.FirstDetectedTime = proc.FirstDetectedTime
			state.Processes[i] = processInfo
			logger.Debugf("%s 更新进程信息: PID %s (%s)", alertType, pid, processName)
			return
		}
	}

	state.Processes = append(state.Processes, processInfo)
	logger.Debugf("%s 添加新进程: PID %s (%s), EventID: %s", alertType, pid, processName, state.EventID)
}

// getProcessMetrics 获取进程的CPU和内存使用率
func (p *StatefulPolicy) getProcessMetrics(alertType entity.AlertType, currentValue float64, pid string) (cpuPercent, memPercent float64) {
	if pidInt, err := strconv.Atoi(pid); err == nil {
		if foundProcess, err := utils.FindProcessByPID(p.hostCollector, alertType, int32(pidInt), 50); err == nil && foundProcess != nil {
			return foundProcess.CPUPercent, float64(foundProcess.MemPercent)
		}
	}

	switch alertType {
	case entity.CPUHigh:
		return currentValue, 0
	case entity.MemHigh:
		return 0, currentValue
	}
	return 0, 0
}

// checkDiskScanIfNeeded 检查是否需要触发磁盘占用扫描
func (p *StatefulPolicy) checkDiskScanIfNeeded(alertType entity.AlertType, state *monitoring.AlertState, now time.Time) {
	if alertType != entity.DiskHigh || p.diskEnricher == nil {
		return
	}

	shouldRescan := state.DiskUsage == nil || len(state.DiskUsage) == 0
	if !shouldRescan && !state.DiskUsageUpdateTime.IsZero() {
		shouldRescan = now.Sub(state.DiskUsageUpdateTime) > 5*time.Minute
	}

	if shouldRescan {
		p.diskEnricher.EnrichAsync(alertType, "default", state.EventID)
	}
}

// createNewState 创建新的告警状态并保存到Redis
func (p *StatefulPolicy) createNewState(alertType entity.AlertType, severity string, currentThreshold, currentValue float64, pid, processName string, now time.Time, peekMode bool) (*monitoring.AlertState, error) {
	eventID := p.generateOrReuseEventID(alertType, severity, processName)
	if eventID == "" {
		return nil, nil
	}

	cpuPercent, memPercent := p.getProcessMetrics(alertType, currentValue, pid)

	state := &monitoring.AlertState{
		EventID: eventID, AlertType: alertType, StartTime: now,
		Severity: severity, SeverityStartTime: now,
		Threshold: currentThreshold, CurrentValue: currentValue,
		AlertSent: false,
		Processes: []monitoring.ProcessInfo{{
			PID: pid, ProcessName: processName,
			FirstDetectedTime: now, LastSeenTime: now,
			CPUPercent: cpuPercent, MemPercent: memPercent,
		}},
	}

	if !peekMode {
		logger.Debugf("[状态保存] %s 保存新状态到Redis: EventID=%s", alertType, eventID)
		if err := p.alertStateStorage.SetAlertStateByEventID(state); err != nil {
			logger.Warnf("保存告警状态失败: %v", err)
			return nil, err
		}
	}

	logger.Debugf("%s 首次触发阈值（等级: %s, 值: %.2f, EventID: %s）", alertType, severity, currentValue, eventID)

	if alertType == entity.DiskHigh && p.diskEnricher != nil {
		p.diskEnricher.EnrichAsync(alertType, "default", eventID)
	}

	return state, nil
}

// generateOrReuseEventID 生成新事件ID或复用相同进程名的现有事件ID
func (p *StatefulPolicy) generateOrReuseEventID(alertType entity.AlertType, severity, processName string) string {
	if (alertType == entity.CPUHigh || alertType == entity.MemHigh) && processName != "" {
		if existingState, _ := p.alertStateStorage.GetAlertStateByProcessName(alertType, processName); existingState != nil {
			logger.Debugf("%s 复用现有事件ID: %s (进程名: %s)", alertType, existingState.EventID, processName)
			return existingState.EventID
		}
	}

	eventID, err := p.eventIDGenerator.GenerateUniqueEventID(string(alertType), severity, 3)
	if err != nil {
		logger.Warnf("生成唯一事件ID失败: %v，回退到普通生成方法", err)
		eventID = p.eventIDGenerator.GenerateEventID(string(alertType), severity)
	}
	logger.Debugf("%s 生成新事件ID: %s", alertType, eventID)
	return eventID
}
