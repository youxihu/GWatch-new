// 定时推送调度器领域接口
package scheduled_push

import shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"

// ScheduledPushScheduler 全局定时推送调度器接口
type ScheduledPushScheduler interface {
	// Start 启动全局定时推送调度
	Start(config *shared.Config, stopCh <-chan struct{}) error

	// Stop 停止全局定时推送调度
	Stop() error

	// IsTimeToPush 检查是否到了推送时间
	IsTimeToPush(pushTimes []string) bool
}

// ScheduledPushUseCase 全局定时推送用例接口（统一接口，内部根据mode调用client或server）
type ScheduledPushUseCase interface {
	// RunScheduledPush 执行全局定时推送（根据 mode 决定是 client 还是 server 模式）
	RunScheduledPush(config *shared.Config) error
}
