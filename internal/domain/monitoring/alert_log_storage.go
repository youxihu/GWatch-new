package monitoring

import (
	monitoringEntity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"time"
)

// AlertLogStorage 告警日志存储接口
// 用于将告警信息保存到日志文件，便于在不发送钉钉通知时进行日志追踪
type AlertLogStorage interface {
	// Init 初始化告警日志存储服务
	Init(config *shared.Config) error

	// SaveAlert 保存告警信息到日志文件
	// title: 告警标题
	// markdown: 告警内容（Markdown格式）
	// metrics: 监控指标
	// alertTypes: 触发的告警类型列表
	// timestamp: 告警时间
	SaveAlert(title string, markdown string, metrics *monitoringEntity.SystemMetrics, alertTypes []monitoringEntity.AlertType, timestamp time.Time) error

	// SaveAlertWithEventID 按事件ID保存告警或恢复信息到日志文件（用于日志追溯）
	// title: 告警/恢复标题
	// markdown: 告警/恢复内容（Markdown格式）
	// eventID: 事件ID
	// isRecovery: 是否为恢复通知
	// timestamp: 告警/恢复时间
	SaveAlertWithEventID(title string, markdown string, eventID string, isRecovery bool, timestamp time.Time) error

	// CleanupOldLogs 清理过期日志
	CleanupOldLogs() error
}
