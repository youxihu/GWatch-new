package monitoring

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	domainMonitor "github.com/youxihu/GWatch-new/internal/domain/monitoring"
)

// BuildSecurityMarkdown 生成安全告警 markdown 内容
func BuildSecurityMarkdown(ev domainMonitor.AlertEvent, recovery bool, at time.Time) string {
	status := "告警"
	if recovery {
		status = "故障恢复"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## [%s] %s\n\n", status, ev.Title))
	b.WriteString(fmt.Sprintf("- 事件ID: %s\n", ev.EventID))
	if recovery {
		b.WriteString("- 告警等级: 恢复正常\n")
	} else if ev.Severity != "" {
		b.WriteString(fmt.Sprintf("- 告警等级: %s\n", GetSeverityText(ev.Severity)))
	} else {
		b.WriteString("- 告警等级: 提醒\n")
	}
	b.WriteString(fmt.Sprintf("- 触发条件: %s\n", ev.Condition))
	b.WriteString(fmt.Sprintf("- 触发对象: %s\n", ev.Object))
	for _, detail := range ev.Details {
		b.WriteString(fmt.Sprintf("- %s\n", detail))
	}
	b.WriteString(fmt.Sprintf("- 触发时间: %s\n", at.Format("2006-01-02 15:04:05")))
	return b.String()
}

// SaveModuleLog 将告警内容写入文件
func SaveModuleLog(logCfg *shared.AlertLogConfig, title, markdown, eventID string, recovery bool, at time.Time) (string, error) {
	if logCfg == nil {
		return "", fmt.Errorf("alert_log 配置为空")
	}
	if !logCfg.Enabled {
		return "", fmt.Errorf("alert_log 未启用")
	}
	if logCfg.LogPathTemplate == "" {
		return "", fmt.Errorf("alert_log.log_path_template 为空")
	}
	path := expandPathTemplate(logCfg.LogPathTemplate, at)
	if eventID != "" {
		path = strings.ReplaceAll(path, "{event_id}", eventID)
	}
	absPath, err := filepath.Abs(path)
	if err == nil {
		path = absPath
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("创建告警日志目录失败: %w", err)
	}
	kind := "告警"
	if recovery {
		kind = "恢复"
	}
	content := fmt.Sprintf("# %s - %s\n\n生成时间: %s\n\n%s", title, kind, at.Format("2006-01-02 15:04:05"), markdown)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("写入告警日志失败: %w", err)
	}
	return path, nil
}