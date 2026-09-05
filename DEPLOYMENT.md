# new-api 部署文档

## 版本：v1.1

## 一、更新内容

### 1. 默认货币改为人民币

- 系统默认货币从 USD 改为 CNY（人民币）

- 充值、计费、显示等全链路均以人民币为默认单位

### 2. 新增模型看板

**路径**：侧边栏 → 数据看板 → 模型看板（仅管理员可见）

**功能**：

- 按 5 分钟粒度聚合展示请求量、Prompt Tokens、Completion Tokens 趋势

- 支持按 Key / 模型 筛选，默认"忽略 Key"仅按模型分组

- **实时模式**：最近 1/2/4 小时选项，可选刷新频率（低 1分/中 30秒/高 10秒），默认 1 小时

- **历史模式**：按时间段筛选，最长 48 小时，X 轴显示 `MM-DD HH:mm`

- 受运营设置中"启用模型看板"开关控制

**API**：

- `GET /api/log/model_dashboard?hours=N&ignore_key=true` — 实时模式

- `GET /api/log/model_dashboard?start_timestamp=X&end_timestamp=Y&ignore_key=true` — 历史模式

### 3. 新增限流看板

**路径**：侧边栏 → 数据看板 → 限流看板（仅管理员可见）

**功能**：

- 按限流周期（默认 10 分钟）统计请求数和 Token 数

- **实时模式**：最近 1/2/4 小时选项，可选刷新频率，默认 1 小时

- **历史模式**：按时间段筛选，最长 48 小时，按限流周期统计呈现

- Account 筛选需至少输入 7 位才进行模糊匹配

- 受运营设置中"启用限流看板"开关控制

**API**：

- `GET /api/log/rate_limit_dashboard?periods=N` — 实时模式

- `GET /api/log/rate_limit_dashboard?start_timestamp=X&end_timestamp=Y` — 历史模式

### 4. 新增性能看板

**路径**：侧边栏 → 数据看板 → 性能看板（仅管理员可见）

**功能**：

- 按模型/Key 分组，统计以下指标的最大、最小、平均值：

  - **首字节时间 (FRT)**：仅 `is_stream=true` 的请求

  - **Token 生成速率**：流式 `completion_tokens / (use_time - frt/1000)`，非流式 `completion_tokens / use_time`

  - **RPM**：每分钟请求数

  - **TPM**：每分钟 Token 数

- 顶部 4 个 KPI 卡片展示汇总 min/avg/max

- 4 个 Tab 切换趋势图（FRT、Token 速率、RPM、TPM）

- 支持按 Key / 模型 筛选，默认"忽略 Key"

- **实时模式**：最近 1/2/4 小时，默认 1 小时

- **历史模式**：最长 48 小时

- 受运营设置中"启用性能看板"开关控制

**API**：

- `GET /api/log/performance_dashboard?hours=N&ignore_key=true` — 实时模式

- `GET /api/log/performance_dashboard?start_timestamp=X&end_timestamp=Y&ignore_key=true` — 历史模式

### 5. 新增 DB 看板（MySQL 监控）

**路径**：侧边栏 → 数据看板 → DB 看板（仅管理员可见）

**功能**：

- 仅支持 MySQL 数据库

- **5 个指标卡片**：连接数、缓冲池命中率、慢查询数、死锁数、临时表落盘

- **实时折线图**：QPS/TPS 趋势、网络流入/流出趋势（每次刷新累积数据点，最多 60 个）

- **慢查询表格**：运行 >10 秒的用户查询，支持分页和 KILL 操作

- **锁等待表格**：通过 `performance_schema.data_lock_waits` 查询

- **刷新模式**：低(1分)/中(30秒)/高(10秒)/不刷新，默认中；选择"不刷新"时显示手动刷新按钮

- 受运营设置中"启用 MySQL 面板"开关控制

**API**：

- `GET /api/log/mysql_monitor` — 获取监控数据

- `POST /api/log/mysql_kill?id=N` — KILL 指定进程

### 6. 管理端和 API 服务容器分离

- **管理端**：处理管理后台、设置、仪表板等管理功能

- **API 服务端**：设置 `RELAY_ONLY=true`，仅处理 AI API relay 流量

- 通过 Nginx 负载均衡将 API 请求分发到多个 API 服务节点

- 所有节点共享同一 MySQL 数据库和 Redis 缓存

- 系统任务锁使用 `INSERT IGNORE` 避免多节点竞争时的重复键错误

### 7. 看板通用配置

- **运营设置**：系统设置 → 运营设置中新增"看板设置"卡片，可控制各看板是否展示

- **实时刷新频率**：低(1分)/中(30秒)/高(10秒)，默认中

- **历史查询限制**：前后端均校验，最多 48 小时

- **实时时间选项**：模型/限流/性能看板均仅提供最近 1/2/4 小时选项（模型看板额外保留 24 小时）

### 8. 侧边栏图标

- 模型看板：`LineChart`（折线图）

- 限流看板：`Gauge`（仪表盘）

- 性能看板：`Activity`（脉冲线）

- DB 看板：`Database`（数据库）

***

## 二、部署架构

```
                    ┌─────────────────────────────────────┐
                    │           外部请求                    │
                    └──────────┬───────────┬──────────────┘
                               │           │
                    端口 3000   │           │  端口 3001
                               ▼           ▼
                    ┌──────────────┐  ┌──────────────┐
                    │  new-api-admin│  │ new-api-nginx│
                    │  (管理端)     │  │ (负载均衡)    │
                    └──────┬───────┘  └──┬───┬───┬───┘
                           │             │   │   │
                           │     ┌───────┘   │   └───────┐
                           │     ▼           ▼           ▼
                           │  ┌──────┐  ┌──────┐  ┌──────┐
                           │  │api-1 │  │api-2 │  │api-3 │
                           │  │RELAY │  │RELAY │  │RELAY │
                           │  └──┬───┘  └──┬───┘  └──┬───┘
                           │     │         │         │
                    ┌──────┴─────┴─────────┴─────────┘
                    │           共享层
                    ▼
              ┌───────────┐     ┌───────────┐
              │  MySQL 8   │     │  Redis 7  │
              │  (主库)    │     │  (缓存)   │
              └───────────┘     └───────────┘
```

### 组件说明

| 容器              | 角色       | 端口   | 说明                          |
| --------------- | -------- | ---- | --------------------------- |
| `new-api-admin` | 管理端      | 3000 | 管理后台、设置、看板                  |
| `new-api-nginx` | 负载均衡     | 3001 | Nginx 反向代理，least\_conn      |
| `new-api-1`     | API 服务 1 | -    | RELAY\_ONLY 模式，处理 AI API 请求 |
| `new-api-2`     | API 服务 2 | -    | 同上                          |
| `new-api-3`     | API 服务 3 | -    | 同上                          |
| `new-api-redis` | 缓存       | -    | Redis 7，会话共享、限流、缓存          |

***

## 三、部署步骤

### 前置条件

- Docker 20.10+

- Docker Compose 2.0+

- MySQL 5.7+ / 8.0（已运行或单独部署）

- 已构建的 new-api 镜像

### 步骤 1：构建镜像

```bash
cd /path/to/new-api
docker build -t new-api:rc.23-split .
```

### 步骤 2：准备配置文件

创建部署目录并准备配置：

```bash
mkdir -p deploy
```

**nginx.conf**（`deploy/nginx.conf`）：

```nginx
upstream new_api_servers {
    server new-api-1:3000;
    server new-api-2:3000;
    server new-api-3:3000;
    least_conn;
}

server {
    listen 3000;
    server_name _;

    location / {
        proxy_pass http://new_api_servers;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_buffering off;
        proxy_read_timeout 300s;
        proxy_connect_timeout 10s;
    }
}
```

**docker-compose.yml**（`deploy/docker-compose.yml`）：

```yaml
services:
  # 管理端
  new-api-admin:
    image: new-api:rc.23-split
    container_name: new-api-admin
    restart: always
    environment:
      - TZ=Asia/Shanghai
      - NODE_NAME=admin-1
      - SQL_DSN=root:PASSWORD@tcp(host.docker.internal:3306)/oneapi
      - REDIS_CONN_STRING=redis://:REDIS_PASSWORD@new-api-redis:6379
      - SESSION_SECRET=your-session-secret-change-me
      - SYNC_FREQUENCY=60
      - BATCH_UPDATE_ENABLED=true
    volumes:
      - admin_data:/data
    ports:
      - "3000:3000"
    extra_hosts:
      - "host.docker.internal:host-gateway"
    networks:
      - new-api-net

  # API 服务端 1
  new-api-1:
    image: new-api:rc.23-split
    container_name: new-api-1
    restart: always
    environment:
      - TZ=Asia/Shanghai
      - NODE_NAME=api-1
      - SQL_DSN=root:PASSWORD@tcp(host.docker.internal:3306)/oneapi
      - REDIS_CONN_STRING=redis://:REDIS_PASSWORD@new-api-redis:6379
      - SESSION_SECRET=your-session-secret-change-me
      - RELAY_ONLY=true
      - SYNC_FREQUENCY=60
      - BATCH_UPDATE_ENABLED=true
    volumes:
      - api1_data:/data
    extra_hosts:
      - "host.docker.internal:host-gateway"
    networks:
      - new-api-net

  # API 服务端 2（同上，NODE_NAME=api-2）
  # API 服务端 3（同上，NODE_NAME=api-3）

  # Nginx 负载均衡
  new-api-nginx:
    image: nginx:alpine
    container_name: new-api-nginx
    restart: always
    ports:
      - "3001:3000"
    volumes:
      - ./nginx.conf:/etc/nginx/conf.d/default.conf:ro
    depends_on:
      - new-api-1
      - new-api-2
      - new-api-3
    networks:
      - new-api-net

  # Redis
  new-api-redis:
    image: redis:7-alpine
    container_name: new-api-redis
    restart: always
    command: redis-server --requirepass REDIS_PASSWORD
    volumes:
      - redis_data:/data
    networks:
      - new-api-net

networks:
  new-api-net:
    driver: bridge

volumes:
  admin_data:
  api1_data:
  api2_data:
  api3_data:
  redis_data:
```

### 步骤 3：关键配置说明

| 环境变量                   | 说明                                             |   必填  |
| ---------------------- | ---------------------------------------------- | :---: |
| `SQL_DSN`              | MySQL 连接串，格式 `user:pass@tcp(host:port)/dbname` |   是   |
| `REDIS_CONN_STRING`    | Redis 连接串，格式 `redis://:pass@host:port`         |   是   |
| `SESSION_SECRET`       | 会话密钥，所有节点必须一致                                  |   是   |
| `RELAY_ONLY=true`      | 仅运行 relay 模式（API 服务端设置）                        | API 端 |
| `NODE_NAME`            | 节点标识，每个节点唯一                                    |   是   |
| `SYNC_FREQUENCY`       | 配置同步频率（秒），建议 60                                |   否   |
| `BATCH_UPDATE_ENABLED` | 批量更新开关，多节点建议开启                                 |   否   |
| `TZ`                   | 时区，建议 `Asia/Shanghai`                          |   否   |

### 步骤 4：启动服务

```bash
cd deploy
docker compose up -d
```

### 步骤 5：验证

```bash
# 检查容器状态
docker compose ps

# 验证管理端
curl http://localhost:3000/api/status

# 验证 API 端（通过 nginx，应返回 401 未认证）
curl http://localhost:3001/v1/models

# 查看日志
docker compose logs -f new-api-admin
docker compose logs -f new-api-1
docker compose logs -f new-api-nginx
```

***

## 四、运维操作

### 扩缩容 API 节点

在 `docker-compose.yml` 中复制 `new-api-1` 配置，修改 `container_name` 和 `NODE_NAME`，然后在 `nginx.conf` 的 upstream 中添加新节点：

```nginx
upstream new_api_servers {
    server new-api-1:3000;
    server new-api-2:3000;
    server new-api-3:3000;
    server new-api-4:3000;  # 新增
    least_conn;
}
```

然后：

```bash
docker compose up -d
docker exec new-api-nginx nginx -s reload
```

### 更新镜像

```bash
cd /path/to/new-api
docker build -t new-api:rc.23-split .
cd deploy
docker compose up -d
```

### 查看日志

```bash
# 管理端日志
docker logs -f new-api-admin

# 某个 API 节点日志
docker logs -f new-api-1

# Nginx 访问日志
docker logs -f new-api-nginx
```

### 压力测试

```bash
# 管理端 100 次并发
for i in $(seq 1 100); do curl -s -o /dev/null -w '%{http_code} ' http://localhost:3000/api/status & done; wait

# API 端 100 次并发（通过 nginx）
for i in $(seq 1 100); do curl -s -o /dev/null -w '%{http_code} ' http://localhost:3001/v1/models & done; wait
```

***

## 五、注意事项

1. **SESSION\_SECRET**：所有节点必须使用相同的 SESSION\_SECRET，否则会话无法跨节点共享
2. **MySQL 连接**：如果 MySQL 不在 Docker 网络中，使用 `host.docker.internal:port` 并配置 `extra_hosts`
3. **Redis 密码**：生产环境务必设置 Redis 密码
4. **数据卷**：各节点有独立的数据卷（`/data`），仅用于存储临时文件，业务数据在 MySQL 中
5. **RELAY\_ONLY 模式**：API 服务端不提供管理后台和 `/api/status` 接口，仅处理 `/v1/*` relay 请求
6. **看板开关**：在管理后台 → 系统设置 → 运营设置中控制各看板是否展示
7. **历史查询限制**：所有看板历史查询最长 48 小时，前后端双重校验
8. **DB 看板**：仅支持 MySQL，非 MySQL 数据库会返回提示信息

