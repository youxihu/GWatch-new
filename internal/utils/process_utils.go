package utils

import (
	"fmt"
	"os"

	"github.com/youxihu/GWatch-new/internal/domain/collector"
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
)

// selfPID 当前进程PID，用于排除自身
var selfPID = int32(os.Getpid())

// GetTopProcessInfo 获取Top进程的PID和名称（用于CPU或内存告警）
func GetTopProcessInfo(hostCollector collector.HostCollector, alertType entity.AlertType) (string, string) {
	if hostCollector == nil {
		return "default", ""
	}

	topCPUProcesses, topMemProcesses, err := hostCollector.GetTopProcesses(5)
	if err != nil {
		return "default", ""
	}

	var processes []entity.ProcessInfo
	switch alertType {
	case entity.CPUHigh:
		processes = topCPUProcesses
	case entity.MemHigh:
		processes = topMemProcesses
	}

	for _, process := range processes {
		if process.PID != selfPID {
			return fmt.Sprintf("%d", process.PID), process.Name
		}
	}

	if len(processes) > 0 {
		topProcess := processes[0]
		return fmt.Sprintf("%d", topProcess.PID), topProcess.Name
	}

	return "default", ""
}

// GetTopProcessByType 根据告警类型获取Top N进程中的第一个非自身进程
func GetTopProcessByType(hostCollector collector.HostCollector, alertType entity.AlertType, n int) (*entity.ProcessInfo, error) {
	if hostCollector == nil {
		return nil, fmt.Errorf("hostCollector is nil")
	}

	topCPUProcesses, topMemProcesses, err := hostCollector.GetTopProcesses(n)
	if err != nil {
		return nil, err
	}

	var processes []entity.ProcessInfo
	switch alertType {
	case entity.CPUHigh:
		processes = topCPUProcesses
	case entity.MemHigh:
		processes = topMemProcesses
	default:
		return nil, fmt.Errorf("unsupported alert type: %s", alertType)
	}

	for i := range processes {
		if processes[i].PID != selfPID {
			return &processes[i], nil
		}
	}

	if len(processes) > 0 {
		return &processes[0], nil
	}

	return nil, fmt.Errorf("no processes found")
}

// FindProcessByPID 在进程列表中根据PID查找进程
func FindProcessByPID(hostCollector collector.HostCollector, alertType entity.AlertType, pid int32, n int) (*entity.ProcessInfo, error) {
	if hostCollector == nil {
		return nil, fmt.Errorf("hostCollector is nil")
	}

	topCPUProcesses, topMemProcesses, err := hostCollector.GetTopProcesses(n)
	if err != nil {
		return nil, err
	}

	var processes []entity.ProcessInfo
	switch alertType {
	case entity.CPUHigh:
		processes = topCPUProcesses
	case entity.MemHigh:
		processes = topMemProcesses
	default:
		return nil, fmt.Errorf("unsupported alert type: %s", alertType)
	}

	for i := range processes {
		if processes[i].PID == pid {
			return &processes[i], nil
		}
	}

	return nil, fmt.Errorf("process with PID %d not found", pid)
}

// GetProcessValue 根据告警类型获取进程的对应指标值
func GetProcessValue(process *entity.ProcessInfo, alertType entity.AlertType) (float64, error) {
	if process == nil {
		return 0, fmt.Errorf("process is nil")
	}

	switch alertType {
	case entity.CPUHigh:
		return process.CPUPercent, nil
	case entity.MemHigh:
		return float64(process.MemPercent), nil
	default:
		return 0, fmt.Errorf("unsupported alert type: %s", alertType)
	}
}
