package filesystem

import (
	"context"
	"sort"
	"time"

	domainFS "github.com/youxihu/GWatch-new/internal/domain/filesystem"
)

// 磁盘扫描器实现
type DiskScanner struct {
	pathValidator   domainFS.PathValidator
	resourceMonitor domainFS.ResourceMonitor
}

// NewDiskScanner 创建磁盘扫描器
func NewDiskScanner(validator domainFS.PathValidator, monitor domainFS.ResourceMonitor) *DiskScanner {
	return &DiskScanner{
		pathValidator:   validator,
		resourceMonitor: monitor,
	}
}

// Scan 执行磁盘扫描
func (ds *DiskScanner) Scan(ctx context.Context, rootPath string, topN int, config domainFS.ScanConfig) (domainFS.ScanResult, error) {
	if topN <= 0 {
		topN = 3
	}

	startTime := time.Now()

	// 创建带超时的 context
	ctx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	// 启动资源监控
	var resourceStarted bool
	if ds.resourceMonitor != nil {
		ds.resourceMonitor.Start()
		resourceStarted = true
	}

	// 确保资源监控器被正确停止
	defer func() {
		if resourceStarted && ds.resourceMonitor != nil {
			ds.resourceMonitor.Stop()
		}
	}()

	// 执行扫描
	items, stats := ds.scanRecursively(ctx, rootPath, 0, config)

	// 更新统计信息
	stats.ElapsedTime = time.Since(startTime)
	if ds.resourceMonitor != nil {
		stats.MemoryUsageMB = ds.resourceMonitor.GetCurrentMemoryUsage()
	}

	// 转换为领域对象
	diskItems := ds.convertToItems(items, config.MinSizeThreshold)

	// 排序并限制数量
	sort.Slice(diskItems, func(i, j int) bool {
		return diskItems[i].SizeBytes > diskItems[j].SizeBytes
	})

	if len(diskItems) > topN {
		diskItems = diskItems[:topN]
	}

	// 如果没有结果，返回备用结果
	if len(diskItems) == 0 {
		diskItems = ds.getFallbackResults(rootPath, topN)
	}

	return domainFS.ScanResult{
		Items:     diskItems,
		Stats:     stats,
		Timestamp: time.Now(),
	}, nil
}

// QuickScan 快速扫描（向后兼容）
func (ds *DiskScanner) QuickScan(ctx context.Context, rootPath string, topN int) ([]domainFS.DiskUsageItem, error) {
	config := domainFS.ScanConfig{
		Timeout:            60 * time.Second,
		MaxResults:         10,
		MinSizeThreshold:   1 * 1024 * 1024, // 1MB
		RecursiveThreshold: 0.5,
		MaxDepth:           15,
		MaxConcurrency:     8,
	}

	result, err := ds.Scan(ctx, rootPath, topN, config)
	return result.Items, err
}

// scanItem 内部扫描项
type scanItem struct {
	Path      string
	SizeBytes uint64
	IsDir     bool
	Depth     int
}
