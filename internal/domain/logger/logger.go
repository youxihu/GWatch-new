// 日志领域接口
package logger

// Logger 日志领域服务接口
type Logger interface {
	// Info 输出信息日志
	Info(v ...interface{})

	// Infof 输出格式化信息日志
	Infof(format string, v ...interface{})

	// Warn 输出警告日志
	Warn(v ...interface{})

	// Warnf 输出格式化警告日志
	Warnf(format string, v ...interface{})

	// Error 输出错误日志
	Error(v ...interface{})

	// Errorf 输出格式化错误日志
	Errorf(format string, v ...interface{})

	// Debug 输出调试日志
	Debug(v ...interface{})

	// Debugf 输出格式化调试日志
	Debugf(format string, v ...interface{})
}

// LoggerFactory 日志工厂接口
type LoggerFactory interface {
	// CreateLogger 创建日志器实例
	CreateLogger() (Logger, error)
}
