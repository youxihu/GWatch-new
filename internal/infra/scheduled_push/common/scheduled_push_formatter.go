// 定时推送格式化器实现
package common

import (
	monitoringEntity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	scheduledPushEntity "github.com/youxihu/GWatch-new/internal/domain/entity/scheduled_push"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"github.com/youxihu/GWatch-new/internal/domain/scheduled_push/common"
	monitoringInfra "github.com/youxihu/GWatch-new/internal/infra/monitoring"
	"github.com/youxihu/GWatch-new/internal/utils"
	"fmt"
	"strings"
	"time"
)

// ScheduledPushFormatterImpl 定时推送格式化器实现
type ScheduledPushFormatterImpl struct{}

// NewScheduledPushFormatter 创建定时推送格式化器
func NewScheduledPushFormatter() common.ScheduledPushFormatter {
	return &ScheduledPushFormatterImpl{}
}

// FormatClientReport 格式化合并后的客户端报告（按照 dingformat.md 的格式）
// 只显示已开启的监控项，如果没有开启就取消该栏
// title: 配置的title，用于每个主机的二级标题（#### {title}）
// config: 配置信息，用于判断阈值和状态
func (f *ScheduledPushFormatterImpl) FormatClientReport(data []*scheduledPushEntity.ClientMonitorData, title string, config *shared.Config) string {
	if len(data) == 0 {
		return "暂无监控数据"
	}

	var sb strings.Builder

	// 报告内容的大标题（固定为"[性能监控定时推送]"）
	sb.WriteString("## [性能监控定时推送]\n\n")

	// 遍历所有客户端数据
	for i, clientData := range data {
		metrics := clientData.Metrics
		if metrics == nil {
			continue
		}

		// 每个主机的二级标题（优先使用配置的title，如果没有则使用主机名，最后使用IP）
		hostTitle := clientData.Title
		if hostTitle == "" {
			hostTitle = clientData.HostName
		}
		if hostTitle == "" || hostTitle == "unknown-host" {
			hostTitle = clientData.HostIP
		}
		// 如果配置的title、主机名和IP都没有，使用传入的默认title
		if hostTitle == "" || hostTitle == "unknown-ip" {
			hostTitle = title
		}
		sb.WriteString(fmt.Sprintf("#### %s\n", hostTitle))

		// 主机信息（显示IP和主机名）
		displayHostName := clientData.HostName
		if displayHostName == "" || displayHostName == "unknown-host" {
			// 如果主机名不可用，使用"未命名主机"
			displayHostName = "未命名主机"
		}

		if clientData.HostIP != "" && clientData.HostIP != "unknown-ip" {
			sb.WriteString(fmt.Sprintf("主机IP: %s (%s)\n", clientData.HostIP, displayHostName))
		} else {
			// 如果IP也不可用，只显示主机名（或未命名主机）
			sb.WriteString(fmt.Sprintf("主机: %s\n", displayHostName))
		}

		// CPU 信息（必须有）
		if metrics.CPU != nil && metrics.CPU.Error == nil {
			cpuStatus := f.getHostMetricStatus(metrics.CPU.Percent, config, "cpu")
			if metrics.CPU.Cores > 0 {
				sb.WriteString(fmt.Sprintf("- CPU: %.2f%% (%d核心) %s\n", metrics.CPU.Percent, metrics.CPU.Cores, cpuStatus))
			} else {
				sb.WriteString(fmt.Sprintf("- CPU: %.2f%% %s\n", metrics.CPU.Percent, cpuStatus))
			}
		}

		// 内存信息（必须有）
		if metrics.Memory != nil && metrics.Memory.Error == nil {
			memStatus := f.getHostMetricStatus(metrics.Memory.Percent, config, "memory")
			sb.WriteString(fmt.Sprintf("- 内存: %.2f%% (%d/%d MB) %s\n",
				metrics.Memory.Percent, metrics.Memory.UsedMB, metrics.Memory.TotalMB, memStatus))
		}

		// 磁盘信息（必须有）
		if metrics.Disk != nil && metrics.Disk.Error == nil {
			diskStatus := f.getHostMetricStatus(metrics.Disk.Percent, config, "disk")
			sb.WriteString(fmt.Sprintf("- 磁盘: %.2f%% (%d/%d GB) %s\n",
				metrics.Disk.Percent, metrics.Disk.UsedGB, metrics.Disk.TotalGB, diskStatus))
		}

		// 网络IO（必须有）
		if metrics.Network != nil && metrics.Network.Error == nil {
			sb.WriteString(fmt.Sprintf("- 网络IO: %s\n",
				utils.FormatIOSpeedPair(metrics.Network.DownloadKBps, metrics.Network.UploadKBps, "下载", "上传")))
		}

		// 磁盘IO（必须有）
		if metrics.Disk != nil && metrics.Disk.Error == nil {
			sb.WriteString(fmt.Sprintf("- 磁盘IO: %s\n",
				utils.FormatIOSpeedPair(metrics.Disk.ReadKBps, metrics.Disk.WriteKBps, "读", "写")))
		}

		// Redis（如果开启才显示）
		if metrics.Redis != nil && metrics.Redis.ConnectionError == nil {
			redisStatusText := f.getRedisStatus(metrics.Redis.ClientCount, config)
			sb.WriteString(fmt.Sprintf("- Redis: %d个连接 %s\n", metrics.Redis.ClientCount, redisStatusText))
		}

		// HTTP接口（如果开启才显示，只显示正常的接口）
		if metrics.HTTP != nil && metrics.HTTP.Error == nil && len(metrics.HTTP.Interfaces) > 0 {
			// 过滤出正常的接口
			var validInterfaces []monitoringEntity.HTTPInterfaceMetrics
			for _, iface := range metrics.HTTP.Interfaces {
				if iface.Error == nil && iface.IsAccessible {
					validInterfaces = append(validInterfaces, iface)
				}
			}
			// 只有当有正常接口时才显示HTTP接口这一栏
			if len(validInterfaces) > 0 {
				sb.WriteString("- HTTP接口:\n")
				for _, iface := range validInterfaces {
					// 响应时间转换为毫秒，保留小数
					responseTimeMs := float64(iface.ResponseTime.Nanoseconds()) / 1e6
					sb.WriteString(fmt.Sprintf("    - %s: 正常 (状态码: %d, 响应时间: %.6fms)\n",
						iface.Name, iface.StatusCode, responseTimeMs))
				}
			}
		}

		// 如果不是最后一个，添加分隔符
		if i < len(data)-1 {
			sb.WriteString("---\n")
		}
	}

	// 监控时间（使用最后一个数据的时间戳，或当前时间）
	var timestamp time.Time
	if len(data) > 0 {
		timestamp = data[len(data)-1].Timestamp
	} else {
		timestamp = time.Now()
	}
	reportTime := timestamp.Format("2006-01-02 15:04:05")
	sb.WriteString(fmt.Sprintf("---\n监控时间: %s\n", reportTime))

	return sb.String()
}

// getHostMetricStatus 根据主机监控指标值和阈值判断状态（CPU/内存/磁盘）
func (f *ScheduledPushFormatterImpl) getHostMetricStatus(value float64, config *shared.Config, metricType string) string {
	// 如果配置不存在或未启用主机监控，默认返回正常
	if config == nil || config.HostMonitoring == nil || !config.HostMonitoring.Enabled {
		return "[正常]"
	}

	var alertType monitoringEntity.AlertType
	switch metricType {
	case "cpu":
		alertType = monitoringEntity.CPUHigh
	case "memory":
		alertType = monitoringEntity.MemHigh
	case "disk":
		alertType = monitoringEntity.DiskHigh
	default:
		return "[正常]"
	}

	// 使用GetThresholdConfig函数获取阈值配置
	thresholdConfig := monitoringInfra.GetThresholdConfig(config, alertType)
	if !thresholdConfig.HasConfig {
		return "[正常]"
	}

	// 如果值超过阈值，返回异常
	if value > thresholdConfig.Base {
		return "[异常]"
	}

	return "[正常]"
}

// getRedisStatus 根据Redis连接数和阈值判断状态
func (f *ScheduledPushFormatterImpl) getRedisStatus(count int, config *shared.Config) string {
	if config == nil || config.AppMonitoring == nil || !config.AppMonitoring.Enabled {
		return "[正常]"
	}
	if config.AppMonitoring.Redis == nil || !config.AppMonitoring.Redis.Enabled {
		return "[正常]"
	}
	if config.AppMonitoring.Redis.Thresholds.Clients != nil && config.AppMonitoring.Redis.Thresholds.Clients.Enabled {
		// 使用utils.GetRedisStatus函数获取Redis状态
		status := utils.GetRedisStatus(count, config.AppMonitoring.Redis.Thresholds.Clients.Min, config.AppMonitoring.Redis.Thresholds.Clients.Max)
		return string(status)
	}
	return "[正常]"
}
