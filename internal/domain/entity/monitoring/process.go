// Package entity 领域实体 - 监控进程信息
package entity

// 进程资源使用快照
type ProcessInfo struct {
	PID        int32
	Name       string
	CPUPercent float64
	MemPercent float32
	MemRSS     uint64 // MB
}
