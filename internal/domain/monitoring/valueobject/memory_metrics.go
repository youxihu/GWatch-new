// Package valueobject 值对象 - 内存指标
package valueobject

// MemoryMetrics 内存指标值对象（不可变）
type MemoryMetrics struct {
	Percent float64
	UsedMB  uint64
	TotalMB uint64
	Error   error
}

// NewMemoryMetrics 创建内存指标值对象
func NewMemoryMetrics(percent float64, usedMB, totalMB uint64, err error) MemoryMetrics {
	return MemoryMetrics{
		Percent: percent,
		UsedMB:  usedMB,
		TotalMB: totalMB,
		Error:   err,
	}
}

// IsValid 检查内存指标是否有效
func (m MemoryMetrics) IsValid() bool {
	return m.Error == nil && m.Percent >= 0 && m.Percent <= 100 && m.UsedMB <= m.TotalMB
}

// UsageRatio 返回使用率（0-1）
func (m MemoryMetrics) UsageRatio() float64 {
	if m.TotalMB == 0 {
		return 0
	}
	return float64(m.UsedMB) / float64(m.TotalMB)
}
