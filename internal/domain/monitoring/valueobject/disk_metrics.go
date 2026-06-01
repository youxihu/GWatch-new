// Package valueobject 值对象 - 磁盘指标
package valueobject

// DiskMetrics 磁盘指标值对象（不可变）
type DiskMetrics struct {
	Percent   float64
	UsedGB    uint64
	TotalGB   uint64
	ReadKBps  float64
	WriteKBps float64
	Error     error
}

// NewDiskMetrics 创建磁盘指标值对象
func NewDiskMetrics(percent float64, usedGB, totalGB uint64, readKBps, writeKBps float64, err error) DiskMetrics {
	return DiskMetrics{
		Percent:   percent,
		UsedGB:    usedGB,
		TotalGB:   totalGB,
		ReadKBps:  readKBps,
		WriteKBps: writeKBps,
		Error:     err,
	}
}

// IsValid 检查磁盘指标是否有效
func (d DiskMetrics) IsValid() bool {
	return d.Error == nil && d.Percent >= 0 && d.Percent <= 100 && d.UsedGB <= d.TotalGB
}

// UsageRatio 返回使用率（0-1）
func (d DiskMetrics) UsageRatio() float64 {
	if d.TotalGB == 0 {
		return 0
	}
	return float64(d.UsedGB) / float64(d.TotalGB)
}
