// 客户端数据仓库实现
package common

import (
	scheduledPushEntity "github.com/youxihu/GWatch-new/internal/domain/entity/scheduled_push"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"github.com/youxihu/GWatch-new/internal/domain/scheduled_push/common"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ClientDataRepositoryImpl Redis 客户端数据仓库实现
type ClientDataRepositoryImpl struct {
	client *redis.Client
}

// NewClientDataRepository 创建客户端数据仓库
func NewClientDataRepository() common.ClientDataRepository {
	return &ClientDataRepositoryImpl{}
}

// Init 初始化 Redis 连接
func (r *ClientDataRepositoryImpl) Init(config *shared.Config) error {
	if config.ScheduledPush == nil {
		return fmt.Errorf("scheduled_push 配置未找到")
	}

	sp := config.ScheduledPush

	// 优先使用scheduled_push中的配置，如果未配置则使用公共redis_connection配置
	var addr, password string
	var db, poolSize, minIdleConns, maxIdleConns int
	var timeout time.Duration

	if sp.RdsURL != "" {
		// 使用scheduled_push中的配置
		addr = sp.RdsURL
		password = sp.RdsPassword
		db = sp.RdsDB
		timeout = 2 * time.Second
		poolSize = 10
		minIdleConns = 2
		maxIdleConns = 5
	} else if config.RedisConnection != nil {
		// 使用公共redis_connection配置
		addr = config.RedisConnection.Addr
		password = config.RedisConnection.Password
		db = config.RedisConnection.DB
		timeout = config.RedisConnection.Timeout
		poolSize = config.RedisConnection.PoolSize
		minIdleConns = config.RedisConnection.MinIdleConns
		maxIdleConns = config.RedisConnection.MaxIdleConns
	} else {
		return fmt.Errorf("Redis配置缺少连接信息，请在scheduled_push或redis_connection中配置")
	}

	// 检查必要的连接配置
	if addr == "" {
		return fmt.Errorf("Redis配置缺少addr字段，请在scheduled_push或redis_connection中配置")
	}

	// 设置默认值
	if timeout == 0 {
		timeout = 2 * time.Second
	}
	if poolSize == 0 {
		poolSize = 10
	}
	if minIdleConns == 0 {
		minIdleConns = 2
	}
	if maxIdleConns == 0 {
		maxIdleConns = 5
	}

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

	r.client = redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := r.client.Ping(ctx).Result(); err != nil {
		return fmt.Errorf("Redis 连接失败: %v", err)
	}

	return nil
}

// SaveClientData 保存客户端监控数据到 Redis
func (r *ClientDataRepositoryImpl) SaveClientData(data *scheduledPushEntity.ClientMonitorData, ttl time.Duration) error {
	if r.client == nil {
		return fmt.Errorf("Redis 客户端未初始化")
	}

	key := scheduledPushEntity.ClientDataKey(data.HostIP, data.Timestamp)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化数据失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := r.client.Set(ctx, key, jsonData, ttl).Err(); err != nil {
		return fmt.Errorf("保存数据到 Redis 失败: %v", err)
	}

	return nil
}

// GetClientDataByKey 根据 key 获取客户端数据
func (r *ClientDataRepositoryImpl) GetClientDataByKey(key string) (*scheduledPushEntity.ClientMonitorData, error) {
	if r.client == nil {
		return nil, fmt.Errorf("Redis 客户端未初始化")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("从 Redis 读取数据失败: %v", err)
	}

	var data scheduledPushEntity.ClientMonitorData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("反序列化数据失败: %v", err)
	}

	return &data, nil
}

// GetAllClientDataKeys 获取所有客户端数据 key
func (r *ClientDataRepositoryImpl) GetAllClientDataKeys() ([]string, error) {
	if r.client == nil {
		return nil, fmt.Errorf("Redis 客户端未初始化")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pattern := "gwatch:client:*"
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("获取 keys 失败: %v", err)
	}

	return keys, nil
}

// DeleteClientData 删除客户端数据
func (r *ClientDataRepositoryImpl) DeleteClientData(key string) error {
	if r.client == nil {
		return fmt.Errorf("Redis 客户端未初始化")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("删除数据失败: %v", err)
	}

	return nil
}
