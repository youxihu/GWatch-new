// 用例层共享接口
// 定义 usecase 层共享的接口
package usecase

import (
	monitoringEntity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
)

// RedisClient 是 redis 操作接口
type RedisClient interface {
	Init() error
	GetClients() (int, error)
	GetClientsDetail() ([]monitoringEntity.ClientInfo, error)
}

// Notifier 发送告警通知接口
type Notifier interface {
	Send(title, markdown string) error
}
