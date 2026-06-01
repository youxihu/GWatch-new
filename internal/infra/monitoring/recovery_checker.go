package monitoring

import (
	"github.com/youxihu/GWatch-new/internal/domain/collector"
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	"github.com/youxihu/GWatch-new/internal/utils"
	"strconv"
)

// RecoveryChecker 恢复检查器
type RecoveryChecker struct {
	hostCollector collector.HostCollector
}

// NewRecoveryChecker 创建恢复检查器
func NewRecoveryChecker(hostCollector collector.HostCollector) *RecoveryChecker {
	return &RecoveryChecker{hostCollector: hostCollector}
}

// CheckRecovery 检查是否恢复（仅检查当前值）
func CheckRecovery(alertType entity.AlertType, currentValue, base float64) bool {
	if alertType == entity.RedisLow {
		return currentValue >= base
	}
	return currentValue < base
}

// CheckHTTPRecovery 检查HTTP告警是否恢复
func CheckHTTPRecovery(currentValue float64, allowedCodes []int) bool {
	return utils.IsValidHTTPStatusCode(int(currentValue), allowedCodes)
}

// CheckProcessRecovery 检查Redis中记录的PID对应的进程是否恢复
func (c *RecoveryChecker) CheckProcessRecovery(alertType entity.AlertType, pid string, base float64) (recovered, exists bool, value float64) {
	if c.hostCollector == nil || pid == "default" {
		return true, false, 0
	}

	pidInt, err := strconv.ParseInt(pid, 10, 32)
	if err != nil {
		logger.Debugf("[进程恢复检查] %s PID %s 解析失败: %v", alertType, pid, err)
		return true, false, 0
	}

	// 使用统一的函数查找进程
	processInfo, err := utils.FindProcessByPID(c.hostCollector, alertType, int32(pidInt), 100)
	if err != nil {
		logger.Debugf("[进程恢复检查] %s PID %s 对应的进程不存在: %v", alertType, pid, err)
		return true, false, 0
	}

	// 使用统一的函数获取进程值
	processValue, err := utils.GetProcessValue(processInfo, alertType)
	if err != nil {
		logger.Debugf("[进程恢复检查] %s 获取进程值失败: %v", alertType, err)
		return true, true, 0
	}

	return processValue < base, true, processValue
}
