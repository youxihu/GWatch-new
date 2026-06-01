package security_monitoring

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	monitoringEntity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	domainMonitor "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	infraMonitor "github.com/youxihu/GWatch-new/internal/infra/monitoring"
)

// ──────────────────────────── 告警状态持久化 ────────────────────────────

func (s *Service) persistAlert(ev alertEvent, logCfg *shared.AlertLogConfig, markdown string, at time.Time) {
	logger.Debugf("[安全监控] persistAlert: setActive eventID=%s", ev.EventID)
	if err := s.setActive(ev, at); err != nil {
		logger.Warnf("[安全监控] 写入 Redis 告警状态失败: type=%s eventID=%s err=%v", ev.Type, ev.EventID, err)
	} else if s.alertStateStorage != nil {
		logger.Infof("[安全监控] 已写入 Redis 告警状态: type=%s eventID=%s", ev.Type, ev.EventID)
	}
	logger.Debugf("[安全监控] persistAlert: writeModuleAlertLog eventID=%s", ev.EventID)
	s.writeModuleAlertLog(logCfg, ev, markdown, false, at)
}

func (s *Service) writeModuleAlertLog(logCfg *shared.AlertLogConfig, ev alertEvent, markdown string, recovery bool, at time.Time) {
	if path, err := infraMonitor.SaveModuleLog(logCfg, ev.Title, markdown, ev.EventID, recovery, at); err != nil {
		logger.Warnf("[安全监控] 写入模块告警日志失败: type=%s eventID=%s err=%v", ev.Type, ev.EventID, err)
	} else if path != "" {
		logger.Infof("[安全监控] 已写入模块告警日志: %s", path)
	}
}

func (s *Service) restoreActiveAlerts() {
	states, err := s.alertStateStorage.GetAllAlertStates()
	if err != nil {
		logger.Warnf("[安全监控] 恢复告警状态失败: %v", err)
		return
	}
	restored := 0
	s.mu.Lock()
	for _, state := range states {
		if state == nil || !state.AlertSent || state.EventID == "" {
			continue
		}
		s.active[state.EventID] = struct{}{}
		restored++
	}
	s.mu.Unlock()
	if restored > 0 {
		logger.Infof("[安全监控] 已从 Redis 恢复 %d 条活跃告警状态", restored)
	}
}

// ──────────────────────────── 告警状态查询/写入 ────────────────────────────

func (s *Service) isActive(eventID string) bool {
	s.mu.Lock()
	_, inMemory := s.active[eventID]
	s.mu.Unlock()
	if inMemory {
		return true
	}
	if s.alertStateStorage == nil {
		return false
	}
	logger.Debugf("[安全监控] isActive: 查询 Redis eventID=%s", eventID)
	state, err := s.alertStateStorage.GetAlertStateByEventID(eventID)
	if err != nil {
		logger.Warnf("[安全监控] 查询告警状态失败: eventID=%s err=%v", eventID, err)
		return false
	}
	logger.Debugf("[安全监控] isActive: Redis 返回 eventID=%s alertSent=%v", eventID, state != nil && state.AlertSent)
	return state != nil && state.AlertSent
}

func (s *Service) getActiveSeverity(eventID string) string {
	if s.alertStateStorage == nil {
		return ""
	}
	state, err := s.alertStateStorage.GetAlertStateByEventID(eventID)
	if err != nil || state == nil || !state.AlertSent {
		return ""
	}
	return state.Severity
}

func (s *Service) setActive(ev alertEvent, now time.Time) error {
	s.mu.Lock()
	s.active[ev.EventID] = struct{}{}
	s.mu.Unlock()
	if s.alertStateStorage == nil {
		return nil
	}
	severity := ev.Severity
	if severity == "" {
		severity = "p2"
	}
	state := &domainMonitor.AlertState{
		EventID:           ev.EventID,
		AlertType:         ev.Type,
		StartTime:         now,
		Severity:          severity,
		SeverityStartTime: now,
		CurrentValue:      1,
		AlertSent:         true,
	}
	if err := s.alertStateStorage.SetAlertStateByEventID(state); err != nil {
		s.mu.Lock()
		delete(s.active, ev.EventID)
		s.mu.Unlock()
		return err
	}
	return nil
}

func (s *Service) deleteActive(eventID string) {
	s.mu.Lock()
	delete(s.active, eventID)
	s.mu.Unlock()
	if s.alertStateStorage == nil {
		return
	}
	if err := s.alertStateStorage.DeleteAlertStateByEventID(eventID); err != nil {
		logger.Warnf("[安全监控] 删除告警状态失败: eventID=%s err=%v", eventID, err)
	}
}

// ──────────────────────────── 事件ID ────────────────────────────

func (s *Service) eventIDFor(alertType monitoringEntity.AlertType, logicalKey string) string {
	s.mu.Lock()
	if eventID := s.logicalEventIDs[logicalKey]; eventID != "" {
		s.mu.Unlock()
		return eventID
	}
	s.mu.Unlock()

	eventID := stableSecurityEventID(string(alertType), logicalKey)
	s.mu.Lock()
	s.logicalEventIDs[logicalKey] = eventID
	s.mu.Unlock()
	return eventID
}

func stableSecurityEventID(alertType, logicalKey string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(alertType))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(logicalKey))
	suffix := h.Sum32() % 100000

	var prefix string
	var number int
	switch alertType {
	case "certificate_expiring":
		prefix, number = "t", 106
	case "certificate_check_error":
		prefix, number = "t", 107
	default:
		prefix, number = "x", 999
	}
	return fmt.Sprintf("%s%d%05d", prefix, number, suffix)
}

func stableEventKey(parts ...string) string {
	joined := strings.Join(parts, "_")
	replacer := strings.NewReplacer(" ", "_", ":", "_", "/", "_", "\\", "_", ".", "_")
	return replacer.Replace(strings.ToLower(joined))
}
