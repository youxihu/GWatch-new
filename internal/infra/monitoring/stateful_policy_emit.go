package monitoring

import (
	"time"

	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	"github.com/youxihu/GWatch-new/internal/domain/monitoring"
	domainMonitor "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
)

// evaluateAndEmitAlert 评估持续时间与防抖，决定是否发送告警
func (p *StatefulPolicy) evaluateAndEmitAlert(state *monitoring.AlertState, alertType entity.AlertType, severity string, currentThreshold, currentValue float64, pid, processName string, now time.Time, alertInterval, durationRequired time.Duration, notifyImmediately bool) *domainMonitor.AlertResult {
	duration := now.Sub(state.StartTime)
	logger.Debugf("[告警评估] %s 当前等级: %s, 累计持续时间: %v, 需要: %v", alertType, severity, duration, durationRequired)

	if duration < durationRequired {
		logger.Debugf("[告警评估] %s 未达到持续时间要求，继续等待", alertType)
		state.CurrentValue = currentValue
		p.persistAlertState(state, alertType)
		return nil
	}

	if !p.passedDebounce(state.EventID, now, alertInterval, notifyImmediately, alertType) {
		state.CurrentValue = currentValue
		p.persistAlertState(state, alertType)
		return nil
	}

	return p.emitAlertResult(state, alertType, severity, currentThreshold, currentValue, pid, processName, now)
}

// passedDebounce 检查是否通过防抖（立即通知等级只发送一次）
func (p *StatefulPolicy) passedDebounce(eventID string, now time.Time, alertInterval time.Duration, notifyImmediately bool, alertType entity.AlertType) bool {
	lastAlert, hasLastAlert := p.lastAlertTimes[eventID]

	if notifyImmediately {
		if !hasLastAlert {
			logger.Debugf("[告警评估] %s 需要立即通知，且未发送过告警 (EventID: %s)", alertType, eventID)
			return true
		}
		logger.Debugf("[告警评估] %s 需要立即通知，但已发送过告警，跳过 (EventID: %s)", alertType, eventID)
		return false
	}

	if !hasLastAlert {
		logger.Debugf("[告警评估] %s 无上次告警记录，可以发送 (EventID: %s)", alertType, eventID)
		return true
	}

	timeSinceLastAlert := now.Sub(lastAlert)
	if timeSinceLastAlert >= alertInterval {
		logger.Debugf("[告警评估] %s 防抖检查通过（已过: %v >= 防抖间隔: %v）", alertType, timeSinceLastAlert, alertInterval)
		return true
	}

	logger.Debugf("[告警评估] %s 仍在防抖期内（已过: %v < 防抖间隔: %v）", alertType, timeSinceLastAlert, alertInterval)
	return false
}

// emitAlertResult 构建并返回告警结果，更新Redis状态
func (p *StatefulPolicy) emitAlertResult(state *monitoring.AlertState, alertType entity.AlertType, severity string, currentThreshold, currentValue float64, pid, processName string, now time.Time) *domainMonitor.AlertResult {
	state.CurrentValue = currentValue
	state.AlertSent = true
	p.persistAlertState(state, alertType)
	p.lastAlertTimes[state.EventID] = now

	var diskUsageItems []domainMonitor.DiskUsageItem
	if alertType == entity.DiskHigh && len(state.DiskUsage) > 0 {
		for _, item := range state.DiskUsage {
			diskUsageItems = append(diskUsageItems, domainMonitor.DiskUsageItem{
				Path: item.Path, Size: item.Size, SizeBytes: item.SizeBytes,
				Type: item.Type, Depth: item.Depth, IsLeafResult: item.IsLeafResult,
			})
		}
	}

	mainPID, mainProcessName := pid, processName
	if len(state.Processes) > 0 {
		mainProcess := state.Processes[len(state.Processes)-1]
		mainPID = mainProcess.PID
		mainProcessName = mainProcess.ProcessName
	}

	duration := now.Sub(state.StartTime)
	logger.Warnf("[告警触发] %s 告警已触发（等级: %s, 累计持续时间: %v, 值: %.2f, 阈值: %.2f, 事件ID: %s）",
		alertType, severity, duration, currentValue, currentThreshold, state.EventID)

	return &domainMonitor.AlertResult{
		AlertType: alertType, EventID: state.EventID, Severity: severity,
		Duration: FormatDuration(duration), CurrentValue: currentValue,
		Threshold: currentThreshold, IsRecovery: false,
		StartTime: state.StartTime, TriggerTime: now,
		PID: mainPID, ProcessName: mainProcessName, DiskUsage: diskUsageItems,
	}
}

// persistAlertState 持久化告警状态到Redis
func (p *StatefulPolicy) persistAlertState(state *monitoring.AlertState, alertType entity.AlertType) {
	if p.alertStateStorage == nil {
		return
	}
	if err := p.alertStateStorage.UpdateAlertStateByEventID(state); err != nil {
		logger.Warnf("[告警评估] %s 更新告警状态失败: %v", alertType, err)
	}
}
