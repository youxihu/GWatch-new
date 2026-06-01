// Package valueobject 值对象 - CPU指标
package valueobject

// CPUMetrics CPU指标值对象（不可变）
type CPUMetrics struct {
	Percent float64
	Cores   int
	Error   error
}

// NewCPUMetrics 创建CPU指标值对象
func NewCPUMetrics(percent float64, cores int, err error) CPUMetrics {
	return CPUMetrics{
		Percent: percent,
		Cores:   cores,
		Error:   err,
	}
}

// IsValid 检查CPU指标是否有效
func (c CPUMetrics) IsValid() bool {
	return c.Error == nil && c.Percent >= 0 && c.Percent <= 100
}
