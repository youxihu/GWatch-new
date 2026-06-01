package monitoring

import (
	"github.com/youxihu/GWatch-new/internal/app/usecase"
	"github.com/youxihu/GWatch-new/internal/domain/collector"
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	domainAlert "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	domainMonitor "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	"github.com/youxihu/GWatch-new/internal/utils"
	"time"
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
	}
}

// Run 执行一次完整的监控流程
func (useCase *MonitoringUseCase) Run(config *shared.Config) error {
	// 初始化告警日志存储（如果启用）
	if useCase.alertLogStorage != nil {
		if err := useCase.alertLogStorage.Init(config); err != nil {
			logger.Infof("[告警日志] 初始化告警日志存储失败: %v", err)
		}
	}

	// 1. 采集指标
	metrics := useCase.CollectOnce(config)

	// 2. 打印采集结果（可选，用于本地观察）
	useCase.PrintMetrics(config, metrics)

	// 3. 阈值判断与告警处理
	return useCase.EvaluateAndNotify(config, metrics)
}

func (useCase *MonitoringUseCase) CollectOnce(config *shared.Config) *entity.SystemMetrics {
	// 使用SystemMetricsService统一采集指标
	metricsService := NewSystemMetricsService(useCase.hostCollector, useCase.redisClient, useCase.httpCollector)

	// 收集基础指标
	metrics := metricsService.CollectBasicMetrics()

	// Redis监控：只有当app_monitoring.redis.enabled为true时，才收集连接数
	shouldCollectRedis := false
	if config != nil && config.AppMonitoring != nil && config.AppMonitoring.Enabled {
		if config.AppMonitoring.Redis != nil && config.AppMonitoring.Redis.Enabled {
			shouldCollectRedis = true
		}
	}

	if shouldCollectRedis {
		if !useCase.isRedisInited {
			if err := useCase.redisClient.Init(); err != nil {
				logger.Warnf("Redis初始化失败: %v", err)
				metrics.Redis.ConnectionError = err
			} else {
				useCase.isRedisInited = true
				logger.Debugf("Redis初始化成功")
			}
		}

		if useCase.isRedisInited {
			clientCount, err := useCase.redisClient.GetClients()
			if err != nil {
				logger.Warnf("获取Redis连接数失败: %v", err)
				metrics.Redis.ConnectionError = err
			} else {
				metrics.Redis.ClientCount = clientCount
				logger.Debugf("Redis连接数: %d", clientCount)
			}
			metrics.Redis.ClientDetails, metrics.Redis.DetailError = useCase.redisClient.GetClientsDetail()
		}
	}

	// HTTP接口监控：只有当http_monitoring配置存在且启用时才执行
	if config != nil && config.HTTPMonitoring != nil && config.HTTPMonitoring.Enabled {
		if !useCase.isHTTPInited {
			if err := useCase.httpCollector.Init(); err != nil {
				metrics.HTTP.Error = err
			} else {
				useCase.isHTTPInited = true
			}
		}

		if useCase.isHTTPInited {
			// 使用SystemMetricsService收集HTTP指标
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

	// 直接委托给 NotifyWithAlertResults，不再自己处理告警构造
	return useCase.NotifyWithAlertResults(config, metrics, alertResults)
}

func (useCase *MonitoringUseCase) EvaluateAndNotifyBaseOnly(config *shared.Config, metrics *entity.SystemMetrics) error {
	decisions, _ := useCase.evaluator.Evaluate(config, metrics)
	var filteredDecisions []domainMonitor.Decision
	for _, decision := range decisions {
		if decision.Type != entity.HTTPErr {
			filteredDecisions = append(filteredDecisions, decision)
		}
	}
	alertResults := useCase.alertPolicy.Apply(config, metrics, filteredDecisions)
	return useCase.NotifyWithAlertResults(config, metrics, alertResults) // ← 统一出口
}

func (useCase *MonitoringUseCase) EvaluateAndNotifyHTTPOnly(config *shared.Config, metrics *entity.SystemMetrics) error {
	decisions, _ := useCase.evaluator.Evaluate(config, metrics)
	var filteredDecisions []domainMonitor.Decision
	for _, decision := range decisions {
		if decision.Type == entity.HTTPErr {
			filteredDecisions = append(filteredDecisions, decision)
		}
	}
	alertResults := useCase.alertPolicy.Apply(config, metrics, filteredDecisions)
	return useCase.NotifyWithAlertResults(config, metrics, alertResults) // ← 统一出口
}

// PrintMetrics 仅用于本地观察，不属于核心业务
func (useCase *MonitoringUseCase) PrintMetrics(config *shared.Config, metrics *entity.SystemMetrics) {
	now := time.Now() // 获取当前时间
	logger.Info("===========采集数据============")

	// 主机类监控信息 - 只有当host_monitoring配置存在且启用时才显示
	if config != nil && config.HostMonitoring != nil && config.HostMonitoring.Enabled {
		if metrics.CPU.Error != nil {
			logger.Errorf("CPU 监控失败: %v", metrics.CPU.Error)
		} else {
			if metrics.CPU.Cores > 0 {
				logger.Infof("CPU 使用率: %.2f%% (%d核心)", metrics.CPU.Percent, metrics.CPU.Cores)
			} else {
				logger.Infof("CPU 使用率: %.2f%%", metrics.CPU.Percent)
			}
		}
		if metrics.Memory.Error != nil {
			logger.Errorf("内存监控失败: %v", metrics.Memory.Error)
		} else {
			logger.Infof("内存使用: %.2f%% (%d/%d MB)", metrics.Memory.Percent, metrics.Memory.UsedMB, metrics.Memory.TotalMB)
		}
		if metrics.Disk.Error != nil {
			logger.Errorf("磁盘监控失败: %v", metrics.Disk.Error)
		} else {
			logger.Infof("磁盘使用: %.2f%% (%d/%d GB)",
				metrics.Disk.Percent, metrics.Disk.UsedGB, metrics.Disk.TotalGB)
		}
		if metrics.Network.Error != nil {
			logger.Errorf("网络监控失败: %v", metrics.Network.Error)
		} else {
			logger.Infof("网络: %s", utils.FormatIOSpeedPair(metrics.Network.DownloadKBps, metrics.Network.UploadKBps, "下载", "上传"))
		}
		logger.Infof("磁盘IO: %s", utils.FormatIOSpeedPair(metrics.Disk.ReadKBps, metrics.Disk.WriteKBps, "读", "写"))
	}

	// Redis监控信息 - 只有当app_monitoring和redis配置存在且启用时才显示
	if config != nil && config.AppMonitoring != nil && config.AppMonitoring.Enabled && config.AppMonitoring.Redis != nil && config.AppMonitoring.Redis.Enabled {
		if metrics.Redis.ConnectionError != nil {
			logger.Errorf("Redis 连接失败: %v", metrics.Redis.ConnectionError)
		} else {
			logger.Infof("Redis 连接数: %d", metrics.Redis.ClientCount)
		}
	}

	// HTTP接口监控信息 - 只有当http_monitoring配置存在且启用时才显示
	if config != nil && config.HTTPMonitoring != nil && config.HTTPMonitoring.Enabled {
		if metrics.HTTP.Error != nil {
			logger.Errorf("HTTP接口监控失败: %v", metrics.HTTP.Error)
		} else {
			for _, httpInterface := range metrics.HTTP.Interfaces {
				alertMark := ""
				if httpInterface.NeedAlert {
					alertMark = " [需告警]"
				} else {
					alertMark = " [仅监控]"
				}

				// 检查状态码是否在允许的范围内
				isValidCode := utils.IsValidHTTPStatusCode(httpInterface.StatusCode, httpInterface.AllowedCodes)

				if isValidCode {
					logger.Infof("HTTP接口 %s%s: 正常 (状态码: %d, 响应时间: %v)",
						httpInterface.Name, alertMark, httpInterface.StatusCode, httpInterface.ResponseTime)
				} else {
					logger.Warnf("HTTP接口 %s%s: 异常 (状态码: %d) - %v",
						httpInterface.Name, alertMark, httpInterface.StatusCode, httpInterface.Error)
				}
			}
		}
	}

	logger.Infof("监控时间: %s", now.Format(time.DateTime))
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
	// 创建系统指标服务实例
	metricsService := NewSystemMetricsService(useCase.hostCollector, useCase.redisClient, useCase.httpCollector)
	metrics := metricsService.CollectBasicMetrics()

	// Redis监控：只有当app_monitoring.redis.enabled为true时，才收集连接数
	shouldCollectRedis := false
	if config != nil && config.AppMonitoring != nil && config.AppMonitoring.Enabled {
		if config.AppMonitoring.Redis != nil && config.AppMonitoring.Redis.Enabled {
			shouldCollectRedis = true
		}
	}

	if shouldCollectRedis {
		if !useCase.isRedisInited {
			if err := useCase.redisClient.Init(); err != nil {
				metrics.Redis.ConnectionError = err
			} else {
				useCase.isRedisInited = true
			}
		}

		if useCase.isRedisInited {
			clientCount, err := useCase.redisClient.GetClients()
			if err != nil {
				metrics.Redis.ConnectionError = err
			} else {
				metrics.Redis.ClientCount = clientCount
			}
			metrics.Redis.ClientDetails, metrics.Redis.DetailError = useCase.redisClient.GetClientsDetail()
		}
	}

	return metrics
}

// CollectHTTPOnce 仅采集 HTTP 接口指标
func (useCase *MonitoringUseCase) CollectHTTPOnce(config *shared.Config) *entity.SystemMetrics {
	// 确保HTTP客户端已初始化
	if !useCase.isHTTPInited {
		if err := useCase.httpCollector.Init(); err != nil {
			// 如果初始化失败，返回一个包含错误的指标对象
			metrics := &entity.SystemMetrics{Timestamp: time.Now()}
			metrics.HTTP.Error = err
			return metrics
		}
		useCase.isHTTPInited = true
	}

	// 创建系统指标服务实例
	metricsService := NewSystemMetricsService(useCase.hostCollector, useCase.redisClient, useCase.httpCollector)
	return metricsService.CollectFullMetrics(config)
}

