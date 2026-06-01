package collector

import (
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	"time"
)

// HostCollector defines capabilities for collecting host-level metrics.
type HostCollector interface {
	GetCPUPercent() (float64, error)
	GetCPUInfo() (float64, int, error) // 获取CPU使用率和核心数
	GetMemoryUsage() (float64, uint64, uint64, error)
	GetDiskUsage() (float64, uint64, uint64, error)
	GetDiskIORate() (float64, float64, error)
	GetNetworkRate() (float64, float64, error)
	// 获取 CPU 和内存占用最高的前 N 个进程
	GetTopProcesses(n int) ([]entity.ProcessInfo, []entity.ProcessInfo, error)
}

// RedisCollector defines capabilities for collecting Redis service metrics.
type RedisCollector interface {
	// 根据全局配置初始化底层连接
	Init() error
	// 返回客户端连接数（尽可能排除自身）
	GetClients() (int, error)
	// 返回详细客户端列表（尽可能排除自身）
	GetClientsDetail() ([]entity.ClientInfo, error)
	// 释放资源
	Close()
}

// HTTPCollector defines capabilities for collecting HTTP interface monitoring metrics.
type HTTPCollector interface {
	// 根据全局配置初始化 HTTP 客户端
	Init() error
	// CheckInterface checks if a specific HTTP interface is accessible and returns response time and status code.
	CheckInterface(url string, timeout time.Duration) (bool, time.Duration, int, error)
	// 释放资源
	Close()
}
