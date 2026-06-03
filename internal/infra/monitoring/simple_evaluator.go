package monitoring

import (
	"github.com/youxihu/GWatch-new/internal/domain/collector"
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	domainMonitor "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	"github.com/youxihu/GWatch-new/internal/utils"
)

// 仅做阈值比较，不做防抖与连续计数
type SimpleEvaluator struct {
	hostCollector collector.HostCollector
}

func NewSimpleEvaluator() *SimpleEvaluator {
	return &SimpleEvaluator{}
}

// SetHostCollector 设置主机收集器（用于获取进程PID）
func (s *SimpleEvaluator) SetHostCollector(hostCollector collector.HostCollector) {
	s.hostCollector = hostCollector
}

func (s *SimpleEvaluator) Evaluate(cfg *shared.Config, metrics *entity.SystemMetrics) ([]domainMonitor.Decision, error) {
	var decisions []domainMonitor.Decision

	// 主机类监控评估：只有当host_monitoring配置存在且启用时才评估
	if cfg != nil && cfg.HostMonitoring != nil && cfg.HostMonitoring.Enabled {
		// CPU监控
		if metrics.CPU.Error != nil {
			decisions = append(decisions, domainMonitor.Decision{Type: entity.CPUErr, CurrentValue: 0, PID: "default", ProcessName: ""})
		} else if cfg.HostMonitoring.Thresholds.CPU != nil && cfg.HostMonitoring.Thresholds.CPU.Enabled {
			// 无论值是否超过阈值，都创建Decision，以便policy层能处理恢复逻辑
			pid, processName := s.getTopProcessInfo(entity.CPUHigh)
			decisions = append(decisions, domainMonitor.Decision{Type: entity.CPUHigh, CurrentValue: metrics.CPU.Percent, PID: pid, ProcessName: processName})
		}

		// 内存监控
		if metrics.Memory.Error != nil {
			decisions = append(decisions, domainMonitor.Decision{Type: entity.MemErr, CurrentValue: 0, PID: "default", ProcessName: ""})
		} else if cfg.HostMonitoring.Thresholds.Memory != nil && cfg.HostMonitoring.Thresholds.Memory.Enabled {
			// 无论值是否超过阈值，都创建Decision，以便policy层能处理恢复逻辑
			pid, processName := s.getTopProcessInfo(entity.MemHigh)
			decisions = append(decisions, domainMonitor.Decision{Type: entity.MemHigh, CurrentValue: metrics.Memory.Percent, PID: pid, ProcessName: processName})
		}

		// 磁盘监控
		if metrics.Disk.Error != nil {
			decisions = append(decisions, domainMonitor.Decision{Type: entity.DiskErr, CurrentValue: 0, PID: "default", ProcessName: ""})
		} else if cfg.HostMonitoring.Thresholds.Disk != nil && cfg.HostMonitoring.Thresholds.Disk.Enabled {
			// 无论值是否超过阈值，都创建Decision，以便policy层能处理恢复逻辑
			decisions = append(decisions, domainMonitor.Decision{Type: entity.DiskHigh, CurrentValue: metrics.Disk.Percent, PID: "default", ProcessName: ""})
		}

		if metrics.Network.Error != nil {
			decisions = append(decisions, domainMonitor.Decision{Type: entity.NetworkErr, CurrentValue: 0, PID: "default", ProcessName: ""})
		}
	}

	// 应用层类监控评估：只有当app_monitoring配置存在且启用时才评估
	if cfg != nil && cfg.AppMonitoring != nil && cfg.AppMonitoring.Enabled {
		// Redis监控评估：只有当Redis配置存在且启用时才评估
		if cfg.AppMonitoring.Redis != nil && cfg.AppMonitoring.Redis.Enabled {
			if metrics.Redis.ConnectionError != nil {
				decisions = append(decisions, domainMonitor.Decision{Type: entity.RedisErr, CurrentValue: 0, PID: "default", ProcessName: ""})
			} else if cfg.AppMonitoring.Redis.Thresholds.Clients != nil && cfg.AppMonitoring.Redis.Thresholds.Clients.Enabled {
				// 无论值是否超过阈值，都创建Decision，以便policy层能处理恢复逻辑
				clientCount := float64(metrics.Redis.ClientCount)
				if metrics.Redis.ClientCount < cfg.AppMonitoring.Redis.Thresholds.Clients.Min {
					decisions = append(decisions, domainMonitor.Decision{Type: entity.RedisLow, CurrentValue: clientCount, PID: "default", ProcessName: ""})
				} else if metrics.Redis.ClientCount > cfg.AppMonitoring.Redis.Thresholds.Clients.Max {
					decisions = append(decisions, domainMonitor.Decision{Type: entity.RedisHigh, CurrentValue: clientCount, PID: "default", ProcessName: ""})
				} else {
					// 值在正常范围内，创建两个Decision以检查两个告警类型的恢复
					decisions = append(decisions, domainMonitor.Decision{Type: entity.RedisLow, CurrentValue: clientCount, PID: "default", ProcessName: ""})
					decisions = append(decisions, domainMonitor.Decision{Type: entity.RedisHigh, CurrentValue: clientCount, PID: "default", ProcessName: ""})
				}
			}
		}

		// HTTP接口监控评估：只有当HTTP配置存在且启用时才评估
		// 无论HTTP接口是否正常，都要创建Decision，以便StatefulPolicy能够检测恢复
		if cfg.HTTPMonitoring != nil && cfg.HTTPMonitoring.Enabled {
			if metrics.HTTP.Error != nil {
				// HTTP采集错误，使用状态码0表示错误
				decisions = append(decisions, domainMonitor.Decision{Type: entity.HTTPErr, CurrentValue: 0, PID: "default", ProcessName: ""})
			} else {
				// 检查是否有需要告警的异常接口（基于配置的 need_alert 和 allowed_codes）
				for _, httpInterface := range metrics.HTTP.Interfaces {
					isValidCode := utils.IsValidHTTPStatusCode(httpInterface.StatusCode, httpInterface.AllowedCodes)

					if httpInterface.NeedAlert {
						statusCode := float64(httpInterface.StatusCode)
						if isValidCode {
							statusCode = 200.0
						}
						decisions = append(decisions, domainMonitor.Decision{
							Type:         entity.HTTPErr,
							CurrentValue: statusCode,
							PID:          httpInterface.Name,
							ProcessName:  "",
						})
					}
				}
			}
		}
	}

	return decisions, nil
}

// getTopProcessInfo 获取Top进程的PID和名称（用于CPU或内存告警）
func (s *SimpleEvaluator) getTopProcessInfo(alertType entity.AlertType) (string, string) {
	return utils.GetTopProcessInfo(s.hostCollector, alertType)
}
