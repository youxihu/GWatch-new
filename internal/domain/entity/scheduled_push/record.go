// Package entity 领域实体 - 定时推送记录
package entity

import "time"

// ScheduledPushAlertRecord 全局定时推送告警记录（领域实体）
type ScheduledPushAlertRecord struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Message    string     `json:"message"`
	Timestamp  time.Time  `json:"timestamp"`
	Source     string     `json:"source"`    // 固定为 "scheduled_push"
	Severity   string     `json:"severity"`  // 固定为 "info"
	PushTime   string     `json:"push_time"` // 推送时间点
	IsResolved bool       `json:"is_resolved"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// NewScheduledPushAlertRecord 创建新的全局定时推送告警记录
func NewScheduledPushAlertRecord(id, title, message, pushTime string) *ScheduledPushAlertRecord {
	now := time.Now()
	return &ScheduledPushAlertRecord{
		ID:         id,
		Title:      title,
		Message:    message,
		Timestamp:  now,
		Source:     "scheduled_push",
		Severity:   "info",
		PushTime:   pushTime,
		IsResolved: false,
	}
}

// Resolve 标记告警为已解决
func (r *ScheduledPushAlertRecord) Resolve() {
	now := time.Now()
	r.IsResolved = true
	r.ResolvedAt = &now
}
