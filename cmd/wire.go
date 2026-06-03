//go:build wireinject
// +build wireinject

package main

import (
	"os"
	"os/signal"
	"syscall"

	usecaseBase "github.com/youxihu/GWatch-new/internal/app/usecase"
	usecaseLogger "github.com/youxihu/GWatch-new/internal/app/usecase/logger"
	usecaseMonitoring "github.com/youxihu/GWatch-new/internal/app/usecase/monitoring"
	usecaseScheduledPush "github.com/youxihu/GWatch-new/internal/app/usecase/scheduled_push"
	usecaseScheduler "github.com/youxihu/GWatch-new/internal/app/usecase/scheduler"
	usecaseSecurityMonitoring "github.com/youxihu/GWatch-new/internal/app/usecase/security_monitoring"
	"github.com/youxihu/GWatch-new/internal/domain/collector"
	"github.com/youxihu/GWatch-new/internal/domain/config"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	domainLogger "github.com/youxihu/GWatch-new/internal/domain/logger"
	"github.com/youxihu/GWatch-new/internal/domain/monitoring"
	"github.com/youxihu/GWatch-new/internal/domain/scheduled_push"
	"github.com/youxihu/GWatch-new/internal/domain/scheduled_push/client"
	"github.com/youxihu/GWatch-new/internal/domain/scheduled_push/common"
	"github.com/youxihu/GWatch-new/internal/domain/scheduled_push/server"

	// infra 实现
	redisCollector "github.com/youxihu/GWatch-new/internal/infra/collector/external"
	hostCollector "github.com/youxihu/GWatch-new/internal/infra/collector/host"
	configimpl "github.com/youxihu/GWatch-new/internal/infra/config"
	loggerImpl "github.com/youxihu/GWatch-new/internal/infra/logger"
	monitoringImpl "github.com/youxihu/GWatch-new/internal/infra/monitoring"
	scheduledPushCommon "github.com/youxihu/GWatch-new/internal/infra/scheduled_push/common"

	"github.com/google/wire"
)

// BasePolicy 基础告警策略类型别名
type BasePolicy *monitoringImpl.StatefulPolicy

// HTTPPolicy HTTP告警策略类型别名
type HTTPPolicy *monitoringImpl.StatefulPolicy

// BaseMonitoringUseCase 基础监控用例类型别名
type BaseMonitoringUseCase *usecaseMonitoring.MonitoringUseCase

// HTTPMonitoringUseCase HTTP监控用例类型别名
type HTTPMonitoringUseCase *usecaseMonitoring.MonitoringUseCase

// ProviderSet 定义所有基础设施提供者
var ProviderSet = wire.NewSet(
	// 配置提供者
	NewConfigProvider,
	NewConfig,

	// 收集器提供者
	NewHostCollector,
	NewRedisCollector,
	NewHTTPCollector,
	// 评估器和格式化器提供者
	NewEvaluator,
	NewMarkdownFormatter,

	// 通知器提供者
	NewDingTalkNotifier,

	// 告警日志存储提供者
	NewAlertLogStorage,

	// 告警状态存储提供者
	NewAlertStateStorage,

	// 告警策略提供者
	NewBasePolicy,
	NewHTTPPolicy,

	// 系统指标服务提供者
	NewSystemMetricsService,

	// 监控用例提供者
	NewBaseMonitoringUseCase,
	NewHTTPMonitoringUseCase,
	NewSecurityMonitoringService,

	NewClientDataRepository,
	NewScheduledPushFormatter,
	NewDataLogStorage,
	NewMetricsCollector,
	NewClientUseCase,
	NewServerUseCase,
	NewCertificateFetcher,
)

// NewConfigProvider 创建配置提供者
func NewConfigProvider() (config.Provider, error) {
	// 配置文件路径优先级：
	// 1. 命令行参数 -config 或 -c（在 main.go 中会设置到环境变量 GWATCH_CONFIG）
	// 2. 环境变量 GWATCH_CONFIG
	// 3. 默认值 config/config.yml
	configPath := os.Getenv("GWATCH_CONFIG")
	if configPath == "" {
		configPath = "config/config.yml"
	}
	return configimpl.NewYAMLProvider(configPath)
}

// NewHostCollector 创建主机信息收集器
func NewHostCollector() collector.HostCollector {
	return hostCollector.New()
}

// NewRedisCollector 创建 Redis 收集器
func NewRedisCollector(provider config.Provider) usecaseBase.RedisClient {
	return redisCollector.NewRedisCollector(provider)
}

// NewHTTPCollector 创建 HTTP 收集器
func NewHTTPCollector(provider config.Provider) collector.HTTPCollector {
	return redisCollector.NewHTTPCollector(provider)
}

// NewEvaluator 创建评估器
func NewEvaluator(hostCollector collector.HostCollector) monitoring.Evaluator {
	evaluator := monitoringImpl.NewSimpleEvaluator()
	evaluator.SetHostCollector(hostCollector)
	return evaluator
}

// NewMarkdownFormatter 创建 Markdown 格式化器
func NewMarkdownFormatter() monitoring.Formatter {
	return monitoringImpl.NewMarkdownFormatter()
}

// NewDingTalkNotifier 创建钉钉通知器
func NewDingTalkNotifier(provider config.Provider) monitoring.Notifier {
	return monitoringImpl.NewDingTalkNotifier(provider)
}

// NewAlertLogStorage 创建告警日志存储服务
func NewAlertLogStorage() monitoring.AlertLogStorage {
	return monitoringImpl.NewAlertLogStorage()
}

// NewAlertStateStorage 创建告警状态存储服务
func NewAlertStateStorage() monitoring.AlertStateStorage {
	return monitoringImpl.NewAlertStateStorage()
}

// createStatefulPolicy 创建告警策略（公共方法）
func createStatefulPolicy(alertStateStorage monitoring.AlertStateStorage, hostCollector collector.HostCollector, config *shared.Config) *monitoringImpl.StatefulPolicy {
	policy := monitoringImpl.NewStatefulPolicy().(*monitoringImpl.StatefulPolicy)
	policy.SetAlertStateStorage(alertStateStorage)
	policy.SetHostCollector(hostCollector)
	policy.SetConfig(config)
	return policy
}

// NewBasePolicy 创建基础告警策略
func NewBasePolicy(alertStateStorage monitoring.AlertStateStorage, hostCollector collector.HostCollector, config *shared.Config) BasePolicy {
	return createStatefulPolicy(alertStateStorage, hostCollector, config)
}

// NewHTTPPolicy 创建 HTTP 告警策略
func NewHTTPPolicy(alertStateStorage monitoring.AlertStateStorage, hostCollector collector.HostCollector, config *shared.Config) HTTPPolicy {
	return createStatefulPolicy(alertStateStorage, hostCollector, config)
}

// InitializeApp 初始化应用程序的所有依赖
func InitializeApp() (*App, error) {
	wire.Build(
		ProviderSet,
		NewCoordinator,
		NewScheduledPushUseCase,
		NewScheduledPushScheduler,
		NewLoggerFactory,
		NewLogger,
		NewLoggerService,
		NewApp,
	)
	return &App{}, nil
}

// NewBaseMonitoringUseCase 创建基础监控用例
func NewBaseMonitoringUseCase(
	hostInfo collector.HostCollector,
	redisInfo usecaseBase.RedisClient,
	httpInfo collector.HTTPCollector,
	evaluator monitoring.Evaluator,
	policy BasePolicy,
	formatter monitoring.Formatter,
	notifier monitoring.Notifier,
	alertLogStorage monitoring.AlertLogStorage,
	log domainLogger.Logger,
) BaseMonitoringUseCase {
	return usecaseMonitoring.NewMonitoringUseCase(
		hostInfo,
		redisInfo,
		httpInfo,
		evaluator,
		(*monitoringImpl.StatefulPolicy)(policy),
		formatter,
		notifier,
		alertLogStorage,
		log,
	)
}

// NewHTTPMonitoringUseCase 创建HTTP监控用例
func NewHTTPMonitoringUseCase(
	hostInfo collector.HostCollector,
	redisInfo usecaseBase.RedisClient,
	httpInfo collector.HTTPCollector,
	evaluator monitoring.Evaluator,
	policy HTTPPolicy,
	formatter monitoring.Formatter,
	notifier monitoring.Notifier,
	alertLogStorage monitoring.AlertLogStorage,
	log domainLogger.Logger,
) HTTPMonitoringUseCase {
	return usecaseMonitoring.NewMonitoringUseCase(
		hostInfo,
		redisInfo,
		httpInfo,
		evaluator,
		(*monitoringImpl.StatefulPolicy)(policy),
		formatter,
		notifier,
		alertLogStorage,
		log,
	)
}

// NewSystemMetricsService 创建系统指标服务
func NewSystemMetricsService(
	hostInfo collector.HostCollector,
	redisInfo usecaseBase.RedisClient,
	httpInfo collector.HTTPCollector,
) *usecaseMonitoring.SystemMetricsService {
	return usecaseMonitoring.NewSystemMetricsService(hostInfo, redisInfo, httpInfo)
}

// NewCoordinator 创建协调器
func NewCoordinator(
	runnerBase BaseMonitoringUseCase,
	runnerHTTP HTTPMonitoringUseCase,
	policyBase BasePolicy,
	policyHTTP HTTPPolicy,
) *usecaseMonitoring.Coordinator {
	return usecaseMonitoring.NewCoordinator(
		(*usecaseMonitoring.MonitoringUseCase)(runnerBase),
		(*usecaseMonitoring.MonitoringUseCase)(runnerHTTP),
		(*monitoringImpl.StatefulPolicy)(policyBase),
		(*monitoringImpl.StatefulPolicy)(policyHTTP),
	)
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector(
	hostCollector collector.HostCollector,
	redisClient usecaseBase.RedisClient,
	httpCollector collector.HTTPCollector,
) *usecaseScheduledPush.MetricsCollector {
	return usecaseScheduledPush.NewMetricsCollector(hostCollector, redisClient, httpCollector)
}

// NewDataLogStorage 创建数据日志存储服务
func NewDataLogStorage() common.ScheduledPushDataLogStorage {
	return scheduledPushCommon.NewScheduledPushDataLogStorage()
}

// NewClientUseCase 创建客户端用例
func NewClientUseCase(
	metricsCollector *usecaseScheduledPush.MetricsCollector,
	clientDataRepository common.ClientDataRepository,
	dataLogStorage common.ScheduledPushDataLogStorage,
) client.ClientUseCase {
	return usecaseScheduledPush.NewClientUseCase(metricsCollector, clientDataRepository, dataLogStorage)
}

// NewServerUseCase 创建服务端用例
func NewServerUseCase(
	metricsCollector *usecaseScheduledPush.MetricsCollector,
	clientDataRepository common.ClientDataRepository,
	scheduledPushFormatter common.ScheduledPushFormatter,
	notifier monitoring.Notifier,
	dataLogStorage common.ScheduledPushDataLogStorage,
) server.ServerUseCase {
	return usecaseScheduledPush.NewServerUseCase(metricsCollector, clientDataRepository, scheduledPushFormatter, notifier, dataLogStorage)
}

// NewScheduledPushUseCase 创建全局定时推送用例
func NewScheduledPushUseCase(
	clientUseCase client.ClientUseCase,
	serverUseCase server.ServerUseCase,
) scheduled_push.ScheduledPushUseCase {
	return usecaseScheduledPush.NewScheduledPushUseCase(clientUseCase, serverUseCase)
}

// NewClientDataRepository 创建客户端数据仓库
func NewClientDataRepository() common.ClientDataRepository {
	return scheduledPushCommon.NewClientDataRepository()
}

// NewScheduledPushFormatter 创建定时推送格式化器
func NewScheduledPushFormatter() common.ScheduledPushFormatter {
	return scheduledPushCommon.NewScheduledPushFormatter()
}

// NewScheduledPushScheduler 创建全局定时推送调度器
func NewScheduledPushScheduler(scheduledPushUseCase scheduled_push.ScheduledPushUseCase) scheduled_push.ScheduledPushScheduler {
	return usecaseScheduler.NewScheduledPushScheduler(scheduledPushUseCase)
}

// NewLoggerFactory 创建日志工厂
func NewLoggerFactory(config *shared.Config) domainLogger.LoggerFactory {
	return loggerImpl.NewLoggerFactory(&config.Log)
}

// NewLogger 创建日志器
func NewLogger(factory domainLogger.LoggerFactory) domainLogger.Logger {
	l, err := factory.CreateLogger()
	if err != nil {
		// 如果创建失败，返回控制台日志器
		return loggerImpl.NewConsoleLogger()
	}
	return l
}

// NewLoggerService 创建日志服务
func NewLoggerService(l domainLogger.Logger) *usecaseLogger.LoggerService {
	return usecaseLogger.NewLoggerService(l)
}

// App 应用程序结构体，包含所有需要的组件
type App struct {
	Config                 *shared.Config
	Coordinator            *usecaseMonitoring.Coordinator
	ScheduledPushScheduler scheduled_push.ScheduledPushScheduler
	LoggerService          *usecaseLogger.LoggerService
	AlertStateStorage      monitoring.AlertStateStorage // 告警状态存储（用于退出时关闭连接）
	SecurityMonitor        *usecaseSecurityMonitoring.Service
}

// Start 启动应用程序
func (app *App) Start() error {

	// 根据模式显示不同的启动信息
	if app.Config.ScheduledPush != nil && app.Config.ScheduledPush.Enabled {
		mode := app.Config.ScheduledPush.Mode
		if mode == "client" {
			loggerImpl.Info("Client模式开始监控...")
		} else if mode == "server" {
			loggerImpl.Info("Server模式开始监控...")
		} else {
			loggerImpl.Info("开始监控...")
		}
	} else {
		loggerImpl.Info("开始监控...")
	}

	// 打印监控状态
	app.printMonitoringStatus()

	// 设置信号监听
	stopCh := make(chan struct{})
	go app.handleSignals(stopCh)

	// 启动调度器
	if err := app.startSchedulers(stopCh); err != nil {
		return err
	}

	// 启动监控协调器（阻塞运行）
	app.Coordinator.RunWithIntervals(app.Config, stopCh)

	// 优雅退出：关闭所有资源
	loggerImpl.Info("GWatch 正在退出...")
	app.cleanup()
	return nil
}

// cleanup 清理资源
func (app *App) cleanup() {
	// 关闭告警状态存储的Redis连接
	if app.AlertStateStorage != nil {
		if err := app.AlertStateStorage.Close(); err != nil {
			loggerImpl.Warnf("关闭告警状态存储连接失败: %v", err)
		} else {
			loggerImpl.Info("已关闭告警状态存储连接")
		}
	}
}

// printMonitoringStatus 打印监控状态
func (app *App) printMonitoringStatus() {
	cfg := app.Config

	if cfg.HostMonitoring != nil && cfg.HostMonitoring.Enabled {
		loggerImpl.Infof("主机监控已启用，监控间隔: %v", cfg.HostMonitoring.CollectInterval)
	} else if cfg.HostMonitoring != nil && !cfg.HostMonitoring.Enabled {
		loggerImpl.Info("主机监控已禁用")
	}

	// 应用层监控状态
	if cfg.AppMonitoring != nil && cfg.AppMonitoring.Enabled {
		loggerImpl.Info("应用层监控已启用")
		if cfg.AppMonitoring.Redis != nil && cfg.AppMonitoring.Redis.Enabled {
			loggerImpl.Info("  - Redis监控已启用")
		} else if cfg.AppMonitoring.Redis != nil && !cfg.AppMonitoring.Redis.Enabled {
			loggerImpl.Info("  - Redis监控已禁用")
		}
	} else if cfg.AppMonitoring != nil && !cfg.AppMonitoring.Enabled {
		loggerImpl.Info("应用层监控已禁用")
	}

	if cfg.HTTPMonitoring != nil && cfg.HTTPMonitoring.Enabled {
		loggerImpl.Info("HTTP接口监控已启用")
	} else if cfg.HTTPMonitoring != nil && !cfg.HTTPMonitoring.Enabled {
		loggerImpl.Info("HTTP接口监控已禁用")
	}

	if cfg.CertificateExpirationMonitoring != nil && cfg.CertificateExpirationMonitoring.Enabled {
		loggerImpl.Info("HTTPS证书过期监控已启用")
	} else if cfg.CertificateExpirationMonitoring != nil && !cfg.CertificateExpirationMonitoring.Enabled {
		loggerImpl.Info("HTTPS证书过期监控已禁用")
	}
}

// handleSignals 处理系统信号
func (app *App) handleSignals(stopCh chan struct{}) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	sig := <-c
	loggerImpl.Infof("接收到信号 %v，正在优雅退出...", sig)
	close(stopCh)
}

// startSchedulers 启动所有调度器
func (app *App) startSchedulers(stopCh <-chan struct{}) error {
	cfg := app.Config

	// 启动全局定时推送调度器
	if cfg.ScheduledPush != nil && cfg.ScheduledPush.Enabled {
		loggerImpl.Info("启动全局定时推送调度器...")
		if err := app.ScheduledPushScheduler.Start(cfg, stopCh); err != nil {
			loggerImpl.Errorf("启动全局定时推送调度器失败: %v", err)
			return err
		}
	}

	if app.SecurityMonitor != nil {
		if err := app.SecurityMonitor.Start(cfg, stopCh); err != nil {
			loggerImpl.Errorf("启动安全监控调度器失败: %v", err)
			return err
		}
	}

	return nil
}

// NewApp 创建应用程序实例
func NewApp(
	config *shared.Config,
	coordinator *usecaseMonitoring.Coordinator,
	scheduledPushScheduler scheduled_push.ScheduledPushScheduler,
	loggerService *usecaseLogger.LoggerService,
	alertStateStorage monitoring.AlertStateStorage,
	securityMonitor *usecaseSecurityMonitoring.Service,
) *App {
	return &App{
		Config:                 config,
		Coordinator:            coordinator,
		ScheduledPushScheduler: scheduledPushScheduler,
		LoggerService:          loggerService,
		AlertStateStorage:      alertStateStorage,
		SecurityMonitor:        securityMonitor,
	}
}

func NewSecurityMonitoringService(
	notifier monitoring.Notifier,
	alertStateStorage monitoring.AlertStateStorage,
	certFetcher monitoring.CertificateFetcher,
) *usecaseSecurityMonitoring.Service {
	return usecaseSecurityMonitoring.NewService(notifier, alertStateStorage, certFetcher)
}

// NewCertificateFetcher 证书拉取器
func NewCertificateFetcher() monitoring.CertificateFetcher {
	return monitoringImpl.NewTLSCertificateFetcher()
}

// NewConfig 从配置提供者获取配置
func NewConfig(provider config.Provider) *shared.Config {
	return provider.GetConfig()
}
