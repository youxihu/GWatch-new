package monitoring

import (
	monitoringEntity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"github.com/youxihu/GWatch-new/internal/domain/monitoring"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AlertLogStorageImpl 告警日志存储实现
type AlertLogStorageImpl struct {
	config          *shared.Config
	logPathTemplate string
	retentionDays   int
	baseDir         string // 日志文件基础目录
}

// NewAlertLogStorage 创建告警日志存储服务
func NewAlertLogStorage() monitoring.AlertLogStorage {
	return &AlertLogStorageImpl{}
}

// Init 初始化存储服务
func (s *AlertLogStorageImpl) Init(config *shared.Config) error {
	s.config = config

	if config.HostMonitoring == nil || config.HostMonitoring.AlertLog == nil {
		return nil // 未配置告警日志，不需要初始化
	}

	alertLogConfig := config.HostMonitoring.AlertLog
	if !alertLogConfig.Enabled {
		return nil // 未启用告警日志
	}

	s.logPathTemplate = alertLogConfig.LogPathTemplate
	s.retentionDays = alertLogConfig.RetentionDays
	if s.retentionDays <= 0 {
		s.retentionDays = 30 // 默认保留30天
	}

	// 确定基础目录（从模板路径中提取到alerts目录）
	// 例如：从 "logs/alerts/%y/%m-%d/alert-%H%M-%S.md"
	// 提取到 "logs/alerts"
	templatePath := s.logPathTemplate
	if templatePath != "" {
		if idx := strings.Index(templatePath, "alerts"); idx != -1 {
			// 提取到alerts的父目录
			s.baseDir = templatePath[:idx+len("alerts")]
		} else {
			// 如果没有找到alerts，使用模板路径的目录部分
			s.baseDir = filepath.Dir(templatePath)
		}
	}

	return nil
}

// SaveAlert 保存告警信息到日志文件
func (s *AlertLogStorageImpl) SaveAlert(title string, markdown string, metrics *monitoringEntity.SystemMetrics, alertTypes []monitoringEntity.AlertType, timestamp time.Time) error {
	// 如果未初始化，尝试使用传入的config（如果可能）
	if s.config == nil || s.config.HostMonitoring == nil ||
		s.config.HostMonitoring.AlertLog == nil || !s.config.HostMonitoring.AlertLog.Enabled {
		logger.Infof("告警日志存储未启用或未配置: config=%v", s.config != nil)
		return nil // 未启用告警日志存储
	}

	if s.logPathTemplate == "" {
		logger.Info("告警日志路径模板为空")
		return nil // 未配置日志路径模板
	}

	// 展开路径模板
	filePath := expandPathTemplate(s.logPathTemplate, timestamp)

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建告警日志目录失败: %v", err)
	}

	// 构建完整的告警内容（包含标题和时间戳）
	alertContent := fmt.Sprintf("# %s\n\n生成时间: %s\n\n%s",
		title, timestamp.Format("2006-01-02 15:04:05"), markdown)

	// 写入文件
	if err := os.WriteFile(filePath, []byte(alertContent), 0644); err != nil {
		return fmt.Errorf("写入告警日志文件失败: %v", err)
	}

	// 记录成功日志
	logger.Infof("已保存告警日志: %s", filePath)

	return nil
}

// CleanupOldLogs 清理过期日志
func (s *AlertLogStorageImpl) CleanupOldLogs() error {
	if s.config == nil || s.config.HostMonitoring == nil ||
		s.config.HostMonitoring.AlertLog == nil || !s.config.HostMonitoring.AlertLog.Enabled {
		return nil // 未启用告警日志存储
	}

	if s.baseDir == "" {
		return nil // 基础目录未设置
	}

	// 计算过期时间
	cutoffTime := time.Now().AddDate(0, 0, -s.retentionDays)

	// 遍历目录删除过期文件
	err := filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误，继续遍历
		}

		if info.IsDir() {
			return nil // 跳过目录
		}

		// 检查文件是否过期
		if info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err != nil {
				// 记录错误但不中断清理
				return nil
			}
		}

		return nil
	})

	// 清理空目录
	s.cleanupEmptyDirs(s.baseDir)

	return err
}

// cleanupEmptyDirs 清理空目录
func (s *AlertLogStorageImpl) cleanupEmptyDirs(dir string) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}

		// 检查目录是否为空
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil
		}

		if len(entries) == 0 {
			os.Remove(path) // 删除空目录
		}

		return nil
	})
}

// expandPathTemplate 展开路径模板
// 支持的时间格式化占位符：%y(年), %m(月), %d(日), %H(时), %M(分), %S(秒)
func expandPathTemplate(template string, t time.Time) string {
	result := template
	result = strings.ReplaceAll(result, "%y", fmt.Sprintf("%04d", t.Year()))
	result = strings.ReplaceAll(result, "%m", fmt.Sprintf("%02d", int(t.Month())))
	result = strings.ReplaceAll(result, "%d", fmt.Sprintf("%02d", t.Day()))
	result = strings.ReplaceAll(result, "%H", fmt.Sprintf("%02d", t.Hour()))
	result = strings.ReplaceAll(result, "%M", fmt.Sprintf("%02d", t.Minute()))
	result = strings.ReplaceAll(result, "%S", fmt.Sprintf("%02d", t.Second()))
	return result
}

// SaveAlertWithEventID 按事件ID保存告警或恢复信息到日志文件（用于日志追溯）
// 保存路径：logs/alert/{server|client}/{年}/{月-日}/{事件ID}/alert.md 或 recovery.md
func (s *AlertLogStorageImpl) SaveAlertWithEventID(title string, markdown string, eventID string, isRecovery bool, timestamp time.Time) error {
	if s.config == nil || s.config.HostMonitoring == nil ||
		s.config.HostMonitoring.AlertLog == nil || !s.config.HostMonitoring.AlertLog.Enabled {
		return nil // 未启用告警日志存储
	}

	if eventID == "" {
		return nil // 没有事件ID，跳过
	}

	// 确定模式（server或client）
	mode := "server"
	if s.config.ScheduledPush != nil && s.config.ScheduledPush.Enabled && s.config.ScheduledPush.Mode == "client" {
		mode = "client"
	}

	// 构建路径：logs/alert/{server|client}/{年}/{月-日}/{事件ID}/alert.md 或 recovery.md
	year := fmt.Sprintf("%04d", timestamp.Year())
	monthDay := fmt.Sprintf("%02d-%02d", int(timestamp.Month()), timestamp.Day())

	fileName := "alert.md"
	if isRecovery {
		fileName = "recovery.md"
	}

	// 构建完整路径
	eventDir := filepath.Join("logs", "alert", mode, year, monthDay, eventID)
	filePath := filepath.Join(eventDir, fileName)

	// 确保目录存在
	if err := os.MkdirAll(eventDir, 0755); err != nil {
		return fmt.Errorf("创建告警日志目录失败: %v", err)
	}

	// 构建完整的告警/恢复内容
	contentType := "告警"
	if isRecovery {
		contentType = "恢复"
	}
	alertContent := fmt.Sprintf("# %s - %s\n\n**事件ID**: %s\n\n**时间**: %s\n\n%s",
		title, contentType, eventID, timestamp.Format("2006-01-02 15:04:05"), markdown)

	// 写入文件
	if err := os.WriteFile(filePath, []byte(alertContent), 0644); err != nil {
		return fmt.Errorf("写入告警日志文件失败: %v", err)
	}

	// 记录成功日志
	logger.Infof("已保存%s日志（事件ID: %s）: %s", contentType, eventID, filePath)

	return nil
}
