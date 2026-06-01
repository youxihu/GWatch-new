// 定时推送数据仓库领域接口
package common

import (
	scheduledPushEntity "github.com/youxihu/GWatch-new/internal/domain/entity/scheduled_push"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"time"
)

// ClientDataRepository 客户端数据仓库接口（领域层）
type ClientDataRepository interface {
	// SaveClientData 保存客户端监控数据到 Redis
	SaveClientData(data *scheduledPushEntity.ClientMonitorData, ttl time.Duration) error

	// GetClientDataByKey 根据 key 获取客户端数据
	GetClientDataByKey(key string) (*scheduledPushEntity.ClientMonitorData, error)

	// GetAllClientDataKeys 获取所有客户端数据 key
	GetAllClientDataKeys() ([]string, error)

	// DeleteClientData 删除客户端数据
	DeleteClientData(key string) error

	// Init 初始化 Redis 连接
	Init(config *shared.Config) error
}
