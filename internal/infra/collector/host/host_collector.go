package host

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// diskStats 磁盘统计信息
type diskStats struct {
	name      string
	readSect  uint64 // 读扇区数（包括缓存读取）
	writeSect uint64 // 写扇区数
}

// Collector 基于 gopsutil 的主机指标采集器
type Collector struct {
	mu sync.Mutex

	// 网络速率采样状态
	lastNetIO   *net.IOCountersStat
	lastNetTime time.Time

	// 磁盘IO采样状态
	lastDiskStats  *diskStats
	lastDiskTime   time.Time
	diskDeviceName string

	// CPU采样状态
	lastCPUTime     time.Time
	cpuInitialized  bool
}

// New 创建主机采集器
func New() *Collector {
	c := &Collector{
		lastNetTime:  time.Now(),
		lastDiskTime: time.Now(),
	}

	// 初始化网络IO基准值
	counters, err := net.IOCounters(false)
	if err == nil && len(counters) > 0 {
		c.lastNetIO = &counters[0]
	}

	// 初始化磁盘IO基准值
	stats, deviceName, err := readDiskStatsFromProc()
	if err == nil && stats != nil {
		c.lastDiskStats = stats
		c.diskDeviceName = deviceName
	}

	return c
}

// GetCPUInfo 获取 CPU 使用率和核心数
// 第一次调用会进行初始化并返回0，第二次调用会返回真实的CPU使用率
func (c *Collector) GetCPUInfo() (float64, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	coreCount, err := cpu.Counts(true)
	if err != nil {
		coreCount = 0
	}

	if !c.cpuInitialized {
		_, err := cpu.Percent(1*time.Second, true)
		if err != nil {
			return 0, coreCount, err
		}
		c.cpuInitialized = true
		c.lastCPUTime = time.Now()
		return 0, coreCount, nil
	}

	elapsed := now.Sub(c.lastCPUTime).Seconds()
	if elapsed < 0.5 {
		time.Sleep(time.Duration((0.5 - elapsed) * float64(time.Second)))
	}

	percent, err := cpu.Percent(1*time.Second, true)
	if err != nil {
		return 0, coreCount, err
	}
	c.lastCPUTime = time.Now()

	var total float64
	for _, p := range percent {
		total += p
	}
	if len(percent) == 0 {
		return 0, coreCount, nil
	}

	return total / float64(len(percent)), coreCount, nil
}

// GetCPUPercent 返回 CPU 使用率百分比（向后兼容）
func (c *Collector) GetCPUPercent() (float64, error) {
	percent, _, err := c.GetCPUInfo()
	return percent, err
}

// GetMemoryUsage 获取内存使用情况
func (c *Collector) GetMemoryUsage() (float64, uint64, uint64, error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0, 0, err
	}
	return vm.UsedPercent, vm.Used / 1024 / 1024, vm.Total / 1024 / 1024, nil
}

// GetDiskUsage 获取磁盘使用情况
func (c *Collector) GetDiskUsage() (float64, uint64, uint64, error) {
	rootPath := os.Getenv("GWATCH_ROOTFS")
	if rootPath == "" {
		rootPath = "/"
	}
	usage, err := disk.Usage(rootPath)
	if err != nil {
		return 0, 0, 0, err
	}
	return usage.UsedPercent, usage.Used / 1024 / 1024 / 1024, usage.Total / 1024 / 1024 / 1024, nil
}

// GetNetworkRate 获取网络速率（下载/上传 KB/s）
func (c *Collector) GetNetworkRate() (float64, float64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	counters, err := net.IOCounters(false)
	if err != nil || len(counters) == 0 {
		return 0, 0, err
	}
	now := time.Now()
	curr := counters[0]
	elapsed := now.Sub(c.lastNetTime).Seconds()
	if elapsed <= 0 || c.lastNetIO == nil {
		c.lastNetIO = &curr
		c.lastNetTime = now
		return 0, 0, nil
	}
	bytesRecv := float64(curr.BytesRecv - c.lastNetIO.BytesRecv)
	bytesSent := float64(curr.BytesSent - c.lastNetIO.BytesSent)
	dl := bytesRecv / elapsed / 1024
	ul := bytesSent / elapsed / 1024
	c.lastNetIO = &curr
	c.lastNetTime = now
	return dl, ul, nil
}

// GetTopProcesses 获取 CPU 和内存占用最高的前 N 个进程
func (c *Collector) GetTopProcesses(n int) ([]entity.ProcessInfo, []entity.ProcessInfo, error) {
	pids, err := process.Pids()
	if err != nil {
		return nil, nil, fmt.Errorf("无法获取 PID 列表: %w", err)
	}
	var processesList []*process.Process
	for _, pid := range pids {
		p, err := process.NewProcess(pid)
		if err != nil || p == nil {
			continue
		}
		processesList = append(processesList, p)
	}
	var cpuList, memList []entity.ProcessInfo
	time.Sleep(300 * time.Millisecond)
	for _, p := range processesList {
		if p == nil {
			continue
		}
		cpuPercent, err := p.CPUPercent()
		if err != nil {
			continue
		}
		memInfo, err := p.MemoryInfo()
		if err != nil {
			continue
		}
		memPercent, err := p.MemoryPercent()
		if err != nil {
			continue
		}
		name, err := p.Name()
		if err != nil {
			name = "unknown"
		}
		info := entity.ProcessInfo{PID: p.Pid, Name: name, CPUPercent: cpuPercent, MemPercent: memPercent, MemRSS: memInfo.RSS / 1024 / 1024}
		if cpuPercent > 0.1 {
			cpuList = append(cpuList, info)
		}
		if memPercent > 0.1 {
			memList = append(memList, info)
		}
	}
	sort.Slice(cpuList, func(i, j int) bool { return cpuList[i].CPUPercent > cpuList[j].CPUPercent })
	sort.Slice(memList, func(i, j int) bool { return memList[i].MemPercent > memList[j].MemPercent })
	if len(cpuList) > n {
		cpuList = cpuList[:n]
	}
	if len(memList) > n {
		memList = memList[:n]
	}
	return cpuList, memList, nil
}

// readDiskStatsFromProc 从 /proc/diskstats 读取磁盘统计信息
func readDiskStatsFromProc() (*diskStats, string, error) {
	file, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, "", fmt.Errorf("无法打开 /proc/diskstats: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	preferredDevices := []string{"sda", "nvme0n1", "vda", "sdb", "nvme0n2", "vdb"}
	var foundStats *diskStats
	var foundDevice string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}

		deviceName := fields[2]
		if strings.HasPrefix(deviceName, "loop") ||
			strings.HasPrefix(deviceName, "dm-") ||
			strings.HasPrefix(deviceName, "ram") {
			continue
		}

		for _, preferred := range preferredDevices {
			if deviceName == preferred {
				readSect, err1 := strconv.ParseUint(fields[5], 10, 64)
				writeSect, err2 := strconv.ParseUint(fields[9], 10, 64)
				if err1 != nil || err2 != nil {
					continue
				}
				return &diskStats{name: deviceName, readSect: readSect, writeSect: writeSect}, deviceName, nil
			}
		}

		if foundStats == nil {
			readSect, err1 := strconv.ParseUint(fields[5], 10, 64)
			writeSect, err2 := strconv.ParseUint(fields[9], 10, 64)
			if err1 == nil && err2 == nil {
				foundStats = &diskStats{name: deviceName, readSect: readSect, writeSect: writeSect}
				foundDevice = deviceName
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, "", fmt.Errorf("读取 /proc/diskstats 失败: %w", err)
	}

	if foundStats != nil {
		return foundStats, foundDevice, nil
	}

	return nil, "", fmt.Errorf("未找到有效的磁盘设备")
}

// GetDiskIORate 获取磁盘读写速率（KB/s）
func (c *Collector) GetDiskIORate() (float64, float64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	currStats, deviceName, err := readDiskStatsFromProc()
	if err != nil {
		return 0, 0, err
	}

	if c.diskDeviceName != "" && deviceName != c.diskDeviceName {
		c.lastDiskStats = currStats
		c.diskDeviceName = deviceName
		c.lastDiskTime = now
		return 0, 0, nil
	}

	elapsed := now.Sub(c.lastDiskTime).Seconds()
	const minIntervalSeconds = 1.0
	if elapsed < minIntervalSeconds || c.lastDiskStats == nil {
		c.lastDiskStats = currStats
		c.diskDeviceName = deviceName
		c.lastDiskTime = now
		return 0, 0, nil
	}

	const sectorSize = 512
	readSectDiff := float64(currStats.readSect - c.lastDiskStats.readSect)
	writeSectDiff := float64(currStats.writeSect - c.lastDiskStats.writeSect)

	if readSectDiff < 0 {
		fmt.Printf("[GetDiskIORate] 警告：读扇区数回绕，忽略负值\n")
		readSectDiff = 0
	}
	if writeSectDiff < 0 {
		fmt.Printf("[GetDiskIORate] 警告：写扇区数回绕，忽略负值\n")
		writeSectDiff = 0
	}

	readRate := readSectDiff * sectorSize / elapsed / 1024
	writeRate := writeSectDiff * sectorSize / elapsed / 1024

	c.lastDiskStats = currStats
	c.diskDeviceName = deviceName
	c.lastDiskTime = now

	return readRate, writeRate, nil
}
