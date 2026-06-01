// 日志初始化入口
//
// 日志初始化入口
//
// InitLogWrapper 用于初始化全局 logger，所有代码文件应该直接使用 logger 包的全局函数
// 不再拦截标准库 log，所有日志调用必须显式使用 logger.Info/Error/Warn/Debug 等方法
package logger

import (
	"github.com/youxihu/GWatch-new/internal/domain/logger"
)

// InitLogWrapper 初始化全局日志系统
//
// 参数:
//
//	l: 已配置的 logger 实例（通常由 LoggerFactory 创建）
//
// 初始化后，所有代码可以直接使用以下全局函数进行日志记录：
//   - logger.Info() / logger.Infof()   // 信息日志
//   - logger.Warn() / logger.Warnf()   // 警告日志
//   - logger.Error() / logger.Errorf() // 错误日志
//   - logger.Debug() / logger.Debugf() // 调试日志
//
// 示例:
//
//	logger.InitLogWrapper(app.LoggerService.GetLogger())
//	logger.Info("系统启动")
//	logger.Infof("处理了 %d 条记录", count)
func InitLogWrapper(l logger.Logger) {
	// 初始化全局 logger
	InitGlobalLogger(l)
	// 注意：不再拦截标准库 log，所有代码必须直接使用 logger.Info/Error/Warn/Debug 等全局函数
}
