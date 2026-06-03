// 主机网络工具
package utils

import (
	"fmt"
	"net"
	"os"
)

// GetLocalIP 获取本机IP地址
func GetLocalIP() (string, error) {
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
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("无法获取主机名: %v", err)
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
		hostname = ip
	}

	return ip, hostname, nil
}
