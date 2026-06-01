// 定时推送服务端用例
package scheduled_push

import (
	"fmt"
	"github.com/youxihu/GWatch-new/internal/app/usecase"
	scheduledPushEntity "github.com/youxihu/GWatch-new/internal/domain/entity/scheduled_push"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"github.com/youxihu/GWatch-new/internal/domain/scheduled_push/common"
	"github.com/youxihu/GWatch-new/internal/domain/scheduled_push/server"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	"time"
)

// ServerUseCaseImpl 服务端模式用例实现
type ServerUseCaseImpl struct {
	metricsCollector       *MetricsCollector
	clientDataRepository   common.ClientDataRepository
	scheduledPushFormatter common.ScheduledPushFormatter
	notifier               usecase.Notifier
	dataLogStorage         common.ScheduledPushDataLogStorage
}

// NewServerUseCase 创建服务端模式用例
func NewServerUseCase(
	metricsCollector *MetricsCollector,
	clientDataRepository common.ClientDataRepository,
	scheduledPushFormatter common.ScheduledPushFormatter,
	notifier usecase.Notifier,
	dataLogStorage common.ScheduledPushDataLogStorage,
) server.ServerUseCase {
	return &ServerUseCaseImpl{
		metricsCollector:       metricsCollector,
		clientDataRepository:   clientDataRepository,
		scheduledPushFormatter: scheduledPushFormatter,
		notifier:               notifier,
		dataLogStorage:         dataLogStorage,
	}
}

// Run 执行服务端模式：从 Redis 读取数据并聚合成报告发送
func (su *ServerUseCaseImpl) Run(config *shared.Config) error {
	// 明确记录：Server模式会发送通知
	logger.Infof("开始执行：将从Redis读取客户端数据并聚合成报告发送通知")

	// 初始化 Repository（如果未初始化）
	if su.clientDataRepository == nil {
		return fmt.Errorf("clientDataRepository 未初始化")
	}

	// 初始化 Redis 连接
	if err := su.clientDataRepository.Init(config); err != nil {
		return fmt.Errorf("初始化 Redis 连接失败: %v", err)
	}

	// 获取所有客户端数据的 keys
	keys, err := su.clientDataRepository.GetAllClientDataKeys()
	if err != nil {
		logger.Infof("获取客户端数据 keys 失败: %v，将继续收集Server自己的数据", err)
		keys = []string{} // 设置为空，继续执行
	}

	logger.Infof("从Redis获取到 %d 个客户端数据key", len(keys))

	// 读取所有客户端数据
	var clientDataList []*scheduledPushEntity.ClientMonitorData
	validKeys := []string{}
	// 用于去重的map：key为"IP:Title"，value为最新的数据
	dedupMap := make(map[string]*scheduledPushEntity.ClientMonitorData)

	if len(keys) > 0 {
		for _, key := range keys {
			clientData, err := su.clientDataRepository.GetClientDataByKey(key)
			if err != nil {
				logger.Infof("读取客户端数据失败 (key=%s): %v", key, err)
				continue
			}
			if clientData != nil {
				// 生成去重key：IP + Title（同一台机器的不同title视为不同实例）
				dedupKey := fmt.Sprintf("%s:%s", clientData.HostIP, clientData.Title)

				// 如果已存在，比较时间戳，保留最新的
				if existing, exists := dedupMap[dedupKey]; exists {
					if clientData.Timestamp.After(existing.Timestamp) {
						dedupMap[dedupKey] = clientData
						logger.Infof("发现重复数据，保留更新的: %s (%s, Title: %s, 时间戳: %s)",
							clientData.HostIP, clientData.HostName, clientData.Title, clientData.Timestamp.Format("15:04:05"))
					} else {
						logger.Infof("发现重复数据，保留旧的（更新）: %s (%s, Title: %s, 旧时间戳: %s, 新时间戳: %s)",
							clientData.HostIP, clientData.HostName, clientData.Title,
							existing.Timestamp.Format("15:04:05"), clientData.Timestamp.Format("15:04:05"))
					}
				} else {
					dedupMap[dedupKey] = clientData
					logger.Infof("成功读取客户端数据: %s (%s, Title: %s)",
						clientData.HostIP, clientData.HostName, clientData.Title)
				}
				validKeys = append(validKeys, key)
			}
		}

		// 将去重后的数据转换为列表
		for _, data := range dedupMap {
			clientDataList = append(clientDataList, data)
		}

		logger.Infof("去重后共有 %d 台主机的数据（原始key数量: %d）", len(clientDataList), len(keys))
	}

	if len(clientDataList) == 0 {
		logger.Info("暂无客户端数据，将只发送Server自己的监控数据")
	}

	// 收集Server自己的监控数据并合并到聚合报告中
	serverIP := GetHostIP()
	serverHostName := GetHostName()

	logger.Infof("Server自身IP: %s, HostName: %s", serverIP, serverHostName)

	// 收集Server的主机监控数据
	serverHostMetrics := su.metricsCollector.CollectBasicHostMetrics()

	// 构建Server的监控数据
	serverMetrics := &scheduledPushEntity.ClientMetrics{
		CPU:     &serverHostMetrics.CPU,
		Memory:  &serverHostMetrics.Memory,
		Disk:    &serverHostMetrics.Disk,
		Network: &serverHostMetrics.Network,
	}

	// 如果应用监控已启用，收集Server的应用监控数据
	if config.AppMonitoring != nil && config.AppMonitoring.Enabled {
		logger.Infof("应用监控已启用，开始收集Server的应用监控数据")

		// 收集Redis指标
		if config.AppMonitoring.Redis != nil && config.AppMonitoring.Redis.Enabled {
			redisMetrics := su.metricsCollector.CollectRedisMetrics(config)
			if redisMetrics != nil {
				serverMetrics.Redis = redisMetrics
				logger.Infof("已收集Redis指标")
			}
		}
	}

	// 收集HTTP指标（独立于应用监控）
	if config.HTTPMonitoring != nil && config.HTTPMonitoring.Enabled {
		httpMetrics := su.metricsCollector.CollectHTTPMetrics(config)
		if httpMetrics != nil {
			serverMetrics.HTTP = httpMetrics
			logger.Infof("已收集HTTP指标")
		}
	}

	// 获取server配置的title
	serverTitle := ""
	if config.ScheduledPush != nil && config.ScheduledPush.Title != "" {
		serverTitle = config.ScheduledPush.Title
	}

	// 构建Server的监控数据对象
	serverData := &scheduledPushEntity.ClientMonitorData{
		HostIP:    serverIP,
		HostName:  serverHostName,
		Title:     serverTitle,
		Timestamp: time.Now(),
		Metrics:   serverMetrics,
	}

	// 检查clientDataList中是否已经有Server自己的数据（通过IP+Title组合判断，因为同一机器可能有多个实例）
	var serverDataIndex int = -1
	for i, data := range clientDataList {
		// 通过 IP + Title 组合判断是否是同一个实例
		if data.HostIP == serverIP && data.Title == serverTitle {
			// 如果已经存在相同的实例，更新它
			clientDataList[i] = serverData
			serverDataIndex = i
			logger.Infof("发现Server自己的数据已在Redis中，将更新: %s (%s, Title: %s)",
				serverIP, serverHostName, serverTitle)
			break
		}
	}

	// 如果没有找到Server自己的数据，添加到列表（允许同一机器上有多个不同title的实例）
	if serverDataIndex == -1 {
		clientDataList = append(clientDataList, serverData)
		logger.Infof("Server自己的数据未在Redis中找到，将添加到聚合列表: %s (%s, Title: %s)",
			serverIP, serverHostName, serverTitle)
	}

	// 如果没有任何数据（包括Server自己的数据），返回
	if len(clientDataList) == 0 {
		logger.Info("未找到有效的监控数据，不发送通知")
		return nil
	}

	// 格式化并发送报告
	// 通知标题：统一使用"性能监控定时推送"（固定标题，不读取配置中的title）
	notificationTitle := "性能监控定时推送"

	logger.Infof("准备发送聚合报告，标题: %s，包含 %d 台主机的数据", notificationTitle, len(clientDataList))
	for i, data := range clientDataList {
		logger.Infof("聚合报告主机 %d: IP=%s, HostName=%s, Title=%s",
			i+1, data.HostIP, data.HostName, data.Title)
	}

	// 格式化报告（传入server的title作为默认值，用于每个主机的二级标题，传入config用于判断阈值）
	// 注意：这里使用serverTitle作为格式化器的默认值，但通知标题使用的是notificationTitle
	reportTitle := su.getScheduledPushTitle(config)
	report := su.scheduledPushFormatter.FormatClientReport(clientDataList, reportTitle, config)

	// 检查是否启用通知（全局配置）
	// 注意：由于bool的零值是false，如果配置文件中没有设置enable_notification字段，
	// 读取到的值会是false，但为了向后兼容，我们默认启用通知
	// 只有当配置文件中明确设置为false时，才禁用通知
	enableNotification := true
	// 检查配置中是否明确设置了EnableNotification
	// 如果配置文件中字段存在且为false，才禁用通知
	// 如果字段不存在（omitempty），默认启用通知
	if config.EnableNotification == false {
		// 这里需要判断：如果配置文件中明确设置了false，才禁用
		// 但由于bool无法区分"未设置"和"设置为false"，我们采用保守策略：
		// 只有当明确为false时才禁用，否则默认启用
		enableNotification = false
		logger.Infof("全局通知开关已设置为false，将禁用钉钉通知")
	} else {
		// 如果为true或未设置（零值），都使用true（默认启用）
		enableNotification = true
	}

	// 发送通知（如果启用）
	if enableNotification {
		logger.Infof("正在发送通知到钉钉，标题: %s", notificationTitle)
		if err := su.notifier.Send(notificationTitle, report); err != nil {
			return fmt.Errorf("发送合并报告失败: %v", err)
		}
		logger.Infof("成功发送合并报告到钉钉，标题: %s，包含 %d 台主机的数据", notificationTitle, len(clientDataList))
	} else {
		logger.Infof("通知功能已禁用，跳过发送钉钉通知（标题: %s，包含 %d 台主机的数据）", notificationTitle, len(clientDataList))
	}

	// 保存报告到数据日志文件（如果启用）
	if su.dataLogStorage != nil {
		if err := su.dataLogStorage.Init(config); err != nil {
			logger.Infof("初始化数据日志存储失败: %v", err)
		} else {
			reportTimestamp := time.Now()
			if err := su.dataLogStorage.SaveServerReport(report, notificationTitle, reportTimestamp); err != nil {
				logger.Infof("保存报告日志失败: %v", err)
			} else {
				logger.Infof("已保存报告日志")
				// 清理过期日志（后台执行，不阻塞）
				go func() {
					if err := su.dataLogStorage.CleanupOldLogs(); err != nil {
						logger.Infof("清理过期日志失败: %v", err)
					} else {
						logger.Infof("已清理过期日志")
					}
				}()
			}
		}
	}

	// 清理已发送的数据（可选）
	for _, key := range validKeys {
		if err := su.clientDataRepository.DeleteClientData(key); err != nil {
			logger.Infof("删除客户端数据失败 (key=%s): %v", key, err)
		}
	}

	return nil
}

// getScheduledPushTitle 获取全局定时推送标题
// 直接从配置的 scheduled_push.title 字段获取，不使用主机名
func (su *ServerUseCaseImpl) getScheduledPushTitle(config *shared.Config) string {
	if config.ScheduledPush != nil && config.ScheduledPush.Title != "" {
		return config.ScheduledPush.Title
	}
	return "系统监控定时报告"
}
