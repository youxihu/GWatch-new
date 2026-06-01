// 定时推送数据日志存储实现
package common

import (
	scheduledPushEntity "github.com/youxihu/GWatch-new/internal/domain/entity/scheduled_push"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"github.com/youxihu/GWatch-new/internal/domain/scheduled_push/common"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ScheduledPushDataLogStorageImpl 定时推送数据日志存储实现
type ScheduledPushDataLogStorageImpl struct {
	config                      *shared.Config
	clientDataLogPathTemplate   string
	serverReportLogPathTemplate string
	retentionDays               int
	baseDir                     string // 日志文件基础目录
}

// NewScheduledPushDataLogStorage 创建数据日志存储服务
func NewScheduledPushDataLogStorage() common.ScheduledPushDataLogStorage {
	return &ScheduledPushDataLogStorageImpl{}
}

// Init 初始化存储服务
func (s *ScheduledPushDataLogStorageImpl) Init(config *shared.Config) error {
	s.config = config

	if config.ScheduledPush == nil || config.ScheduledPush.DataLog == nil {
		return nil // 未配置数据日志，不需要初始化
	}

	dataLogConfig := config.ScheduledPush.DataLog
	if !dataLogConfig.Enabled {
		return nil // 未启用数据日志
	}

	s.clientDataLogPathTemplate = dataLogConfig.ClientPathTemplate
	s.serverReportLogPathTemplate = dataLogConfig.ServerPathTemplate
	s.retentionDays = dataLogConfig.RetentionDays
	if s.retentionDays <= 0 {
		s.retentionDays = 30 // 默认保留30天
	}

	// 确定基础目录（从模板路径中提取到scheduled_push目录）
	// 例如：从 "logs/scheduled_push/client/%y/%m-%d/client-%H%M-%S.json"
	// 提取到 "logs/scheduled_push"
	templatePath := s.clientDataLogPathTemplate
	if templatePath == "" {
		templatePath = s.serverReportLogPathTemplate
	}
	if templatePath != "" {
		if idx := strings.Index(templatePath, "scheduled_push"); idx != -1 {
			// 提取到scheduled_push的父目录
			s.baseDir = templatePath[:idx+len("scheduled_push")]
		} else {
			// 如果没有找到scheduled_push，使用模板路径的目录部分
			s.baseDir = filepath.Dir(templatePath)
		}
	}

	return nil
}

// SaveClientData 保存客户端监控数据到日志文件
func (s *ScheduledPushDataLogStorageImpl) SaveClientData(data *scheduledPushEntity.ClientMonitorData, timestamp time.Time) error {
	if s.config == nil || s.config.ScheduledPush == nil ||
		s.config.ScheduledPush.DataLog == nil || !s.config.ScheduledPush.DataLog.Enabled {
		return nil // 未启用数据日志存储
	}

	if s.clientDataLogPathTemplate == "" {
		return nil // 未配置客户端日志路径模板
	}

	// 展开路径模板
	filePath := expandPathTemplate(s.clientDataLogPathTemplate, timestamp)

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}

	// 序列化为JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化数据失败: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入日志文件失败: %v", err)
	}

	return nil
}

// SaveServerReport 保存服务器聚合报告到日志文件
func (s *ScheduledPushDataLogStorageImpl) SaveServerReport(report string, title string, timestamp time.Time) error {
	if s.config == nil || s.config.ScheduledPush == nil ||
		s.config.ScheduledPush.DataLog == nil || !s.config.ScheduledPush.DataLog.Enabled {
		return nil // 未启用数据日志存储
	}

	if s.serverReportLogPathTemplate == "" {
		return nil // 未配置服务器报告日志路径模板
	}

	// 展开路径模板
	filePath := expandPathTemplate(s.serverReportLogPathTemplate, timestamp)

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}

	// 构建完整的报告内容（包含标题和时间戳）
	reportContent := fmt.Sprintf("# %s\n\n生成时间: %s\n\n%s",
		title, timestamp.Format("2006-01-02 15:04:05"), report)

	// 写入文件
	if err := os.WriteFile(filePath, []byte(reportContent), 0644); err != nil {
		return fmt.Errorf("写入报告文件失败: %v", err)
	}

	return nil
}

// CleanupOldLogs 清理过期日志
func (s *ScheduledPushDataLogStorageImpl) CleanupOldLogs() error {
	if s.config == nil || s.config.ScheduledPush == nil ||
		s.config.ScheduledPush.DataLog == nil || !s.config.ScheduledPush.DataLog.Enabled {
		return nil // 未启用数据日志存储
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
func (s *ScheduledPushDataLogStorageImpl) cleanupEmptyDirs(dir string) {
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
