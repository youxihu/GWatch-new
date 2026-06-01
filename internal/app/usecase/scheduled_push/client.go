// 定时推送客户端用例
package scheduled_push

import (
	"fmt"
	scheduledPushEntity "github.com/youxihu/GWatch-new/internal/domain/entity/scheduled_push"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"github.com/youxihu/GWatch-new/internal/domain/scheduled_push/client"
	"github.com/youxihu/GWatch-new/internal/domain/scheduled_push/common"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	"github.com/youxihu/GWatch-new/internal/utils"
	"time"
)

// ClientUseCaseImpl 客户端模式用例实现
type ClientUseCaseImpl struct {
	metricsCollector     *MetricsCollector
	clientDataRepository common.ClientDataRepository
	dataLogStorage       common.ScheduledPushDataLogStorage
}

// NewClientUseCase 创建客户端模式用例
func NewClientUseCase(
	metricsCollector *MetricsCollector,
	clientDataRepository common.ClientDataRepository,
	dataLogStorage common.ScheduledPushDataLogStorage,
) client.ClientUseCase {
	return &ClientUseCaseImpl{
		metricsCollector:     metricsCollector,
		clientDataRepository: clientDataRepository,
		dataLogStorage:       dataLogStorage,
	}
}

// Run 执行客户端模式：收集数据并上传到 Redis
func (cu *ClientUseCaseImpl) Run(config *shared.Config) error {
	// 明确记录：Client模式绝对不应该发送通知
	logger.Infof("开始执行：只上传数据到Redis，不会发送任何通知")

	// 初始化 Repository（如果未初始化）
	if cu.clientDataRepository == nil {
		return fmt.Errorf("clientDataRepository 未初始化")
	}

	// 初始化 Redis 连接
	if err := cu.clientDataRepository.Init(config); err != nil {
		return fmt.Errorf("初始化 Redis 连接失败: %v", err)
	}

	// 收集主机监控指标（Client模式目前只收集主机监控数据）
	hostMetrics := cu.metricsCollector.CollectBasicHostMetrics()

	// 调试日志：记录 disk 数据收集情况
	if hostMetrics.Disk.Error != nil {
		logger.Warnf("警告：磁盘数据收集失败: %v", hostMetrics.Disk.Error)
	} else {
		logger.Infof("磁盘数据收集成功: 使用率=%.2f%%, 已用=%dGB, 总计=%dGB, 读IO=%s, 写IO=%s",
			hostMetrics.Disk.Percent, hostMetrics.Disk.UsedGB, hostMetrics.Disk.TotalGB,
			utils.FormatIOSpeed(hostMetrics.Disk.ReadKBps), utils.FormatIOSpeed(hostMetrics.Disk.WriteKBps))
	}

	// 构建客户端数据（目前只包含主机监控）
	clientMetrics := &scheduledPushEntity.ClientMetrics{
		CPU:     &hostMetrics.CPU,
		Memory:  &hostMetrics.Memory,
		Disk:    &hostMetrics.Disk,
		Network: &hostMetrics.Network,
		// 注意：目前Client模式只收集主机监控数据，不收集应用监控数据（Redis、HTTP）
		// 这些字段为nil，格式化器会根据条件渲染来决定是否显示
		// 未来如果需要，可以在这里根据配置来决定是否收集应用监控数据
	}

	// 获取client配置的title
	clientTitle := ""
	if config.ScheduledPush != nil && config.ScheduledPush.Title != "" {
		clientTitle = config.ScheduledPush.Title
	}

	clientData := &scheduledPushEntity.ClientMonitorData{
		HostIP:    GetHostIP(),
		HostName:  GetHostName(),
		Title:     clientTitle,
		Timestamp: time.Now(),
		Metrics:   clientMetrics,
	}

	// 保存到 Redis，设置 5 分钟过期时间
	if err := cu.clientDataRepository.SaveClientData(clientData, 5*time.Minute); err != nil {
		return fmt.Errorf("保存客户端数据到 Redis 失败: %v", err)
	}

	// 保存到数据日志文件（如果启用）
	if cu.dataLogStorage != nil {
		if err := cu.dataLogStorage.Init(config); err != nil {
			logger.Errorf("初始化数据日志存储失败: %v", err)
		} else {
			if err := cu.dataLogStorage.SaveClientData(clientData, clientData.Timestamp); err != nil {
				logger.Errorf("保存数据日志失败: %v", err)
			} else {
				logger.Info("已保存数据日志")
			}
		}
	}

	logger.Infof("成功上报监控数据到 Redis: %s (%s, Title: %s)，注意：Client模式不会发送任何通知",
		clientData.HostIP, clientData.HostName, clientData.Title)
	return nil
}
