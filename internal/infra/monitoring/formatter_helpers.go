package monitoring

import (
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"fmt"
)

// GetSeverityText 获取告警等级的中文文本
func GetSeverityText(severity string) string {
	switch severity {
	case "p1":
		return "紧急"
	case "p2":
		return "严重"
	case "p3":
		return "警告"
	case "reminder":
		return "提醒"
	default:
		return "严重"
	}
}

// FormatTriggerCondition 格式化触发条件
func FormatTriggerCondition(alertType entity.AlertType, severity string, threshold float64, duration string, isRecovery bool, cfg *shared.Config) string {
	var metricName string
	var thresholdStr string

	var actualThreshold float64 = threshold
	if !isRecovery && cfg != nil {
		// 使用GetThresholdConfig函数获取阈值配置
		thresholdConfig := GetThresholdConfig(cfg, alertType)

		// 根据告警类型设置指标名称和格式化方式
		switch alertType {
		case entity.CPUHigh:
			metricName = "CPU使用率"
			if thresholdConfig.HasConfig {
				actualThreshold = thresholdConfig.GetThresholdBySeverity(severity)
				thresholdStr = fmt.Sprintf("%.1f%%", actualThreshold)
			}
		case entity.MemHigh:
			metricName = "内存使用率"
			if thresholdConfig.HasConfig {
				actualThreshold = thresholdConfig.GetThresholdBySeverity(severity)
				thresholdStr = fmt.Sprintf("%.1f%%", actualThreshold)
			}
		case entity.DiskHigh:
			metricName = "磁盘使用率"
			if thresholdConfig.HasConfig {
				actualThreshold = thresholdConfig.GetThresholdBySeverity(severity)
				thresholdStr = fmt.Sprintf("%.1f%%", actualThreshold)
			}
		case entity.RedisHigh, entity.RedisLow:
			metricName = "Redis连接数"
			if thresholdConfig.HasConfig {
				actualThreshold = thresholdConfig.GetThresholdBySeverity(severity)
				thresholdStr = fmt.Sprintf("%.0f", actualThreshold)
			}
		case entity.HTTPErr:
			metricName = "HTTP接口"
			thresholdStr = "异常"
		default:
			metricName = alertType.String()
			thresholdStr = fmt.Sprintf("%.1f", actualThreshold)
		}
	} else {
		switch alertType {
		case entity.CPUHigh:
			metricName = "CPU使用率"
			thresholdStr = fmt.Sprintf("%.1f%%", threshold)
		case entity.MemHigh:
			metricName = "内存使用率"
			thresholdStr = fmt.Sprintf("%.1f%%", threshold)
		case entity.DiskHigh:
			metricName = "磁盘使用率"
			thresholdStr = fmt.Sprintf("%.1f%%", threshold)
		case entity.RedisHigh, entity.RedisLow:
			metricName = "Redis连接数"
			thresholdStr = fmt.Sprintf("%.0f", threshold)
		case entity.HTTPErr:
			metricName = "HTTP接口"
			thresholdStr = "正常"
		default:
			metricName = alertType.String()
			thresholdStr = fmt.Sprintf("%.1f", threshold)
		}
	}

	if alertType == entity.HTTPErr {
		if isRecovery {
			return fmt.Sprintf("%s %s", metricName, "正常")
		}
		return fmt.Sprintf("%s %s 且持续 %s", metricName, thresholdStr, duration)
	}

	if isRecovery {
		return fmt.Sprintf("%s < %s", metricName, thresholdStr)
	}
	return fmt.Sprintf("%s ≥ %s 且持续 %s", metricName, thresholdStr, duration)
}

// FormatTriggerObject 格式化触发对象（进程信息）
func FormatTriggerObject(alertType entity.AlertType, processName string, pid int32, cpuPercent, memPercent float64, memRSS uint64) string {
	if processName == "" {
		return "-"
	}

	switch alertType {
	case entity.CPUHigh:
		return fmt.Sprintf("%s PID=%d %.1f%% CPU", processName, pid, cpuPercent)
	case entity.MemHigh:
		return fmt.Sprintf("%s PID=%d %.1f%% MEM, %dMB", processName, pid, memPercent, memRSS)
	default:
		return fmt.Sprintf("%s PID=%d", processName, pid)
	}
}
