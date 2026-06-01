package monitoring

import (
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	domainAlert "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	domainMonitor "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	"github.com/youxihu/GWatch-new/internal/utils"
	"strconv"
)

// NotifyWithAlertTypes 按给定的告警类型集合直接构建并发送通知（用于"同时告警"合并场景）
// 保留此方法以保持向后兼容，内部调用NotifyWithAlertResults
func (useCase *MonitoringUseCase) NotifyWithAlertTypes(config *shared.Config, metrics *entity.SystemMetrics, alertTypes []entity.AlertType) error {
	// 转换为AlertResult（兼容旧接口）
	alertResults := make([]domainMonitor.AlertResult, len(alertTypes))
	for i, alertType := range alertTypes {
		alertResults[i] = domainMonitor.AlertResult{
			AlertType:    alertType,
			EventID:      "",
			Severity:     "reminder",
			Duration:     "0s",
			CurrentValue: 0,
			Threshold:    0,
			IsRecovery:   false,
		}
	}
	return useCase.NotifyWithAlertResults(config, metrics, alertResults)
}

// NotifyWithAlertResults 按给定的告警结果集合直接构建并发送通知（用于"同时告警"合并场景）
func (useCase *MonitoringUseCase) NotifyWithAlertResults(config *shared.Config, metrics *entity.SystemMetrics, alertResults []domainMonitor.AlertResult) error {
	if len(alertResults) == 0 {
		logger.Debugf("[通知处理] 告警结果列表为空，跳过通知")
		return nil
	}

	logger.Infof("[通知处理] 收到 %d 个告警结果，开始处理", len(alertResults))
	var triggeredAlerts []domainAlert.TriggeredAlert

	for i, alertResult := range alertResults {
		logger.Infof("[通知处理] 处理第 %d/%d 个告警结果: 类型=%v, 事件ID=%s, 是否恢复=%v, 等级=%s, 持续时间=%s",
			i+1, len(alertResults), alertResult.AlertType, alertResult.EventID, alertResult.IsRecovery, alertResult.Severity, alertResult.Duration)
		message, processInfo, diskUsageItems, isSkipped := useCase.buildAlertMessageFromResult(config, metrics, alertResult)

		// 只有白名单才跳过告警，message为空不应该跳过（因为formatter会构建完整的告警消息）
		if isSkipped {
			logger.Infof("[通知处理] 完全跳过告警: 类型=%v, 原因=白名单", alertResult.AlertType)
			continue
		}

		var process *entity.ProcessInfo
		if processInfo != nil {
			process = &entity.ProcessInfo{
				PID:        processInfo.PID,
				Name:       processInfo.Name,
				CPUPercent: processInfo.CPUPercent,
				MemPercent: processInfo.MemPercent,
				MemRSS:     processInfo.MemRSS,
			}
		}

		triggeredAlerts = append(triggeredAlerts, domainAlert.TriggeredAlert{
			Type:         alertResult.AlertType,
			Message:      message,
			EventID:      alertResult.EventID,
			Severity:     alertResult.Severity,
			Duration:     alertResult.Duration,
			CurrentValue: alertResult.CurrentValue,
			Threshold:    alertResult.Threshold,
			IsRecovery:   alertResult.IsRecovery,
			StartTime:    alertResult.StartTime,
			TriggerTime:  alertResult.TriggerTime,
			Process:      process,
			DiskUsage:    diskUsageItems,
		})
	}

	// 核心逻辑：如果所有告警项都被过滤，则不发送通知
	if len(triggeredAlerts) == 0 {
		logger.Warnf("[通知处理] 所有告警项均被过滤，本次通知已取消（原始结果数: %d）", len(alertResults))
		return nil // 彻底静默，不发任何消息
	}

	logger.Infof("[通知处理] 成功处理 %d/%d 个告警项，准备发送通知", len(triggeredAlerts), len(alertResults))

	alertTitle := "GWatch 服务器告警" // 默认标题
	if config.HostMonitoring != nil && config.HostMonitoring.AlertTitle != "" {
		alertTitle = config.HostMonitoring.AlertTitle
	}
	alertBody := useCase.alertFormatter.Build(alertTitle, config, metrics, triggeredAlerts)

	// 保存告警日志（无论是否发送钉钉通知，都记录日志）
	if useCase.alertLogStorage != nil {
		// 确保告警日志存储已初始化（如果还未初始化）
		if err := useCase.alertLogStorage.Init(config); err != nil {
			logger.Infof("[告警日志] 初始化告警日志存储失败: %v", err)
		}

		// 按事件ID保存告警和恢复日志（日志追溯）
		for _, alert := range triggeredAlerts {
			if alert.EventID != "" {
				if err := useCase.alertLogStorage.SaveAlertWithEventID(alertTitle, alertBody, alert.EventID, alert.IsRecovery, alert.TriggerTime); err != nil {
					logger.Infof("[告警日志] 保存事件日志失败 (事件ID: %s): %v", alert.EventID, err)
				}
			}
		}
	} else {
		logger.Infof("[告警日志] alertLogStorage 为 nil，无法保存告警日志")
	}

	// 检查是否启用通知（全局配置）
	// 默认启用通知（向后兼容：如果配置中未设置该字段，默认启用）
	enableNotification := true
	// 如果全局配置中明确设置了EnableNotification为false，则禁用通知
	if config.EnableNotification == false {
		enableNotification = false
	}

	// 发送钉钉通知（如果启用）
	if enableNotification {
		logger.Infof("[通知发送] 准备发送钉钉通知（标题: %s, 包含 %d 个告警项）", alertTitle, len(triggeredAlerts))
		if err := useCase.alertNotifier.Send(alertTitle, alertBody); err != nil {
			logger.Errorf("[通知发送] 发送钉钉通知失败: %v", err)
			return err
		}
		logger.Infof("[通知发送] 钉钉通知发送成功（标题: %s）", alertTitle)
		return nil
	} else {
		logger.Infof("[通知发送] 通知功能已禁用（全局配置），跳过发送钉钉通知（标题: %s）", alertTitle)
		return nil
	}
}

// buildAlertMessageFromResult 根据告警结果构建消息
// 返回: 最终消息, 进程信息, 磁盘占用信息, 是否应跳过该告警（如白名单）
func (useCase *MonitoringUseCase) buildAlertMessageFromResult(
	config *shared.Config,
	metrics *entity.SystemMetrics,
	alertResult domainMonitor.AlertResult,
) (string, *entity.ProcessInfo, []domainAlert.DiskUsageItem, bool) {
	alertType := alertResult.AlertType
	message := "" // 消息由formatter构建，这里只返回简单消息
	var processInfo *entity.ProcessInfo
	var diskUsageItems []domainAlert.DiskUsageItem

	// 如果是恢复通知，从Redis中获取原始告警的进程信息和磁盘占用信息
	if alertResult.IsRecovery {
		// 恢复通知应该显示原始告警时的磁盘占用信息（从AlertResult中获取）
		if alertType == entity.DiskHigh {
			if alertResult.DiskUsage != nil {
				diskUsageItems = alertResult.DiskUsage
				logger.Debugf("[恢复通知] 从AlertResult读取磁盘占用信息，共 %d 项", len(diskUsageItems))
			} else {
				logger.Debugf("[恢复通知] AlertResult中磁盘占用信息为空")
			}
		}

		// 恢复通知应该显示原始告警时的进程信息（从Redis中获取的PID和ProcessName）
		if alertType == entity.CPUHigh || alertType == entity.MemHigh {
			// 使用AlertResult中的PID和ProcessName（从Redis获取）
			if alertResult.PID != "" && alertResult.PID != "default" {
				pidInt, err := strconv.ParseInt(alertResult.PID, 10, 32)
				if err == nil {
					// 使用统一的函数根据PID查找进程信息（如果进程还存在）
					foundProcess, err := utils.FindProcessByPID(useCase.hostCollector, alertType, int32(pidInt), 100)
					if err == nil && foundProcess != nil {
						// 如果找到了进程，使用当前进程信息
						processInfo = &entity.ProcessInfo{
							PID:        foundProcess.PID,
							Name:       foundProcess.Name,
							CPUPercent: foundProcess.CPUPercent,
							MemPercent: foundProcess.MemPercent,
							MemRSS:     foundProcess.MemRSS,
						}
					} else if alertResult.ProcessName != "" {
						// 进程已不存在或查找失败，使用Redis中存储的进程名
						processInfo = &entity.ProcessInfo{
							PID:        int32(pidInt),
							Name:       alertResult.ProcessName,
							CPUPercent: 0, // 进程已不存在，无法获取当前值
							MemPercent: 0,
							MemRSS:     0,
						}
					}
				}
			}
		}
		return message, processInfo, diskUsageItems, false
	}

	// 获取磁盘占用信息（用于磁盘告警和恢复通知）
	// 从 AlertResult 中读取（已在 stateful_policy 中同步获取）
	if alertType == entity.DiskHigh {
		if alertResult.DiskUsage != nil {
			diskUsageItems = alertResult.DiskUsage
			logger.Debugf("[磁盘告警] 从AlertResult读取磁盘占用信息，共 %d 项", len(diskUsageItems))
		}
	}

	// 获取进程信息（用于CPU和内存告警）
	if alertType == entity.CPUHigh || alertType == entity.MemHigh {
		// 使用统一的函数获取Top进程
		culpritProcess, err := utils.GetTopProcessByType(useCase.hostCollector, alertType, 5)
		if err == nil && culpritProcess != nil {
			// 白名单核心逻辑：只在这里写一次
			if config != nil && utils.IsProcessInWhiteList(culpritProcess.Name, config.WhiteProcessList) {
				logger.Infof("[白名单忽略] 进程 '%s' (PID=%d) 触发 %v，已在白名单中，跳过告警", culpritProcess.Name, culpritProcess.PID, alertType)
				return "", nil, nil, true // 跳过告警：空消息、标记跳过
			}

			// 保存进程信息
			processInfo = &entity.ProcessInfo{
				PID:        culpritProcess.PID,
				Name:       culpritProcess.Name,
				CPUPercent: culpritProcess.CPUPercent,
				MemPercent: culpritProcess.MemPercent,
				MemRSS:     culpritProcess.MemRSS,
			}

		}
	}

	return message, processInfo, diskUsageItems, false
}
