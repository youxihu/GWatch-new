# 更新日志

---

## [Unreleased] - 2026-06-03

### 修复
- **Makefile**: 修复 LDFLAGS 路径错误，版本信息注入不生效
- **HTTP 监控**: 修复 NeedAlert 字段未传递导致配置 `need_alert: true` 不生效的问题
- **日志调度**: 修复 HTTP 接口日志在基础采集周期（5s）重复打印，改为仅在 HTTP 采集周期（30s）打印
- **并发安全**: 修复 host_collector.go 包级全局变量竞态风险，封装到 Collector 结构体并加锁保护
- **进程过滤**: 移除 process_utils.go 中硬编码的 "kiro" 进程名，改为通过 PID 排除自身进程
- **HTTP 告警**: 修复 simple_evaluator.go 中 HTTP 监控只报告第一个异常接口的问题，支持同时报告所有异常接口
- **主机名获取**: 修复 GetHostname 使用不可靠的 `net.LookupCNAME("")`，改用 `os.Hostname()`

### 改进
- **代码拆分**: 将 stateful_policy_engine.go 847行巨型函数拆分为 4 个职责清晰的文件（engine/recovery/state/emit）
- **消除重复**: 提取 metrics_collect.go 中 Redis 采集公共方法，消除 CollectOnce 和 CollectBaseOnce 的重复代码
- **消除重复**: 提取 wire.go 中 StatefulPolicy 创建公共方法，消除 NewBasePolicy 和 NewHTTPPolicy 的重复代码
- **架构优化**: 修复 usecase 层直接依赖 infra logger 的层级违规，改为通过依赖注入使用 domain logger 接口
- **死代码清理**: 删除未使用的 getTypeIndicator 和 AlertTypeRequiresConsecutive
- **性能优化**: Redis AlertStateStorage 使用 Set 索引替代 KEYS 全量扫描，O(1) 查找替代 O(n) 扫描

---

## [4.0.2] - 2026-06-02

### 改进
- 构建系统重构：Makefile 增加版本变量、ldflags、UPX 压缩，新增 docker/clean 目标
- cmd/main.go：修复配置环境变量设置时机（移到 Wire 初始化前）
- Docker：Dockerfile 移至 docker/，优化镜像层，新增 docker-compose.yml
- README：Docker 部署章节改用 docker-compose，补充 `user: "1000:1000"` 权限陷阱和 `log_path_template` 容器路径陷阱

## [4.0.1] - 2026-06-01

### 新增
- 主机监控：CPU / 内存 / 磁盘使用率实时采集，P1/P2/P3 三级阈值可独立配置
- 进程级监控：自动识别高资源占用进程，支持进程白名单过滤
- 磁盘 IO 读写速率和网络流量监控
- 磁盘占用深度分析：递归扫描热点目录，自动定位大文件/大目录
  - 子目录占比超阈值才深入，分布均匀时停止递归
  - 并发扫描 + 内存限制 + 超时保护
- Redis 连接数区间监控（min/max），支持客户端连接详情分析
- HTTP 多接口并发健康检查，可配置状态码白名单，独立监控周期
- HTTPS 证书过期监控：多域名独立配置，TLS 握手直接拉取叶证书
  - 到期前自动提醒 / 拉取失败独立告警 / 异步通知不阻塞主循环

### 告警系统
- 四级告警：P1(紧急) / P2(严重) / P3(警告) / Reminder(提醒)
- 防抖机制：可配置告警间隔，避免刷屏
- 持续时长要求：告警须持续超过阈值一定时间才触发
- 恢复通知：自动检测恢复并推送恢复通知（本机 → 进程 → 全局三层验证）
- 状态持久化：Redis 存储告警状态，程序重启后自动恢复活跃告警
- 事件追踪：每个告警唯一 EventID，完整生命周期追踪
  - 见名知意格式：`{类型}_{等级}_{时间}_{hash}_{次数}`，如 `cpu_high_p1_202606011530_a3f2c1_3`
  - 一眼可辨：类型、等级、时间、第几次告警

### 分布式定时推送
- Client/Server 双模式：Client 采集数据上传 Redis，Server 聚合所有客户端数据统一推送
- 多时间点推送：可配置每天多个推送时间
- 钉钉 Markdown 格式通知，支持 @指定人员
- 数据日志存储：Client 原始数据 JSON + Server 聚合报告 Markdown

### 架构
- DDD 三层架构：domain（接口+值对象） → infra（实现） → app（编排）
- Wire 编译时依赖注入，零运行时反射
- 用例层零 infra 类型依赖，全链路使用 domain 接口
- 策略引擎 StatefulPolicy 支持多进程告警合并、等级变化、恢复检测
- 双周期调度协调器：基础监控 + HTTP 监控独立周期，共享触发联动

### 代码质量
- 84 个 Go 源文件，9,943 行
- 零全局变量，零废弃代码，零英文注释
- 大文件拆分：StatefulPolicy 936→101+847，metrics 524→309+226，disk_scanner 529→105+432
- 并发安全：全局 logger 加锁保护，Redis 连接改为实例字段
- 单文件编译部署，UPX 压缩后 ~3.3MB
- Docker 支持：`--pid=host --net=host` 读取宿主机指标，`GWATCH_ROOTFS` 环境变量适配容器磁盘路径

### 配置
- YAML 配置文件，支持环境变量 `GWATCH_CONFIG`
- 命令行 `-c / --config` 指定配置文件路径
- `-v / --version` 显示版本信息
- 完整配置模板 `config/config_example.yml`，所有配置项均有中文注释