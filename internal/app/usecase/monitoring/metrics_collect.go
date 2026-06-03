package monitoring

import (
	"time"

	"github.com/youxihu/GWatch-new/internal/app/usecase"
	"github.com/youxihu/GWatch-new/internal/domain/collector"
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"github.com/youxihu/GWatch-new/internal/domain/logger"
	domainAlert "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	domainMonitor "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	"github.com/youxihu/GWatch-new/internal/utils"
)

// MonitoringUseCase 负责完整的监控流程：采集 → 判断 → 策略 → 格式化 → 发送
type MonitoringUseCase struct {
	hostCollector   collector.HostCollector
	redisClient     usecase.RedisClient
	httpCollector   collector.HTTPCollector
	evaluator       domainMonitor.Evaluator
	alertPolicy     domainAlert.Policy
	alertFormatter  domainAlert.Formatter
	alertNotifier   usecase.Notifier
	alertLogStorage domainAlert.AlertLogStorage
	logger          logger.Logger
	isRedisInited   bool
	isHTTPInited    bool
}

// NewMonitoringUseCase 创建监控用例
func NewMonitoringUseCase(
	hostCollector collector.HostCollector,
	redisClient usecase.RedisClient,
	httpCollector collector.HTTPCollector,
	evaluator domainMonitor.Evaluator,
	alertPolicy domainAlert.Policy,
	alertFormatter domainAlert.Formatter,
	alertNotifier usecase.Notifier,
	alertLogStorage domainAlert.AlertLogStorage,
	log logger.Logger,
) *MonitoringUseCase {
	return &MonitoringUseCase{
		hostCollector:   hostCollector,
		redisClient:     redisClient,
		httpCollector:   httpCollector,
		evaluator:       evaluator,
		alertPolicy:     alertPolicy,
		alertFormatter:  alertFormatter,
		alertNotifier:   alertNotifier,
		alertLogStorage: alertLogStorage,
		logger:          log,
	}
}

// Run 执行一次完整的监控流程
func (useCase *MonitoringUseCase) Run(config *shared.Config) error {
	if useCase.alertLogStorage != nil {
		if err := useCase.alertLogStorage.Init(config); err != nil {
			useCase.logger.Infof("[告警日志] 初始化告警日志存储失败: %v", err)
		}
	}

	metrics := useCase.CollectOnce(config)
	useCase.PrintMetrics(config, metrics)
	return useCase.EvaluateAndNotify(config, metrics)
}

// collectRedisMetrics 采集Redis连接数指标（公共方法）
func (useCase *MonitoringUseCase) collectRedisMetrics(config *shared.Config, metrics *entity.SystemMetrics) {
	shouldCollectRedis := config != nil && config.AppMonitoring != nil &&
		config.AppMonitoring.Enabled && config.AppMonitoring.Redis != nil &&
		config.AppMonitoring.Redis.Enabled

	if !shouldCollectRedis {
		return
	}

	if !useCase.isRedisInited {
		if err := useCase.redisClient.Init(); err != nil {
			useCase.logger.Warnf("Redis初始化失败: %v", err)
			metrics.Redis.ConnectionError = err
			return
		}
		useCase.isRedisInited = true
		useCase.logger.Debugf("Redis初始化成功")
	}

	clientCount, err := useCase.redisClient.GetClients()
	if err != nil {
		useCase.logger.Warnf("获取Redis连接数失败: %v", err)
		metrics.Redis.ConnectionError = err
		return
	}
	metrics.Redis.ClientCount = clientCount
	useCase.logger.Debugf("Redis连接数: %d", clientCount)
	metrics.Redis.ClientDetails, metrics.Redis.DetailError = useCase.redisClient.GetClientsDetail()
}

func (useCase *MonitoringUseCase) CollectOnce(config *shared.Config) *entity.SystemMetrics {
	metricsService := NewSystemMetricsService(useCase.hostCollector, useCase.redisClient, useCase.httpCollector)
	metrics := metricsService.CollectBasicMetrics()

	useCase.collectRedisMetrics(config, metrics)

	if config != nil && config.HTTPMonitoring != nil && config.HTTPMonitoring.Enabled {
		if !useCase.isHTTPInited {
			if err := useCase.httpCollector.Init(); err != nil {
				metrics.HTTP.Error = err
			} else {
				useCase.isHTTPInited = true
			}
		}

		if useCase.isHTTPInited {
			fullMetrics := metricsService.CollectFullMetrics(config)
			metrics.HTTP = fullMetrics.HTTP
		}
	}

	return metrics
}

func (useCase *MonitoringUseCase) EvaluateAndNotify(config *shared.Config, metrics *entity.SystemMetrics) error {
	decisions, _ := useCase.evaluator.Evaluate(config, metrics)
	alertResults := useCase.alertPolicy.Apply(config, metrics, decisions)
	if len(alertResults) == 0 {
		return nil
	}
	return useCase.NotifyWithAlertResults(config, metrics, alertResults)
}

// evaluateAndNotifyWithFilter 按过滤条件评估并发送告警（公共方法）
func (useCase *MonitoringUseCase) evaluateAndNotifyWithFilter(config *shared.Config, metrics *entity.SystemMetrics, filter func(entity.AlertType) bool) error {
	decisions, _ := useCase.evaluator.Evaluate(config, metrics)
	var filteredDecisions []domainMonitor.Decision
	for _, decision := range decisions {
		if filter(decision.Type) {
			filteredDecisions = append(filteredDecisions, decision)
		}
	}
	alertResults := useCase.alertPolicy.Apply(config, metrics, filteredDecisions)
	return useCase.NotifyWithAlertResults(config, metrics, alertResults)
}

// EvaluateAndNotifyBaseOnly 仅评估非HTTP类型的告警
func (useCase *MonitoringUseCase) EvaluateAndNotifyBaseOnly(config *shared.Config, metrics *entity.SystemMetrics) error {
	return useCase.evaluateAndNotifyWithFilter(config, metrics, func(t entity.AlertType) bool { return t != entity.HTTPErr })
}

// EvaluateAndNotifyHTTPOnly 仅评估HTTP类型的告警
func (useCase *MonitoringUseCase) EvaluateAndNotifyHTTPOnly(config *shared.Config, metrics *entity.SystemMetrics) error {
	return useCase.evaluateAndNotifyWithFilter(config, metrics, func(t entity.AlertType) bool { return t == entity.HTTPErr })
}

// PrintMetrics 仅用于本地观察，不属于核心业务
func (useCase *MonitoringUseCase) PrintMetrics(config *shared.Config, metrics *entity.SystemMetrics) {
	now := time.Now()
	useCase.logger.Info("===========采集数据============")

	if config != nil && config.HostMonitoring != nil && config.HostMonitoring.Enabled {
		if metrics.CPU.Error != nil {
			useCase.logger.Errorf("CPU 监控失败: %v", metrics.CPU.Error)
		} else {
			if metrics.CPU.Cores > 0 {
				useCase.logger.Infof("CPU 使用率: %.2f%% (%d核心)", metrics.CPU.Percent, metrics.CPU.Cores)
			} else {
				useCase.logger.Infof("CPU 使用率: %.2f%%", metrics.CPU.Percent)
			}
		}
		if metrics.Memory.Error != nil {
			useCase.logger.Errorf("内存监控失败: %v", metrics.Memory.Error)
		} else {
			useCase.logger.Infof("内存使用: %.2f%% (%d/%d MB)", metrics.Memory.Percent, metrics.Memory.UsedMB, metrics.Memory.TotalMB)
		}
		if metrics.Disk.Error != nil {
			useCase.logger.Errorf("磁盘监控失败: %v", metrics.Disk.Error)
		} else {
			useCase.logger.Infof("磁盘使用: %.2f%% (%d/%d GB)",
				metrics.Disk.Percent, metrics.Disk.UsedGB, metrics.Disk.TotalGB)
		}
		if metrics.Network.Error != nil {
			useCase.logger.Errorf("网络监控失败: %v", metrics.Network.Error)
		} else {
			useCase.logger.Infof("网络: %s", utils.FormatIOSpeedPair(metrics.Network.DownloadKBps, metrics.Network.UploadKBps, "下载", "上传"))
		}
		useCase.logger.Infof("磁盘IO: %s", utils.FormatIOSpeedPair(metrics.Disk.ReadKBps, metrics.Disk.WriteKBps, "读", "写"))
	}

	if config != nil && config.AppMonitoring != nil && config.AppMonitoring.Enabled && config.AppMonitoring.Redis != nil && config.AppMonitoring.Redis.Enabled {
		if metrics.Redis.ConnectionError != nil {
			useCase.logger.Errorf("Redis 连接失败: %v", metrics.Redis.ConnectionError)
		} else {
			useCase.logger.Infof("Redis 连接数: %d", metrics.Redis.ClientCount)
		}
	}

	if config != nil && config.HTTPMonitoring != nil && config.HTTPMonitoring.Enabled {
		if metrics.HTTP.Error != nil {
			useCase.logger.Errorf("HTTP接口监控失败: %v", metrics.HTTP.Error)
		} else {
			for _, httpInterface := range metrics.HTTP.Interfaces {
				alertMark := ""
				if httpInterface.NeedAlert {
					alertMark = " [需告警]"
				} else {
					alertMark = " [仅监控]"
				}

				isValidCode := utils.IsValidHTTPStatusCode(httpInterface.StatusCode, httpInterface.AllowedCodes)

				if isValidCode {
					useCase.logger.Infof("HTTP接口 %s%s: 正常 (状态码: %d, 响应时间: %v)",
						httpInterface.Name, alertMark, httpInterface.StatusCode, httpInterface.ResponseTime)
				} else {
					useCase.logger.Warnf("HTTP接口 %s%s: 异常 (状态码: %d) - %v",
						httpInterface.Name, alertMark, httpInterface.StatusCode, httpInterface.Error)
				}
			}
		}
	}

	useCase.logger.Infof("监控时间: %s", now.Format(time.DateTime))
}

// CombineMetrics 将基础指标与 HTTP 指标合并为一个整体快照
func CombineMetrics(baseMetrics, httpMetrics *entity.SystemMetrics) *entity.SystemMetrics {
	mergedMetrics := &entity.SystemMetrics{Timestamp: time.Now()}
	if baseMetrics != nil {
		mergedMetrics.CPU = baseMetrics.CPU
		mergedMetrics.Memory = baseMetrics.Memory
		mergedMetrics.Disk = baseMetrics.Disk
		mergedMetrics.Network = baseMetrics.Network
		mergedMetrics.Redis = baseMetrics.Redis
	}
	if httpMetrics != nil {
		mergedMetrics.HTTP = httpMetrics.HTTP
	}
	return mergedMetrics
}

// CollectBaseOnce 仅采集基础主机/Redis/网络等指标（不采集 HTTP）
func (useCase *MonitoringUseCase) CollectBaseOnce(config *shared.Config) *entity.SystemMetrics {
	metricsService := NewSystemMetricsService(useCase.hostCollector, useCase.redisClient, useCase.httpCollector)
	metrics := metricsService.CollectBasicMetrics()
	useCase.collectRedisMetrics(config, metrics)
	return metrics
}

// CollectHTTPOnce 仅采集 HTTP 接口指标
func (useCase *MonitoringUseCase) CollectHTTPOnce(config *shared.Config) *entity.SystemMetrics {
	if !useCase.isHTTPInited {
		if err := useCase.httpCollector.Init(); err != nil {
			metrics := &entity.SystemMetrics{Timestamp: time.Now()}
			metrics.HTTP.Error = err
			return metrics
		}
		useCase.isHTTPInited = true
	}

	metricsService := NewSystemMetricsService(useCase.hostCollector, useCase.redisClient, useCase.httpCollector)
	return metricsService.CollectFullMetrics(config)
}
