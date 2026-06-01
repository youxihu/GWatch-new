package host

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// 基于 gopsutil 的主机指标采集器
type Collector struct{}

func New() *Collector { return &Collector{} }

// 获取 CPU 使用率和核心数
// 使用明确的采样间隔（1秒）来确保获取准确的CPU使用率
// 第一次调用会进行初始化并返回0，第二次调用会返回真实的CPU使用率
func (c *Collector) GetCPUInfo() (float64, int, error) {
	now := time.Now()

	// 获取CPU核心数（逻辑核心数）
	coreCount, err := cpu.Counts(true)
	if err != nil {
		coreCount = 0 // 如果获取失败，使用0
	}

	// 如果是第一次调用，先进行一次采样初始化
	if !cpuInitialized {
		_, err := cpu.Percent(1*time.Second, true)
		if err != nil {
			return 0, coreCount, err
		}
		cpuInitialized = true
		lastCPUTime = time.Now()
		// 第一次调用返回0，因为需要两次采样才能计算差值
		return 0, coreCount, nil
	}

	// 确保距离上次采样有足够间隔（防止连续调用导致数据不准）
	elapsed := now.Sub(lastCPUTime).Seconds()
	if elapsed < 0.5 {
		time.Sleep(time.Duration((0.5 - elapsed) * float64(time.Second)))
	}

	// 采样 1 秒获取 CPU 使用率
	percent, err := cpu.Percent(1*time.Second, true)
	if err != nil {
		return 0, coreCount, err
	}
	lastCPUTime = time.Now()

	// 计算所有CPU核心的平均使用率
	var total float64
	for _, p := range percent {
		total += p
	}
	if len(percent) == 0 {
		return 0, coreCount, nil
	}

	avgPercent := total / float64(len(percent))
	return avgPercent, coreCount, nil
}

// 返回 CPU 使用率百分比（向后兼容）
// 为了保持向后兼容，保留此方法，内部调用GetCPUInfo
func (c *Collector) GetCPUPercent() (float64, error) {
	percent, _, err := c.GetCPUInfo()
	return percent, err
}

func (c *Collector) GetMemoryUsage() (float64, uint64, uint64, error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0, 0, err
	}
	return vm.UsedPercent, vm.Used / 1024 / 1024, vm.Total / 1024 / 1024, nil
}

func (c *Collector) GetDiskUsage() (float64, uint64, uint64, error) {
	usage, err := disk.Usage("/")
	if err != nil {
		return 0, 0, 0, err
	}
	return usage.UsedPercent, usage.Used / 1024 / 1024 / 1024, usage.Total / 1024 / 1024 / 1024, nil
}

var lastNetIO *net.IOCountersStat
var lastNetTime time.Time

// 添加磁盘IO统计变量（使用 /proc/diskstats 以包含缓存读取）
// diskStats 结构体用于存储磁盘统计信息
type diskStats struct {
	name      string
	readSect  uint64 // 读扇区数（包括缓存读取）
	writeSect uint64 // 写扇区数
}

var lastDiskStats *diskStats
var lastDiskTime time.Time
var diskDeviceName string // 记录当前监控的磁盘设备名

// CPU统计变量：用于跟踪上次CPU采样时间，确保有足够的时间间隔
var lastCPUTime time.Time
var cpuInitialized bool

func init() {
	counters, err := net.IOCounters(false)
	if err == nil && len(counters) > 0 {
		lastNetIO = &counters[0]
	}
	lastNetTime = time.Now()

	// 初始化磁盘IO统计（从 /proc/diskstats，包含缓存读取）
	stats, deviceName, err := readDiskStatsFromProc()
	if err == nil && stats != nil {
		lastDiskStats = stats
		diskDeviceName = deviceName
	}
	lastDiskTime = time.Now()

	// 初始化CPU统计
	cpuInitialized = false
	lastCPUTime = time.Now()
}

func (c *Collector) GetNetworkRate() (float64, float64, error) {
	counters, err := net.IOCounters(false)
	if err != nil || len(counters) == 0 {
		return 0, 0, err
	}
	now := time.Now()
	curr := counters[0]
	elapsed := now.Sub(lastNetTime).Seconds()
	if elapsed <= 0 || lastNetIO == nil {
		lastNetIO = &curr
		lastNetTime = now
		return 0, 0, nil
	}
	bytesRecv := float64(curr.BytesRecv - lastNetIO.BytesRecv)
	bytesSent := float64(curr.BytesSent - lastNetIO.BytesSent)
	dl := bytesRecv / elapsed / 1024
	ul := bytesSent / elapsed / 1024
	lastNetIO = &curr
	lastNetTime = now
	return dl, ul, nil
}

// 获取 CPU 和内存占用最高的前 N 个进程
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
	// sampling interval
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
// 返回磁盘统计信息、设备名称和错误
// /proc/diskstats 格式: major minor name rio rmerge rsect ruse wio wmerge wsect wuse running use aveq
// rsect (第6列，索引5) 包括缓存读取，wsect (第10列，索引9) 是写扇区数
func readDiskStatsFromProc() (*diskStats, string, error) {
	file, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, "", fmt.Errorf("无法打开 /proc/diskstats: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// 优先查找的磁盘设备（物理磁盘，排除虚拟设备）
	preferredDevices := []string{"sda", "nvme0n1", "vda", "sdb", "nvme0n2", "vdb"}
	var foundStats *diskStats
	var foundDevice string

	// 第一遍：查找优先设备
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

		// 跳过虚拟设备
		if strings.HasPrefix(deviceName, "loop") ||
			strings.HasPrefix(deviceName, "dm-") ||
			strings.HasPrefix(deviceName, "ram") {
			continue
		}

		// 检查是否是优先设备
		for _, preferred := range preferredDevices {
			if deviceName == preferred {
				readSect, err1 := strconv.ParseUint(fields[5], 10, 64)  // rsect (第6列，索引5)
				writeSect, err2 := strconv.ParseUint(fields[9], 10, 64) // wsect (第10列，索引9)

				if err1 != nil || err2 != nil {
					continue
				}

				return &diskStats{
					name:      deviceName,
					readSect:  readSect,
					writeSect: writeSect,
				}, deviceName, nil
			}
		}

		// 如果没有找到优先设备，记录第一个物理设备作为备选
		if foundStats == nil {
			readSect, err1 := strconv.ParseUint(fields[5], 10, 64)
			writeSect, err2 := strconv.ParseUint(fields[9], 10, 64)

			if err1 == nil && err2 == nil {
				foundStats = &diskStats{
					name:      deviceName,
					readSect:  readSect,
					writeSect: writeSect,
				}
				foundDevice = deviceName
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, "", fmt.Errorf("读取 /proc/diskstats 失败: %w", err)
	}

	// 如果找到了备选设备，返回它
	if foundStats != nil {
		return foundStats, foundDevice, nil
	}

	return nil, "", fmt.Errorf("未找到有效的磁盘设备")
}

// 获取磁盘读写速率（KB/s）
// 使用 /proc/diskstats 以包含缓存读取（rsect字段包括所有读取，包括缓存）
// 该方法需要至少1秒的时间间隔才能准确计算速率
func (c *Collector) GetDiskIORate() (float64, float64, error) {
	now := time.Now()

	// 从 /proc/diskstats 读取当前统计信息
	currStats, deviceName, err := readDiskStatsFromProc()
	if err != nil {
		return 0, 0, err
	}

	// 如果设备名称发生变化，更新并返回0
	if diskDeviceName != "" && deviceName != diskDeviceName {
		lastDiskStats = currStats
		diskDeviceName = deviceName
		lastDiskTime = now
		return 0, 0, nil
	}

	elapsed := now.Sub(lastDiskTime).Seconds()

	// 如果这是第一次调用，或者时间间隔不足1秒，更新基准值并返回0
	const minIntervalSeconds = 1.0
	if elapsed < minIntervalSeconds || lastDiskStats == nil {
		lastDiskStats = currStats
		diskDeviceName = deviceName
		lastDiskTime = now
		return 0, 0, nil
	}

	// 计算时间差内的IO变化量（扇区数）
	// 每个扇区通常是512字节
	const sectorSize = 512
	readSectDiff := float64(currStats.readSect - lastDiskStats.readSect)
	writeSectDiff := float64(currStats.writeSect - lastDiskStats.writeSect)

	// 处理计数器回绕的情况
	if readSectDiff < 0 {
		fmt.Printf("[GetDiskIORate] 警告：读扇区数回绕，忽略负值\n")
		readSectDiff = 0
	}
	if writeSectDiff < 0 {
		fmt.Printf("[GetDiskIORate] 警告：写扇区数回绕，忽略负值\n")
		writeSectDiff = 0
	}

	// 基于时间差计算速率（KB/s）
	// 扇区数 * 512字节 / 1024 = KB
	readRate := readSectDiff * sectorSize / elapsed / 1024
	writeRate := writeSectDiff * sectorSize / elapsed / 1024

	// 更新基准值
	lastDiskStats = currStats
	diskDeviceName = deviceName
	lastDiskTime = now

	return readRate, writeRate, nil
}
