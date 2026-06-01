// Package monitoring 领域层 — 告警事件定义
package monitoring

import (
	monitoringEntity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
)

// AlertEvent 告警事件（DDD 领域类型，use case 和 infra 层共用）
type AlertEvent struct {
	Type      monitoringEntity.AlertType
	EventID   string
	Severity  string
	Title     string
	Object    string
	Condition string
	Details   []string
}