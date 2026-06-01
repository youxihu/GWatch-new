package monitoring

import (
	monitoringEntity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"time"
)

// DiskUsageItem 磁盘占用项类型别名，使用共享定义
type DiskUsageItem = shared.DiskUsageItem

// Decision 表示一次阈值判断的结果（不包含消息体与发送）
type Decision struct {
	Type         monitoringEntity.AlertType
	CurrentValue float64 // 当前值（用于判断阈值和恢复）
	PID          string  // 进程ID（如果有，用于CPU/内存告警，否则为"default"）
	ProcessName  string  // 进程名称（如果有，用于存储到Redis）
}

// Evaluator 负责根据配置与指标进行阈值判断，返回触发的决策
// 注意：不做防抖、连续计数、消息拼接与发送
type Evaluator interface {
	Evaluate(cfg *shared.Config, metrics *monitoringEntity.SystemMetrics) ([]Decision, error)
}

// TriggeredAlert 携带具体的告警类型与详细消息
type TriggeredAlert struct {
	Type         monitoringEntity.AlertType
	Message      string
	EventID      string                        // 事件ID
	Severity     string                        // 告警等级：p1/p2/p3/reminder
	Duration     string                        // 持续时间（格式化后的字符串）
	CurrentValue float64                       // 当前值
	Threshold    float64                       // 触发阈值
	IsRecovery   bool                          // 是否为恢复通知
	StartTime    time.Time                     // 告警开始时间（用于恢复通知中的触发时间）
	TriggerTime  time.Time                     // 当前触发时间
	DiskUsage    []DiskUsageItem               // 磁盘占用信息（用于磁盘告警）
	Process      *monitoringEntity.ProcessInfo // 进程信息（用于CPU/内存告警）
}

// Formatter 负责将告警信息与指标拼成可读文本（例如 Markdown）
type Formatter interface {
	Build(title string, cfg *shared.Config, metrics *monitoringEntity.SystemMetrics, alerts []TriggeredAlert) string
}

// AlertResult 告警结果（包含告警和恢复信息）
type AlertResult struct {
	AlertType    monitoringEntity.AlertType
	EventID      string
	Severity     string
	Duration     string
	CurrentValue float64
	Threshold    float64
	IsRecovery   bool
	StartTime    time.Time       // 告警开始时间（用于恢复通知中的触发时间）
	TriggerTime  time.Time       // 当前触发时间
	PID          string          // 进程ID（用于恢复通知时从Redis获取原始进程信息）
	ProcessName  string          // 进程名称（用于恢复通知时从Redis获取原始进程信息）
	DiskUsage    []DiskUsageItem // 磁盘占用信息（从Redis读取，异步收集）
}

// Policy 将 monitor 层的阈值判断结果，结合防抖/连续计数等策略，输出最终需要通知的告警结果
type Policy interface {
	Apply(cfg *shared.Config, metrics *monitoringEntity.SystemMetrics, decisions []Decision) []AlertResult
	PeekApply(cfg *shared.Config, metrics *monitoringEntity.SystemMetrics, decisions []Decision) []AlertResult
}

// Notifier 定义消息通知的能力
type Notifier interface {
	Send(title string, markdown string) error
}
