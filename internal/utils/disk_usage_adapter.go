package utils

import (
	"time"

	"github.com/youxihu/GWatch-new/internal/app/usecase"
	sharedEntity "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	domainFS "github.com/youxihu/GWatch-new/internal/domain/filesystem"
	infraFS "github.com/youxihu/GWatch-new/internal/infra/filesystem"
)

// DiskUsageItem 保持向后兼容的类型别名
type DiskUsageItem = sharedEntity.DiskUsageItem

var (
	// 全局实例，用于向后兼容
	diskMonitoringUseCase *usecase.DiskMonitoringUseCase
)

// init 初始化全局实例
func init() {
	// 创建基础设施层实例
	pathValidator := infraFS.NewPathValidator()
	resourceMonitor := infraFS.NewResourceMonitor(512.0, 1*time.Second) // 512MB 内存限制
	diskScanner := infraFS.NewDiskScanner(pathValidator, resourceMonitor)

	// 创建用例实例
	diskMonitoringUseCase = usecase.NewDiskMonitoringUseCase(diskScanner)
}

// GetTopDiskUsage 获取占用最大的前N个目录或文件（向后兼容接口）
func GetTopDiskUsage(rootPath string, topN int) ([]DiskUsageItem, error) {
	items, err := diskMonitoringUseCase.GetTopDiskUsage(rootPath, topN)
	if err != nil {
		return nil, err
	}

	// 转换为共享实体类型
	result := make([]DiskUsageItem, len(items))
	for i, item := range items {
		result[i] = DiskUsageItem{
			Path:      item.Path,
			Size:      item.Size,
			SizeBytes: item.SizeBytes,
			Type:      item.Type,
			Depth:     item.Depth,
		}
	}

	return result, nil
}

// GetTopDiskUsageWithConfig 根据配置获取磁盘占用信息（向后兼容接口）
func GetTopDiskUsageWithConfig(rootPath string, topN int, diskScanConfig interface{}) ([]DiskUsageItem, error) {
	items, err := diskMonitoringUseCase.GetTopDiskUsageWithConfig(rootPath, topN, diskScanConfig)
	if err != nil {
		return nil, err
	}

	// 转换为共享实体类型
	result := make([]DiskUsageItem, len(items))
	for i, item := range items {
		result[i] = DiskUsageItem{
			Path:      item.Path,
			Size:      item.Size,
			SizeBytes: item.SizeBytes,
			Type:      item.Type,
			Depth:     item.Depth,
		}
	}

	return result, nil
}

// ScanDiskUsage 扫描磁盘占用信息（新接口）
func ScanDiskUsage(rootPath string, topN int, config domainFS.ScanConfig) (domainFS.ScanResult, error) {
	return diskMonitoringUseCase.ScanDiskUsage(rootPath, topN, config)
}

// CreateScanConfigFromDomainConfig 从领域配置创建扫描配置
func CreateScanConfigFromDomainConfig(domainConfig interface{}) domainFS.ScanConfig {
	// 返回默认配置
	return domainFS.ScanConfig{
		Timeout:            60 * time.Second,
		MaxResults:         10,
		MinSizeThreshold:   1 * 1024 * 1024, // 1MB
		RecursiveThreshold: 0.5,             // 50%
		MaxDepth:           15,
		MaxConcurrency:     8,
	}
}
