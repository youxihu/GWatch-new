package filesystem

import (
	"os"
	"strings"
	"sync"
	"time"
)

// 路径验证器实现
type PathValidator struct {
	virtualFSCache map[string]bool
	cacheMutex     sync.RWMutex
	cacheTime      time.Time
	cacheDuration  time.Duration
}

// NewPathValidator 创建路径验证器
func NewPathValidator() *PathValidator {
	return &PathValidator{
		virtualFSCache: make(map[string]bool),
		cacheDuration:  5 * time.Minute,
	}
}

// ShouldSkip 检查是否应该跳过路径
func (pv *PathValidator) ShouldSkip(path string) (bool, string) {
	// 检查虚拟文件系统
	if pv.IsVirtualFileSystem(path) {
		return true, "virtual filesystem"
	}

	// 检查特殊目录
	specialDirs := []string{
		"/lost+found",
		"/System/Volumes",
		"/.Spotlight-V100",
		"/.fseventsd",
		"/.Trashes",
		"/Windows",
		"/pagefile.sys",
		"/hiberfil.sys",
	}

	for _, specialDir := range specialDirs {
		if strings.HasPrefix(path, specialDir) || path == specialDir {
			return true, "special directory"
		}
	}

	// 检查网络文件系统
	if pv.isNetworkFileSystem(path) {
		return true, "network filesystem"
	}

	// 检查只读系统路径
	if pv.isReadOnlySystemPath(path) {
		return true, "readonly system path"
	}

	return false, ""
}

// HasReadPermission 检查是否有读取权限
func (pv *PathValidator) HasReadPermission(path string) bool {
	if info, err := os.Stat(path); err != nil {
		return false
	} else if info.IsDir() {
		// 对于目录，尝试读取目录内容
		if _, err := os.ReadDir(path); err != nil {
			return false
		}
	} else {
		// 对于文件，尝试打开文件
		if file, err := os.Open(path); err != nil {
			return false
		} else {
			file.Close()
		}
	}
	return true
}

// IsVirtualFileSystem 检查是否为虚拟文件系统
func (pv *PathValidator) IsVirtualFileSystem(path string) bool {
	pv.cacheMutex.RLock()

	// 检查缓存
	if cached, exists := pv.virtualFSCache[path]; exists && time.Since(pv.cacheTime) < pv.cacheDuration {
		pv.cacheMutex.RUnlock()
		return cached
	}

	pv.cacheMutex.RUnlock()

	// 检查虚拟文件系统
	isVirtual := pv.checkVirtualFileSystem(path)

	// 更新缓存
	pv.cacheMutex.Lock()
	pv.virtualFSCache[path] = isVirtual
	pv.cacheTime = time.Now()
	pv.cacheMutex.Unlock()

	return isVirtual
}

// checkVirtualFileSystem 检查虚拟文件系统
func (pv *PathValidator) checkVirtualFileSystem(path string) bool {
	virtualDirs := []string{
		"/proc", "/sys", "/dev", "/run", "/snap", "/boot",
		"/sys/fs", "/proc/sys", "/dev/pts", "/dev/shm",
	}

	for _, vdir := range virtualDirs {
		if strings.HasPrefix(path, vdir+"/") || path == vdir {
			return true
		}
	}

	return false
}

// isNetworkFileSystem 检查是否为网络文件系统
func (pv *PathValidator) isNetworkFileSystem(path string) bool {
	networkPrefixes := []string{
		"/mnt/nfs",
		"/mnt/cifs",
		"/mnt/smb",
		"/net",
		"/afs",
	}

	for _, prefix := range networkPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// isReadOnlySystemPath 检查是否为只读系统路径
func (pv *PathValidator) isReadOnlySystemPath(path string) bool {
	readOnlyPaths := []string{
		"/usr/share/doc",
		"/usr/share/man",
		"/usr/share/info",
		"/usr/lib/locale",
		"/lib/modules",
		"/boot/grub",
	}

	for _, roPath := range readOnlyPaths {
		if strings.HasPrefix(path, roPath) {
			return true
		}
	}

	return false
}
