// 定时推送数据日志存储领域接口
package common

import (
	scheduledPushEntity "github.com/youxihu/GWatch-new/internal/domain/entity/scheduled_push"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"time"
)

// ScheduledPushDataLogStorage 定时推送数据日志存储接口（领域层）
type ScheduledPushDataLogStorage interface {
	// SaveClientData 保存客户端监控数据到日志文件
	// data: 客户端监控数据
	// timestamp: 数据时间戳
	SaveClientData(data *scheduledPushEntity.ClientMonitorData, timestamp time.Time) error

	// SaveServerReport 保存服务器聚合报告到日志文件
	// report: 报告内容（Markdown格式）
	// title: 报告标题
	// timestamp: 报告时间戳
	SaveServerReport(report string, title string, timestamp time.Time) error

	// Init 初始化存储服务
	Init(config *shared.Config) error

	// CleanupOldLogs 清理过期日志（根据配置的保留天数）
	CleanupOldLogs() error
}
