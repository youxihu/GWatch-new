// 全局日志入口
//
// 全局日志接口
//
// 使用方式：
//  1. 在代码文件顶部导入：logger "github.com/youxihu/GWatch-new/internal/infra/logger"
//  2. 直接调用全局函数：
//     - logger.Info("消息")           // 信息日志
//     - logger.Infof("格式: %s", v)   // 格式化信息日志
//     - logger.Warn("警告")           // 警告日志
//     - logger.Warnf("警告: %s", v)   // 格式化警告日志
//     - logger.Error("错误")          // 错误日志
//     - logger.Errorf("错误: %v", err) // 格式化错误日志
//     - logger.Debug("调试")          // 调试日志
//     - logger.Debugf("调试: %s", v)  // 格式化调试日志
//
// 日志基于 zap 框架实现，支持以下日志级别（由低到高）：
//   - Debug: 调试信息
//   - Info:  一般信息
//   - Warn:  警告信息
//   - Error: 错误信息
//
// 全局 logger 需要在 main 函数中通过 logger.InitLogWrapper() 初始化
package logger

import (
	"sync"

	"github.com/youxihu/GWatch-new/internal/domain/logger"
)

// globalLogger 全局 logger 实例（基于 zap 框架）
var (
	globalLogger logger.Logger
	loggerMu     sync.RWMutex
)

// InitGlobalLogger 初始化全局 logger
func InitGlobalLogger(l logger.Logger) {
	loggerMu.Lock()
	globalLogger = l
	loggerMu.Unlock()
}

// GetGlobalLogger 获取全局 logger 实例
func GetGlobalLogger() logger.Logger {
	loggerMu.RLock()
	l := globalLogger
	loggerMu.RUnlock()
	if l == nil {
		return NewConsoleLogger()
	}
	return l
}

// Info 记录信息级别日志
// 示例: logger.Info("服务启动成功")
func Info(v ...interface{}) {
	GetGlobalLogger().Info(v...)
}

// Infof 记录格式化信息级别日志
// 示例: logger.Infof("处理了 %d 条记录", count)
func Infof(format string, v ...interface{}) {
	GetGlobalLogger().Infof(format, v...)
}

// Warn 记录警告级别日志
// 示例: logger.Warn("配置项缺失，使用默认值")
func Warn(v ...interface{}) {
	GetGlobalLogger().Warn(v...)
}

// Warnf 记录格式化警告级别日志
// 示例: logger.Warnf("连接超时: %v", err)
func Warnf(format string, v ...interface{}) {
	GetGlobalLogger().Warnf(format, v...)
}

// Error 记录错误级别日志
// 示例: logger.Error("数据库连接失败")
func Error(v ...interface{}) {
	GetGlobalLogger().Error(v...)
}

// Errorf 记录格式化错误级别日志
// 示例: logger.Errorf("操作失败: %v", err)
func Errorf(format string, v ...interface{}) {
	GetGlobalLogger().Errorf(format, v...)
}

// Debug 记录调试级别日志
// 示例: logger.Debug("进入处理函数")
func Debug(v ...interface{}) {
	GetGlobalLogger().Debug(v...)
}

// Debugf 记录格式化调试级别日志
// 示例: logger.Debugf("变量值: %+v", data)
func Debugf(format string, v ...interface{}) {
	GetGlobalLogger().Debugf(format, v...)
}
