// 定时推送共享模块
package scheduled_push

import (
	"github.com/youxihu/GWatch-new/internal/app/usecase"
	usecaseMonitoring "github.com/youxihu/GWatch-new/internal/app/usecase/monitoring"
	"github.com/youxihu/GWatch-new/internal/domain/collector"
	monitoringEntity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	"net"
	"os"
)

// MetricsCollector 指标收集器（供Client和Server共享使用）
type MetricsCollector struct {
	hostCollector collector.HostCollector
	redisClient   usecase.RedisClient
	httpCollector collector.HTTPCollector
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector(
	hostCollector collector.HostCollector,
	redisClient usecase.RedisClient,
	httpCollector collector.HTTPCollector,
) *MetricsCollector {
	return &MetricsCollector{
		hostCollector: hostCollector,
		redisClient:   redisClient,
		httpCollector: httpCollector,
	}
}

// CollectBasicHostMetrics 收集基本主机指标（不依赖外部服务）
func (mc *MetricsCollector) CollectBasicHostMetrics() *monitoringEntity.SystemMetrics {
	// 使用SystemMetricsService统一采集指标
	metricsService := usecaseMonitoring.NewSystemMetricsService(mc.hostCollector, mc.redisClient, mc.httpCollector)
	return metricsService.CollectBasicMetrics()
}

// CollectRedisMetrics 收集Redis指标
func (mc *MetricsCollector) CollectRedisMetrics(config *shared.Config) *monitoringEntity.RedisMetrics {
	redisMetrics := &monitoringEntity.RedisMetrics{
		ClientCount: 0,
	}

	// 初始化Redis连接
	if err := mc.redisClient.Init(); err != nil {
		redisMetrics.ConnectionError = err
		return redisMetrics
	}

	// 获取Redis连接数
	clientCount, err := mc.redisClient.GetClients()
	if err != nil {
		redisMetrics.ConnectionError = err
	} else {
		redisMetrics.ClientCount = clientCount
	}

	// 获取Redis连接详情
	clientDetails, err := mc.redisClient.GetClientsDetail()
	if err != nil {
		redisMetrics.DetailError = err
	} else {
		redisMetrics.ClientDetails = clientDetails
	}

	return redisMetrics
}

// CollectHTTPMetrics 收集HTTP指标
func (mc *MetricsCollector) CollectHTTPMetrics(config *shared.Config) *monitoringEntity.HTTPMetrics {
	// 使用SystemMetricsService统一采集指标
	metricsService := usecaseMonitoring.NewSystemMetricsService(mc.hostCollector, mc.redisClient, mc.httpCollector)
	fullMetrics := metricsService.CollectFullMetrics(config)
	return &fullMetrics.HTTP
}

// GetHostIP 获取本机 IP 地址（优先获取非回环的IPv4地址）
func GetHostIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		logger.Errorf("获取网络接口地址失败: %v", err)
		return "unknown-ip"
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}

	// 如果没找到，尝试通过连接外部地址获取本机IP
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		logger.Errorf("获取本机IP失败: %v", err)
		return "unknown-ip"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// GetHostName 获取本机主机名
func GetHostName() string {
	hostname, err := os.Hostname()
	if err != nil {
		logger.Errorf("获取主机名失败: %v", err)
		return "unknown-host"
	}
	return hostname
}
