// 日志服务用例
package logger

import (
	"github.com/youxihu/GWatch-new/internal/domain/logger"
)

// LoggerService 日志服务用例
type LoggerService struct {
	logger logger.Logger
}

// NewLoggerService 创建日志服务
func NewLoggerService(logger logger.Logger) *LoggerService {
	return &LoggerService{
		logger: logger,
	}
}

// GetLogger 获取日志器实例
func (s *LoggerService) GetLogger() logger.Logger {
	return s.logger
}
