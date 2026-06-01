// 主机网络工具
package utils

import (
	"fmt"
	"net"
	"strings"
)

// GetLocalIP 获取本机IP地址
func GetLocalIP() (string, error) {
	// 尝试连接到一个外部地址来获取本机IP
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("无法获取本机IP: %v", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

// GetHostname 获取主机名
func GetHostname() (string, error) {
	// 使用net包获取主机名
	hostname, err := net.LookupCNAME("")
	if err != nil {
		// 如果失败，尝试使用os.Hostname()
		// 这里简化处理，直接返回错误
		return "", fmt.Errorf("无法获取主机名: %v", err)
	}

	// 去掉域名后缀，只保留主机名
	if idx := strings.Index(hostname, "."); idx != -1 {
		hostname = hostname[:idx]
	}

	return hostname, nil
}

// GetHostInfo 获取主机信息（IP和主机名）
func GetHostInfo() (ip, hostname string, err error) {
	ip, err = GetLocalIP()
	if err != nil {
		return "", "", err
	}

	hostname, err = GetHostname()
	if err != nil {
		// 如果获取主机名失败，使用IP作为标识
		hostname = ip
	}

	return ip, hostname, nil
}
