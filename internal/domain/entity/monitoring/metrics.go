// Package entity 领域实体 - 监控指标（过渡期保留）
package entity

import "time"

// SystemMetrics 系统监控指标结构
type SystemMetrics struct {
	Timestamp time.Time
	CPU       CPUMetrics
	Memory    MemoryMetrics
	Disk      DiskMetrics
	Network   NetworkMetrics
	Redis     RedisMetrics
	HTTP      HTTPMetrics
}

// CPUMetrics CPU指标
type CPUMetrics struct {
	Percent float64
	Cores   int // CPU核心数（逻辑核心数）
	Error   error
}

// MemoryMetrics 内存指标
type MemoryMetrics struct {
	Percent float64
	UsedMB  uint64
	TotalMB uint64
	Error   error
}

// DiskMetrics 磁盘指标
type DiskMetrics struct {
	Percent   float64
	UsedGB    uint64
	TotalGB   uint64
	ReadKBps  float64
	WriteKBps float64
	Error     error
}

// NetworkMetrics 网络指标
type NetworkMetrics struct {
	DownloadKBps float64
	UploadKBps   float64
	Error        error
}

// RedisMetrics Redis指标
type RedisMetrics struct {
	ClientCount     int
	ClientDetails   []ClientInfo
	ConnectionError error
	DetailError     error
}

// ClientInfo Redis客户端信息
type ClientInfo struct {
	ID    string
	Addr  string
	Age   string
	Idle  string
	Flags string
	Db    string
	Cmd   string
}

// HTTPMetrics HTTP接口监控指标
type HTTPMetrics struct {
	Interfaces []HTTPInterfaceMetrics
	Error      error
}

// HTTPInterfaceMetrics 单个HTTP接口监控指标
type HTTPInterfaceMetrics struct {
	Name         string
	URL          string
	IsAccessible bool
	ResponseTime time.Duration
	StatusCode   int
	Error        error
	NeedAlert    bool
	AllowedCodes []int
}

// ScheduledPushMetrics 全局定时推送指标
type ScheduledPushMetrics struct {
	Timestamp   time.Time      `json:"timestamp"`
	HostMetrics *SystemMetrics `json:"host_metrics,omitempty"`
	AppMetrics  *AppMetrics    `json:"app_metrics,omitempty"`
}

// AppMetrics 应用监控指标
type AppMetrics struct {
	Redis *RedisMetrics `json:"redis,omitempty"`
	HTTP  *HTTPMetrics  `json:"http,omitempty"`
}
