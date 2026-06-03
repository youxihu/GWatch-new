package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	monitoringEntity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"github.com/youxihu/GWatch-new/internal/domain/monitoring"
	logger "github.com/youxihu/GWatch-new/internal/infra/logger"

	"github.com/redis/go-redis/v9"
)

// AlertStateStorageImpl Redis告警状态存储实现
type AlertStateStorageImpl struct {
	client     *redis.Client
	config     *shared.Config
	hostID     string     // 主机标识符（alert_title），用于区分不同主机的数据
	mu         sync.Mutex // 保护client的并发访问
	hostIDOnce sync.Once  // 确保hostID只初始化一次
}

// NewAlertStateStorage 创建告警状态存储服务
func NewAlertStateStorage() monitoring.AlertStateStorage {
	return &AlertStateStorageImpl{}
}

// AddOrUpdateProcess 添加或更新进程信息到指定事件
func (s *AlertStateStorageImpl) AddOrUpdateProcess(eventID string, processInfo monitoring.ProcessInfo) error {
	if err := s.ensureConnected(); err != nil {
		return err
	}

	// 获取现有状态
	state, err := s.GetAlertStateByEventID(eventID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("事件ID %s 不存在", eventID)
	}

	// 查找是否已存在该PID的进程
	found := false
	for i, proc := range state.Processes {
		if proc.PID == processInfo.PID {
			// 更新现有进程信息，保留首次检测时间
			processInfo.FirstDetectedTime = proc.FirstDetectedTime
			state.Processes[i] = processInfo
			found = true
			break
		}
	}

	// 如果不存在，添加新进程
	if !found {
		state.Processes = append(state.Processes, processInfo)
	}

	return s.UpdateAlertStateByEventID(state)
}

// RemoveProcess 从指定事件中移除进程信息
func (s *AlertStateStorageImpl) RemoveProcess(eventID string, pid string) error {
	if err := s.ensureConnected(); err != nil {
		return err
	}

	// 获取现有状态
	state, err := s.GetAlertStateByEventID(eventID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("事件ID %s 不存在", eventID)
	}

	// 查找并移除指定PID的进程
	for i, proc := range state.Processes {
		if proc.PID == pid {
			state.Processes = append(state.Processes[:i], state.Processes[i+1:]...)
			break
		}
	}

	return s.UpdateAlertStateByEventID(state)
}

// GetAlertStateByEventID 根据事件ID获取告警状态
func (s *AlertStateStorageImpl) GetAlertStateByEventID(eventID string) (*monitoring.AlertState, error) {
	if err := s.ensureConnected(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 使用新格式的key直接获取
	newKey := s.getRedisKeyByEventID(eventID)
	val, err := s.client.Get(ctx, newKey).Result()
	if err == redis.Nil {
		return nil, nil // 未找到
	}
	if err != nil {
		return nil, fmt.Errorf("获取告警状态失败: %v", err)
	}

	var state monitoring.AlertState
	if err := json.Unmarshal([]byte(val), &state); err != nil {
		return nil, fmt.Errorf("解析告警状态失败: %v", err)
	}
	return &state, nil
}

// DeleteAlertStateByEventID 根据事件ID删除告警状态
func (s *AlertStateStorageImpl) DeleteAlertStateByEventID(eventID string) error {
	if err := s.ensureConnected(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 先获取状态，用于删除索引
	state, _ := s.GetAlertStateByEventID(eventID)

	// 删除主key
	newKey := s.getRedisKeyByEventID(eventID)
	if err := s.client.Del(ctx, newKey).Err(); err != nil {
		return fmt.Errorf("删除告警状态失败: %v", err)
	}

	// 删除索引
	if state != nil {
		s.removeIndexes(ctx, state)
	}

	return nil
}

// Init 初始化存储服务（只保存配置，不立即连接）
func (s *AlertStateStorageImpl) Init(config *shared.Config) error {
	s.config = config
	// 不立即连接，等待第一次使用时再连接
	return nil
}

// ensureConnected 确保Redis连接已建立（按需连接）
func (s *AlertStateStorageImpl) ensureConnected() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已经连接，直接返回
	if s.client != nil {
		// 测试连接是否有效
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		err := s.client.Ping(ctx).Err()
		cancel()
		if err == nil {
			return nil // 连接有效
		}
		// 连接无效，关闭并重新连接
		s.client.Close()
		s.client = nil
	}

	// 检查配置
	if s.config == nil {
		return fmt.Errorf("配置未初始化，请先调用Init")
	}

	// 获取Redis连接配置（优先使用公共配置）
	var addr, password string
	var db, poolSize, minIdleConns, maxIdleConns int
	var timeout time.Duration

	if s.config.RedisConnection != nil {
		addr = s.config.RedisConnection.Addr
		password = s.config.RedisConnection.Password
		db = s.config.RedisConnection.DB
		timeout = s.config.RedisConnection.Timeout
		poolSize = s.config.RedisConnection.PoolSize
		minIdleConns = s.config.RedisConnection.MinIdleConns
		maxIdleConns = s.config.RedisConnection.MaxIdleConns
	} else {
		return fmt.Errorf("Redis连接配置未找到，请在redis_connection中配置")
	}

	// 设置默认值
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	if poolSize == 0 {
		poolSize = 5
	}
	if minIdleConns == 0 {
		minIdleConns = 1
	}
	if maxIdleConns == 0 {
		maxIdleConns = 3
	}

	// 创建连接（使用较小的连接池，因为按需连接）
	options := &redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  timeout,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     poolSize,
		MinIdleConns: minIdleConns,
		MaxIdleConns: maxIdleConns,
		PoolTimeout:  2 * time.Second,
	}

	s.client = redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := s.client.Ping(ctx).Result(); err != nil {
		s.client = nil // 连接失败，清空client
		return fmt.Errorf("Redis连接测试失败: %v", err)
	}

	return nil
}

// getHostID 获取主机标识符（alert_title），用于区分不同主机的数据
func (s *AlertStateStorageImpl) getHostID() string {
	s.hostIDOnce.Do(func() {
		// 优先使用配置的alert_title作为主机标识符
		if s.config != nil && s.config.HostMonitoring != nil && s.config.HostMonitoring.AlertTitle != "" {
			// 清理alert_title中的特殊字符，确保可以作为Redis key的一部分
			title := strings.TrimSpace(s.config.HostMonitoring.AlertTitle)
			// 替换可能存在的特殊字符为下划线
			title = strings.ReplaceAll(title, " ", "_")
			title = strings.ReplaceAll(title, ":", "_")
			title = strings.ReplaceAll(title, "/", "_")
			title = strings.ReplaceAll(title, "\\", "_")
			s.hostID = title
		} else {
			// 如果没有配置alert_title，使用默认值
			s.hostID = "default-host"
		}
	})
	return s.hostID
}

// getRedisKeyByEventID 生成基于事件ID的Redis key（新格式）
func (s *AlertStateStorageImpl) getRedisKeyByEventID(eventID string) string {
	hostID := s.getHostID()
	return fmt.Sprintf("gwatch:alert:%s:%s", hostID, eventID)
}

// getTypeIndexKey 获取告警类型索引key
func (s *AlertStateStorageImpl) getTypeIndexKey(alertType monitoringEntity.AlertType) string {
	hostID := s.getHostID()
	return fmt.Sprintf("gwatch:alert:type:%s:%s", hostID, alertType)
}

// getProcessIndexKey 获取进程名索引key
func (s *AlertStateStorageImpl) getProcessIndexKey(alertType monitoringEntity.AlertType, processName string) string {
	hostID := s.getHostID()
	return fmt.Sprintf("gwatch:alert:proc:%s:%s:%s", hostID, alertType, processName)
}

// addIndexes 添加告警索引
func (s *AlertStateStorageImpl) addIndexes(ctx context.Context, state *monitoring.AlertState) {
	hostID := s.getHostID()
	ttl := s.getRetentionTTL()

	// 添加告警类型索引
	typeIndexKey := s.getTypeIndexKey(state.AlertType)
	s.client.SAdd(ctx, typeIndexKey, state.EventID)
	s.client.Expire(ctx, typeIndexKey, ttl)

	// 添加进程名索引（每个进程一个索引）
	for _, proc := range state.Processes {
		if proc.ProcessName != "" {
			procIndexKey := s.getProcessIndexKey(state.AlertType, proc.ProcessName)
			s.client.SAdd(ctx, procIndexKey, state.EventID)
			s.client.Expire(ctx, procIndexKey, ttl)
		}
	}

	// 维护主机级别的eventID列表（用于GetAllAlertStates）
	hostIndexKey := fmt.Sprintf("gwatch:alert:all:%s", hostID)
	s.client.SAdd(ctx, hostIndexKey, state.EventID)
	s.client.Expire(ctx, hostIndexKey, ttl)
}

// removeIndexes 删除告警索引
func (s *AlertStateStorageImpl) removeIndexes(ctx context.Context, state *monitoring.AlertState) {
	hostID := s.getHostID()

	// 删除告警类型索引
	typeIndexKey := s.getTypeIndexKey(state.AlertType)
	s.client.SRem(ctx, typeIndexKey, state.EventID)

	// 删除进程名索引
	for _, proc := range state.Processes {
		if proc.ProcessName != "" {
			procIndexKey := s.getProcessIndexKey(state.AlertType, proc.ProcessName)
			s.client.SRem(ctx, procIndexKey, state.EventID)
		}
	}

	// 从主机级别索引中移除
	hostIndexKey := fmt.Sprintf("gwatch:alert:all:%s", hostID)
	s.client.SRem(ctx, hostIndexKey, state.EventID)
}

// getRetentionTTL 获取保留时间
func (s *AlertStateStorageImpl) getRetentionTTL() time.Duration {
	retentionHours := 24 // 默认24小时
	if s.config != nil && s.config.Alerting != nil {
		retentionHours = s.config.Alerting.Event.RetentionHours
	}
	if retentionHours <= 0 {
		retentionHours = 24
	}
	return time.Duration(retentionHours) * time.Hour
}

// SetAlertStateByEventID 使用事件ID设置告警状态
func (s *AlertStateStorageImpl) SetAlertStateByEventID(state *monitoring.AlertState) error {
	if err := s.ensureConnected(); err != nil {
		return err
	}

	key := s.getRedisKeyByEventID(state.EventID)

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("序列化告警状态失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ttl := s.getRetentionTTL()
	if err := s.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("设置告警状态失败: %v", err)
	}

	// 添加索引
	s.addIndexes(ctx, state)

	return nil
}

// UpdateAlertStateByEventID 使用事件ID更新告警状态
func (s *AlertStateStorageImpl) UpdateAlertStateByEventID(state *monitoring.AlertState) error {
	return s.SetAlertStateByEventID(state) // 更新和设置逻辑相同
}

// GetAllAlertStates 获取所有告警状态
func (s *AlertStateStorageImpl) GetAllAlertStates() ([]*monitoring.AlertState, error) {
	if err := s.ensureConnected(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 使用索引获取所有eventID
	hostID := s.getHostID()
	hostIndexKey := fmt.Sprintf("gwatch:alert:all:%s", hostID)
	eventIDs, err := s.client.SMembers(ctx, hostIndexKey).Result()
	if err != nil {
		return nil, fmt.Errorf("获取告警状态索引失败: %v", err)
	}

	var states []*monitoring.AlertState
	for _, eventID := range eventIDs {
		state, err := s.GetAlertStateByEventID(eventID)
		if err != nil || state == nil {
			continue // 跳过获取失败的key
		}
		states = append(states, state)
	}

	return states, nil
}

// GetAlertStateByType 获取指定告警类型的第一个告警状态
// 使用索引快速查找，避免KEYS扫描
func (s *AlertStateStorageImpl) GetAlertStateByType(alertType monitoringEntity.AlertType) (*monitoring.AlertState, error) {
	if err := s.ensureConnected(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 使用索引获取该类型的所有eventID
	typeIndexKey := s.getTypeIndexKey(alertType)
	eventIDs, err := s.client.SMembers(ctx, typeIndexKey).Result()
	if err != nil {
		return nil, fmt.Errorf("获取告警类型索引失败: %v", err)
	}

	// 返回第一个有效的状态
	for _, eventID := range eventIDs {
		state, err := s.GetAlertStateByEventID(eventID)
		if err == nil && state != nil {
			return state, nil
		}
	}

	return nil, nil // 未找到
}

// GetAlertStateByProcessName 根据进程名查找相同告警类型的告警状态
// 使用索引快速查找，避免KEYS扫描
func (s *AlertStateStorageImpl) GetAlertStateByProcessName(alertType monitoringEntity.AlertType, processName string) (*monitoring.AlertState, error) {
	if err := s.ensureConnected(); err != nil {
		return nil, err
	}

	if processName == "" {
		return nil, nil // 没有进程名，无法查找
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 使用索引获取该进程名的所有eventID
	procIndexKey := s.getProcessIndexKey(alertType, processName)
	eventIDs, err := s.client.SMembers(ctx, procIndexKey).Result()
	if err != nil {
		return nil, fmt.Errorf("获取进程索引失败: %v", err)
	}

	// 返回第一个有效的状态
	for _, eventID := range eventIDs {
		state, err := s.GetAlertStateByEventID(eventID)
		if err == nil && state != nil {
			// 验证告警类型是否匹配（索引可能有延迟）
			if state.AlertType == alertType {
				return state, nil
			}
		}
	}

	return nil, nil // 未找到
}

// Close 关闭Redis连接（释放资源）
func (s *AlertStateStorageImpl) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		err := s.client.Close()
		s.client = nil
		return err
	}
	return nil
}

// EventIDExists 检查事件ID是否已存在（实现EventIDChecker接口）
func (s *AlertStateStorageImpl) EventIDExists(eventID string) (bool, error) {
	if err := s.ensureConnected(); err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 检查新格式的key是否存在
	newKey := s.getRedisKeyByEventID(eventID)
	exists, err := s.client.Exists(ctx, newKey).Result()
	if err != nil {
		return false, fmt.Errorf("检查key存在性失败: %v", err)
	}

	return exists > 0, nil
}

// FindExistingEventByProcess 根据告警类型和进程名查找现有事件ID
// 使用索引快速查找，避免KEYS扫描
func (s *AlertStateStorageImpl) FindExistingEventByProcess(alertType monitoringEntity.AlertType, processName string) (*monitoring.AlertState, error) {
	return s.GetAlertStateByProcessName(alertType, processName)
}

// FindExistingEventByProcessForEventID 根据告警类型和进程名查找现有事件ID（实现EventIDFinder接口）
// 返回事件ID字符串，用于EventIDGenerator
func (s *AlertStateStorageImpl) FindExistingEventByProcessForEventID(alertType string, processName string) (string, error) {
	// 将字符串类型转换为AlertType枚举
	var alertTypeEnum monitoringEntity.AlertType
	switch alertType {
	case "cpu_high":
		alertTypeEnum = monitoringEntity.CPUHigh
	case "mem_high":
		alertTypeEnum = monitoringEntity.MemHigh
	case "disk_high":
		alertTypeEnum = monitoringEntity.DiskHigh
	case "redis_high":
		alertTypeEnum = monitoringEntity.RedisHigh
	case "redis_low":
		alertTypeEnum = monitoringEntity.RedisLow
	case "redis_error":
		alertTypeEnum = monitoringEntity.RedisErr
	case "http_error":
		alertTypeEnum = monitoringEntity.HTTPErr
	default:
		return "", fmt.Errorf("未知的告警类型: %s", alertType)
	}

	state, err := s.FindExistingEventByProcess(alertTypeEnum, processName)
	if err != nil {
		return "", err
	}
	if state == nil {
		return "", nil // 未找到
	}

	return state.EventID, nil
}

// FindActiveEventsByType 根据告警类型查找所有活跃事件
// 使用索引快速查找，避免KEYS扫描
func (s *AlertStateStorageImpl) FindActiveEventsByType(alertType monitoringEntity.AlertType) ([]*monitoring.AlertState, error) {
	if err := s.ensureConnected(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 使用索引获取该类型的所有eventID
	typeIndexKey := s.getTypeIndexKey(alertType)
	eventIDs, err := s.client.SMembers(ctx, typeIndexKey).Result()
	if err != nil {
		return nil, fmt.Errorf("获取告警类型索引失败: %v", err)
	}

	var activeEvents []*monitoring.AlertState

	// 获取所有有效状态
	for _, eventID := range eventIDs {
		state, err := s.GetAlertStateByEventID(eventID)
		if err == nil && state != nil && state.AlertType == alertType {
			activeEvents = append(activeEvents, state)
		}
	}

	return activeEvents, nil
}

// GetEventLifecycleInfo 获取事件的生命周期信息
// 用于事件ID管理和监控
func (s *AlertStateStorageImpl) GetEventLifecycleInfo(eventID string) (*monitoring.EventLifecycleInfo, error) {
	state, err := s.GetAlertStateByEventID(eventID)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}

	info := &monitoring.EventLifecycleInfo{
		EventID:      eventID,
		AlertType:    state.AlertType,
		StartTime:    state.StartTime,
		ProcessCount: len(state.Processes),
		IsActive:     len(state.Processes) > 0,
	}

	// 计算最后活跃时间
	var lastActiveTime time.Time
	for _, proc := range state.Processes {
		if proc.LastSeenTime.After(lastActiveTime) {
			lastActiveTime = proc.LastSeenTime
		}
	}
	info.LastActiveTime = lastActiveTime

	// 计算事件持续时间
	if info.IsActive {
		info.Duration = time.Since(state.StartTime)
	} else {
		info.Duration = lastActiveTime.Sub(state.StartTime)
	}

	return info, nil
}

// CleanupExpiredEvents 清理已过期的事件（用于事件ID生命周期管理）
func (s *AlertStateStorageImpl) CleanupExpiredEvents() error {
	if err := s.ensureConnected(); err != nil {
		return err
	}

	// 获取所有告警状态
	states, err := s.GetAllAlertStates()
	if err != nil {
		return fmt.Errorf("获取所有告警状态失败: %v", err)
	}

	now := time.Now()
	cleanupCount := 0

	retentionDuration := s.getRetentionTTL()

	for _, state := range states {
		// 检查事件是否已过期（基于最后更新时间）
		var lastUpdateTime time.Time
		for _, proc := range state.Processes {
			if proc.LastSeenTime.After(lastUpdateTime) {
				lastUpdateTime = proc.LastSeenTime
			}
		}

		// 如果没有进程信息，使用开始时间
		if lastUpdateTime.IsZero() {
			lastUpdateTime = state.StartTime
		}

		if now.Sub(lastUpdateTime) > retentionDuration {
			if err := s.DeleteAlertStateByEventID(state.EventID); err != nil {
				logger.Warnf("清理过期事件失败: %s, %v", state.EventID, err)
				continue
			}
			cleanupCount++
			logger.Debugf("清理过期事件: %s (最后更新: %v)", state.EventID, lastUpdateTime)
		}
	}

	if cleanupCount > 0 {
		logger.Infof("清理了 %d 个过期事件", cleanupCount)
	}

	return nil
}
