// 文件日志实现
//
// # FileLogger 基于 zap 框架的文件日志实现
//
// 支持特性：
//   - 文件输出（file 模式）
//   - 文件+控制台输出（both 模式）
//   - 日志轮转（lumberjack）
//   - JSON 格式输出（file 模式）
//   - 控制台格式输出（both/console 模式）
//   - 日志级别过滤（debug/info/warn/error）
//
// 由 LoggerFactory 根据配置自动创建，不需要直接使用
package logger

import (
	"fmt"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"github.com/youxihu/GWatch-new/internal/domain/logger"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// FileLogger 文件日志实现
type FileLogger struct {
	zapLogger *zap.Logger
	config    *shared.LogConfig
}

// NewFileLogger 创建文件日志器
func NewFileLogger(config *shared.LogConfig) (logger.Logger, error) {
	// 确保日志目录存在
	dir := filepath.Dir(config.Output)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %v", err)
	}

	// 解析日志级别
	level := parseLogLevel(config.Level)

	// 创建编码器配置
	var encoder zapcore.Encoder
	if config.Mode == "both" || config.Mode == "console" {
		// both 或 console 模式使用控制台编码器（更易读）
		consoleEncoderConfig := zap.NewDevelopmentEncoderConfig()
		consoleEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		consoleEncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		encoder = zapcore.NewConsoleEncoder(consoleEncoderConfig)
	} else {
		// 文件模式使用 JSON 编码器
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// 创建写入器
	var writeSyncer zapcore.WriteSyncer
	if config.EnableRotation {
		// 使用 lumberjack 进行日志轮转
		lumberJackLogger := &lumberjack.Logger{
			Filename:   config.Output,
			MaxSize:    config.MaxSize,    // MB
			MaxBackups: config.MaxBackups, // 保留的旧日志文件个数
			MaxAge:     config.MaxAge,     // 保留天数
			Compress:   false,             // 是否压缩
		}
		if config.Mode == "both" {
			writeSyncer = zapcore.NewMultiWriteSyncer(
				zapcore.AddSync(lumberJackLogger),
				zapcore.AddSync(os.Stdout),
			)
		} else {
			writeSyncer = zapcore.AddSync(lumberJackLogger)
		}
	} else {
		// 不使用日志轮转
		file, err := os.OpenFile(config.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("打开日志文件失败: %v", err)
		}
		if config.Mode == "both" {
			writeSyncer = zapcore.NewMultiWriteSyncer(
				zapcore.AddSync(file),
				zapcore.AddSync(os.Stdout),
			)
		} else {
			writeSyncer = zapcore.AddSync(file)
		}
	}

	// 创建核心
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// 创建 logger
	// AddCallerSkip(3) 跳过以下调用栈：
	// 1. FileLogger/ConsoleLogger 的实现方法 (Infof/Info等)
	// 2. global.go 中的包装函数 (logger.Infof等)
	// 3. GetGlobalLogger() 调用
	// 最终显示实际调用 logger.Info/Infof 的代码位置
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(3))

	return &FileLogger{
		zapLogger: zapLogger,
		config:    config,
	}, nil
}

// parseLogLevel 解析日志级别字符串为 zapcore.Level
func parseLogLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// Info 输出信息日志
func (l *FileLogger) Info(v ...interface{}) {
	l.zapLogger.Info(fmt.Sprint(v...))
}

// Infof 输出格式化信息日志
func (l *FileLogger) Infof(format string, v ...interface{}) {
	l.zapLogger.Info(fmt.Sprintf(format, v...))
}

// Warn 输出警告日志
func (l *FileLogger) Warn(v ...interface{}) {
	l.zapLogger.Warn(fmt.Sprint(v...))
}

// Warnf 输出格式化警告日志
func (l *FileLogger) Warnf(format string, v ...interface{}) {
	l.zapLogger.Warn(fmt.Sprintf(format, v...))
}

// Error 输出错误日志
func (l *FileLogger) Error(v ...interface{}) {
	l.zapLogger.Error(fmt.Sprint(v...))
}

// Errorf 输出格式化错误日志
func (l *FileLogger) Errorf(format string, v ...interface{}) {
	l.zapLogger.Error(fmt.Sprintf(format, v...))
}

// Debug 输出调试日志
func (l *FileLogger) Debug(v ...interface{}) {
	l.zapLogger.Debug(fmt.Sprint(v...))
}

// Debugf 输出格式化调试日志
func (l *FileLogger) Debugf(format string, v ...interface{}) {
	l.zapLogger.Debug(fmt.Sprintf(format, v...))
}

// Sync 同步日志（zap 特有方法，用于确保所有日志都被写入）
func (l *FileLogger) Sync() error {
	return l.zapLogger.Sync()
}
