package utils

import "fmt"

// VersionInfo 版本信息结构体
type VersionInfo struct {
	Version   string
	GitCommit string
	GitAuthor string
	BuildTime string
}

// 版本信息，通过 ldflags 在构建时注入（必须是导出的变量）
var (
	Version   = "dev"
	GitCommit = "unknown"
	GitAuthor = "unknown"
	BuildTime = "unknown"
)

// GetVersionInfo 获取版本信息
func GetVersionInfo() VersionInfo {
	return VersionInfo{
		Version:   Version,
		GitCommit: GitCommit,
		GitAuthor: GitAuthor,
		BuildTime: BuildTime,
	}
}

// FormatVersion 格式化版本信息为字符串（用于启动日志）
func FormatVersion() string {
	return fmt.Sprintf("版本: %s, 提交: %s, 作者: %s, 构建时间: %s",
		Version,
		GitCommit,
		GitAuthor,
		BuildTime)
}

// PrintVersion 打印版本信息（用于 -v 参数）
func PrintVersion() {
	fmt.Printf("GWatch 版本: %s\n", Version)
	fmt.Printf("Git 提交: %s\n", GitCommit)
	fmt.Printf("Git 作者: %s\n", GitAuthor)
	fmt.Printf("构建时间: %s\n", BuildTime)
}
