// 控制台日志实现
//
// # ConsoleLogger 基于 zap 框架的控制台日志实现
//
// 特点：
//   - 彩色输出（不同级别使用不同颜色）
//   - 易读格式（开发模式编码器）
//   - 包含调用者信息（文件名和行号）
//
// 由 LoggerFactory 根据配置自动创建，也可以作为未初始化时的后备 logger
package logger

import (
	"github.com/youxihu/GWatch-new/internal/domain/logger"
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ConsoleLogger 控制台日志实现
type ConsoleLogger struct {
	zapLogger *zap.Logger
}

// NewConsoleLogger 创建控制台日志器
func NewConsoleLogger() logger.Logger {
	// 创建开发模式的编码器配置（更易读）
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	// 创建控制台编码器
	encoder := zapcore.NewConsoleEncoder(encoderConfig)

	// 创建写入器（控制台输出）
	writeSyncer := zapcore.AddSync(os.Stdout)

	// 创建核心（默认 INFO 级别）
	core := zapcore.NewCore(encoder, writeSyncer, zapcore.InfoLevel)

	// 创建 logger
	// AddCallerSkip(3) 跳过以下调用栈：
	// 1. FileLogger/ConsoleLogger 的实现方法 (Infof/Info等)
	// 2. global.go 中的包装函数 (logger.Infof等)
	// 3. GetGlobalLogger() 调用
	// 最终显示实际调用 logger.Info/Infof 的代码位置
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(3))

	return &ConsoleLogger{
		zapLogger: zapLogger,
	}
}

// Info 输出信息日志
func (l *ConsoleLogger) Info(v ...interface{}) {
	l.zapLogger.Info(fmt.Sprint(v...))
}

// Infof 输出格式化信息日志
func (l *ConsoleLogger) Infof(format string, v ...interface{}) {
	l.zapLogger.Info(fmt.Sprintf(format, v...))
}

// Warn 输出警告日志
func (l *ConsoleLogger) Warn(v ...interface{}) {
	l.zapLogger.Warn(fmt.Sprint(v...))
}

// Warnf 输出格式化警告日志
func (l *ConsoleLogger) Warnf(format string, v ...interface{}) {
	l.zapLogger.Warn(fmt.Sprintf(format, v...))
}

// Error 输出错误日志
func (l *ConsoleLogger) Error(v ...interface{}) {
	l.zapLogger.Error(fmt.Sprint(v...))
}

// Errorf 输出格式化错误日志
func (l *ConsoleLogger) Errorf(format string, v ...interface{}) {
	l.zapLogger.Error(fmt.Sprintf(format, v...))
}

// Debug 输出调试日志
func (l *ConsoleLogger) Debug(v ...interface{}) {
	l.zapLogger.Debug(fmt.Sprint(v...))
}

// Debugf 输出格式化调试日志
func (l *ConsoleLogger) Debugf(format string, v ...interface{}) {
	l.zapLogger.Debug(fmt.Sprintf(format, v...))
}

// Sync 同步日志（zap 特有方法，用于确保所有日志都被写入）
func (l *ConsoleLogger) Sync() error {
	return l.zapLogger.Sync()
}
