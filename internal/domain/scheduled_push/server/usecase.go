// 定时推送服务端领域接口
package server

import shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"

// ServerUseCase 服务端模式用例接口
type ServerUseCase interface {
	// Run 执行服务端模式：从 Redis 读取数据并聚合成报告发送
	Run(config *shared.Config) error
}
