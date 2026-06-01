package monitoring

import (
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	sharedEntity "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	domainFS "github.com/youxihu/GWatch-new/internal/domain/filesystem"
	"github.com/youxihu/GWatch-new/internal/domain/monitoring"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	"github.com/youxihu/GWatch-new/internal/utils"
	"time"
)

// DiskUsageEnricher 磁盘占用信息补充器
// 用于在磁盘告警时异步收集详细的磁盘占用信息（top 3目录/文件）
type DiskUsageEnricher struct {
	storage monitoring.AlertStateStorage
	config  *sharedEntity.DiskUsageScanConfig
}

// NewDiskUsageEnricher 创建磁盘占用信息补充器
func NewDiskUsageEnricher(storage monitoring.AlertStateStorage) *DiskUsageEnricher {
	return &DiskUsageEnricher{
		storage: storage,
		config:  getDefaultDiskScanConfig(),
	}
}

// NewDiskUsageEnricherWithConfig 使用指定配置创建磁盘占用信息补充器
func NewDiskUsageEnricherWithConfig(storage monitoring.AlertStateStorage, config *sharedEntity.DiskUsageScanConfig) *DiskUsageEnricher {
	if config == nil {
		config = getDefaultDiskScanConfig()
	}
	return &DiskUsageEnricher{
		storage: storage,
		config:  config,
	}
}

// EnrichAsync 异步补充磁盘占用信息
func (e *DiskUsageEnricher) EnrichAsync(alertType entity.AlertType, pid, eventID string) {
	go e.enrich(alertType, pid, eventID)
}

func (e *DiskUsageEnricher) enrich(alertType entity.AlertType, pid, eventID string) {
	startTime := time.Now()
	logger.Infof("[磁盘占用补充] 开始异步补充磁盘占用信息，事件ID: %s，配置: 超时=%v, 最大结果=%d",
		eventID, e.config.Timeout, e.config.MaxResults)

	// 检查是否启用递归扫描
	if !e.config.Enabled {
		logger.Infof("[磁盘占用补充] 递归扫描已禁用，使用基础扫描，事件ID: %s", eventID)
		e.enrichBasic(alertType, pid, eventID)
		return
	}

	// 创建扫描配置
	scanConfig := e.createScanConfig()

	// 执行增强扫描
	result, err := utils.ScanDiskUsage("/", e.config.MaxResults, scanConfig)

	// 记录扫描统计信息
	elapsedTime := time.Since(startTime)
	logger.Infof("[磁盘占用补充] 扫描完成，事件ID: %s，耗时: %v，统计: 扫描项=%d, 跳过=%d, 深度=%d, 超时=%v, 内存=%.2fMB",
		eventID, elapsedTime, result.Stats.TotalScanned, result.Stats.SkippedPaths,
		result.Stats.CompletedDepth, result.Stats.TimeoutOccurred, result.Stats.MemoryUsageMB)

	if err != nil && len(result.Items) == 0 {
		logger.Warnf("[磁盘占用补充] 获取磁盘占用信息失败: %v，回退到基础扫描", err)
		e.enrichBasic(alertType, pid, eventID)
		return
	}

	if len(result.Items) == 0 {
		logger.Warnf("[磁盘占用补充] 未获取到任何磁盘占用信息，事件ID: %s", eventID)
		return
	}

	// 转换结果格式
	var diskUsageItems []monitoring.DiskUsageItem
	for _, item := range result.Items {
		diskUsageItems = append(diskUsageItems, monitoring.DiskUsageItem{
			Path:         item.Path,
			Size:         item.Size,
			SizeBytes:    item.SizeBytes,
			Type:         item.Type,
			Depth:        item.Depth,
			IsLeafResult: true, // 递归扫描的结果都是叶子结果（真正的热点）
		})
	}

	// 更新告警状态
	e.updateAlertState(eventID, diskUsageItems, &result.Stats)

	logger.Infof("[磁盘占用补充] 成功补充并更新磁盘占用信息，事件ID: %s，共 %d 项，总耗时: %v",
		eventID, len(diskUsageItems), elapsedTime)
}

// enrichBasic 基础磁盘占用补充（回退方案）
func (e *DiskUsageEnricher) enrichBasic(alertType entity.AlertType, pid, eventID string) {
	logger.Infof("[磁盘占用补充] 使用基础扫描模式，事件ID: %s", eventID)

	topDiskUsage, err := utils.GetTopDiskUsage("/", 3)
	if err != nil && len(topDiskUsage) == 0 {
		logger.Warnf("[磁盘占用补充] 基础扫描失败: %v", err)
		return
	}

	if len(topDiskUsage) == 0 {
		logger.Warnf("[磁盘占用补充] 基础扫描未获取到任何磁盘占用信息")
		return
	}

	var diskUsageItems []monitoring.DiskUsageItem
	for _, item := range topDiskUsage {
		diskUsageItems = append(diskUsageItems, monitoring.DiskUsageItem{
			Path:         item.Path,
			Size:         item.Size,
			SizeBytes:    item.SizeBytes,
			Type:         item.Type,
			Depth:        item.Depth,
			IsLeafResult: false, // 基础扫描的结果不是叶子结果
		})
	}

	e.updateAlertState(eventID, diskUsageItems, nil)

	logger.Infof("[磁盘占用补充] 基础扫描完成，事件ID: %s，共 %d 项", eventID, len(diskUsageItems))
}

// updateAlertState 更新告警状态
func (e *DiskUsageEnricher) updateAlertState(eventID string, diskUsageItems []monitoring.DiskUsageItem, stats *domainFS.ScanStats) {
	if e.storage == nil {
		logger.Warnf("[磁盘占用补充] 存储服务未初始化，事件ID: %s", eventID)
		return
	}

	// 使用EventID获取告警状态
	state, err := e.storage.GetAlertStateByEventID(eventID)
	if err != nil {
		logger.Warnf("[磁盘占用补充] 获取告警状态失败，事件ID: %s, 错误: %v", eventID, err)
		return
	}
	if state == nil {
		logger.Warnf("[磁盘占用补充] 告警状态不存在，事件ID: %s", eventID)
		return
	}

	// 更新磁盘使用信息
	state.DiskUsage = diskUsageItems
	state.DiskUsageUpdateTime = time.Now()

	logger.Debugf("[磁盘占用补充] 准备更新告警状态，事件ID: %s, 磁盘使用项数: %d", eventID, len(diskUsageItems))

	if err := e.storage.UpdateAlertStateByEventID(state); err != nil {
		logger.Warnf("[磁盘占用补充] 更新告警状态失败，事件ID: %s, 错误: %v", eventID, err)
		return
	}

	logger.Infof("[磁盘占用补充] 成功更新告警状态，事件ID: %s, 磁盘使用项数: %d", eventID, len(diskUsageItems))
}

// createScanConfig 创建扫描配置
func (e *DiskUsageEnricher) createScanConfig() domainFS.ScanConfig {
	config := domainFS.ScanConfig{
		Timeout:            e.config.Timeout,
		MaxResults:         e.config.MaxResults,
		MinSizeThreshold:   e.config.MinSizeThreshold,
		RecursiveThreshold: e.config.RecursiveThreshold,
		MaxDepth:           e.config.MaxDepth,
		MaxConcurrency:     4, // 默认值
	}

	// 应用性能配置
	if e.config.Performance != nil {
		if e.config.Performance.MaxConcurrency > 0 {
			config.MaxConcurrency = e.config.Performance.MaxConcurrency
		}
	}

	return config
}

// getDefaultDiskScanConfig 获取默认磁盘扫描配置
func getDefaultDiskScanConfig() *sharedEntity.DiskUsageScanConfig {
	return &sharedEntity.DiskUsageScanConfig{
		Enabled:               true,
		Timeout:               30 * time.Second,
		MaxResults:            5,
		MinSizeThreshold:      10 * 1024 * 1024, // 10MB
		RecursiveThreshold:    0.7,              // 70%
		MaxDepth:              10,
		DistributionThreshold: 0.3, // 30%
		Performance: &sharedEntity.DiskScanPerformanceConfig{
			MaxConcurrency:        4,
			EnableResourceMonitor: true,
			MaxMemoryMB:           512.0,
			ResourceCheckInterval: 1 * time.Second,
			MaxFilesPerDirectory:  10000,
			BatchSize:             100,
		},
	}
}
