// Package valueobject 值对象 - 网络指标
package valueobject

// NetworkMetrics 网络指标值对象（不可变）
type NetworkMetrics struct {
	DownloadKBps float64
	UploadKBps   float64
	Error        error
}

// NewNetworkMetrics 创建网络指标值对象
func NewNetworkMetrics(downloadKBps, uploadKBps float64, err error) NetworkMetrics {
	return NetworkMetrics{
		DownloadKBps: downloadKBps,
		UploadKBps:   uploadKBps,
		Error:        err,
	}
}

// IsValid 检查网络指标是否有效
func (n NetworkMetrics) IsValid() bool {
	return n.Error == nil && n.DownloadKBps >= 0 && n.UploadKBps >= 0
}
