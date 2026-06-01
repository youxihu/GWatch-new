// Package entity 领域实体 - 共享配置实体
package entity

import (
	"time"
)

// DiskUsageItem 磁盘占用项（共享数据结构）
type DiskUsageItem struct {
	Path         string `json:"path"`           // 路径
	Size         string `json:"size"`           // 格式化后的大小，如 "1.2G", "500M"
	SizeBytes    uint64 `json:"size_bytes"`     // 字节数，用于排序和比较
	Type         string `json:"type"`           // 类型："file" 或 "directory"
	Depth        int    `json:"depth"`          // 扫描深度
	IsLeafResult bool   `json:"is_leaf_result"` // 是否为叶子结果（通过递归扫描找到的真正占用空间的根源）
}

// Config 总配置结构
type Config struct {
	// 全局告警策略配置
	Alerting *AlertingConfig `yaml:"alerting,omitempty"`

	// 主机类监控配置
	HostMonitoring *HostMonitoringConfig `yaml:"host_monitoring,omitempty"` // 主机类监控，nil表示不监控

	// 应用层类监控配置
	AppMonitoring *AppMonitoringConfig `yaml:"app_monitoring,omitempty"` // 应用层类监控，nil表示不监控

	// HTTP接口监控配置
	HTTPMonitoring *HTTPMonitoringConfig `yaml:"http_monitoring,omitempty"`

	// HTTPS证书过期监控配置
	CertificateExpirationMonitoring *CertificateExpirationMonitoringConfig `yaml:"certificate_expiration_monitoring,omitempty"`


	// 全局定时推送配置
	ScheduledPush *ScheduledPushConfig `yaml:"scheduled_push,omitempty"` // 全局定时推送，nil表示不启用

	// 公共Redis连接配置（app_monitoring和scheduled_push可以共享）
	RedisConnection *RedisConnectionConfig `yaml:"redis_connection,omitempty"`

	// 通用配置
	DingTalk         DingTalkConfig `yaml:"dingtalk"`
	Log              LogConfig      `yaml:"log"`
	WhiteProcessList []string       `yaml:"white_process_list"`

	// 全局通知开关（当为false时，定时任务和普通监控都不会发送钉钉通知）
	// 默认true（如果配置文件中未设置该字段，默认启用通知）
	EnableNotification bool `yaml:"enable_notification,omitempty"`
}

// AlertingConfig 全局告警策略配置
type AlertingConfig struct {
	// 告警事件生命周期控制
	Event AlertingEventConfig `yaml:"event"`

	// 防抖与触发机制
	Trigger AlertingTriggerConfig `yaml:"trigger"`

	// 告警等级定义（所有监控项复用）
	SeverityLevels AlertingSeverityLevelsConfig `yaml:"severity_levels"`
}

// AlertingEventConfig 告警事件生命周期控制
type AlertingEventConfig struct {
	// 告警事件状态在内存/Redis中保留时间（用于恢复匹配）
	RetentionHours int `yaml:"retention_hours"`

	// 事件ID生成模板
	IDTemplate string `yaml:"id_template"`
}

// AlertingTriggerConfig 防抖与触发机制
type AlertingTriggerConfig struct {
	// 同一事件在 N 分钟内不再重复告警（防刷屏）
	AlertInterval time.Duration `yaml:"alert_interval"`

	// 统一持续时间：所有阈值超过此时间才发送告警通知
	DurationRequired time.Duration `yaml:"duration_required"`

	// 恢复通知持续时间：阈值恢复正常超过此时间才发送恢复通知
	RecoveryDurationRequired time.Duration `yaml:"recovery_duration_required"`
}

// AlertingSeverityLevelsConfig 告警等级定义
type AlertingSeverityLevelsConfig struct {
	P1       AlertingSeverityLevel `yaml:"p1"`       // 紧急（立即响应）
	P2       AlertingSeverityLevel `yaml:"p2"`       // 严重
	P3       AlertingSeverityLevel `yaml:"p3"`       // 警告
	Reminder AlertingSeverityLevel `yaml:"reminder"` // 提醒（瞬时超阈即告）
}

// AlertingSeverityLevel 单个告警等级配置
type AlertingSeverityLevel struct {
	// 立即通知（不等待防抖窗口结束）
	NotifyImmediately bool `yaml:"notify_immediately"`
}

// HostMonitoringConfig 主机类监控配置
type HostMonitoringConfig struct {
	// 是否启用主机监控
	Enabled bool `yaml:"enabled"` // 是否启用监控

	// 采集间隔
	CollectInterval time.Duration `yaml:"collect_interval"`

	// 告警标题
	AlertTitle string `yaml:"alert_title"`

	// 阈值配置
	Thresholds HostMonitoringThresholds `yaml:"thresholds"`

	// 告警日志存储配置（用于在不发送钉钉通知时进行日志追踪）
	AlertLog *AlertLogConfig `yaml:"alert_log,omitempty"`

	// 磁盘使用率扫描配置
	DiskUsageScan *DiskUsageScanConfig `yaml:"disk_usage_scan,omitempty"`
}

// HostMonitoringThresholds 主机监控阈值配置
type HostMonitoringThresholds struct {
	CPU    *HostThresholdConfig `yaml:"cpu,omitempty"`
	Memory *HostThresholdConfig `yaml:"memory,omitempty"`
	Disk   *HostThresholdConfig `yaml:"disk,omitempty"`
}

// HostThresholdConfig 主机单个指标阈值配置
type HostThresholdConfig struct {
	// 是否启用此指标的监控
	Enabled bool `yaml:"enabled"`

	// 基础触发阈值（用于 reminder 级别）
	Base float64 `yaml:"base"`

	// 等级阈值
	Levels HostThresholdLevels `yaml:"levels"`
}

// HostThresholdLevels 主机阈值等级
type HostThresholdLevels struct {
	P3 float64 `yaml:"p3"`
	P2 float64 `yaml:"p2"`
	P1 float64 `yaml:"p1"`
}

// AlertLogConfig 告警日志存储配置
type AlertLogConfig struct {
	// 是否启用告警日志存储
	Enabled bool `yaml:"enabled"`

	// 告警日志路径模板（支持时间格式化：%y(年), %m(月), %d(日), %H(时), %M(分), %S(秒)）
	// 例如：logs/alerts/%y/%m-%d/alert-%H%M-%S.md
	LogPathTemplate string `yaml:"log_path_template"`

	// 日志文件保留天数
	RetentionDays int `yaml:"retention_days"`
}

// DiskUsageScanConfig 磁盘使用率扫描配置
type DiskUsageScanConfig struct {
	// 是否启用递归扫描
	Enabled bool `yaml:"enabled"`

	// 扫描超时时间
	Timeout time.Duration `yaml:"timeout"`

	// 最大返回结果数量
	MaxResults int `yaml:"max_results"`

	// 最小文件大小阈值（字节）
	MinSizeThreshold uint64 `yaml:"min_size_threshold"`

	// 递归阈值（占父目录的比例，如0.7表示70%以上才深入）
	RecursiveThreshold float64 `yaml:"recursive_threshold"`

	// 最大递归深度
	MaxDepth int `yaml:"max_depth"`

	// 分布均匀度阈值（最大子项占比小于此值时停止递归）
	DistributionThreshold float64 `yaml:"distribution_threshold"`

	// 性能优化配置
	Performance *DiskScanPerformanceConfig `yaml:"performance,omitempty"`
}

// DiskScanPerformanceConfig 磁盘扫描性能配置
type DiskScanPerformanceConfig struct {
	// 最大并发数（0表示使用CPU核心数）
	MaxConcurrency int `yaml:"max_concurrency"`

	// 是否启用资源监控
	EnableResourceMonitor bool `yaml:"enable_resource_monitor"`

	// 内存使用限制（MB）
	MaxMemoryMB float64 `yaml:"max_memory_mb"`

	// 资源检查间隔
	ResourceCheckInterval time.Duration `yaml:"resource_check_interval"`

	// 单个目录最大扫描文件数
	MaxFilesPerDirectory int `yaml:"max_files_per_directory"`

	// 批处理大小
	BatchSize int `yaml:"batch_size"`
}

// AppMonitoringConfig 应用层类监控配置
type AppMonitoringConfig struct {
	// 是否启用应用监控
	Enabled bool `yaml:"enabled"` // 是否启用应用监控

	// Redis监控
	Redis *RedisConfig `yaml:"redis,omitempty"`
}

// HTTPMonitoringConfig HTTP接口监控配置
type HTTPMonitoringConfig struct {
	// 是否启用HTTP监控
	Enabled bool `yaml:"enabled"` // 是否启用HTTP监控

	// HTTP监控采集间隔
	CollectInterval time.Duration `yaml:"collect_interval"`

	Interfaces []HTTPInterface `yaml:"interfaces"`
}

// CertificateExpirationMonitoringConfig HTTPS证书过期监控配置
type CertificateExpirationMonitoringConfig struct {
	Enabled         bool                `yaml:"enabled"`
	CollectInterval time.Duration       `yaml:"collect_interval"`
	AlertTitle      string              `yaml:"alert_title"`
	WarningDays     int                 `yaml:"warning_days"`
	Domains         []CertificateDomain `yaml:"domains"`
	AlertLog        *AlertLogConfig     `yaml:"alert_log,omitempty"`
}

// CertificateDomain 单个证书检查目标
type CertificateDomain struct {
	Name    string `yaml:"name"`
	Port    int    `yaml:"port"`
	Enabled bool   `yaml:"enabled"`
}

type LogConfig struct {
	// 日志模式: file, console, both
	Mode string `yaml:"mode"`

	// 日志级别: debug, info, warn, error
	Level string `yaml:"level"`

	// 日志输出路径（文件模式时使用）
	Output string `yaml:"output"`

	// 是否启用日志轮转
	EnableRotation bool `yaml:"enable_rotation"`

	// 日志文件最大大小（MB）
	MaxSize int `yaml:"max_size"`

	// 日志文件保留天数
	MaxAge int `yaml:"max_age"`

	// 日志文件最大备份数量
	MaxBackups int `yaml:"max_backups"`
}

// RedisConnectionConfig 公共Redis连接配置
type RedisConnectionConfig struct {
	Addr         string        `yaml:"addr"`
	Password     string        `yaml:"password"`
	DB           int           `yaml:"db"`
	Timeout      time.Duration `yaml:"timeout"`
	PoolSize     int           `yaml:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns"`
	MaxIdleConns int           `yaml:"max_idle_conns"`
}

// RedisConfig Redis连接配置
type RedisConfig struct {
	// 是否启用Redis监控
	Enabled bool `yaml:"enabled"` // 是否启用Redis监控

	// Redis连接信息（如果未配置，则使用公共redis_connection配置）
	Addr         string        `yaml:"addr,omitempty"`
	Password     string        `yaml:"password,omitempty"`
	DB           int           `yaml:"db,omitempty"`
	Timeout      time.Duration `yaml:"timeout,omitempty"`
	PoolSize     int           `yaml:"pool_size,omitempty"`
	MinIdleConns int           `yaml:"min_idle_conns,omitempty"`
	MaxIdleConns int           `yaml:"max_idle_conns,omitempty"`

	// Redis监控阈值
	Thresholds RedisThresholds `yaml:"thresholds"`
}

// RedisThresholds Redis监控阈值配置
type RedisThresholds struct {
	Clients *RedisClientsThresholdConfig `yaml:"clients,omitempty"`
}

// RedisClientsThresholdConfig Redis客户端连接数阈值配置
type RedisClientsThresholdConfig struct {
	// 是否启用此指标的监控
	Enabled bool `yaml:"enabled"`

	// 最小连接数（低于告警）
	Min int `yaml:"min"`

	// 最大连接数（高于告警）
	Max int `yaml:"max"`

	// 用于 reminder 级别（>max 即提醒）
	Base float64 `yaml:"base"`

	// 等级阈值
	Levels RedisThresholdLevels `yaml:"levels"`
}

// RedisThresholdLevels Redis阈值等级
type RedisThresholdLevels struct {
	P3 int `yaml:"p3"`
	P2 int `yaml:"p2"`
	P1 int `yaml:"p1"`
}

// DingTalkConfig 钉钉配置
type DingTalkConfig struct {
	WebhookURL string   `yaml:"webhook_url"`
	Secret     string   `yaml:"secret"`
	AtMobiles  []string `yaml:"at_mobiles"`
}

// HTTPInterface HTTP接口监控配置
type HTTPInterface struct {
	Name         string        `yaml:"name"`
	URL          string        `yaml:"url"`
	Timeout      time.Duration `yaml:"request_timeout"`
	NeedAlert    bool          `yaml:"need_alert"`
	AllowedCodes []int         `yaml:"allowed_codes"`
}

// ScheduledPushConfig 全局定时推送配置
type ScheduledPushConfig struct {
	// 是否启用全局定时推送
	Enabled bool `yaml:"enabled"`

	// 运行模式: "client" 或 "server"
	Mode string `yaml:"mode"` // client: 上传数据到Redis, server: 从Redis聚合并发送

	// Redis 连接配置（用于 client/server 模式数据交换）
	// 如果未配置，则使用公共redis_connection配置
	RdsURL        string `yaml:"rds_url,omitempty"`
	RdsPassword   string `yaml:"rds_password,omitempty"`
	RdsDB         int    `yaml:"rds_db,omitempty"`
	RdsInstanceID string `yaml:"rds_instance_id,omitempty"`

	// 推送时间点列表，格式: ["7:00", "9:00", "11:00", "13:00", "15:00", "17:00", "19:00"]
	PushTimes []string `yaml:"push_times"`

	// 推送标题
	Title string `yaml:"title"`

	// Server模式聚合延迟时间（秒），用于等待所有Client上传完数据
	// 默认60秒，表示在推送时间点后延迟60秒再聚合
	ServerAggregationDelaySeconds int `yaml:"server_aggregation_delay_seconds"`

	// 数据日志存储配置
	DataLog *ScheduledPushDataLogConfig `yaml:"data_log,omitempty"`
}

// ScheduledPushDataLogConfig 定时推送数据日志存储配置
type ScheduledPushDataLogConfig struct {
	// 是否启用数据日志存储
	Enabled bool `yaml:"enabled"`

	// 客户端数据日志路径模板
	// 支持时间格式化：%y(年), %m(月), %d(日), %H(时), %M(分), %S(秒)
	// 示例: "logs/scheduled_push/client/%y/%m-%d/client-%H%M-%S.json"
	ClientPathTemplate string `yaml:"client_path_template"`

	// 服务器报告日志路径模板
	// 示例: "logs/scheduled_push/server/%y/%m-%d/report-%H%M-%S.md"
	ServerPathTemplate string `yaml:"server_path_template"`

	// 日志文件保留天数（超过此天数的文件将被自动清理）
	RetentionDays int `yaml:"retention_days"`
}
