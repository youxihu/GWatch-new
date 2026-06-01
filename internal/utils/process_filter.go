package utils

import "strings"

// isProcessInWhiteList 检查进程名是否在白名单中（忽略大小写，支持部分匹配或精确匹配，按需调整）
func IsProcessInWhiteList(processName string, whiteList []string) bool {
	if len(whiteList) == 0 {
		return false
	}
	for _, white := range whiteList {
		// 精确匹配
		if strings.EqualFold(processName, white) {
			return true
		}
		// 如果你想支持部分匹配（比如进程名包含 "goland" 就跳过），可以用：
		// if strings.Contains(strings.ToLower(processName), strings.ToLower(white)) {
		//     return true
		// }
	}
	return false
}
