package security_monitoring

import (
	"sync"

	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	domainMonitor "github.com/youxihu/GWatch-new/internal/domain/monitoring"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"
	"github.com/youxihu/GWatch-new/internal/utils"
)

type Service struct {
	notifier          domainMonitor.Notifier
	alertStateStorage domainMonitor.AlertStateStorage
	eventIDGenerator  *utils.EventIDGenerator
	certFetcher       domainMonitor.CertificateFetcher

	mu              sync.Mutex
	active          map[string]struct{}
	logicalEventIDs map[string]string
}

func NewService(
	notifier domainMonitor.Notifier,
	alertStateStorage domainMonitor.AlertStateStorage,
	certFetcher domainMonitor.CertificateFetcher,
) *Service {
	return &Service{
		notifier:          notifier,
		alertStateStorage: alertStateStorage,
		eventIDGenerator:  utils.NewEventIDGenerator(),
		certFetcher:       certFetcher,
		active:            make(map[string]struct{}),
		logicalEventIDs:   make(map[string]string),
	}
}

func (s *Service) Start(cfg *shared.Config, stopCh <-chan struct{}) error {
	if !certEnabled(cfg) {
		return nil
	}
	if s.alertStateStorage != nil {
		if err := s.alertStateStorage.Init(cfg); err != nil {
			logger.Warnf("[安全监控] 初始化告警状态存储失败: %v", err)
		}
		if checker, ok := s.alertStateStorage.(utils.EventIDChecker); ok {
			s.eventIDGenerator.SetChecker(checker)
		}
		s.restoreActiveAlerts()
	}
	s.startCertificateLoop(cfg, stopCh)
	return nil
}

func certEnabled(cfg *shared.Config) bool {
	return cfg != nil && cfg.CertificateExpirationMonitoring != nil && cfg.CertificateExpirationMonitoring.Enabled
}
