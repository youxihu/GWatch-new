package monitoring

import (
	domaincfg "github.com/youxihu/GWatch-new/internal/domain/config"
	"github.com/youxihu/GWatch-new/internal/domain/monitoring"

	"github.com/youxihu/dingtalk/dingtalk"
)

// 基于 YouXiHu/dingtalk 的钉钉通知器实现
type DingTalkNotifier struct{ provider domaincfg.Provider }

func NewDingTalkNotifier(p domaincfg.Provider) monitoring.Notifier {
	return &DingTalkNotifier{provider: p}
}

func (d *DingTalkNotifier) Send(title string, markdown string) error {
	cfg := d.provider.GetConfig()
	if cfg == nil {
		return nil
	}
	return dingtalk.SendDingDingNotification(cfg.DingTalk.WebhookURL, cfg.DingTalk.Secret, title, markdown, cfg.DingTalk.AtMobiles, false) // isAtAll: false，不@所有人
}
