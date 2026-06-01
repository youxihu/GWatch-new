package monitoring

import (
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	domainMonitor "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	"sync"
	"time"
)

// Coordinator 负责在用例层承接调度、补采与合并通知的逻辑
type Coordinator struct {
	runnerBase *MonitoringUseCase
	runnerHTTP *MonitoringUseCase
	policyBase domainMonitor.Policy
	policyHTTP domainMonitor.Policy

	mu         sync.RWMutex
	latestBase *entity.SystemMetrics
	latestHTTP *entity.SystemMetrics
}

func NewCoordinator(
	runnerBase, runnerHTTP *MonitoringUseCase,
	policyBase, policyHTTP domainMonitor.Policy,
) *Coordinator {
	return &Coordinator{
		runnerBase: runnerBase,
		runnerHTTP: runnerHTTP,
		policyBase: policyBase,
		policyHTTP: policyHTTP,
	}
}

// cycleConfig 参数化双周期的差异
type cycleConfig struct {
	collect      func(*shared.Config) *entity.SystemMetrics
	crossCollect func(*shared.Config) *entity.SystemMetrics
	latest       **entity.SystemMetrics
	crossLatest  **entity.SystemMetrics
	runner       *MonitoringUseCase
	isBase       bool
}

// RunWithIntervals 启动双周期调度，stopCh 关闭时退出
func (c *Coordinator) RunWithIntervals(cfg *shared.Config, stopCh <-chan struct{}) {
	baseInterval := 5 * time.Second
	if cfg.HostMonitoring != nil && cfg.HostMonitoring.CollectInterval > 0 {
		baseInterval = cfg.HostMonitoring.CollectInterval
	}

	httpEnabled := cfg.HTTPMonitoring != nil && cfg.HTTPMonitoring.Enabled
	httpInterval := 10 * time.Second
	if httpEnabled && cfg.HTTPMonitoring.CollectInterval > 0 {
		httpInterval = cfg.HTTPMonitoring.CollectInterval
	}

	// 首次采集
	c.latestBase = c.runnerBase.CollectBaseOnce(cfg)
	if httpEnabled {
		c.latestHTTP = c.runnerHTTP.CollectHTTPOnce(cfg)
	}

	var wg sync.WaitGroup

	// 基础周期
	baseCfg := &cycleConfig{
		collect:      c.runnerBase.CollectBaseOnce,
		crossCollect: c.runnerHTTP.CollectHTTPOnce,
		latest:       &c.latestBase,
		crossLatest:  &c.latestHTTP,
		runner:       c.runnerBase,
		isBase:       true,
	}
	wg.Add(1)
	go func() { defer wg.Done(); c.runCycle(cfg, baseCfg, baseInterval, httpEnabled, stopCh) }()

	// HTTP 周期
	if httpEnabled {
		httpCfg := &cycleConfig{
			collect:      c.runnerHTTP.CollectHTTPOnce,
			crossCollect: c.runnerBase.CollectBaseOnce,
			latest:       &c.latestHTTP,
			crossLatest:  &c.latestBase,
			runner:       c.runnerHTTP,
			isBase:       false,
		}
		wg.Add(1)
		go func() { defer wg.Done(); c.runCycle(cfg, httpCfg, httpInterval, httpEnabled, stopCh) }()
	}

	<-stopCh
	wg.Wait()
}

func (c *Coordinator) runCycle(
	cfg *shared.Config,
	cc *cycleConfig,
	interval time.Duration,
	httpEnabled bool,
	stopCh <-chan struct{},
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			primary := cc.collect(cfg)
			c.mu.Lock()
			*cc.latest = primary
			merged := CombineMetrics(c.latestBase, c.latestHTTP)
			c.mu.Unlock()

			if httpEnabled && c.wouldTrigger(cfg, merged, cc.isBase) {
				cross := cc.crossCollect(cfg)
				c.mu.Lock()
				*cc.crossLatest = cross
				merged = CombineMetrics(c.latestBase, c.latestHTTP)
				c.mu.Unlock()

				decisions := c.evaluate(cfg, merged)
				var results []domainMonitor.AlertResult
				if cc.isBase {
					baseResults := c.policyBase.Apply(cfg, merged, filterNonHTTP(decisions))
					httpResults := c.policyHTTP.Apply(cfg, merged, filterOnlyHTTP(decisions))
					results = unionAlertResults(baseResults, httpResults)
				} else {
					httpResults := c.policyHTTP.Apply(cfg, merged, filterOnlyHTTP(decisions))
					baseResults := c.policyBase.Apply(cfg, merged, filterNonHTTP(decisions))
					results = unionAlertResults(baseResults, httpResults)
				}
				_ = cc.runner.NotifyWithAlertResults(cfg, merged, results)
				continue
			}
			cc.runner.PrintMetrics(cfg, merged)
			if cc.isBase {
				_ = c.runnerBase.EvaluateAndNotifyBaseOnly(cfg, merged)
			} else {
				_ = c.runnerHTTP.EvaluateAndNotifyHTTPOnly(cfg, merged)
			}
		case <-stopCh:
			return
		}
	}
}

func (c *Coordinator) evaluate(cfg *shared.Config, m *entity.SystemMetrics) []domainMonitor.Decision {
	dec, _ := c.runnerBase.evaluator.Evaluate(cfg, m)
	return dec
}

// wouldTrigger 使用已有 policy 的 PeekApply 预判
func (c *Coordinator) wouldTrigger(cfg *shared.Config, m *entity.SystemMetrics, base bool) bool {
	decisions := c.evaluate(cfg, m)
	if base {
		alerts := c.policyBase.PeekApply(cfg, m, filterNonHTTP(decisions))
		return len(alerts) > 0
	}
	alerts := c.policyHTTP.PeekApply(cfg, m, filterOnlyHTTP(decisions))
	return len(alerts) > 0
}

func filterOnlyHTTP(decisions []domainMonitor.Decision) []domainMonitor.Decision {
	res := make([]domainMonitor.Decision, 0, len(decisions))
	for _, d := range decisions {
		if d.Type == entity.HTTPErr {
			res = append(res, d)
		}
	}
	return res
}

func filterNonHTTP(decisions []domainMonitor.Decision) []domainMonitor.Decision {
	res := make([]domainMonitor.Decision, 0, len(decisions))
	for _, d := range decisions {
		if d.Type != entity.HTTPErr {
			res = append(res, d)
		}
	}
	return res
}

func unionAlertResults(a, b []domainMonitor.AlertResult) []domainMonitor.AlertResult {
	return append(a, b...)
}