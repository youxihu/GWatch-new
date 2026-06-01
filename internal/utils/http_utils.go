package utils

// IsValidHTTPStatusCode 检查HTTP状态码是否在允许的范围内
func IsValidHTTPStatusCode(statusCode int, allowedCodes []int) bool {
	if len(allowedCodes) == 0 {
		// 如果没有配置allowed_codes，默认只允许200
		return statusCode == 200
	}

	for _, allowedCode := range allowedCodes {
		if statusCode == allowedCode {
			return true
		}
	}
	return false
}
