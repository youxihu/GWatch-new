package monitoring

import (
	"github.com/youxihu/GWatch-new/internal/domain/collector"
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"github.com/youxihu/GWatch-new/internal/domain/monitoring"
	domainMonitor "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	"github.com/youxihu/GWatch-new/internal/utils"
	"sync"
	"time"
)

// StatefulPolicy 实现基于阈值+持续时间的告警策略
// 使用Redis存储告警状态，支持P1/P2/P3/Reminder四个等级
type StatefulPolicy struct {
	mu                 sync.RWMutex
	lastAlertTimes     map[string]time.Time // 上次告警时间（用于防抖），key是string(AlertType)或AlertType_recovery
	recoveryStartTimes map[string]time.Time // 恢复开始时间记录（key: alertType_pid）

	// 告警状态存储和事件ID生成器
	alertStateStorage monitoring.AlertStateStorage
	eventIDGenerator  *utils.EventIDGenerator
	hostCollector     collector.HostCollector // 用于根据PID获取进程信息
	recoveryChecker   *RecoveryChecker        // 恢复检查器
	diskEnricher      *DiskUsageEnricher      // 磁盘占用信息补充器
	initialized       bool
}

func NewStatefulPolicy() monitoring.Policy {
	return &StatefulPolicy{
		lastAlertTimes:     make(map[string]time.Time),
		recoveryStartTimes: make(map[string]time.Time),
		eventIDGenerator:   utils.NewEventIDGenerator(),
		initialized:        false,
	}
}

// SetAlertStateStorage 设置告警状态存储（用于依赖注入）
func (p *StatefulPolicy) SetAlertStateStorage(storage monitoring.AlertStateStorage) {
	p.alertStateStorage = storage
	// 使用默认配置创建磁盘使用补充器，稍后会通过SetConfig更新
	p.diskEnricher = NewDiskUsageEnricher(storage)

	// 设置事件ID检查器（如果存储实现了EventIDChecker接口）
	if checker, ok := storage.(utils.EventIDChecker); ok {
		p.eventIDGenerator.SetChecker(checker)
	}
}

// SetConfig 设置配置（用于依赖注入）
func (p *StatefulPolicy) SetConfig(config *shared.Config) {
	if config != nil && config.HostMonitoring != nil && config.HostMonitoring.DiskUsageScan != nil {
		// 使用配置文件中的磁盘扫描配置重新创建磁盘使用补充器
		p.diskEnricher = NewDiskUsageEnricherWithConfig(p.alertStateStorage, config.HostMonitoring.DiskUsageScan)
	}
}

// SetHostCollector 设置主机收集器（用于依赖注入）
func (p *StatefulPolicy) SetHostCollector(hostCollector collector.HostCollector) {
	p.hostCollector = hostCollector
	p.recoveryChecker = NewRecoveryChecker(hostCollector)
}

// cleanupStaleProcesses 清理长时间未更新的进程信息
func (p *StatefulPolicy) cleanupStaleProcesses(state *monitoring.AlertState, maxAge time.Duration) bool {
	now := time.Now()
	var activeProcesses []monitoring.ProcessInfo
	removedCount := 0

	for _, proc := range state.Processes {
		if now.Sub(proc.LastSeenTime) <= maxAge {
			activeProcesses = append(activeProcesses, proc)
		} else {
			removedCount++
			logger.Debugf("清理过期进程: PID %s (%s), 最后更新: %v",
				proc.PID, proc.ProcessName, proc.LastSeenTime)
		}
	}

	if removedCount > 0 {
		state.Processes = activeProcesses
		logger.Debugf("清理了 %d 个过期进程，剩余 %d 个活跃进程", removedCount, len(activeProcesses))
		return true
	}

	return false
}

// Apply 应用策略，处理决策并返回告警结果
func (p *StatefulPolicy) Apply(cfg *shared.Config, metrics *entity.SystemMetrics, decisions []domainMonitor.Decision) []domainMonitor.AlertResult {
	return p.applyInternal(cfg, metrics, decisions, false)
}

// PeekApply 预览应用策略，处理决策但不更新状态，返回告警结果
func (p *StatefulPolicy) PeekApply(cfg *shared.Config, metrics *entity.SystemMetrics, decisions []domainMonitor.Decision) []domainMonitor.AlertResult {
	return p.applyInternal(cfg, metrics, decisions, true)
}

// applyInternal 内部实现，支持预览模式
