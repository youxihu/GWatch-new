package security_monitoring

import (
	"time"

	monitoringEntity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	domainMonitor "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	infraMonitor "github.com/youxihu/GWatch-new/internal/infra/monitoring"
)

// alertEvent 类型别名，使用领域层定义
type alertEvent = domainMonitor.AlertEvent

// ──────────────────────────── 告警发送 ────────────────────────────

func (s *Service) emitAlert(cfg *shared.Config, logCfg *shared.AlertLogConfig, ev alertEvent, escalate ...bool) {
	allowEscalate := len(escalate) > 0 && escalate[0]
	if !allowEscalate && s.isActive(ev.EventID) {
		logger.Debugf("[安全监控] 告警已激活，跳过重复通知: type=%s eventID=%s", ev.Type, ev.EventID)
		return
	}
	now := time.Now()
	markdown := infraMonitor.BuildSecurityMarkdown(ev, false, now)
	logger.Debugf("[安全监控] emitAlert: 开始持久化 eventID=%s", ev.EventID)
	s.persistAlert(ev, logCfg, markdown, now)
	logger.Debugf("[安全监控] emitAlert: 持久化完成 eventID=%s，检查通知配置", ev.EventID)
	if cfg.EnableNotification && s.notifier != nil {
		title, body := ev.Title, markdown
		go func() {
			if err := s.notifier.Send(title, body); err != nil {
				logger.Errorf("[安全监控] 发送通知失败: type=%s eventID=%s err=%v", ev.Type, ev.EventID, err)
				return
			}
			logger.Infof("[安全监控] 已发送通知: type=%s eventID=%s title=%s", ev.Type, ev.EventID, title)
		}()
	}
	logger.Debugf("[安全监控] emitAlert: 完成 eventID=%s", ev.EventID)
}

func (s *Service) recoverIfActive(cfg *shared.Config, logCfg *shared.AlertLogConfig, alertType monitoringEntity.AlertType, eventID, title, message string) {
	if !s.isActive(eventID) {
		return
	}
	now := time.Now()
	ev := alertEvent{Type: alertType, EventID: eventID, Title: title, Object: "-", Condition: "恢复正常", Details: []string{message}}
	markdown := infraMonitor.BuildSecurityMarkdown(ev, true, now)
	s.writeModuleAlertLog(logCfg, ev, markdown, true, now)
	if cfg.EnableNotification && s.notifier != nil {
		go func() {
			if err := s.notifier.Send(title, markdown); err != nil {
				logger.Errorf("[安全监控] 发送恢复通知失败: eventID=%s err=%v", eventID, err)
				return
			}
			logger.Infof("[安全监控] 已发送恢复通知: eventID=%s title=%s", eventID, title)
		}()
	}
	s.deleteActive(eventID)
}
