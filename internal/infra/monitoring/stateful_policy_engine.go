package monitoring

import (
	"time"

	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	domainMonitor "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
)

// applyInternal 策略引擎主入口：初始化 → 加锁 → 逐个处理决策
func (p *StatefulPolicy) applyInternal(cfg *shared.Config, metrics *entity.SystemMetrics, decisions []domainMonitor.Decision, peekMode bool) []domainMonitor.AlertResult {
	now := time.Now()
	var result []domainMonitor.AlertResult

	if !p.initialized && p.alertStateStorage != nil {
		if err := p.alertStateStorage.Init(cfg); err != nil {
			logger.Warnf("告警状态存储初始化失败: %v", err)
		} else {
			p.initialized = true
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	alertInterval := p.getAlertInterval(cfg)

	for _, decision := range decisions {
		if alertResult := p.processDecision(cfg, metrics, decision, now, alertInterval, peekMode); alertResult != nil {
			result = append(result, *alertResult)
		}
	}

	return result
}

// getAlertInterval 获取告警防抖间隔
func (p *StatefulPolicy) getAlertInterval(cfg *shared.Config) time.Duration {
	if cfg.Alerting != nil && cfg.Alerting.Trigger.AlertInterval > 0 {
		return cfg.Alerting.Trigger.AlertInterval
	}
	return 30 * time.Minute
}

// processDecision 处理单个决策的主流程编排
func (p *StatefulPolicy) processDecision(cfg *shared.Config, metrics *entity.SystemMetrics, decision domainMonitor.Decision, now time.Time, alertInterval time.Duration, peekMode bool) *domainMonitor.AlertResult {
	alertType := decision.Type
	currentValue := decision.CurrentValue
	pid := decision.PID
	processName := decision.ProcessName
	if pid == "" {
		pid = "default"
	}

	thresholdConfig := GetThresholdConfig(cfg, alertType)
	if !thresholdConfig.HasConfig {
		return p.handleNoConfigAlert(alertType, currentValue, now, alertInterval)
	}

	isLocalRecovered := p.checkLocalRecovery(cfg, metrics, alertType, currentValue, pid, thresholdConfig.Base)
	if isLocalRecovered {
		return p.handleRecovery(cfg, alertType, currentValue, pid, processName, thresholdConfig.Base, now, alertInterval)
	}

	p.clearRecoveryStartTimeIfNeeded(cfg, alertType, currentValue, pid, processName, thresholdConfig.Base)

	severity, currentThreshold, shouldContinue := p.determineAlertSeverityAndThreshold(cfg, metrics, alertType, currentValue, pid, thresholdConfig)
	if shouldContinue {
		return nil
	}

	durationRequired, notifyImmediately := p.getDurationConfig(cfg, severity)

	state, err := p.findOrCreateAlertState(alertType, severity, currentThreshold, currentValue, pid, processName, now, peekMode)
	if err != nil {
		logger.Warnf("%s 处理告警状态失败: %v", alertType, err)
		return nil
	}
	if state == nil {
		return nil
	}

	return p.evaluateAndEmitAlert(state, alertType, severity, currentThreshold, currentValue, pid, processName, now, alertInterval, durationRequired, notifyImmediately)
}

// handleNoConfigAlert 处理无阈值配置的告警类型（如错误类告警），直接触发
func (p *StatefulPolicy) handleNoConfigAlert(alertType entity.AlertType, currentValue float64, now time.Time, alertInterval time.Duration) *domainMonitor.AlertResult {
	eventID, err := p.eventIDGenerator.GenerateUniqueEventID(string(alertType), "reminder", 3)
	if err != nil {
		logger.Warnf("生成唯一事件ID失败: %v，回退到普通生成方法", err)
		eventID = p.eventIDGenerator.GenerateEventID(string(alertType), "reminder")
	}

	lastAlert, hasLastAlert := p.lastAlertTimes[eventID]
	if !hasLastAlert || now.Sub(lastAlert) >= alertInterval {
		p.lastAlertTimes[eventID] = now
		logger.Warnf("%s 告警触发（无阈值配置，EventID: %s）", alertType, eventID)
		return &domainMonitor.AlertResult{
			AlertType: alertType, EventID: eventID, Severity: "reminder",
			Duration: "0s", CurrentValue: currentValue, Threshold: 0,
			IsRecovery: false, StartTime: now, TriggerTime: now,
		}
	}
	return nil
}

// checkLocalRecovery 检查本机指标是否已恢复到正常范围
func (p *StatefulPolicy) checkLocalRecovery(cfg *shared.Config, metrics *entity.SystemMetrics, alertType entity.AlertType, currentValue float64, pid string, base float64) bool {
	if alertType == entity.HTTPErr {
		if metrics != nil && len(metrics.HTTP.Interfaces) > 0 {
			for _, httpInterface := range metrics.HTTP.Interfaces {
				if httpInterface.Name == pid {
					return CheckHTTPRecovery(currentValue, httpInterface.AllowedCodes)
				}
			}
		}
		return currentValue >= 200.0
	}

	if alertType == entity.RedisLow || alertType == entity.RedisHigh {
		if cfg.AppMonitoring != nil && cfg.AppMonitoring.Redis != nil && cfg.AppMonitoring.Redis.Thresholds.Clients != nil {
			min := float64(cfg.AppMonitoring.Redis.Thresholds.Clients.Min)
			max := float64(cfg.AppMonitoring.Redis.Thresholds.Clients.Max)
			return currentValue >= min && currentValue <= max
		}
		return true
	}

	return CheckRecovery(alertType, currentValue, base)
}

// determineAlertSeverityAndThreshold 根据告警类型确定等级和对应阈值
func (p *StatefulPolicy) determineAlertSeverityAndThreshold(cfg *shared.Config, metrics *entity.SystemMetrics, alertType entity.AlertType, currentValue float64, pid string, thresholdConfig ThresholdConfig) (string, float64, bool) {
	if alertType == entity.RedisLow || alertType == entity.RedisHigh {
		return p.determineRedisAlertSeverity(cfg, alertType, currentValue)
	}
	if alertType == entity.HTTPErr {
		return p.determineHTTPAlertSeverity(metrics, currentValue, pid)
	}

	severity := DetermineSeverity(currentValue, thresholdConfig, alertType)
	if severity == "" {
		return "", 0, true
	}
	return severity, thresholdConfig.GetThresholdBySeverity(severity), false
}

// determineRedisAlertSeverity 确定Redis连接数告警等级
func (p *StatefulPolicy) determineRedisAlertSeverity(cfg *shared.Config, alertType entity.AlertType, currentValue float64) (string, float64, bool) {
	if cfg.AppMonitoring == nil || cfg.AppMonitoring.Redis == nil || cfg.AppMonitoring.Redis.Thresholds.Clients == nil {
		return "", 0, true
	}
	min := float64(cfg.AppMonitoring.Redis.Thresholds.Clients.Min)
	max := float64(cfg.AppMonitoring.Redis.Thresholds.Clients.Max)

	if alertType == entity.RedisLow && currentValue < min {
		return "p2", min, false
	}
	if alertType == entity.RedisHigh && currentValue > max {
		return "p2", max, false
	}
	return "", 0, true
}

// determineHTTPAlertSeverity 确定HTTP接口告警等级
func (p *StatefulPolicy) determineHTTPAlertSeverity(metrics *entity.SystemMetrics, currentValue float64, pid string) (string, float64, bool) {
	shouldAlert := false
	if metrics != nil && len(metrics.HTTP.Interfaces) > 0 {
		for _, httpInterface := range metrics.HTTP.Interfaces {
			if httpInterface.Name == pid {
				if httpInterface.NeedAlert {
					shouldAlert = !isValidHTTPStatusCode(int(currentValue), httpInterface.AllowedCodes)
				}
				break
			}
		}
	} else {
		shouldAlert = currentValue != 0 && int(currentValue) != 200
	}

	if shouldAlert {
		return "p2", currentValue, false
	}
	return "", 0, true
}

// getDurationConfig 获取持续时间要求和立即通知配置
func (p *StatefulPolicy) getDurationConfig(cfg *shared.Config, severity string) (time.Duration, bool) {
	var durationRequired time.Duration
	if cfg.Alerting != nil && cfg.Alerting.Trigger.DurationRequired > 0 {
		durationRequired = cfg.Alerting.Trigger.DurationRequired
	} else {
		durationRequired = 5 * time.Minute
	}

	var notifyImmediately bool
	if cfg.Alerting != nil {
		switch severity {
		case "p1":
			notifyImmediately = cfg.Alerting.SeverityLevels.P1.NotifyImmediately
		case "p2":
			notifyImmediately = cfg.Alerting.SeverityLevels.P2.NotifyImmediately
		case "p3":
			notifyImmediately = cfg.Alerting.SeverityLevels.P3.NotifyImmediately
		case "reminder":
			notifyImmediately = cfg.Alerting.SeverityLevels.Reminder.NotifyImmediately
		}
	}

	return durationRequired, notifyImmediately
}

// isValidHTTPStatusCode 检查状态码是否在允许列表中
func isValidHTTPStatusCode(code int, allowedCodes []int) bool {
	if len(allowedCodes) == 0 {
		return code == 200
	}
	for _, c := range allowedCodes {
		if c == code {
			return true
		}
	}
	return false
}
