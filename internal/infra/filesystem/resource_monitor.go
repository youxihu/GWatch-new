package filesystem

import (
	"runtime"
	"sync"
	"time"
)

// 资源监控器实现
type ResourceMonitor struct {
	maxMemoryMB    float64
	checkInterval  time.Duration
	stopChan       chan struct{}
	memoryExceeded bool
	stopped        bool
	mu             sync.RWMutex
}

// NewResourceMonitor 创建资源监控器
func NewResourceMonitor(maxMemoryMB float64, checkInterval time.Duration) *ResourceMonitor {
	return &ResourceMonitor{
		maxMemoryMB:   maxMemoryMB,
		checkInterval: checkInterval,
		stopChan:      make(chan struct{}),
	}
}

// Start 启动资源监控
func (rm *ResourceMonitor) Start() {
	go func() {
		ticker := time.NewTicker(rm.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				rm.checkMemoryUsage()
			case <-rm.stopChan:
				return
			}
		}
	}()
}

// Stop 停止资源监控
func (rm *ResourceMonitor) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.stopped {
		close(rm.stopChan)
		rm.stopped = true
	}
}

// IsMemoryExceeded 检查是否超过内存限制
func (rm *ResourceMonitor) IsMemoryExceeded() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.memoryExceeded
}

// GetCurrentMemoryUsage 获取当前内存使用量
func (rm *ResourceMonitor) GetCurrentMemoryUsage() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024 / 1024
}

// checkMemoryUsage 检查内存使用情况
func (rm *ResourceMonitor) checkMemoryUsage() {
	currentMemoryMB := rm.GetCurrentMemoryUsage()

	rm.mu.Lock()
	rm.memoryExceeded = currentMemoryMB > rm.maxMemoryMB
	rm.mu.Unlock()
}
