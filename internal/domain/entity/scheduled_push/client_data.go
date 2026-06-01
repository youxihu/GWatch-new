// Package entity 领域实体 - 定时推送客户端数据
package entity

import (
	"fmt"
	"time"

	// 导入监控领域的实体类型
	monitoringEntity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
)

// ClientMonitorData 客户端上报的监控数据
type ClientMonitorData struct {
	// 客户端标识
	HostIP   string `json:"host_ip"`
	HostName string `json:"host_name"`

	// 客户端标题（从配置中读取，用于在聚合报告中显示）
	Title string `json:"title,omitempty"`

	// 时间戳
	Timestamp time.Time `json:"timestamp"`

	// 监控指标
	Metrics *ClientMetrics `json:"metrics"`
}

// ClientMetrics 客户端监控指标（包含主机监控和应用监控）
type ClientMetrics struct {
	// 主机监控（必须）
	CPU     *monitoringEntity.CPUMetrics     `json:"cpu,omitempty"`
	Memory  *monitoringEntity.MemoryMetrics  `json:"memory,omitempty"`
	Disk    *monitoringEntity.DiskMetrics    `json:"disk,omitempty"`
	Network *monitoringEntity.NetworkMetrics `json:"network,omitempty"`

	// 应用监控（可选，根据配置决定是否包含）
	Redis *monitoringEntity.RedisMetrics `json:"redis,omitempty"`
	HTTP  *monitoringEntity.HTTPMetrics  `json:"http,omitempty"`
}

// ClientDataKey 生成 Redis key 的辅助函数
func ClientDataKey(hostIP string, timestamp time.Time) string {
	return fmt.Sprintf("gwatch:client:%s:%d", hostIP, timestamp.Unix())
}
