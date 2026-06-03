// Package entity 领域实体 - 监控告警类型
package entity

type AlertType string

// 所有告警类型的常量定义
const (
	CPUHigh         AlertType = "cpu_high"           // CPU过高
	CPUErr          AlertType = "cpu_error"          // CPU监控失败
	MemHigh         AlertType = "mem_high"           // 内存过高
	MemErr          AlertType = "mem_error"          // 内存监控失败
	DiskHigh        AlertType = "disk_high"          // 磁盘过高
	DiskErr         AlertType = "disk_error"         // 磁盘监控失败
	DiskIOReadHigh  AlertType = "disk_io_read_high"  // 磁盘读IO过高
	DiskIOWriteHigh AlertType = "disk_io_write_high" // 磁盘写IO过高
	RedisHigh       AlertType = "redis_high"         // Redis连接数过高
	RedisLow        AlertType = "redis_low"          // Redis连接数过低
	RedisErr        AlertType = "redis_error"        // Redis连接异常

	NetworkErr AlertType = "network_error" // 网络监控失败
	HTTPErr    AlertType = "http_error"    // HTTP接口监控失败
	Info       AlertType = "info"

	CertificateExpiring AlertType = "certificate_expiring"    // HTTPS证书即将过期
	CertificateCheckErr AlertType = "certificate_check_error" // HTTPS证书检查失败
)

// AlertTypeText 告警类型中文描述映射表
var AlertTypeText = map[AlertType]string{
	CPUHigh:             "CPU使用率过高",
	CPUErr:              "CPU监控失败",
	MemHigh:             "内存使用率过高",
	MemErr:              "内存监控失败",
	DiskHigh:            "磁盘使用率过高",
	DiskErr:             "磁盘监控失败",
	DiskIOReadHigh:      "磁盘读IO过高",
	DiskIOWriteHigh:     "磁盘写IO过高",
	RedisHigh:           "Redis连接数过高",
	RedisLow:            "Redis连接数过低",
	RedisErr:            "Redis连接异常",
	NetworkErr:          "网络监控失败",
	HTTPErr:             "HTTP接口监控失败",
	Info:                "信息",
	CertificateExpiring: "HTTPS证书即将过期",
	CertificateCheckErr: "HTTPS证书检查失败",
}

// 获取告警中文名
func (a AlertType) String() string {
	if text, exists := AlertTypeText[a]; exists {
		return text
	}
	return "未知告警"
}
