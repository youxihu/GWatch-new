package monitoring

import (
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"github.com/youxihu/GWatch-new/internal/domain/monitoring"
	domainMonitor "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	"github.com/youxihu/GWatch-new/internal/utils"
	"strconv"
	"time"
)

func (p *StatefulPolicy) applyInternal(cfg *shared.Config, metrics *entity.SystemMetrics, decisions []domainMonitor.Decision, peekMode bool) []domainMonitor.AlertResult {
	now := time.Now()
	var result []domainMonitor.AlertResult

	// 初始化告警状态存储
	if !p.initialized && p.alertStateStorage != nil {
		if err := p.alertStateStorage.Init(cfg); err != nil {
			logger.Warnf("告警状态存储初始化失败: %v", err)
		} else {
			p.initialized = true
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 获取配置参数
	var alertInterval time.Duration
	if cfg.Alerting != nil {
		alertInterval = cfg.Alerting.Trigger.AlertInterval
	} else {
		alertInterval = 30 * time.Minute // 默认防抖间隔
	}

	// 处理每个决策
	for _, decision := range decisions {
		alertType := decision.Type
		currentValue := decision.CurrentValue
		pid := decision.PID
		processName := decision.ProcessName
		if pid == "" {
			pid = "default"
		}

		// 获取阈值配置
		thresholdConfig := GetThresholdConfig(cfg, alertType)
		if !thresholdConfig.HasConfig {
			// 没有配置的告警类型（如错误类型），直接触发
			eventID, err := p.eventIDGenerator.GenerateUniqueEventID(string(alertType), 3)
			if err != nil {
				// 如果生成唯一ID失败，回退到普通生成方法
				logger.Warnf("生成唯一事件ID失败: %v，回退到普通生成方法", err)
				eventID = p.eventIDGenerator.GenerateEventID(string(alertType))
			}
			alertKey := eventID
			lastAlert, hasLastAlert := p.lastAlertTimes[alertKey]
			if !hasLastAlert || now.Sub(lastAlert) >= alertInterval {
				p.lastAlertTimes[alertKey] = now
				result = append(result, domainMonitor.AlertResult{
					AlertType:    alertType,
					EventID:      eventID,
					Severity:     "reminder",
					Duration:     "0s",
					CurrentValue: currentValue,
					Threshold:    0,
					IsRecovery:   false,
					StartTime:    now,
					TriggerTime:  now,
				})
				logger.Warnf("%s 告警触发（无阈值配置，EventID: %s）", alertType, eventID)
			}
			continue
		}

		// 第一步：检查本机是否恢复（当前值是否低于阈值）
		// HTTPErr和RedisLow/RedisHigh特殊处理
		var isLocalRecovered bool
		if alertType == entity.HTTPErr {
			// HTTPErr：检查状态码是否在allowed_codes中
			if metrics != nil && len(metrics.HTTP.Interfaces) > 0 {
				for _, httpInterface := range metrics.HTTP.Interfaces {
					if httpInterface.Name == pid {
						isLocalRecovered = CheckHTTPRecovery(currentValue, httpInterface.AllowedCodes)
						break
					}
				}
			} else {
				isLocalRecovered = currentValue >= 200.0
			}
		} else if alertType == entity.RedisLow || alertType == entity.RedisHigh {
			// RedisLow/RedisHigh：检查是否在正常区间[min, max]内
			if cfg.AppMonitoring != nil && cfg.AppMonitoring.Redis != nil && cfg.AppMonitoring.Redis.Thresholds.Clients != nil {
				min := float64(cfg.AppMonitoring.Redis.Thresholds.Clients.Min)
				max := float64(cfg.AppMonitoring.Redis.Thresholds.Clients.Max)
				// 在[min, max]区间内即为恢复
				isLocalRecovered = (currentValue >= min && currentValue <= max)
			} else {
				isLocalRecovered = true // 没有配置，认为已恢复
			}
		} else {
			isLocalRecovered = CheckRecovery(alertType, currentValue, thresholdConfig.Base)
		}

		if isLocalRecovered {
			// 本机已恢复，检查Redis中是否有告警状态
			if p.alertStateStorage == nil {
				// 继续处理告警逻辑
			} else {
				// 查找Redis中的告警状态
				var state *monitoring.AlertState
				var err error

				// 优先通过进程名查找（多进程场景：相同进程名的告警已合并）
				if processName != "" && (alertType == entity.CPUHigh || alertType == entity.MemHigh) {
					state, err = p.alertStateStorage.GetAlertStateByProcessName(alertType, processName)
					if err != nil {
						logger.Warnf("[恢复] %s 通过进程名查找告警状态失败: %v", alertType, err)
					}
				}

				// 如果通过进程名找不到，尝试通过告警类型查找
				if state == nil {
					state, err = p.alertStateStorage.GetAlertStateByType(alertType)
					if err != nil {
						logger.Warnf("[恢复] %s 通过告警类型查找告警状态失败: %v", alertType, err)
					}
				}

				if state != nil {
					// 第二步：检查Redis中记录的进程是否都已恢复（仅对CPU/内存告警）
					allProcessesRecovered := true
					if alertType == entity.CPUHigh || alertType == entity.MemHigh {
						for _, proc := range state.Processes {
							isProcessRecovered, processExists, processValue := p.recoveryChecker.CheckProcessRecovery(alertType, proc.PID, thresholdConfig.Base)
							if !isProcessRecovered {
								logger.Debugf("[恢复] %s 本机已恢复，但进程PID %s (%s) 仍高负载(%.2f >= %.2f)，未完全恢复",
									alertType, proc.PID, proc.ProcessName, processValue, thresholdConfig.Base)
								allProcessesRecovered = false
								break
							} else if processExists {
								logger.Debugf("[恢复] %s 进程PID %s (%s) 已恢复，当前值: %.2f", alertType, proc.PID, proc.ProcessName, processValue)
							} else {
								logger.Debugf("[恢复] %s 进程PID %s (%s) 已不存在，视为恢复", alertType, proc.PID, proc.ProcessName)
							}
						}
					}

					if allProcessesRecovered {
						// 检查是否已经发送过告警通知（有告警才有恢复）
						alertKey := state.EventID
						hasSentAlert := state.AlertSent // 从Redis读取持久化标记
						if !hasSentAlert {
							// 如果Redis中没有标记，检查内存中的记录（兼容旧数据）
							_, hasSentAlert = p.lastAlertTimes[alertKey]
						}
						if !hasSentAlert {
							// 还没有发送过告警通知，只是短暂超阈值后恢复正常，不算恢复
							logger.Debugf("[恢复] %s Redis中有告警状态，但未发送过告警通知，不处理恢复（事件ID: %s）",
								alertType, state.EventID)
							// 清除Redis中的告警状态（因为还没发送过告警，不需要恢复通知）
							if err := p.alertStateStorage.DeleteAlertStateByEventID(state.EventID); err != nil {
								logger.Warnf("[恢复] %s 删除Redis状态失败: %v", alertType, err)
							}
							continue // 跳过恢复处理，继续下一个决策
						}

						// 所有进程都已恢复，且已经发送过告警通知，开始恢复流程
						logger.Infof("[恢复] %s 开始恢复流程，事件ID: %s", alertType, state.EventID)

						// 获取恢复通知需要的持续时间
						var recoveryDurationRequired time.Duration
						if cfg.Alerting != nil && cfg.Alerting.Trigger.RecoveryDurationRequired > 0 {
							recoveryDurationRequired = cfg.Alerting.Trigger.RecoveryDurationRequired
						} else {
							recoveryDurationRequired = 5 * time.Minute // 默认5分钟
						}

						// 计算恢复正常后的持续时间（需要记录恢复开始时间）
						recoveryStartKey := string(alertType) + "_recovery_start_" + state.EventID
						recoveryStartTime, hasRecoveryStart := p.recoveryStartTimes[recoveryStartKey]
						if !hasRecoveryStart {
							// 首次检测到恢复，记录恢复开始时间
							if p.recoveryStartTimes == nil {
								p.recoveryStartTimes = make(map[string]time.Time)
							}
							p.recoveryStartTimes[recoveryStartKey] = now
							recoveryStartTime = now
							logger.Infof("[恢复] %s 首次检测到恢复，事件ID: %s，告警等级: %s",
								alertType, state.EventID, state.Severity)
						}

						recoveryDuration := now.Sub(recoveryStartTime)
						faultDuration := recoveryStartTime.Sub(state.StartTime)

						// 检查是否达到恢复通知的持续时间要求
						if recoveryDuration < recoveryDurationRequired {
							logger.Debugf("[恢复] %s 恢复正常持续 %v < 需要 %v，继续等待",
								alertType, recoveryDuration, recoveryDurationRequired)
						} else {
							// 检查恢复通知的防抖（使用EventID确保一致性）
							lastRecoveryKey := state.EventID + "_recovery"
							lastRecovery, hasLastRecovery := p.lastAlertTimes[lastRecoveryKey]
							recoveryDebounceInterval := alertInterval / 2
							if recoveryDebounceInterval < time.Minute {
								recoveryDebounceInterval = time.Minute
							}

							if hasLastRecovery && now.Sub(lastRecovery) < recoveryDebounceInterval {
								logger.Debugf("[恢复] %s 仍在防抖期内，跳过发送 (EventID: %s)", alertType, state.EventID)
							} else {
								// 发送恢复通知
								durationStr := FormatDuration(faultDuration)

								// 保存需要的信息（在删除前）
								eventID := state.EventID
								severity := state.Severity
								threshold := state.Threshold
								startTime := state.StartTime

								// 获取主要进程信息（第一个进程或最新的进程）
								var mainPID, mainProcessName string
								if len(state.Processes) > 0 {
									// 使用最后一个进程作为主要进程信息
									mainProcess := state.Processes[len(state.Processes)-1]
									mainPID = mainProcess.PID
									mainProcessName = mainProcess.ProcessName
								}

								// 如果是磁盘恢复通知，从Redis读取已收集的磁盘占用信息（在删除前）
								var diskUsageItems []domainMonitor.DiskUsageItem
								if alertType == entity.DiskHigh {
									if state.DiskUsage != nil && len(state.DiskUsage) > 0 {
										for _, item := range state.DiskUsage {
											diskUsageItems = append(diskUsageItems, domainMonitor.DiskUsageItem{
												Path:         item.Path,
												Size:         item.Size,
												SizeBytes:    item.SizeBytes,
												Type:         item.Type,
												Depth:        item.Depth,
												IsLeafResult: item.IsLeafResult,
											})
										}
										logger.Debugf("[恢复通知] 使用磁盘占用信息，共 %d 项", len(diskUsageItems))
									}
								}

								// 创建恢复通知结果
								result = append(result, domainMonitor.AlertResult{
									AlertType:    alertType,
									EventID:      eventID,
									Severity:     severity,
									Duration:     durationStr,
									CurrentValue: currentValue,
									Threshold:    threshold,
									IsRecovery:   true,
									StartTime:    startTime,
									TriggerTime:  now,
									PID:          mainPID,         // 主要进程的PID
									ProcessName:  mainProcessName, // 主要进程的名称
									DiskUsage:    diskUsageItems,  // 从Redis读取的磁盘占用信息
								})
								logger.Infof("[恢复] %s 恢复通知已触发，事件ID: %s，故障持续: %v，恢复正常持续: %v",
									alertType, eventID, faultDuration, recoveryDuration)

								// 删除告警状态（在获取完所有信息后）
								if err := p.alertStateStorage.DeleteAlertStateByEventID(eventID); err != nil {
									logger.Warnf("[恢复] %s 删除Redis状态失败: %v", alertType, err)
								}

								// 清除恢复开始时间记录
								delete(p.recoveryStartTimes, recoveryStartKey)
								p.lastAlertTimes[lastRecoveryKey] = now
							}
						}
						continue // 恢复处理完成，继续下一个决策
					}
				}
			}
		}

		// 如果当前值不满足恢复条件，清除恢复开始时间记录（如果存在）
		// 使用EventID而不是PID作为key的一部分
		if p.alertStateStorage != nil {
			// 尝试获取当前告警状态以获取EventID
			var currentState *monitoring.AlertState
			if processName != "" && (alertType == entity.CPUHigh || alertType == entity.MemHigh) {
				currentState, _ = p.alertStateStorage.GetAlertStateByProcessName(alertType, processName)
			}
			if currentState == nil {
				currentState, _ = p.alertStateStorage.GetAlertStateByType(alertType)
			}

			if currentState != nil {
				recoveryStartKey := string(alertType) + "_recovery_start_" + currentState.EventID
				if p.recoveryStartTimes != nil {
					if _, hasRecoveryStart := p.recoveryStartTimes[recoveryStartKey]; hasRecoveryStart {
						// 值回升，清除恢复开始时间记录
						delete(p.recoveryStartTimes, recoveryStartKey)
						recoveryCondition := ">="
						if alertType == entity.RedisLow {
							recoveryCondition = "<"
						}
						logger.Debugf("[恢复] %s 值回升（当前值: %.2f %s base: %.2f），清除恢复开始时间记录",
							alertType, currentValue, recoveryCondition, thresholdConfig.Base)
					}
				}
			}
		}

		// 确定告警等级
		// RedisLow/RedisHigh特殊处理：使用区间判断，不在[min, max]区间即为异常
		var severity string
		if alertType == entity.RedisLow || alertType == entity.RedisHigh {
			// Redis告警：检查是否在正常区间内
			if cfg.AppMonitoring != nil && cfg.AppMonitoring.Redis != nil && cfg.AppMonitoring.Redis.Thresholds.Clients != nil {
				min := float64(cfg.AppMonitoring.Redis.Thresholds.Clients.Min)
				max := float64(cfg.AppMonitoring.Redis.Thresholds.Clients.Max)

				if alertType == entity.RedisLow {
					// RedisLow：当前值 < min 时告警
					if currentValue < min {
						severity = "p2" // Redis告警统一使用p2级别
					} else {
						continue // 在正常区间内，不告警
					}
				} else if alertType == entity.RedisHigh {
					// RedisHigh：当前值 > max 时告警
					if currentValue > max {
						severity = "p2" // Redis告警统一使用p2级别
					} else {
						continue // 在正常区间内，不告警
					}
				}
			} else {
				continue // 没有配置，不告警
			}
		} else if alertType == entity.HTTPErr {
			// HTTPErr：检查状态码是否在allowed_codes中
			// 如果不在allowed_codes中，且need_alert为true，则触发p2级别告警
			shouldAlert := false
			if metrics != nil && len(metrics.HTTP.Interfaces) > 0 {
				// 根据PID（接口名称）查找对应的HTTP接口配置
				for _, httpInterface := range metrics.HTTP.Interfaces {
					if httpInterface.Name == pid {
						if httpInterface.NeedAlert {
							// 检查状态码是否在allowed_codes中
							isValidCode := utils.IsValidHTTPStatusCode(int(currentValue), httpInterface.AllowedCodes)
							// 如果状态码不在allowed_codes中，且need_alert为true，则告警
							shouldAlert = !isValidCode
						}
						break
					}
				}
			} else {
				// 如果无法获取metrics，使用默认逻辑（状态码!=200且!=0为告警）
				shouldAlert = (currentValue != 0 && int(currentValue) != 200)
			}

			if shouldAlert {
				severity = "p2" // HTTP告警使用p2级别
			} else {
				continue // HTTP正常，不告警
			}
		} else {
			severity = DetermineSeverity(currentValue, thresholdConfig, alertType)
			if severity == "" {
				continue // 未达到任何阈值
			}
		}

		// 获取当前等级的阈值
		var currentThreshold float64
		if alertType == entity.HTTPErr {
			// HTTP告警使用p2级别，阈值使用当前状态码
			currentThreshold = currentValue
			// 确保severity是p2
			if severity != "p2" {
				severity = "p2"
			}
		} else if alertType == entity.RedisLow || alertType == entity.RedisHigh {
			// Redis告警：阈值使用min或max
			if cfg.AppMonitoring != nil && cfg.AppMonitoring.Redis != nil && cfg.AppMonitoring.Redis.Thresholds.Clients != nil {
				if alertType == entity.RedisLow {
					currentThreshold = float64(cfg.AppMonitoring.Redis.Thresholds.Clients.Min)
				} else {
					currentThreshold = float64(cfg.AppMonitoring.Redis.Thresholds.Clients.Max)
				}
			} else {
				currentThreshold = currentValue
			}
			// 确保severity是p2
			if severity != "p2" {
				severity = "p2"
			}
		} else {
			switch severity {
			case "p1":
				currentThreshold = thresholdConfig.P1
			case "p2":
				currentThreshold = thresholdConfig.P2
			case "p3":
				currentThreshold = thresholdConfig.P3
			case "reminder":
				currentThreshold = thresholdConfig.Base
			default:
				currentThreshold = thresholdConfig.Base
			}
		}

		// 获取统一持续时间配置和该等级的立即通知配置
		var durationRequired time.Duration
		var notifyImmediately bool
		if cfg.Alerting != nil && cfg.Alerting.Trigger.DurationRequired > 0 {
			durationRequired = cfg.Alerting.Trigger.DurationRequired
		} else {
			durationRequired = 5 * time.Minute // 默认5分钟
		}

		// 获取该等级的立即通知配置
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

		// 从Redis获取或创建告警状态
		var state *monitoring.AlertState
		severityChanged := false
		oldSeverity := ""
		if p.alertStateStorage != nil {
			// 对于CPU和内存告警，优先通过进程名查找现有告警状态（多进程场景合并）
			var existingState *monitoring.AlertState
			var err error
			if (alertType == entity.CPUHigh || alertType == entity.MemHigh) && processName != "" {
				logger.Debugf("[状态查找] %s 尝试通过进程名查找现有状态: %s", alertType, processName)
				existingState, err = p.alertStateStorage.GetAlertStateByProcessName(alertType, processName)
				if err != nil {
					logger.Warnf("通过进程名查找告警状态失败: %v", err)
				} else if existingState != nil {
					logger.Debugf("[状态查找] %s 通过进程名找到现有状态: EventID=%s, StartTime=%v",
						alertType, existingState.EventID, existingState.StartTime)
				} else {
					logger.Debugf("[状态查找] %s 通过进程名未找到现有状态", alertType)
				}
			}

			// 对于非CPU/内存告警，或者没有进程名的情况，尝试通过告警类型查找
			if existingState == nil && (alertType != entity.CPUHigh && alertType != entity.MemHigh) {
				logger.Debugf("[状态查找] %s 尝试通过告警类型查找现有状态", alertType)
				existingState, err = p.alertStateStorage.GetAlertStateByType(alertType)
				if err != nil {
					logger.Warnf("通过告警类型查找告警状态失败: %v", err)
				} else if existingState != nil {
					logger.Debugf("[状态查找] %s 通过告警类型找到现有状态: EventID=%s, StartTime=%v",
						alertType, existingState.EventID, existingState.StartTime)
				} else {
					logger.Debugf("[状态查找] %s 通过告警类型未找到现有状态", alertType)
				}
			}

			if existingState != nil {
				state = existingState

				// 检查等级是否发生变化（升级或降级）
				oldSeverity = state.Severity
				if oldSeverity != severity {
					severityChanged = true
					state.Severity = severity
					state.Threshold = currentThreshold // 更新为当前等级的阈值
					state.SeverityStartTime = now      // 记录等级变化时间
					logger.Debugf("%s 告警等级变化: %s -> %s (值: %.2f -> %.2f, 阈值: %.2f -> %.2f, 累计持续时间: %v)",
						alertType, oldSeverity, severity, state.CurrentValue, currentValue, state.Threshold, currentThreshold, now.Sub(state.StartTime))
				}

				// 获取进程的完整信息（CPU和内存使用率）
				var cpuPercent, memPercent float64
				if pidInt, err := strconv.Atoi(pid); err == nil {
					if foundProcess, err := utils.FindProcessByPID(p.hostCollector, alertType, int32(pidInt), 50); err == nil && foundProcess != nil {
						cpuPercent = foundProcess.CPUPercent
						memPercent = float64(foundProcess.MemPercent)
					} else {
						// 如果无法获取完整信息，根据告警类型设置对应字段
						switch alertType {
						case entity.CPUHigh:
							cpuPercent = currentValue
							memPercent = 0 // 未知
						case entity.MemHigh:
							cpuPercent = 0 // 未知
							memPercent = currentValue
						}
					}
				} else {
					// PID转换失败，根据告警类型设置对应字段
					switch alertType {
					case entity.CPUHigh:
						cpuPercent = currentValue
						memPercent = 0 // 未知
					case entity.MemHigh:
						cpuPercent = 0 // 未知
						memPercent = currentValue
					}
				}

				// 更新或添加当前进程信息
				processInfo := monitoring.ProcessInfo{
					PID:               pid,
					ProcessName:       processName,
					FirstDetectedTime: now, // 如果是新进程，记录首次检测时间
					LastSeenTime:      now,
					CPUPercent:        cpuPercent,
					MemPercent:        memPercent,
				}

				// 查找是否已存在该PID的进程
				found := false
				for i, proc := range state.Processes {
					if proc.PID == pid {
						// 更新现有进程信息，保留首次检测时间
						processInfo.FirstDetectedTime = proc.FirstDetectedTime
						state.Processes[i] = processInfo
						found = true
						logger.Debugf("%s 更新进程信息: PID %s (%s), 当前值: %.2f", alertType, pid, processName, currentValue)
						break
					}
				}

				// 如果不存在，添加新进程
				if !found {
					state.Processes = append(state.Processes, processInfo)
					logger.Debugf("%s 添加新进程: PID %s (%s), 当前值: %.2f, EventID: %s", alertType, pid, processName, currentValue, state.EventID)
				}

				// 清理长时间未更新的进程（超过10分钟未更新的进程）
				processCleanupAge := 10 * time.Minute
				if cfg.Alerting != nil && cfg.Alerting.Trigger.AlertInterval > 0 {
					// 使用告警间隔的3倍作为进程清理时间
					processCleanupAge = cfg.Alerting.Trigger.AlertInterval * 3
				}
				p.cleanupStaleProcesses(state, processCleanupAge)

				// 更新整体状态的当前值（使用最高值或最新值）
				state.CurrentValue = currentValue

			} else {
				// 创建新的告警状态
				// 首先检查是否已存在相同进程名的告警事件（用于事件ID复用）
				var eventID string
				var existingEventState *monitoring.AlertState

				// 对于CPU和内存告警，尝试复用相同进程名的事件ID
				if (alertType == entity.CPUHigh || alertType == entity.MemHigh) && processName != "" {
					existingEventState, err = p.alertStateStorage.GetAlertStateByProcessName(alertType, processName)
					if err != nil {
						logger.Warnf("查找现有事件失败: %v", err)
					}
				}

				if existingEventState != nil {
					// 复用现有事件ID
					eventID = existingEventState.EventID
					logger.Debugf("%s 复用现有事件ID: %s (进程名: %s)", alertType, eventID, processName)
				} else {
					// 生成新的事件ID（带冲突检测）
					var err error
					eventID, err = p.eventIDGenerator.GenerateUniqueEventID(string(alertType), 3)
					if err != nil {
						// 如果生成唯一ID失败，回退到普通生成方法
						logger.Warnf("生成唯一事件ID失败: %v，回退到普通生成方法", err)
						eventID = p.eventIDGenerator.GenerateEventID(string(alertType))
					}
					logger.Debugf("%s 生成新事件ID: %s (进程名: %s)", alertType, eventID, processName)
				}

				// 获取进程的完整信息（CPU和内存使用率）
				var cpuPercent, memPercent float64
				if pidInt, err := strconv.Atoi(pid); err == nil {
					if foundProcess, err := utils.FindProcessByPID(p.hostCollector, alertType, int32(pidInt), 50); err == nil && foundProcess != nil {
						cpuPercent = foundProcess.CPUPercent
						memPercent = float64(foundProcess.MemPercent)
					} else {
						// 如果无法获取完整信息，根据告警类型设置对应字段
						switch alertType {
						case entity.CPUHigh:
							cpuPercent = currentValue
							memPercent = 0 // 未知
						case entity.MemHigh:
							cpuPercent = 0 // 未知
							memPercent = currentValue
						}
					}
				} else {
					// PID转换失败，根据告警类型设置对应字段
					switch alertType {
					case entity.CPUHigh:
						cpuPercent = currentValue
						memPercent = 0 // 未知
					case entity.MemHigh:
						cpuPercent = 0 // 未知
						memPercent = currentValue
					}
				}

				// 创建初始进程信息
				processInfo := monitoring.ProcessInfo{
					PID:               pid,
					ProcessName:       processName,
					FirstDetectedTime: now,
					LastSeenTime:      now,
					CPUPercent:        cpuPercent,
					MemPercent:        memPercent,
				}

				state = &monitoring.AlertState{
					EventID:             eventID,
					AlertType:           alertType,
					StartTime:           now,
					Severity:            severity,
					SeverityStartTime:   now,
					Threshold:           currentThreshold,
					CurrentValue:        currentValue,
					DiskUsage:           nil,                                   // 初始为空，异步收集
					DiskUsageUpdateTime: time.Time{},                           // 初始为空时间
					AlertSent:           false,                                 // 初始未发送告警
					Processes:           []monitoring.ProcessInfo{processInfo}, // 初始包含一个进程
				}

				// 使用新的基于事件ID的存储方法
				if !peekMode {
					logger.Debugf("[状态保存] %s 保存新状态到Redis: EventID=%s, StartTime=%v, peekMode=%v",
						alertType, eventID, now, peekMode)
					if err := p.alertStateStorage.SetAlertStateByEventID(state); err != nil {
						logger.Warnf("保存告警状态失败: %v", err)
						continue
					}
					logger.Debugf("[状态保存] %s 状态保存成功: EventID=%s", alertType, eventID)
				} else {
					logger.Debugf("[状态保存] %s 预览模式，跳过状态保存: EventID=%s", alertType, eventID)
				}
				logger.Debugf("%s 首次触发阈值（等级: %s, 值: %.2f, EventID: %s）", alertType, severity, currentValue, eventID)

				// 如果是磁盘告警，异步启动磁盘占用信息收集
				if alertType == entity.DiskHigh && p.diskEnricher != nil {
					p.diskEnricher.EnrichAsync(alertType, "default", eventID)
				}
			}

			// 已有告警状态时，检查磁盘占用信息是否需要更新（避免重复扫描）
			if state != nil && alertType == entity.DiskHigh && p.diskEnricher != nil {
				// 只有在以下情况才重新扫描：
				// 1. 没有磁盘占用信息
				// 2. 磁盘占用信息超过5分钟未更新
				shouldRescan := false
				if state.DiskUsage == nil || len(state.DiskUsage) == 0 {
					shouldRescan = true
					logger.Debugf("[磁盘扫描] 无磁盘占用信息，需要扫描")
				} else if !state.DiskUsageUpdateTime.IsZero() {
					timeSinceUpdate := now.Sub(state.DiskUsageUpdateTime)
					if timeSinceUpdate > 5*time.Minute {
						shouldRescan = true
						logger.Debugf("[磁盘扫描] 磁盘占用信息已过期（%v），需要重新扫描", timeSinceUpdate)
					} else {
						logger.Debugf("[磁盘扫描] 磁盘占用信息仍然有效（%v前更新），跳过扫描", timeSinceUpdate)
					}
				}

				if shouldRescan {
					p.diskEnricher.EnrichAsync(alertType, "default", state.EventID)
				}
			}
		} else {
			// 如果没有alertStateStorage，HTTPErr不应该触发告警（需要Redis存储状态）
			if alertType == entity.HTTPErr {
				logger.Warnf("HTTPErr告警需要Redis存储，但alertStateStorage为nil，跳过告警")
				continue
			}
		}

		// 计算持续时间（使用累计持续时间：从首次超阈值开始计算）
		// HTTPErr必须创建state才能继续，否则跳过
		if state == nil {
			if alertType == entity.HTTPErr {
				logger.Warnf("HTTPErr告警未创建state，跳过告警")
				continue
			}
			// 其他类型如果没有state，也跳过（不应该发生）
			continue
		}

		if state != nil {
			// 统一使用StartTime计算累计持续时间（无论等级如何变化）
			duration := now.Sub(state.StartTime)
			logger.Debugf("[告警评估] %s 当前等级: %s, 当前值: %.2f, 累计持续时间: %v, 需要持续时间: %v, 等级变化: %v",
				alertType, severity, currentValue, duration, durationRequired, severityChanged)

			// 检查是否达到持续时间要求（所有告警都必须满足持续时间要求，即使等级升级）
			if duration < durationRequired {
				logger.Debugf("[告警评估] %s 未达到持续时间要求（累计持续: %v < 需要: %v），继续等待",
					alertType, duration, durationRequired)
				// 更新当前值和等级，但未达到持续时间要求
				state.CurrentValue = currentValue
				if p.alertStateStorage != nil {
					if err := p.alertStateStorage.UpdateAlertStateByEventID(state); err != nil {
						logger.Warnf("[告警评估] %s 更新告警状态失败: %v", alertType, err)
					} else {
						logger.Debugf("[告警评估] %s 已更新告警状态（等级: %s, 当前值: %.2f, 累计持续: %v）",
							alertType, severity, currentValue, duration)
					}
				}
			} else {
				logger.Debugf("[告警评估] %s 已达到持续时间要求（累计持续: %v >= 需要: %v），检查防抖",
					alertType, duration, durationRequired)

				// 检查防抖（notifyImmediately只影响防抖，不影响持续时间检查）
				// 使用EventID作为key，确保每个告警事件都有独立的防抖记录
				alertKey := state.EventID
				lastAlert, hasLastAlert := p.lastAlertTimes[alertKey]
				shouldAlert := false

				if notifyImmediately {
					// 需要立即通知，但仍然检查是否已经发送过告警（基于EventID）
					// 如果是同一个EventID已经发送过告警，不再重复发送
					if !hasLastAlert {
						shouldAlert = true
						logger.Debugf("[告警评估] %s 需要立即通知，且未发送过告警，可以发送 (EventID: %s)", alertType, state.EventID)
					} else {
						logger.Debugf("[告警评估] %s 需要立即通知，但已发送过告警，跳过重复发送 (EventID: %s)", alertType, state.EventID)
						shouldAlert = false
					}
				} else {
					// 其他级别需要等待防抖窗口
					if !hasLastAlert {
						shouldAlert = true
						logger.Debugf("[告警评估] %s 无上次告警记录，可以发送 (EventID: %s)", alertType, state.EventID)
					} else {
						timeSinceLastAlert := now.Sub(lastAlert)
						if timeSinceLastAlert >= alertInterval {
							shouldAlert = true
							logger.Debugf("[告警评估] %s 防抖检查通过（已过: %v >= 防抖间隔: %v, EventID: %s）",
								alertType, timeSinceLastAlert, alertInterval, state.EventID)
						} else {
							logger.Debugf("[告警评估] %s 仍在防抖期内（已过: %v < 防抖间隔: %v, EventID: %s），跳过发送",
								alertType, timeSinceLastAlert, alertInterval, state.EventID)
						}
					}
				}

				if shouldAlert {
					// 更新告警状态（更新当前值，等级和SeverityStartTime已在上面更新）
					state.CurrentValue = currentValue
					if p.alertStateStorage != nil {
						if err := p.alertStateStorage.UpdateAlertStateByEventID(state); err != nil {
							logger.Warnf("[告警评估] %s 更新告警状态失败: %v", alertType, err)
						}
					}

					// 使用EventID作为key记录告警发送时间（内存记录，用于防抖）
					p.lastAlertTimes[alertKey] = now
					// 在Redis中标记已发送告警（持久化，用于恢复检查）
					state.AlertSent = true
					if p.alertStateStorage != nil {
						if err := p.alertStateStorage.UpdateAlertStateByEventID(state); err != nil {
							logger.Warnf("[告警评估] %s 更新告警发送标记失败: %v", alertType, err)
						}
					}
					durationStr := FormatDuration(duration)

					// 如果是磁盘告警，从Redis读取最新的缓存磁盘占用信息（不阻塞，直接使用缓存）
					var diskUsageItems []domainMonitor.DiskUsageItem
					if alertType == entity.DiskHigh {
						if state.DiskUsage != nil && len(state.DiskUsage) > 0 {
							for _, item := range state.DiskUsage {
								diskUsageItems = append(diskUsageItems, domainMonitor.DiskUsageItem{
									Path:         item.Path,
									Size:         item.Size,
									SizeBytes:    item.SizeBytes,
									Type:         item.Type,
									Depth:        item.Depth,
									IsLeafResult: item.IsLeafResult,
								})
							}

							// 记录缓存数据的新旧程度
							if !state.DiskUsageUpdateTime.IsZero() {
								timeSinceUpdate := now.Sub(state.DiskUsageUpdateTime)
								logger.Debugf("[磁盘告警] 使用缓存的磁盘占用信息（%v前更新），共 %d 项", timeSinceUpdate, len(diskUsageItems))
							} else {
								logger.Debugf("[磁盘告警] 使用缓存的磁盘占用信息（无时间戳），共 %d 项", len(diskUsageItems))
							}
						} else {
							logger.Debugf("[磁盘告警] 缓存中无磁盘占用信息，告警将不包含磁盘占用信息")
						}
					}

					// 获取主要进程信息用于告警通知
					var mainPID, mainProcessName string
					if len(state.Processes) > 0 {
						// 使用最新的进程作为主要进程信息
						mainProcess := state.Processes[len(state.Processes)-1]
						mainPID = mainProcess.PID
						mainProcessName = mainProcess.ProcessName
					} else {
						// 兜底使用当前进程信息
						mainPID = pid
						mainProcessName = processName
					}

					result = append(result, domainMonitor.AlertResult{
						AlertType:    alertType,
						EventID:      state.EventID,
						Severity:     severity,
						Duration:     durationStr,
						CurrentValue: currentValue,
						Threshold:    currentThreshold, // 使用当前等级的阈值
						IsRecovery:   false,
						StartTime:    state.StartTime,
						TriggerTime:  now,
						PID:          mainPID,         // 主要进程的PID
						ProcessName:  mainProcessName, // 主要进程的名称
						DiskUsage:    diskUsageItems,  // 从Redis读取的磁盘占用信息（可能为空）
					})
					logger.Warnf("[告警触发] %s 告警已触发（等级: %s, 累计持续时间: %v, 值: %.2f, 阈值: %.2f, 事件ID: %s）",
						alertType, severity, duration, currentValue, currentThreshold, state.EventID)
				} else {
					// 达到持续时间但防抖未通过，仍需要更新状态
					state.CurrentValue = currentValue
					if p.alertStateStorage != nil {
						if err := p.alertStateStorage.UpdateAlertStateByEventID(state); err != nil {
							logger.Warnf("[告警评估] %s 更新告警状态失败: %v", alertType, err)
						}
					}
				}
			}
		}
	}

	return result
}
