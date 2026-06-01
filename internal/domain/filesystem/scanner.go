package filesystem

import (
	"context"
	"time"
)

// DiskUsageItem 磁盘使用项 - 值对象
type DiskUsageItem struct {
	Path      string `json:"path"`
	Size      string `json:"size"`
	SizeBytes uint64 `json:"size_bytes"`
	Type      string `json:"type"`
	Depth     int    `json:"depth"`
}

// ScanConfig 扫描配置 - 值对象
type ScanConfig struct {
	Timeout            time.Duration
	MaxResults         int
	MinSizeThreshold   uint64
	RecursiveThreshold float64
	MaxDepth           int
	MaxConcurrency     int
}

// ScanResult 扫描结果 - 值对象
type ScanResult struct {
	Items     []DiskUsageItem
	Stats     ScanStats
	Timestamp time.Time
}

// ScanStats 扫描统计 - 值对象
type ScanStats struct {
	TotalScanned     int
	SkippedPaths     int
	ElapsedTime      time.Duration
	CompletedDepth   int
	TimeoutOccurred  bool
	MemoryUsageMB    float64
	PermissionErrors int
	IOErrors         int
}

// Scanner 磁盘扫描器 - 领域服务接口
type Scanner interface {
	Scan(ctx context.Context, rootPath string, topN int, config ScanConfig) (ScanResult, error)
	QuickScan(ctx context.Context, rootPath string, topN int) ([]DiskUsageItem, error)
}

// PathValidator 路径验证器 - 领域服务接口
type PathValidator interface {
	ShouldSkip(path string) (bool, string)
	HasReadPermission(path string) bool
	IsVirtualFileSystem(path string) bool
}

// ResourceMonitor 资源监控器 - 领域服务接口
type ResourceMonitor interface {
	Start()
	Stop()
	IsMemoryExceeded() bool
	GetCurrentMemoryUsage() float64
}
