package external

import (
	domaincfg "github.com/youxihu/GWatch-new/internal/domain/config"
	entity "github.com/youxihu/GWatch-new/internal/domain/entity/monitoring"
	shared "github.com/youxihu/GWatch-new/internal/domain/entity/shared"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCollector struct {
	provider domaincfg.Provider
	rdb      *redis.Client
}

func NewRedisCollector(p domaincfg.Provider) *RedisCollector { return &RedisCollector{provider: p} }

func (c *RedisCollector) Init() error {
	cfg := c.provider.GetConfig()
	if cfg == nil {
		return fmt.Errorf("配置未加载")
	}

	// 优先使用app_monitoring.redis中的配置，如果未配置addr则使用公共redis_connection配置
	// 注意：Init()方法不检查enabled，因为enabled的检查在调用Init()之前已经完成
	var r *shared.RedisConfig
	if cfg.AppMonitoring != nil && cfg.AppMonitoring.Redis != nil {
		r = cfg.AppMonitoring.Redis
	}

	// 获取Redis连接配置
	var addr, password string
	var db, poolSize, minIdleConns, maxIdleConns int
	var timeout time.Duration

	if r != nil && r.Addr != "" {
		// 使用app_monitoring.redis中的配置
		addr = r.Addr
		password = r.Password
		db = r.DB
		timeout = r.Timeout
		poolSize = r.PoolSize
		minIdleConns = r.MinIdleConns
		maxIdleConns = r.MaxIdleConns
	} else if cfg.RedisConnection != nil {
		// 使用公共redis_connection配置
		addr = cfg.RedisConnection.Addr
		password = cfg.RedisConnection.Password
		db = cfg.RedisConnection.DB
		timeout = cfg.RedisConnection.Timeout
		poolSize = cfg.RedisConnection.PoolSize
		minIdleConns = cfg.RedisConnection.MinIdleConns
		maxIdleConns = cfg.RedisConnection.MaxIdleConns
	} else {
		return fmt.Errorf("Redis配置缺少addr字段，请在app_monitoring.redis或redis_connection中配置addr")
	}

	// 检查必要的连接配置
	if addr == "" {
		return fmt.Errorf("Redis配置缺少addr字段，请在app_monitoring.redis或redis_connection中配置addr")
	}

	// 设置默认值
	if poolSize == 0 {
		poolSize = 5
	}
	if minIdleConns == 0 {
		minIdleConns = 1
	}
	if maxIdleConns == 0 {
		maxIdleConns = 3
	}
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	options := &redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  timeout,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		PoolSize:     poolSize,
		MinIdleConns: minIdleConns,
		MaxIdleConns: maxIdleConns,
		PoolTimeout:  2 * time.Second,
	}
	c.rdb = redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := c.rdb.Ping(ctx).Result(); err != nil {
		return fmt.Errorf("Redis连接测试失败: %v", err)
	}
	return nil
}

func (c *RedisCollector) GetClients() (int, error) {
	if c.rdb == nil {
		return 0, fmt.Errorf("Redis客户端未初始化，请先调用Init()")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 获取INFO clients信息
	val, err := c.rdb.Info(ctx, "clients").Result()
	if err != nil {
		return 0, fmt.Errorf("执行 INFO clients 失败: %v", err)
	}

	var total int
	// 解析connected_clients，支持多种分隔符（\r\n, \n）
	// 先尝试按\r\n分割，如果失败则按\n分割
	lines := strings.Split(val, "\r\n")
	if len(lines) == 1 {
		lines = strings.Split(val, "\n")
	}

	found := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行和注释行（以#开头的行，如 "# Clients"）
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "connected_clients:") {
			// 使用SplitN限制分割次数为2，避免值中包含冒号时出错
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				clients, err := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err != nil {
					return 0, fmt.Errorf("解析 connected_clients 失败: %v (值: %s, 原始行: %s)", err, parts[1], line)
				}
				total = clients
				found = true
				break
			}
		}
	}

	// 如果没有找到connected_clients，返回错误
	if !found {
		return 0, fmt.Errorf("未找到 connected_clients 字段，INFO clients 输出: %s", val)
	}

	// 获取CLIENT LIST，直接统计非监控连接的数量（更准确）
	list, err := c.rdb.ClientList(ctx).Result()
	if err != nil {
		// 如果获取CLIENT LIST失败，返回total（至少知道总数）
		return total, nil
	}

	actualConnections := 0
	// 解析CLIENT LIST，支持多种分隔符
	clientLines := strings.Split(list, "\n")
	for _, line := range clientLines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 检查是否是监控相关的命令（当前监控程序自己的连接）
		// Redis的CLIENT LIST输出格式是空格分隔的key=value对
		// 例如：id=123 addr=127.0.0.1:12345 fd=6 name= age=10 idle=0 flags=N db=0 cmd=info
		isMonitorConnection := false

		// 检查cmd字段，排除监控命令
		// cmd=client|list 或 cmd=client list 表示正在执行CLIENT LIST命令
		// cmd=info 表示正在执行INFO命令
		// 注意：Redis中 | 是字段分隔符，实际输出中可能是空格分隔
		if strings.Contains(line, "cmd=client") {
			// 检查是否是client list命令
			// Redis输出格式可能是：cmd=client|list 或 cmd=client list
			if strings.Contains(line, "list") {
				isMonitorConnection = true
			}
		} else if strings.Contains(line, "cmd=info") {
			// 排除正在执行INFO命令的连接（可能是当前监控程序）
			isMonitorConnection = true
		}

		// 只统计非监控连接
		if !isMonitorConnection {
			actualConnections++
		}
	}

	// 如果统计出的连接数明显不合理（比如为0但total>0），使用total作为fallback
	// 但通常actualConnections应该更准确，因为它直接统计了非监控连接
	if actualConnections == 0 && total > 0 {
		// 如果统计为0但总数>0，可能是过滤逻辑有问题，返回total
		return total, nil
	}

	return actualConnections, nil
}

func (c *RedisCollector) GetClientsDetail() ([]entity.ClientInfo, error) {
	if c.rdb == nil {
		return nil, fmt.Errorf("Redis客户端未初始化，请先调用Init()")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	val, err := c.rdb.ClientList(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("执行 CLIENT LIST 失败: %v", err)
	}
	var clients []entity.ClientInfo
	for _, line := range strings.Split(val, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "cmd=client|list") || strings.Contains(line, "cmd=info") || strings.Contains(line, "cmd=ping") || strings.Contains(line, "cmd=NULL") {
			continue
		}
		pairs := strings.Split(line, " ")
		client := entity.ClientInfo{}
		for _, pair := range pairs {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) != 2 {
				continue
			}
			switch kv[0] {
			case "id":
				client.ID = kv[1]
			case "addr":
				client.Addr = kv[1]
			case "age":
				client.Age = kv[1]
			case "idle":
				client.Idle = kv[1]
			case "flags":
				client.Flags = kv[1]
			case "db":
				client.Db = kv[1]
			case "cmd":
				client.Cmd = kv[1]
			}
		}
		clients = append(clients, client)
	}
	return clients, nil
}

func (c *RedisCollector) Close() {
	if c.rdb != nil {
		c.rdb.Close()
	}
}
