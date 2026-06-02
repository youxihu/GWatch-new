// cmd/main.go
package main

import (
	"flag"
	"os"

	"github.com/youxihu/GWatch-new/internal/infra/logger"
	"github.com/youxihu/GWatch-new/internal/utils"
)

func main() {
	// 解析命令行参数
	var configPath string
	var showVersion bool
	flag.StringVar(&configPath, "config", "", "配置文件路径（优先级高于环境变量 GWATCH_CONFIG）")
	flag.StringVar(&configPath, "c", "", "配置文件路径（-config 的简写）")
	flag.BoolVar(&showVersion, "version", false, "显示版本信息")
	flag.BoolVar(&showVersion, "v", false, "显示版本信息（简写）")
	flag.Parse()

	// 如果请求显示版本信息，直接打印并退出
	if showVersion {
		utils.PrintVersion()
		os.Exit(0)
	}

	// 如果通过命令行参数指定了配置文件，提前设置到环境变量中
	// 必须在 InitializeApp() 之前执行，因为 Wire 的 NewConfigProvider 会读取该环境变量
	if configPath != "" {
		os.Setenv("GWATCH_CONFIG", configPath)
	}

	// 1. 使用 Wire 进行依赖注入（需要先初始化，因为需要 logger）
	app, err := InitializeApp()
	if err != nil {
		// 如果初始化失败，使用临时控制台 logger
		tempLogger := logger.NewConsoleLogger()
		tempLogger.Errorf("初始化应用程序失败: %v", err)
		return
	}

	// 2. 初始化全局日志系统
	// 初始化后，所有代码可以直接使用 logger.Info/Error/Warn/Debug 等全局函数进行日志记录
	logger.InitLogWrapper(app.LoggerService.GetLogger())

	if configPath != "" {
		logger.Infof("使用配置文件: %s", configPath)
	}

	logger.Infof("GWatch 服务器监控工具启动 (%s)", utils.FormatVersion())
	logger.Info("正在初始化...")

	// 3. 启动应用程序
	if err := app.Start(); err != nil {
		logger.Errorf("应用程序运行失败: %v", err)
		return
	}
}
