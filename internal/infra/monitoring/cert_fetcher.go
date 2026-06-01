package monitoring

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	domainMonitor "github.com/youxihu/GWatch-new/internal/domain/monitoring"
)

// TLSCertificateFetcher 通过 TLS 握手拉取证书
type TLSCertificateFetcher struct{}

func NewTLSCertificateFetcher() domainMonitor.CertificateFetcher {
	return &TLSCertificateFetcher{}
}

func (f *TLSCertificateFetcher) Fetch(domain string, port int, timeout time.Duration) (*domainMonitor.CertificateInfo, error) {
	addr := domain
	if net.ParseIP(domain) != nil && strings.Contains(domain, ":") {
		addr = fmt.Sprintf("[%s]:%d", domain, port)
	} else {
		addr = fmt.Sprintf("%s:%d", domain, port)
	}

	rawConn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	tlsConn := tls.Client(rawConn, &tls.Config{
		ServerName:         domain,
		InsecureSkipVerify: true,
	})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		rawConn.Close()
		return nil, err
	}
	defer tlsConn.Close()

	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("未获取到服务端证书")
	}

	return &domainMonitor.CertificateInfo{
		Domain:   domain,
		Port:     port,
		NotAfter: certs[0].NotAfter,
	}, nil
}