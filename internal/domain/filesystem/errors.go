package filesystem

import (
	"fmt"
	"time"
)

// ScanError 扫描错误 - 值对象
type ScanError struct {
	Path      string
	Err       error
	Type      ErrorType
	Timestamp time.Time
}

// ErrorType 错误类型 - 枚举值对象
type ErrorType string

const (
	ErrorTypePermission ErrorType = "permission"
	ErrorTypeIO         ErrorType = "io"
	ErrorTypeTimeout    ErrorType = "timeout"
	ErrorTypeSymlink    ErrorType = "symlink"
	ErrorTypeOther      ErrorType = "other"
)

// Error 实现 error 接口
func (se *ScanError) Error() string {
	return fmt.Sprintf("扫描错误 [%s] %s: %v", se.Type, se.Path, se.Err)
}

// NewScanError 创建扫描错误
func NewScanError(path string, err error, errorType ErrorType) *ScanError {
	return &ScanError{
		Path:      path,
		Err:       err,
		Type:      errorType,
		Timestamp: time.Now(),
	}
}
