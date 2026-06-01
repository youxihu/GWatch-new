package utils

import "fmt"

// FormatIOSpeed 格式化IO速率，根据大小动态切换单位
// 输入值单位为 KB/s
// 返回格式化后的字符串，单位可能是 KB/s, MB/s, GB/s
func FormatIOSpeed(kbps float64) string {
	if kbps < 0 {
		return "0.00 KB/s"
	}

	// 小于 1024 KB/s，使用 KB/s
	if kbps < 1024 {
		return fmt.Sprintf("%.2f KB/s", kbps)
	}

	// 大于等于 1024 KB/s 且小于 1024*1024 KB/s，使用 MB/s
	if kbps < 1024*1024 {
		mbps := kbps / 1024
		return fmt.Sprintf("%.2f MB/s", mbps)
	}

	// 大于等于 1024*1024 KB/s，使用 GB/s
	gbps := kbps / (1024 * 1024)
	return fmt.Sprintf("%.2f GB/s", gbps)
}

// FormatIOSpeedPair 格式化一对IO速率（读/写或下载/上传）
// 返回格式化后的字符串，例如: "读 1.23 MB/s | 写 456.78 KB/s"
func FormatIOSpeedPair(readKBps, writeKBps float64, readLabel, writeLabel string) string {
	readStr := FormatIOSpeed(readKBps)
	writeStr := FormatIOSpeed(writeKBps)
	return fmt.Sprintf("%s %s | %s %s", readLabel, readStr, writeLabel, writeStr)
}
