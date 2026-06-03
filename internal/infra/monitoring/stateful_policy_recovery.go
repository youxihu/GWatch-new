package monitoring

import (
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"github.com/youxihu/GWatch-new/internal/domain/monitoring"
	domainMonitor "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	"time"
)

// handleRecovery 处理恢复逻辑：查找Redis中的告警状态，检查进程恢复，执行恢复流程
func (p *StatefulPolicy) handleRecovery(cfg *shared.Config, alertType entity.AlertType, currentValue float64, pid, processName string, base float64, now time.Time, alertInterval time.Duration) *domainMonitor.AlertResult {
	if p.alertStateStorage == nil {
		return nil
	}

	state := p.findAlertState(alertType, pid, processName)
	if state == nil {
		return nil
	}

	if !p.allProcessesRecovered(alertType, state, base) {
		return nil
	}

	if !p.hasPriorAlertNotification(state) {
		logger.Debugf("[恢复] %s 未发送过告警通知，不处理恢复（事件ID: %s）", alertType, state.EventID)
		if err := p.alertStateStorage.DeleteAlertStateByEventID(state.EventID); err != nil {
			logger.Warnf("[恢复] %s 删除Redis状态失败: %v", alertType, err)
		}
		return nil
	}

	return p.executeRecoveryFlow(cfg, state, alertType, currentValue, now, alertInterval)
}

// findAlertState 从Redis查找告警状态（优先按进程名，其次按告警类型）
func (p *StatefulPolicy) findAlertState(alertType entity.AlertType, pid, processName string) *monitoring.AlertState {
	var state *monitoring.AlertState
	var err error

	if processName != "" && (alertType == entity.CPUHigh || alertType == entity.MemHigh) {
		state, err = p.alertStateStorage.GetAlertStateByProcessName(alertType, processName)
		if err != nil {
			logger.Warnf("[恢复] %s 通过进程名查找告警状态失败: %v", alertType, err)
		}
	}

	if state == nil {
		state, err = p.alertStateStorage.GetAlertStateByType(alertType)
		if err != nil {
			logger.Warnf("[恢复] %s 通过告警类型查找告警状态失败: %v", alertType, err)
		}
	}

	return state
}

// allProcessesRecovered 检查Redis中记录的所有进程是否都已恢复
func (p *StatefulPolicy) allProcessesRecovered(alertType entity.AlertType, state *monitoring.AlertState, base float64) bool {
	if alertType != entity.CPUHigh && alertType != entity.MemHigh {
		return true
	}

	for _, proc := range state.Processes {
		recovered, exists, value := p.recoveryChecker.CheckProcessRecovery(alertType, proc.PID, base)
		if !recovered {
			logger.Debugf("[恢复] %s 进程PID %s (%s) 仍高负载(%.2f >= %.2f)，未完全恢复",
				alertType, proc.PID, proc.ProcessName, value, base)
			return false
		}
		if exists {
			logger.Debugf("[恢复] %s 进程PID %s (%s) 已恢复，当前值: %.2f", alertType, proc.PID, proc.ProcessName, value)
		} else {
			logger.Debugf("[恢复] %s 进程PID %s (%s) 已不存在，视为恢复", alertType, proc.PID, proc.ProcessName)
		}
	}
	return true
}

// hasPriorAlertNotification 检查是否已发送过告警通知（有告警才有恢复）
func (p *StatefulPolicy) hasPriorAlertNotification(state *monitoring.AlertState) bool {
	if state.AlertSent {
		return true
	}
	_, hasSentAlert := p.lastAlertTimes[state.EventID]
	return hasSentAlert
}

// executeRecoveryFlow 执行完整恢复流程：持续时间检查 → 防抖检查 → 构建恢复通知 → 清理状态
func (p *StatefulPolicy) executeRecoveryFlow(cfg *shared.Config, state *monitoring.AlertState, alertType entity.AlertType, currentValue float64, now time.Time, alertInterval time.Duration) *domainMonitor.AlertResult {
	logger.Infof("[恢复] %s 开始恢复流程，事件ID: %s", alertType, state.EventID)

	recoveryDurationRequired := p.recoveryDurationRequired(cfg)
	recoveryStartKey := string(alertType) + "_recovery_start_" + state.EventID

	recoveryStartTime := p.getOrSetRecoveryStartTime(recoveryStartKey, alertType, state.EventID, state.Severity, now)
	recoveryDuration := now.Sub(recoveryStartTime)
	faultDuration := recoveryStartTime.Sub(state.StartTime)

	if recoveryDuration < recoveryDurationRequired {
		logger.Debugf("[恢复] %s 恢复正常持续 %v < 需要 %v，继续等待", alertType, recoveryDuration, recoveryDurationRequired)
		return nil
	}

	recoveryDebounceKey := state.EventID + "_recovery"
	recoveryDebounceInterval := alertInterval / 2
	if recoveryDebounceInterval < time.Minute {
		recoveryDebounceInterval = time.Minute
	}

	if lastRecovery, hasLastRecovery := p.lastAlertTimes[recoveryDebounceKey]; hasLastRecovery && now.Sub(lastRecovery) < recoveryDebounceInterval {
		logger.Debugf("[恢复] %s 仍在防抖期内，跳过发送 (EventID: %s)", alertType, state.EventID)
		return nil
	}

	result := p.buildRecoveryAlertResult(state, alertType, currentValue, faultDuration, now)
	logger.Infof("[恢复] %s 恢复通知已触发，事件ID: %s，故障持续: %v，恢复正常持续: %v",
		alertType, state.EventID, faultDuration, recoveryDuration)

	if err := p.alertStateStorage.DeleteAlertStateByEventID(state.EventID); err != nil {
		logger.Warnf("[恢复] %s 删除Redis状态失败: %v", alertType, err)
	}
	delete(p.recoveryStartTimes, recoveryStartKey)
	p.lastAlertTimes[recoveryDebounceKey] = now

	return result
}

// recoveryDurationRequired 获取恢复通知持续时间要求
func (p *StatefulPolicy) recoveryDurationRequired(cfg *shared.Config) time.Duration {
	if cfg.Alerting != nil && cfg.Alerting.Trigger.RecoveryDurationRequired > 0 {
		return cfg.Alerting.Trigger.RecoveryDurationRequired
	}
	return 5 * time.Minute
}

// getOrSetRecoveryStartTime 获取或首次记录恢复开始时间
func (p *StatefulPolicy) getOrSetRecoveryStartTime(key string, alertType entity.AlertType, eventID, severity string, now time.Time) time.Time {
	if recoveryStartTime, exists := p.recoveryStartTimes[key]; exists {
		return recoveryStartTime
	}
	if p.recoveryStartTimes == nil {
		p.recoveryStartTimes = make(map[string]time.Time)
	}
	p.recoveryStartTimes[key] = now
	logger.Infof("[恢复] %s 首次检测到恢复，事件ID: %s，告警等级: %s", alertType, eventID, severity)
	return now
}

// buildRecoveryAlertResult 构建恢复通知结果
func (p *StatefulPolicy) buildRecoveryAlertResult(state *monitoring.AlertState, alertType entity.AlertType, currentValue float64, faultDuration time.Duration, now time.Time) *domainMonitor.AlertResult {
	var mainPID, mainProcessName string
	if len(state.Processes) > 0 {
		mainProcess := state.Processes[len(state.Processes)-1]
		mainPID = mainProcess.PID
		mainProcessName = mainProcess.ProcessName
	}

	var diskUsageItems []domainMonitor.DiskUsageItem
	if alertType == entity.DiskHigh && len(state.DiskUsage) > 0 {
		for _, item := range state.DiskUsage {
			diskUsageItems = append(diskUsageItems, domainMonitor.DiskUsageItem{
				Path: item.Path, Size: item.Size, SizeBytes: item.SizeBytes,
				Type: item.Type, Depth: item.Depth, IsLeafResult: item.IsLeafResult,
			})
		}
		logger.Debugf("[恢复通知] 使用磁盘占用信息，共 %d 项", len(diskUsageItems))
	}

	return &domainMonitor.AlertResult{
		AlertType: alertType, EventID: state.EventID, Severity: state.Severity,
		Duration: FormatDuration(faultDuration), CurrentValue: currentValue,
		Threshold: state.Threshold, IsRecovery: true, StartTime: state.StartTime,
		TriggerTime: now, PID: mainPID, ProcessName: mainProcessName, DiskUsage: diskUsageItems,
	}
}

// clearRecoveryStartTimeIfNeeded 值回升时清除恢复开始时间记录
func (p *StatefulPolicy) clearRecoveryStartTimeIfNeeded(cfg *shared.Config, alertType entity.AlertType, currentValue float64, pid, processName string, base float64) {
	if p.alertStateStorage == nil {
		return
	}

	currentState := p.findAlertState(alertType, pid, processName)
	if currentState == nil {
		return
	}

	recoveryStartKey := string(alertType) + "_recovery_start_" + currentState.EventID
	if _, exists := p.recoveryStartTimes[recoveryStartKey]; exists {
		delete(p.recoveryStartTimes, recoveryStartKey)
		condition := ">="
		if alertType == entity.RedisLow {
			condition = "<"
		}
		logger.Debugf("[恢复] %s 值回升（当前值: %.2f %s base: %.2f），清除恢复开始时间记录",
			alertType, currentValue, condition, base)
	}
}
