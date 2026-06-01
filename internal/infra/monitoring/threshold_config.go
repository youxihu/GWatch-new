package monitoring

import (
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
)

// ThresholdConfig 阈值配置
type ThresholdConfig struct {
	Base, P1, P2, P3 float64
	HasConfig        bool
}

// GetThresholdBySeverity 根据告警等级获取对应的阈值
func (tc ThresholdConfig) GetThresholdBySeverity(severity string) float64 {
	switch severity {
	case "p1":
		return tc.P1
	case "p2":
		return tc.P2
	case "p3":
		return tc.P3
	case "reminder":
		return tc.Base
	default:
		return tc.Base
	}
}

// GetThresholdConfig 获取告警类型的阈值配置
func GetThresholdConfig(cfg *shared.Config, alertType entity.AlertType) ThresholdConfig {
	switch alertType {
	case entity.CPUHigh:
		if cfg.HostMonitoring != nil && cfg.HostMonitoring.Thresholds.CPU != nil {
			return ThresholdConfig{
				Base:      cfg.HostMonitoring.Thresholds.CPU.Base,
				P1:        cfg.HostMonitoring.Thresholds.CPU.Levels.P1,
				P2:        cfg.HostMonitoring.Thresholds.CPU.Levels.P2,
				P3:        cfg.HostMonitoring.Thresholds.CPU.Levels.P3,
				HasConfig: true,
			}
		}
	case entity.MemHigh:
		if cfg.HostMonitoring != nil && cfg.HostMonitoring.Thresholds.Memory != nil {
			return ThresholdConfig{
				Base:      cfg.HostMonitoring.Thresholds.Memory.Base,
				P1:        cfg.HostMonitoring.Thresholds.Memory.Levels.P1,
				P2:        cfg.HostMonitoring.Thresholds.Memory.Levels.P2,
				P3:        cfg.HostMonitoring.Thresholds.Memory.Levels.P3,
				HasConfig: true,
			}
		}
	case entity.DiskHigh:
		if cfg.HostMonitoring != nil && cfg.HostMonitoring.Thresholds.Disk != nil {
			return ThresholdConfig{
				Base:      cfg.HostMonitoring.Thresholds.Disk.Base,
				P1:        cfg.HostMonitoring.Thresholds.Disk.Levels.P1,
				P2:        cfg.HostMonitoring.Thresholds.Disk.Levels.P2,
				P3:        cfg.HostMonitoring.Thresholds.Disk.Levels.P3,
				HasConfig: true,
			}
		}
	case entity.RedisHigh:
		// RedisHigh：使用区间判断，不在[min, max]区间即为异常
		// 返回一个简单的配置，base设置为max，用于恢复判断
		if cfg.AppMonitoring != nil && cfg.AppMonitoring.Redis != nil && cfg.AppMonitoring.Redis.Thresholds.Clients != nil {
			min := float64(cfg.AppMonitoring.Redis.Thresholds.Clients.Min)
			max := float64(cfg.AppMonitoring.Redis.Thresholds.Clients.Max)
			return ThresholdConfig{
				Base:      max, // base设置为max，用于恢复判断（在[min, max]区间内即为恢复）
				P1:        min, // P1设置为min，用于恢复判断
				P2:        0.0, // 不使用
				P3:        0.0, // 不使用
				HasConfig: true,
			}
		}
	case entity.RedisLow:
		// RedisLow：使用区间判断，不在[min, max]区间即为异常
		// 返回一个简单的配置，base设置为min，用于恢复判断
		if cfg.AppMonitoring != nil && cfg.AppMonitoring.Redis != nil && cfg.AppMonitoring.Redis.Thresholds.Clients != nil {
			min := float64(cfg.AppMonitoring.Redis.Thresholds.Clients.Min)
			max := float64(cfg.AppMonitoring.Redis.Thresholds.Clients.Max)
			return ThresholdConfig{
				Base:      min, // base设置为min，用于恢复判断（在[min, max]区间内即为恢复）
				P1:        max, // P1设置为max，用于恢复判断
				P2:        0.0, // 不使用
				P3:        0.0, // 不使用
				HasConfig: true,
			}
		}
	case entity.HTTPErr:
		// HTTP告警使用p2级别（状态码不在allowed_codes中即为p2）
		// 设置一个特殊值，确保HTTP告警能通过阈值检查
		return ThresholdConfig{
			Base:      0.0, // base设置为0，确保任何异常状态码都能触发
			P1:        0.0, // p1设置为0
			P2:        0.0, // p2也设置为0，HTTP告警统一使用p2级别
			P3:        0.0, // p3设置为0
			HasConfig: true,
		}
	}
	return ThresholdConfig{HasConfig: false}
}

// DetermineSeverity 根据当前值和阈值确定告警等级
// 注意：RedisLow使用反向逻辑（<=），其他告警类型使用正向逻辑（>=）
func DetermineSeverity(currentValue float64, config ThresholdConfig, alertType entity.AlertType) string {
	// RedisLow使用反向逻辑：值越小越严重
	if alertType == entity.RedisLow {
		if currentValue <= config.P1 {
			return "p1"
		}
		if currentValue <= config.P2 {
			return "p2"
		}
		if currentValue <= config.P3 {
			return "p3"
		}
		if currentValue <= config.Base {
			return "reminder"
		}
		return ""
	}

	// 其他告警类型使用正向逻辑：值越大越严重
	if currentValue >= config.P1 {
		return "p1"
	}
	if currentValue >= config.P2 {
		return "p2"
	}
	if currentValue >= config.P3 {
		return "p3"
	}
	if currentValue >= config.Base {
		return "reminder"
	}
	return ""
}
