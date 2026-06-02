## GWatch — 通用型服务器监控系统

GWatch 是 Go 语言开发的轻量级服务器监控工具，支持 CPU/内存/磁盘/进程/Redis/HTTP/SSL 证书全方位监控，通过钉钉机器人推送告警。

### 支持架构

`linux/amd64` `linux/arm64`

### 快速启动

```bash
docker run -d \
  --name gwatch \
  --pid=host --net=host \
  -v /:/host:ro \
  -v ./config.yml:/app/acc/config.yml:ro \
  -v ./logs:/app/acc/logs \
  -e GWATCH_ROOTFS=/host \
  youxihu/gwatch:v4.0.2
```

或使用 docker-compose（推荐）：

```yaml
services:
  gwatch:
    image: youxihu/gwatch:v4.0.2
    pid: "host"
    network_mode: "host"
    user: "1000:1000"
    restart: unless-stopped
    environment:
      - GWATCH_ROOTFS=/host
      - TZ=Asia/Shanghai
    volumes:
      - /:/host:ro
      - ./config.yml:/app/acc/config.yml:ro
      - ./logs:/app/acc/logs
```

### 必配项

| 配置项 | 说明 |
|--------|------|
| `dingtalk.webhook_url` | 钉钉机器人 Webhook 地址 |
| `redis_connection.addr` | Redis 地址（告警状态持久化） |
| `host_monitoring.alert_title` | 告警标题（服务器标识） |

完整配置模板见 [config_example.yml](https://github.com/youxihu/GWatch-new/blob/master/config/config_example.yml)。

### 监控能力

- **主机**：CPU / 内存 / 磁盘 / IO / 进程级监控，三级阈值可配
- **应用**：Redis 连接数、HTTP 接口健康检查
- **安全**：HTTPS 证书过期提前预警
- **告警**：四级告警 + 防抖 + 恢复通知 + 状态持久化
- **推送**：Client/Server 分布式定时推送

### 容器陷阱

1. 必须加 `user: "1000:1000"`，否则日志文件为 root 权限，宿主机读不了
2. 配置文件中的 `log_path_template` 必须用容器内相对路径，如 `logs/alerts/...`，不能写宿主机绝对路径

---

源码：https://github.com/youxihu/GWatch-new