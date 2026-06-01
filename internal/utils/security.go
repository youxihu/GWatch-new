package utils

// SeverityRank 告警等级排序权重
func SeverityRank(severity string) int {
	switch severity {
	case "p1":
		return 3
	case "p2":
		return 2
	case "p3":
		return 1
	default:
		return 0
	}
}
