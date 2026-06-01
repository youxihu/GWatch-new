package filesystem

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	domainFS "github.com/youxihu/GWatch-new/internal/domain/filesystem"
)

func (ds *DiskScanner) scanRecursively(ctx context.Context, path string, depth int, config domainFS.ScanConfig) ([]scanItem, domainFS.ScanStats) {
	stats := domainFS.ScanStats{
		CompletedDepth: depth,
	}

	// 检查超时和资源限制
	if err := ds.checkLimits(ctx); err != nil {
		if err == context.DeadlineExceeded {
			stats.TimeoutOccurred = true
		}
		return nil, stats
	}

	// 检查深度限制
	if depth > config.MaxDepth {
		return nil, stats
	}

	// 在每次递归开始时让出CPU
	if depth > 0 {
		time.Sleep(5 * time.Millisecond)
	}

	// 扫描当前层级
	items, err := ds.scanCurrentLevel(ctx, path)
	if err != nil {
		stats.SkippedPaths++
		return nil, stats
	}

	stats.TotalScanned += len(items)

	if len(items) == 0 {
		return nil, stats
	}

	// 排序找到最大的项
	sort.Slice(items, func(i, j int) bool {
		return items[i].SizeBytes > items[j].SizeBytes
	})

	var results []scanItem
	maxItems := config.MaxResults
	if maxItems > len(items) {
		maxItems = len(items)
	}

	// 限制递归的项目数量，避免过度CPU使用
	if depth > 2 {
		maxItems = min(maxItems, 3) // 深层递归时只处理前3个最大项
	}

	// 处理前几个最大的项
	for i := 0; i < maxItems; i++ {
		item := items[i]
		item.Depth = depth

		if item.IsDir && ds.shouldRecurse(item, items, config) {
			// 递归扫描子目录
			subResults, subStats := ds.scanRecursively(ctx, item.Path, depth+1, config)

			// 合并统计信息
			ds.mergeStats(&stats, subStats)

			if len(subResults) > 0 {
				results = append(results, subResults...)
			} else {
				results = append(results, item)
			}
		} else {
			results = append(results, item)
		}

		// 每处理一个项目就检查一次资源限制
		if err := ds.checkLimits(ctx); err != nil {
			if err == context.DeadlineExceeded {
				stats.TimeoutOccurred = true
			}
			break
		}
	}

	return results, stats
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// scanCurrentLevel 扫描当前层级
func (ds *DiskScanner) scanCurrentLevel(ctx context.Context, path string) ([]scanItem, error) {
	// 检查是否应该跳过
	if shouldSkip, _ := ds.pathValidator.ShouldSkip(path); shouldSkip {
		return nil, fmt.Errorf("路径被跳过: %s", path)
	}

	// 检查权限
	if !ds.pathValidator.HasReadPermission(path) {
		return nil, fmt.Errorf("权限不足: %s", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var items []scanItem
	processedCount := 0
	maxFiles := 1000 // 限制单个目录最多处理1000个文件

	for _, entry := range entries {
		// 检查超时和资源限制
		if err := ds.checkLimits(ctx); err != nil {
			break
		}

		// 限制处理的文件数量，避免CPU过载
		if processedCount >= maxFiles {
			break
		}

		entryPath := filepath.Join(path, entry.Name())

		// 检查是否应该跳过
		if shouldSkip, _ := ds.pathValidator.ShouldSkip(entryPath); shouldSkip {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		var sizeBytes uint64
		isDir := info.IsDir()

		if isDir {
			// 对于目录，使用更快的大小估算方法
			sizeBytes = ds.estimateDirectorySize(ctx, entryPath)
		} else {
			sizeBytes = uint64(info.Size())
		}

		items = append(items, scanItem{
			Path:      entryPath,
			SizeBytes: sizeBytes,
			IsDir:     isDir,
		})

		processedCount++

		// 每处理50个文件就让出CPU一次
		if processedCount%50 == 0 {
			time.Sleep(1 * time.Millisecond)
		}
	}

	return items, nil
}

// shouldRecurse 判断是否应该递归
func (ds *DiskScanner) shouldRecurse(item scanItem, allItems []scanItem, config domainFS.ScanConfig) bool {
	if !item.IsDir {
		return false
	}

	// 检查深度限制
	if item.Depth >= config.MaxDepth {
		return false
	}

	// 检查最小大小阈值
	if item.SizeBytes < config.MinSizeThreshold {
		return false
	}

	// 对于根目录扫描，总是递归前几个最大的目录
	if item.Depth == 0 {
		return true
	}

	// 对于更深层级，使用递归阈值判断
	// 但这里改为：如果目录大小超过最小阈值的10倍，就递归
	sizeThreshold := config.MinSizeThreshold * 10
	if item.SizeBytes >= sizeThreshold {
		return true
	}

	// 或者使用原来的比例逻辑，但降低阈值
	var totalSize uint64
	for _, i := range allItems {
		if i.IsDir {
			totalSize += i.SizeBytes
		}
	}

	if totalSize == 0 {
		return false
	}

	// 降低递归阈值，从0.7改为0.3（30%）
	ratio := float64(item.SizeBytes) / float64(totalSize)
	return ratio >= 0.3
}

// estimateDirectorySize 快速估算目录大小（轻量级方法）
func (ds *DiskScanner) estimateDirectorySize(ctx context.Context, path string) uint64 {
	// 首先尝试使用du命令快速获取
	if size := ds.calculateSizeWithDuFast(path); size > 0 {
		return size
	}

	// 如果du命令失败，使用采样估算
	return ds.sampleDirectorySize(ctx, path)
}

// calculateSizeWithDuFast 使用du命令快速计算目录大小
func (ds *DiskScanner) calculateSizeWithDuFast(path string) uint64 {
	// 检查路径是否存在
	if _, err := os.Stat(path); err != nil {
		return 0
	}

	// 使用更快的du命令，限制时间和深度
	cmd := fmt.Sprintf("timeout 2s du -s %s 2>/dev/null | cut -f1", path)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx, "sh", "-c", cmd).Output()
	if err != nil {
		return 0
	}

	sizeStr := strings.TrimSpace(string(output))
	if sizeStr == "" {
		return 0
	}

	// du -s 返回的是KB，需要转换为字节
	sizeKB, err := strconv.ParseUint(sizeStr, 10, 64)
	if err != nil {
		return 0
	}

	return sizeKB * 1024
}

// sampleDirectorySize 通过采样估算目录大小
func (ds *DiskScanner) sampleDirectorySize(ctx context.Context, path string) uint64 {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}

	if len(entries) == 0 {
		return 0
	}

	var totalSize uint64
	sampleCount := 0
	maxSamples := 20 // 最多采样20个文件

	for _, entry := range entries {
		// 检查超时
		select {
		case <-ctx.Done():
			break
		default:
		}

		if sampleCount >= maxSamples {
			break
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if !info.IsDir() {
			totalSize += uint64(info.Size())
		} else {
			// 对于子目录，使用目录项大小的简单估算
			totalSize += uint64(info.Size()) * 100 // 简单估算倍数
		}

		sampleCount++
	}

	// 根据采样比例估算总大小
	if sampleCount > 0 && sampleCount < len(entries) {
		ratio := float64(len(entries)) / float64(sampleCount)
		totalSize = uint64(float64(totalSize) * ratio)
	}

	return totalSize
}

// checkLimits 检查各种限制
func (ds *DiskScanner) checkLimits(ctx context.Context) error {
	// 检查超时
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 检查内存限制
	if ds.resourceMonitor != nil && ds.resourceMonitor.IsMemoryExceeded() {
		return fmt.Errorf("内存使用超限")
	}

	return nil
}

// mergeStats 合并统计信息
func (ds *DiskScanner) mergeStats(main *domainFS.ScanStats, sub domainFS.ScanStats) {
	main.TotalScanned += sub.TotalScanned
	main.SkippedPaths += sub.SkippedPaths
	main.PermissionErrors += sub.PermissionErrors
	main.IOErrors += sub.IOErrors

	if sub.CompletedDepth > main.CompletedDepth {
		main.CompletedDepth = sub.CompletedDepth
	}

	if sub.TimeoutOccurred {
		main.TimeoutOccurred = true
	}
}

// convertToItems 转换为领域对象
func (ds *DiskScanner) convertToItems(items []scanItem, minSize uint64) []domainFS.DiskUsageItem {
	var result []domainFS.DiskUsageItem

	for _, item := range items {
		if item.SizeBytes >= minSize {
			itemType := "file"
			if item.IsDir {
				itemType = "directory"
			}

			result = append(result, domainFS.DiskUsageItem{
				Path:      item.Path,
				Size:      ds.formatBytes(item.SizeBytes),
				SizeBytes: item.SizeBytes,
				Type:      itemType,
				Depth:     item.Depth,
			})
		}
	}

	return result
}

// getFallbackResults 获取备用结果
func (ds *DiskScanner) getFallbackResults(rootPath string, topN int) []domainFS.DiskUsageItem {
	importantDirs := []string{"/var", "/usr", "/home", "/opt", "/etc", "/tmp"}
	var results []domainFS.DiskUsageItem

	for _, dir := range importantDirs {
		if len(results) >= topN {
			break
		}

		// 计算目录的实际大小
		size := ds.estimateDirectorySize(context.Background(), dir)
		if size == 0 {
			// 如果计算失败，跳过这个目录
			continue
		}

		results = append(results, domainFS.DiskUsageItem{
			Path:      dir,
			Size:      ds.formatBytes(size),
			SizeBytes: size,
			Type:      "directory",
			Depth:     0,
		})
	}

	// 按大小排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].SizeBytes > results[j].SizeBytes
	})

	return results
}

// formatBytes 格式化字节数
func (ds *DiskScanner) formatBytes(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2fT", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2fG", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2fM", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2fK", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
