package monitoring

import "time"

// CertificateInfo 证书信息值对象
type CertificateInfo struct {
	Domain   string
	Port     int
	NotAfter time.Time
}

// CertificateFetcher 证书拉取器接口
type CertificateFetcher interface {
	Fetch(domain string, port int, timeout time.Duration) (*CertificateInfo, error)
}