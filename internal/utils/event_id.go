package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// EventIDChecker 事件ID存在性检查接口
type EventIDChecker interface {
	EventIDExists(eventID string) (bool, error)
}

// EventIDFinder 事件ID查找接口
type EventIDFinder interface {
	FindExistingEventByProcessForEventID(alertType string, processName string) (string, error)
}

// EventIDGenerator 事件ID生成器
// 格式: {类型}_{等级}_{时间分钟}_{6位hex}_{次数}
// 示例: cpu_high_p1_202606011530_a3f2c1_1
type EventIDGenerator struct {
	checker EventIDChecker
	finder  EventIDFinder
}

// NewEventIDGenerator 创建事件ID生成器
func NewEventIDGenerator() *EventIDGenerator {
	return &EventIDGenerator{}
}

// SetChecker 设置事件ID检查器
func (g *EventIDGenerator) SetChecker(checker EventIDChecker) {
	g.checker = checker
}

// SetFinder 设置事件ID查找器
func (g *EventIDGenerator) SetFinder(finder EventIDFinder) {
	g.finder = finder
}

// GenerateEventID 生成事件ID
// alertType: 告警类型（cpu_high, mem_high 等）
// severity: 告警等级（p1, p2, p3, reminder）
// 格式: {类型}_{等级}_{YYYYMMDDHHMM}_{6位随机hex}_1
func (g *EventIDGenerator) GenerateEventID(alertType string, severity string) string {
	now := time.Now()
	timeStr := now.Format("200601021504")

	// 6位随机 hex（碰撞概率 16^6 ≈ 1600万分之一）
	hash := randomHex(6)

	return fmt.Sprintf("%s_%s_%s_%s_1", alertType, severity, timeStr, hash)
}

// FindOrGenerateEventID 查找现有事件ID或生成新的事件ID
func (g *EventIDGenerator) FindOrGenerateEventID(alertType string, severity string, processName string, maxRetries int) (string, bool, error) {
	if g.finder != nil && processName != "" {
		existingEventID, err := g.finder.FindExistingEventByProcessForEventID(alertType, processName)
		if err != nil {
			// 查找失败，继续生成新ID
		} else if existingEventID != "" {
			return existingEventID, true, nil
		}
	}

	newEventID, err := g.GenerateUniqueEventID(alertType, severity, maxRetries)
	if err != nil {
		return "", false, err
	}
	return newEventID, false, nil
}

// GenerateUniqueEventID 生成唯一的事件ID（带冲突检测）
func (g *EventIDGenerator) GenerateUniqueEventID(alertType string, severity string, maxRetries int) (string, error) {
	if maxRetries <= 0 {
		maxRetries = 3
	}

	for i := 0; i < maxRetries; i++ {
		eventID := g.GenerateEventID(alertType, severity)

		if g.checker == nil {
			return eventID, nil
		}

		exists, err := g.checker.EventIDExists(eventID)
		if err != nil {
			continue
		}

		if !exists {
			return eventID, nil
		}

		if i < maxRetries-1 {
			time.Sleep(time.Second)
		}
	}

	return "", fmt.Errorf("生成唯一事件ID失败，重试%d次后仍有冲突", maxRetries)
}

// randomHex 生成指定长度的随机 hex 字符串
func randomHex(n int) string {
	b := make([]byte, n/2+1)
	rand.Read(b)
	return hex.EncodeToString(b)[:n]
}