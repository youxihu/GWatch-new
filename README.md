# GWatch — 通用型服务器监控系统

[![Version](https://img.shields.io/badge/version-4.0.1-blue)](CHANGELOG.md)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

GWatch 是 Go 语言开发的通用型服务器监控系统，采用 DDD 三层架构和 Wire 编译时依赖注入，提供主机监控、应用监控、证书监控、智能告警和分布式定时推送功能。

---

## 核心特性

### 主机监控
- CPU / 内存 / 磁盘 使用率实时采集，P1/P2/P3 三级阈值可独立配置
- 进程级监控：自动识别高资源占用进程，支持进程白名单过滤
- 磁盘 IO 读写速率和网络流量监控
- 磁盘占用深度分析：递归扫描热点目录，自动定位大文件/大目录

### 应用层监控
- Redis 连接数区间监控（min/max），支持客户端连接详情分析
- HTTP 多接口并发健康检查，可配置状态码白名单，独立监控周期
- 待持续补充

### 安全监控
- HTTPS 证书过期监控：多域名独立配置，到期前自动提醒
- TLS 握手直接拉取叶证书，不依赖 CA 链验证
- 证书拉取失败独立告警，异步通知不阻塞主循环

### 智能告警
- 四级告警：P1(紧急) / P2(严重) / P3(警告) / Reminder(提醒)
- 防抖机制：可配置告警间隔，避免刷屏
- 持续时长要求：告警须持续超过阈值一定时间才触发
- 恢复通知：自动检测恢复并推送恢复通知（本机 → 进程 → 全局三层验证）
- 状态持久化：Redis 存储告警状态，程序重启后自动恢复活跃告警
- 事件追踪：每个告警唯一 EventID（前缀+编号+时间戳），完整生命周期追踪

### 分布式定时推送
- Client/Server 双模式：Client 采集数据上传 Redis，Server 聚合所有客户端数据统一推送
- 多时间点推送：可配置每天多个推送时间
- 钉钉 Markdown 格式通知，支持 @指定人员

---

## 架构

```
cmd/                          # 入口 + Wire 依赖注入
├── main.go                   # 主程序入口
├── wire.go                   # Wire 注入定义
└── wire_gen.go               # Wire 自动生成

internal/
├── app/usecase/              # 用例层（编排）
│   ├── monitoring/           # 主机/应用监控调度
│   │   ├── coordinator.go    # 双周期调度协调器
│   │   ├── metrics_collect.go # 指标采集
│   │   └── metrics_notify.go  # 告警通知
│   ├── security_monitoring/  # 证书监控
│   ├── scheduled_push/       # 定时推送
│   └── scheduler/            # 推送调度器
│
├── domain/                   # 领域层（接口 + 值对象）
│   ├── entity/               # 实体：AlertType, Config, Metrics
│   ├── monitoring/           # 接口：Policy, Evaluator, Notifier, Formatter
│   ├── collector/            # 采集器接口：HostCollector, RedisCollector, HTTPCollector
│   ├── scheduled_push/       # 推送领域接口
│   └── filesystem/           # 文件系统扫描接口
│
├── infra/                    # 基础设施层（实现）
│   ├── monitoring/           # 策略引擎、告警格式化、钉钉通知、证书拉取
│   ├── collector/            # 主机/Redis/HTTP 采集实现
│   ├── filesystem/           # 磁盘扫描实现
│   ├── scheduled_push/       # 推送数据仓库、格式化器、日志存储
│   ├── config/               # YAML 配置加载
│   └── logger/               # 日志（zap + lumberjack）
│
└── utils/                    # 工具函数：等级计算、进程过滤、事件ID、格式化
```

**DDD 三层**：`domain`（接口+值对象） → `infra`（实现） → `app`（编排）

---

## 快速开始

### 环境要求
- Go 1.24+
- Redis（告警状态存储 + 分布式数据交换）
- 钉钉机器人 Webhook

### 编译

```bash
git clone https://github.com/youxihu/GWatch-new.git
cd GWatch-new
make wire && make build
```

### 配置

```bash
cp config/config_example.yml config/config.yml
```

修改 `config/config.yml` 中的必填项：

| 配置项 | 说明 |
|--------|------|
| `dingtalk.webhook_url` | 钉钉机器人 Webhook 地址 |
| `dingtalk.secret` | 钉钉机器人加签密钥（如使用加签方式） |
| `redis_connection.addr` | Redis 地址 |
| `host_monitoring.alert_title` | 告警标题（服务器标识） |

### 运行

```bash
./bin/Gwatch -c config/config.yml
```

命令行参数：

| 参数 | 说明 |
|------|------|
| `-c / --config` | 指定配置文件路径（优先级高于环境变量） |
| `-v / --version` | 显示版本信息 |

环境变量 `GWATCH_CONFIG` 也可指定配置文件路径。

---

## 告警类型一览

| 类型 | 常量 | 中文名 |
|------|------|--------|
| CPU 过高 | `cpu_high` | CPU使用率过高 |
| CPU 监控失败 | `cpu_error` | CPU监控失败 |
| 内存过高 | `mem_high` | 内存使用率过高 |
| 内存监控失败 | `mem_error` | 内存监控失败 |
| 磁盘过高 | `disk_high` | 磁盘使用率过高 |
| 磁盘监控失败 | `disk_error` | 磁盘监控失败 |
| 磁盘读 IO 过高 | `disk_io_read_high` | 磁盘读IO过高 |
| 磁盘写 IO 过高 | `disk_io_write_high` | 磁盘写IO过高 |
| Redis 连接数过高 | `redis_high` | Redis连接数过高 |
| Redis 连接数过低 | `redis_low` | Redis连接数过低 |
| Redis 异常 | `redis_error` | Redis连接异常 |
| 网络监控失败 | `network_error` | 网络监控失败 |
| HTTP 接口失败 | `http_error` | HTTP接口监控失败 |
| 证书即将过期 | `certificate_expiring` | HTTPS证书即将过期 |
| 证书检查失败 | `certificate_check_error` | HTTPS证书检查失败 |

---

## 告警通知示例

```markdown
## [告警] CPU使用率过高

- 事件ID: c10100123
- 告警等级: 紧急
- 触发条件: CPU使用率超过阈值 95.0%
- 触发对象: java(12345)
- CPU使用率: 96.5%
- 触发时间: 2026-06-01 15:30:00
```

恢复通知：

```markdown
## [故障恢复] CPU使用率过高

- 事件ID: c10100123
- 告警等级: 恢复正常
- CPU使用率已降至 45.2%
- 触发时间: 2026-06-01 16:00:00
```

---

## 证书监控配置示例

```yaml
certificate_expiration_monitoring:
  enabled: true
  collect_interval: 12h
  alert_title: "HTTPS证书过期提醒"
  warning_days: 15
  domains:
    - name: "example.com"
      port: 443
      enabled: true
  alert_log:
    enabled: true
    log_path_template: "logs/certificate/%y/%m-%d/cert-%H%M%S.md"
    retention_days: 30
```

---

## 定时推送配置示例

```yaml
scheduled_push:
  enabled: true
  mode: "server"                          # client 或 server
  push_times: ["8:00", "12:00", "18:00"]
  title: "服务器性能监控定时报告"
  server_aggregation_delay_seconds: 30    # Server 聚合等待时间
  data_log:
    enabled: true
    client_path_template: "logs/scheduled_push/client/%y/%m-%d/client-%H%M-%S.json"
    server_path_template: "logs/scheduled_push/server/%y/%m-%d/report-%H%M-%S.md"
    retention_days: 30
```

---

## License

MIT