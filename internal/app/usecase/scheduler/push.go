// 定时推送调度器
package scheduler

import (
	"fmt"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"github.com/youxihu/GWatch-new/internal/domain/scheduled_push"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	"time"
)

// ScheduledPushSchedulerImpl 全局定时推送调度器实现
type ScheduledPushSchedulerImpl struct {
	scheduledPushUseCase scheduled_push.ScheduledPushUseCase
	config               *shared.Config
	ticker               *time.Ticker
	stopCh               chan struct{}
	lastReported         map[string]time.Time // 记录每个时间点最后报告的时间
}

// NewScheduledPushScheduler 创建全局定时推送调度器
func NewScheduledPushScheduler(scheduledPushUseCase scheduled_push.ScheduledPushUseCase) scheduled_push.ScheduledPushScheduler {
	return &ScheduledPushSchedulerImpl{
		scheduledPushUseCase: scheduledPushUseCase,
		stopCh:               make(chan struct{}),
		lastReported:         make(map[string]time.Time),
	}
}

// Start 启动全局定时推送调度
func (sps *ScheduledPushSchedulerImpl) Start(config *shared.Config, stopCh <-chan struct{}) error {
	sps.config = config

	// 每10秒检查一次是否到了推送时间，提高响应速度
	sps.ticker = time.NewTicker(10 * time.Second)

	go func() {
		defer sps.ticker.Stop()

		// 启动时立即检查一次，避免错过定时任务时间
		mode := "推送"
		if config.ScheduledPush != nil && config.ScheduledPush.Mode == "server" {
			mode = "聚合"
		}
		logger.Infof("启动时检查全局定时任务时间... (模式: %s)", mode)
		sps.executeScheduledPushIfNeeded(config, fmt.Sprintf("启动时匹配到定时任务时间，立即执行%s", mode))

		for {
			select {
			case <-sps.ticker.C:
				// 检查是否到了定时任务时间
				mode := "推送"
				if config.ScheduledPush != nil && config.ScheduledPush.Mode == "server" {
					mode = "聚合"
				}
				sps.executeScheduledPushIfNeeded(config, fmt.Sprintf("定时器触发：开始执行%s", mode))
			case <-stopCh:
				logger.Info("全局定时推送调度器收到停止信号")
				return
			case <-sps.stopCh:
				logger.Info("全局定时推送调度器停止")
				return
			}
		}
	}()

	return nil
}

// executeScheduledPushIfNeeded 如果需要则执行全局定时推送
func (sps *ScheduledPushSchedulerImpl) executeScheduledPushIfNeeded(config *shared.Config, logPrefix string) {
	if config.ScheduledPush == nil || !config.ScheduledPush.Enabled {
		return
	}

	if sps.IsTimeToPush(config.ScheduledPush.PushTimes) {
		mode := config.ScheduledPush.Mode
		if mode == "" {
			mode = "client" // 默认是 client 模式
		}

		// Server模式：延迟执行聚合，等待所有Client上传完数据
		if mode == "server" {
			delaySeconds := config.ScheduledPush.ServerAggregationDelaySeconds
			if delaySeconds <= 0 {
				delaySeconds = 60 // 默认延迟60秒
			}

			logger.Infof("%s (Server模式，将延迟%d秒后聚合)", logPrefix, delaySeconds)

			// 异步延迟执行，避免阻塞调度器
			go func() {
				time.Sleep(time.Duration(delaySeconds) * time.Second)
				logger.Info("[Server模式] 延迟等待完成，开始聚合数据")
				if err := sps.scheduledPushUseCase.RunScheduledPush(config); err != nil {
					logger.Errorf("[Server模式] 执行聚合失败: %v", err)
				} else {
					logger.Info("[Server模式] 聚合报告发送成功")
				}
			}()
		} else {
			// Client模式：立即执行推送
			logger.Info(logPrefix)
			if err := sps.scheduledPushUseCase.RunScheduledPush(config); err != nil {
				logger.Errorf("[Client模式] 执行推送失败: %v", err)
			} else {
				logger.Info("[Client模式] 数据推送成功")
			}
		}
	}
}

// Stop 停止全局定时推送调度
func (sps *ScheduledPushSchedulerImpl) Stop() error {
	close(sps.stopCh)
	return nil
}

// IsTimeToPush 检查是否到了推送时间
func (sps *ScheduledPushSchedulerImpl) IsTimeToPush(pushTimes []string) bool {
	now := time.Now()
	currentTime := fmt.Sprintf("%d:%02d", now.Hour(), now.Minute())

	// 根据当前配置的模式决定日志用词
	mode := "推送"
	if sps.config != nil && sps.config.ScheduledPush != nil {
		if sps.config.ScheduledPush.Mode == "server" {
			mode = "聚合"
		}
	}

	// 只在匹配到时间时才输出日志，避免无意义的日志
	for _, pushTime := range pushTimes {
		if currentTime == pushTime {
			// 检查是否已经在这个时间点执行过
			if lastReported, exists := sps.lastReported[pushTime]; exists {
				// 如果上次执行时间与当前时间在同一分钟内，则不重复执行
				if now.Truncate(time.Minute).Equal(lastReported.Truncate(time.Minute)) {
					logger.Infof("时间点 %s 已在本分钟内执行过%s，跳过", pushTime, mode)
					return false
				}
			}

			logger.Infof("匹配到定时任务时间: %s (将执行%s)", pushTime, mode)
			// 记录执行时间
			sps.lastReported[pushTime] = now
			return true
		}
	}

	return false
}
