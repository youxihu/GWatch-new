package monitoring

import (
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"github.com/youxihu/GWatch-new/internal/domain/monitoring"
	"github.com/youxihu/GWatch-new/internal/utils"
	"fmt"
	"strings"
	"time"
)

type MarkdownFormatter struct{}

func NewMarkdownFormatter() monitoring.Formatter { return &MarkdownFormatter{} }


func (f *MarkdownFormatter) Build(title string, cfg *shared.Config, m *entity.SystemMetrics, alerts []monitoring.TriggeredAlert) string {
	var text strings.Builder

	// 获取主机信息
	ip, hostname, err := utils.GetHostInfo()
	if err != nil {
		ip = "unknown-ip"
		hostname = "unknown-host"
	}

	// 获取账号名称（从配置的AlertTitle获取，如果没有则使用hostname）
	accountName := hostname
	if cfg != nil && cfg.HostMonitoring != nil && cfg.HostMonitoring.AlertTitle != "" {
		accountName = cfg.HostMonitoring.AlertTitle
	}

	// 分离告警和恢复
	var alertItems []monitoring.TriggeredAlert
	var recoveryItems []monitoring.TriggeredAlert
	for _, a := range alerts {
		if a.IsRecovery {
			recoveryItems = append(recoveryItems, a)
		} else {
			alertItems = append(alertItems, a)
		}
	}

	// 处理告警项（按照模板格式，使用Markdown语法）
	for _, a := range alertItems {
		// ## 大标题：告警类型
		text.WriteString(fmt.Sprintf("## [告警] %s\n\n", a.Type.String()))

		// 事件ID
		if a.EventID != "" {
			text.WriteString(fmt.Sprintf("- 事件ID: %s\n", a.EventID))
		}

		// 告警等级
		severityText := GetSeverityText(a.Severity)
		text.WriteString(fmt.Sprintf("- 告警等级: %s\n", severityText))

		// 所属账号
		text.WriteString(fmt.Sprintf("- 所属账号: %s (%s)\n", ip, accountName))

		// 触发条件
		triggerCondition := FormatTriggerCondition(a.Type, a.Severity, a.Threshold, a.Duration, false, cfg)
		text.WriteString(fmt.Sprintf("- 触发条件: %s\n", triggerCondition))

		// 触发对象（从ProcessInfo中获取，或磁盘占用信息）
		if a.Type == entity.DiskHigh && len(a.DiskUsage) > 0 {
			// 磁盘告警：显示占用最大的目录/文件，使用增强的递归扫描结果展示
			text.WriteString("- 触发对象:\n")
			text.WriteString(formatDiskUsageItems(a.DiskUsage, len(a.DiskUsage)))
		} else {
			triggerObject := "-"
			if a.Process != nil {
				triggerObject = FormatTriggerObject(a.Type, a.Process.Name, a.Process.PID, float64(a.Process.CPUPercent), float64(a.Process.MemPercent), a.Process.MemRSS)
			}
			text.WriteString(fmt.Sprintf("- 触发对象: %s\n", triggerObject))
		}

		// 触发时间
		if !a.StartTime.IsZero() {
			text.WriteString(fmt.Sprintf("- 触发时间: %s\n", a.StartTime.Format("2006-01-02 15:04:05")))
		} else {
			text.WriteString(fmt.Sprintf("- 触发时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
		}

		// 持续时间
		if a.Duration != "" && a.Duration != "0s" {
			text.WriteString(fmt.Sprintf("- 持续时间: %s\n", a.Duration))
		} else {
			text.WriteString("- 持续时间: 0秒\n")
		}

		text.WriteString("\n")
	}

	// 处理恢复项（按照模板格式，使用Markdown语法）
	for _, a := range recoveryItems {
		// ## 大标题：故障恢复
		text.WriteString("## [故障恢复]\n\n")

		// 事件ID
		if a.EventID != "" {
			text.WriteString(fmt.Sprintf("- 事件ID: %s\n", a.EventID))
		}

		// 告警等级
		text.WriteString("- 告警等级: 恢复正常\n")

		// 所属账号
		text.WriteString(fmt.Sprintf("- 所属账号: %s (%s)\n", ip, accountName))

		// 触发条件
		triggerCondition := FormatTriggerCondition(a.Type, a.Severity, a.Threshold, "", true, cfg)
		text.WriteString(fmt.Sprintf("- 触发条件: %s\n", triggerCondition))

		// 触发对象（恢复时也尝试显示进程信息或磁盘占用信息）
		if a.Type == entity.DiskHigh && len(a.DiskUsage) > 0 {
			// 磁盘恢复通知：显示原始告警时的磁盘占用信息，使用增强的递归扫描结果展示
			text.WriteString("- 触发对象:\n")
			text.WriteString(formatDiskUsageItems(a.DiskUsage, len(a.DiskUsage)))
		} else {
			triggerObject := "-"
			if a.Process != nil {
				triggerObject = FormatTriggerObject(a.Type, a.Process.Name, a.Process.PID, float64(a.Process.CPUPercent), float64(a.Process.MemPercent), a.Process.MemRSS)
			}
			text.WriteString(fmt.Sprintf("- 触发对象: %s\n", triggerObject))
		}

		// 触发时间（使用StartTime，即告警开始时间）
		if !a.StartTime.IsZero() {
			text.WriteString(fmt.Sprintf("- 触发时间: %s\n", a.StartTime.Format("2006-01-02 15:04:05")))
		} else {
			text.WriteString(fmt.Sprintf("- 触发时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
		}

		// 持续时间（故障持续时间）
		if a.Duration != "" {
			text.WriteString(fmt.Sprintf("- 持续时间: %s\n", a.Duration))
		} else {
			text.WriteString("- 持续时间: 0秒\n")
		}

		text.WriteString("\n")
	}

	// ### 小标题：当前时段资源采集值
	text.WriteString("### [当前时段资源采集值]\n\n")

	// 主机类监控指标 - 只有当host_monitoring配置存在且启用时才显示
	if cfg != nil && cfg.HostMonitoring != nil && cfg.HostMonitoring.Enabled {
		// CPU
		if m.CPU.Error != nil {
			text.WriteString(fmt.Sprintf("- CPU: 监控失败 - %v\n", m.CPU.Error))
		} else {
			thresholdConfig := GetThresholdConfig(cfg, entity.CPUHigh)
			var cpuThreshold float64
			if thresholdConfig.HasConfig {
				cpuThreshold = thresholdConfig.Base
			}
			status := statusText(m.CPU.Percent, cpuThreshold)
			if m.CPU.Cores > 0 {
				text.WriteString(fmt.Sprintf("- CPU: %.2f%% (%d核心) %s\n", m.CPU.Percent, m.CPU.Cores, status))
			} else {
				text.WriteString(fmt.Sprintf("- CPU: %.2f%% %s\n", m.CPU.Percent, status))
			}
		}

		// 内存
		if m.Memory.Error != nil {
			text.WriteString(fmt.Sprintf("- 内存: 监控失败 - %v\n", m.Memory.Error))
		} else {
			thresholdConfig := GetThresholdConfig(cfg, entity.MemHigh)
			var memoryThreshold float64
			if thresholdConfig.HasConfig {
				memoryThreshold = thresholdConfig.Base
			}
			status := statusText(m.Memory.Percent, memoryThreshold)
			text.WriteString(fmt.Sprintf("- 内存: %.2f%% (%d/%d MB) %s\n", m.Memory.Percent, m.Memory.UsedMB, m.Memory.TotalMB, status))
		}

		// 磁盘
		if m.Disk.Error != nil {
			text.WriteString(fmt.Sprintf("- 磁盘: 监控失败 - %v\n", m.Disk.Error))
		} else {
			thresholdConfig := GetThresholdConfig(cfg, entity.DiskHigh)
			var diskThreshold float64
			if thresholdConfig.HasConfig {
				diskThreshold = thresholdConfig.Base
			}
			status := statusText(m.Disk.Percent, diskThreshold)
			text.WriteString(fmt.Sprintf("- 磁盘: %.2f%% (%d/%d GB) %s\n", m.Disk.Percent, m.Disk.UsedGB, m.Disk.TotalGB, status))
		}

		// 网络IO
		if m.Network.Error != nil {
			text.WriteString(fmt.Sprintf("- 网络IO: 监控失败 - %v\n", m.Network.Error))
		} else {
			text.WriteString(fmt.Sprintf("- 网络IO: %s\n", utils.FormatIOSpeedPair(m.Network.DownloadKBps, m.Network.UploadKBps, "下载", "上传")))
		}

		// 磁盘IO
		text.WriteString(fmt.Sprintf("- 磁盘IO: %s\n", utils.FormatIOSpeedPair(m.Disk.ReadKBps, m.Disk.WriteKBps, "读", "写")))
	}

	// Redis监控指标
	if cfg != nil && cfg.AppMonitoring != nil && cfg.AppMonitoring.Enabled && cfg.AppMonitoring.Redis != nil && cfg.AppMonitoring.Redis.Enabled {
		if m.Redis.ConnectionError != nil {
			text.WriteString(fmt.Sprintf("- Redis: 连接失败 - %v\n", m.Redis.ConnectionError))
		} else {
			text.WriteString(fmt.Sprintf("- Redis: %d个连接 %s\n", m.Redis.ClientCount, redisStatusText(m.Redis.ClientCount, cfg)))
		}
	}

	// HTTP接口监控信息
	if cfg != nil && cfg.HTTPMonitoring != nil && cfg.HTTPMonitoring.Enabled {
		if m.HTTP.Error != nil {
			text.WriteString(fmt.Sprintf("- HTTP接口: 监控失败 - %v\n", m.HTTP.Error))
		} else if len(m.HTTP.Interfaces) > 0 {
			for _, httpInterface := range m.HTTP.Interfaces {
				isValidCode := utils.IsValidHTTPStatusCode(httpInterface.StatusCode, httpInterface.AllowedCodes)

				if isValidCode {
					text.WriteString(fmt.Sprintf("- HTTP接口 %s: 正常 (状态码: %d, 响应时间: %v)\n",
						httpInterface.Name, httpInterface.StatusCode, httpInterface.ResponseTime))
				} else {
					text.WriteString(fmt.Sprintf("- HTTP接口 %s: 异常 (状态码: %d) - %v\n",
						httpInterface.Name, httpInterface.StatusCode, httpInterface.Error))
				}
			}
		}
	}

	// 监控时间
	text.WriteString(fmt.Sprintf("- 监控时间: %s\n", m.Timestamp.Format("2006-01-02 15:04:05")))

	return text.String()
}

func statusText(value, threshold float64) string {
	return string(utils.GetMetricStatus(value, threshold))
}

// formatDiskUsageItems 格式化磁盘占用项目列表（使用标准markdown格式）
func formatDiskUsageItems(items []monitoring.DiskUsageItem, maxItems int) string {
	if len(items) == 0 {
		return "  - 暂无磁盘占用信息\n"
	}

	var result strings.Builder

	// 显示指定数量的项目
	displayCount := len(items)
	if maxItems > 0 && maxItems < len(items) {
		displayCount = maxItems
	}

	for i := 0; i < displayCount; i++ {
		item := items[i]
		// 使用标准markdown列表格式
		result.WriteString(fmt.Sprintf("  - %s (%s)\n", item.Path, item.Size))
	}

	return result.String()
}



// getTypeIndicator 根据类型获取标识
func getTypeIndicator(itemType string) string {
	switch itemType {
	case "file":
		return "[文件]" // 文件
	case "directory":
		return "[目录]" // 目录
	default:
		return "[未知]" // 未知类型
	}
}

func redisStatusText(count int, cfg *shared.Config) string {
	if cfg != nil && cfg.AppMonitoring != nil && cfg.AppMonitoring.Enabled && cfg.AppMonitoring.Redis != nil && cfg.AppMonitoring.Redis.Enabled {
		if cfg.AppMonitoring.Redis.Thresholds.Clients != nil && cfg.AppMonitoring.Redis.Thresholds.Clients.Enabled {
			return string(utils.GetRedisStatus(count, cfg.AppMonitoring.Redis.Thresholds.Clients.Min, cfg.AppMonitoring.Redis.Thresholds.Clients.Max))
		}
	}
	return "[正常]"
}
