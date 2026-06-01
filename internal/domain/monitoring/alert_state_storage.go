package monitoring

import (
	monitoringEntity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"time"
)

// 注意：DiskUsageItem 类型别名已在 monitoring.go 中定义

// ProcessInfo 进程信息（用于存储多个进程）
type ProcessInfo struct {
	PID               string    // 进程ID
	ProcessName       string    // 进程名称
	FirstDetectedTime time.Time // 首次检测到该进程的时间
	LastSeenTime      time.Time // 最后一次检测到该进程的时间
	CPUPercent        float64   // CPU使用率
	MemPercent        float64   // 内存使用率
	MemRSS            uint64    // 内存RSS（MB）
}

// AlertState 告警状态信息
type AlertState struct {
	EventID             string // 事件ID
	AlertType           monitoringEntity.AlertType
	StartTime           time.Time       // 告警开始时间（第一次超阈值的时间）
	Severity            string          // 告警等级：p1/p2/p3/reminder
	SeverityStartTime   time.Time       // 当前等级的开始时间（用于等级变化时重新计时）
	Threshold           float64         // 触发阈值
	CurrentValue        float64         // 当前值
	DiskUsage           []DiskUsageItem // 磁盘占用信息（用于磁盘告警，异步收集）
	DiskUsageUpdateTime time.Time       // 磁盘占用信息更新时间（用于判断缓存是否有效）
	AlertSent           bool            // 是否已发送告警通知（用于恢复检查，避免依赖内存状态）
	Processes           []ProcessInfo   // 相关进程信息列表（支持多进程）
}

// EventLifecycleInfo 事件生命周期信息
type EventLifecycleInfo struct {
	EventID        string                     `json:"event_id"`
	AlertType      monitoringEntity.AlertType `json:"alert_type"`
	StartTime      time.Time                  `json:"start_time"`
	LastActiveTime time.Time                  `json:"last_active_time"`
	Duration       time.Duration              `json:"duration"`
	ProcessCount   int                        `json:"process_count"`
	IsActive       bool                       `json:"is_active"`
}

// AlertStateStorage 告警状态存储接口
// 用于在Redis中存储和查询告警事件状态，用于计算持续时间和判断恢复
type AlertStateStorage interface {
	// Init 初始化存储服务（保存配置，但不立即连接）
	Init(config *shared.Config) error

	// GetAlertStateByEventID 根据事件ID获取告警状态
	GetAlertStateByEventID(eventID string) (*AlertState, error)

	// GetAlertStateByProcessName 根据进程名查找相同告警类型的告警状态
	// 用于多进程场景：相同进程名的告警应该合并为一个事件
	GetAlertStateByProcessName(alertType monitoringEntity.AlertType, processName string) (*AlertState, error)

	// SetAlertStateByEventID 使用事件ID设置告警状态
	SetAlertStateByEventID(state *AlertState) error

	// UpdateAlertStateByEventID 使用事件ID更新告警状态
	UpdateAlertStateByEventID(state *AlertState) error

	// DeleteAlertStateByEventID 根据事件ID删除告警状态（按需连接）
	// 当告警恢复时调用
	DeleteAlertStateByEventID(eventID string) error

	// AddOrUpdateProcess 添加或更新进程信息到指定事件
	AddOrUpdateProcess(eventID string, processInfo ProcessInfo) error

	// RemoveProcess 从指定事件中移除进程信息
	RemoveProcess(eventID string, pid string) error

	// GetAllAlertStates 获取所有告警状态（按需连接）
	// 用于清理过期状态
	GetAllAlertStates() ([]*AlertState, error)

	// GetAlertStateByType 获取指定告警类型的第一个告警状态（按需连接）
	// 用于恢复检测时查找相关状态
	GetAlertStateByType(alertType monitoringEntity.AlertType) (*AlertState, error)

	// FindExistingEventByProcess 根据告警类型和进程名查找现有事件ID
	// 用于事件ID复用：相同进程名的告警应该使用相同的事件ID
	FindExistingEventByProcess(alertType monitoringEntity.AlertType, processName string) (*AlertState, error)

	// FindActiveEventsByType 根据告警类型查找所有活跃事件
	// 用于事件生命周期管理和统计
	FindActiveEventsByType(alertType monitoringEntity.AlertType) ([]*AlertState, error)

	// GetEventLifecycleInfo 获取事件的生命周期信息
	// 用于事件ID管理和监控
	GetEventLifecycleInfo(eventID string) (*EventLifecycleInfo, error)

	// CleanupExpiredEvents 清理已过期的事件（用于事件ID生命周期管理）
	CleanupExpiredEvents() error

	// Close 关闭Redis连接（释放资源）
	Close() error
}
