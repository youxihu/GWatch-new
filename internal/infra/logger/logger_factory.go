// 日志工厂
package logger

import (
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"github.com/youxihu/GWatch-new/internal/domain/logger"
)

// LoggerFactory 日志工厂实现
type LoggerFactory struct {
	config *shared.LogConfig
}

// NewLoggerFactory 创建日志工厂
func NewLoggerFactory(config *shared.LogConfig) logger.LoggerFactory {
	return &LoggerFactory{
		config: config,
	}
}

// CreateLogger 创建日志器实例
func (f *LoggerFactory) CreateLogger() (logger.Logger, error) {
	switch f.config.Mode {
	case "file":
		return NewFileLogger(f.config)
	case "console":
		return NewConsoleLogger(), nil
	case "both":
		return NewFileLogger(f.config)
	default:
		// 默认使用控制台日志
		return NewConsoleLogger(), nil
	}
}
