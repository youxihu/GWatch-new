package utils

import (
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
type EventIDGenerator struct {
	checker EventIDChecker // 用于检查事件ID是否已存在
	finder  EventIDFinder  // 用于查找现有事件ID
}

// NewEventIDGenerator 创建事件ID生成器
func NewEventIDGenerator() *EventIDGenerator {
	return &EventIDGenerator{}
}

// SetChecker 设置事件ID检查器（用于依赖注入）
func (g *EventIDGenerator) SetChecker(checker EventIDChecker) {
	g.checker = checker
}

// SetFinder 设置事件ID查找器（用于依赖注入）
func (g *EventIDGenerator) SetFinder(finder EventIDFinder) {
	g.finder = finder
}

// GenerateEventID 生成事件ID
// alertType: 告警类型（cpu_high, mem_high等）
// 返回: 事件ID，格式为：前缀+编号+时间戳后5位
func (g *EventIDGenerator) GenerateEventID(alertType string) string {
	// 获取时间戳后5位
	timestamp := time.Now().Unix()
	last5Digits := timestamp % 100000

	// 根据告警类型生成前缀和编号
	var prefix string
	var number int

	switch alertType {
	case "cpu_high":
		prefix = "c"
		number = 101
	case "mem_high":
		prefix = "m"
		number = 102
	case "disk_high":
		prefix = "d"
		number = 103
	case "redis_high", "redis_low", "redis_error":
		prefix = "r"
		number = 104
	case "http_error":
		prefix = "h"
		number = 105
	case "certificate_expiring":
		prefix = "t"
		number = 106
	case "certificate_check_error":
		prefix = "t"
		number = 107
		number = 108
	default:
		prefix = "x"
		number = 999
	}

	return fmt.Sprintf("%s%d%05d", prefix, number, last5Digits)
}

// FindOrGenerateEventID 查找现有事件ID或生成新的事件ID
// 用于事件ID复用：相同进程名的告警应该使用相同的事件ID
func (g *EventIDGenerator) FindOrGenerateEventID(alertType string, processName string, maxRetries int) (string, bool, error) {
	// 首先尝试查找现有的事件ID
	if g.finder != nil && processName != "" {
		existingEventID, err := g.finder.FindExistingEventByProcessForEventID(alertType, processName)
		if err != nil {
			// 查找失败，记录错误但继续生成新ID
			// 这里不返回错误，因为生成新ID是备选方案
		} else if existingEventID != "" {
			return existingEventID, true, nil // 找到现有事件ID，返回复用标志
		}
	}

	// 没有找到现有事件ID，生成新的
	newEventID, err := g.GenerateUniqueEventID(alertType, maxRetries)
	if err != nil {
		return "", false, err
	}

	return newEventID, false, nil // 返回新生成的事件ID，未复用标志
}

// GenerateUniqueEventID 生成唯一的事件ID（带冲突检测）
func (g *EventIDGenerator) GenerateUniqueEventID(alertType string, maxRetries int) (string, error) {
	if maxRetries <= 0 {
		maxRetries = 3 // 默认重试3次
	}

	for i := 0; i < maxRetries; i++ {
		eventID := g.GenerateEventID(alertType)

		// 如果没有设置检查器，直接返回（向后兼容）
		if g.checker == nil {
			return eventID, nil
		}

		// 检查事件ID是否已存在
		exists, err := g.checker.EventIDExists(eventID)
		if err != nil {
			// 检查失败，记录错误但继续尝试
			continue
		}

		if !exists {
			return eventID, nil
		}

		// 如果冲突，等待1秒后重试（让时间戳变化）
		if i < maxRetries-1 {
			time.Sleep(time.Second)
		}
	}

	return "", fmt.Errorf("生成唯一事件ID失败，重试%d次后仍有冲突", maxRetries)
}
