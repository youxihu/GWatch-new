package security_monitoring

import (
	"fmt"
	"strings"
	"time"

	monitoringEntity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
)

func (s *Service) startCertificateLoop(cfg *shared.Config, stopCh <-chan struct{}) {
	interval := cfg.CertificateExpirationMonitoring.CollectInterval
	if interval <= 0 {
		interval = 12 * time.Hour
	}
	logger.Infof("[证书监控] 已启用，检查间隔: %v", interval)
	go func() {
		run := func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Errorf("[证书监控] 检查过程发生 panic: %v", r)
				}
			}()
			s.runCertificateCheck(cfg)
		}
		go run()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				go run()
			case <-stopCh:
				logger.Info("[证书监控] 收到停止信号")
				return
			}
		}
	}()
}

func (s *Service) runCertificateCheck(cfg *shared.Config) {
	conf := cfg.CertificateExpirationMonitoring
	if conf == nil {
		logger.Warn("[证书监控] 配置为空，跳过检查")
		return
	}

	warningDays := conf.WarningDays
	if warningDays <= 0 {
		warningDays = 15
	}

	logger.Infof("[证书监控] 开始检查，目标域名数: %d", len(conf.Domains))

	for _, d := range conf.Domains {
		if !d.Enabled || strings.TrimSpace(d.Name) == "" {
			continue
		}

		domain := d.Name
		port := d.Port
		if port <= 0 {
			port = 443
		}
		object := fmt.Sprintf("%s", domain)

		// 1️⃣ 拉证书（同步）
		logger.Infof("[证书监控] 正在检查 %s ...", object)
		info, err := s.certFetcher.Fetch(domain, port, 10*time.Second)
		if err != nil {
			logger.Warnf("[证书监控] %s 检查失败: %v", object, err)

			// 2️⃣ 检查失败告警
			s.emitAlert(cfg, conf.AlertLog, alertEvent{
				Type:      monitoringEntity.CertificateCheckErr,
				EventID:   s.eventIDFor(monitoringEntity.CertificateCheckErr, stableEventKey("certificate_check_error", domain)),
				Severity:  "reminder",
				Title:     conf.AlertTitle,
				Object:    object,
				Condition: "TLS连接或证书读取失败",
				Details:   []string{fmt.Sprintf("错误: %v", err)},
			})
			continue
		}

		// 3️⃣ 算剩余天数
		remaining := time.Until(info.NotAfter)
		remainingDays := int(remaining.Hours() / 24)

		logger.Infof(
			"[证书监控] %s 到期时间: %s，剩余 %d 天",
			object,
			info.NotAfter.Format(time.DateTime),
			remainingDays,
		)

		// 4️⃣ 根据天数 → 即将过期告警
		if remainingDays <= warningDays {
			s.emitAlert(cfg, conf.AlertLog, alertEvent{
				Type:      monitoringEntity.CertificateExpiring,
				EventID:   s.eventIDFor(monitoringEntity.CertificateExpiring, stableEventKey("certificate_expiring", domain)),
				Severity:  "reminder",
				Title:     conf.AlertTitle,
				Object:    object,
				Condition: fmt.Sprintf("证书剩余天数 ≤ %d 天", warningDays),
				Details: []string{
					fmt.Sprintf("到期时间: %s", info.NotAfter.Format(time.DateTime)),
					fmt.Sprintf("剩余天数: %d", remainingDays),
				},
			})
		}
	}

	logger.Infof("[证书监控] 本轮检查完成")
}
