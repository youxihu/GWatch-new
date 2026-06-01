package usecase

import (
	"context"
	"time"

	domainFS "github.com/youxihu/GWatch-new/internal/domain/filesystem"
)

// DiskMonitoringUseCase 磁盘监控用例
type DiskMonitoringUseCase struct {
	scanner domainFS.Scanner
}

// NewDiskMonitoringUseCase 创建磁盘监控用例
func NewDiskMonitoringUseCase(scanner domainFS.Scanner) *DiskMonitoringUseCase {
	return &DiskMonitoringUseCase{
		scanner: scanner,
	}
}

// GetTopDiskUsage 获取磁盘使用情况（向后兼容接口）
func (uc *DiskMonitoringUseCase) GetTopDiskUsage(rootPath string, topN int) ([]domainFS.DiskUsageItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	return uc.scanner.QuickScan(ctx, rootPath, topN)
}

// GetTopDiskUsageWithConfig 根据配置获取磁盘使用情况
func (uc *DiskMonitoringUseCase) GetTopDiskUsageWithConfig(rootPath string, topN int, config interface{}) ([]domainFS.DiskUsageItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 这里可以根据传入的配置进行转换
	// 目前使用默认配置
	return uc.scanner.QuickScan(ctx, rootPath, topN)
}

// ScanDiskUsage 执行详细的磁盘扫描
func (uc *DiskMonitoringUseCase) ScanDiskUsage(rootPath string, topN int, config domainFS.ScanConfig) (domainFS.ScanResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	return uc.scanner.Scan(ctx, rootPath, topN, config)
}
