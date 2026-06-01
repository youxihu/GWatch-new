// 定时推送客户端领域接口
package client

import shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"

// ClientUseCase 客户端模式用例接口
type ClientUseCase interface {
	// Run 执行客户端模式：收集数据并上传到 Redis
	Run(config *shared.Config) error
}
