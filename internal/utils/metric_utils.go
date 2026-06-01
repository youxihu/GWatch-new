package utils

// MetricStatus 指标状态
type MetricStatus string

const (
	StatusNormal   MetricStatus = "[正常]"
	StatusAbnormal MetricStatus = "[异常]"
	StatusLow      MetricStatus = "[连接数过低]"
	StatusHigh     MetricStatus = "[连接数过高]"
)

// GetMetricStatus 获取指标状态
func GetMetricStatus(value, threshold float64) MetricStatus {
	if value > threshold {
		return StatusAbnormal
	}
	return StatusNormal
}

// GetRedisStatus 获取Redis连接数状态
func GetRedisStatus(count int, min, max int) MetricStatus {
	if count < min {
		return StatusLow
	}
	if count > max {
		return StatusHigh
	}
	return StatusNormal
}
