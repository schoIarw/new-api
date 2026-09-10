# New-API 2.0 Linux AMD64 部署手册

**适用介质**：`new-api-2.0-linux-amd64.tar.gz`  
**适用架构**：Linux x86-64 / amd64  
**版本标识**：2.0  
**发布基线**：`schoIarw/new-api` Tag `2.0`，Commit `829c95ed5fd8e6bb21f5ddc762eddb66ba652f15`  
**文档版本**：1.0（2026-09-10）

---

## 1. 文档说明

本文档针对已经编译完成的 `new-api-2.0-linux-amd64.tar.gz` 介质，分别给出：

1. **基于 Docker 的部署方法**：直接使用该二进制介质构建本地镜像，不依赖重新编译源码；包含 SQLite 单机部署、MySQL + Redis 生产部署、Nginx 反向代理及多节点扩展。
2. **不基于 Docker 的部署方法**：直接将二进制安装到 Linux 主机，通过 systemd 管理进程；包含 SQLite 单机部署、外置 MySQL + Redis 生产部署、Nginx、日志轮转、升级与回退。

本文档中的生产示例默认使用 `Asia/Shanghai` 时区、TCP 3000 端口。示例密码均为占位符，部署时必须替换。

### 1.1 发布介质内容

发布介质解压后应包含：

```text
new-api-2.0-linux-amd64/
├── new-api
├── VERSION
├── LICENSE
├── NOTICE
└── THIRD-PARTY-LICENSES.md
```

本次发布介质校验值：

```text
new-api-2.0-linux-amd64.tar.gz
SHA256: 3d37a143a5ac6715b9270112fe9f71b9cf190beb070efa06f5b24580130f2a6d

new-api 二进制
SHA256: 0a8b3c4fe9342351f0ccbf16638123878027006ef7a24a673678008137f34253
```

二进制属性：

```text
ELF 64-bit LSB executable, x86-64, statically linked, stripped
```

因此该介质只能原生运行在 Linux amd64/x86-64 环境。ARM64 主机不应直接使用该介质。

### 1.2 常用命令行参数

```text
Usage: newapi [--port <port>] [--log-dir <log directory>] [--version] [--help]
```

常用示例：

```bash
./new-api --version
./new-api --port 3000 --log-dir /var/log/new-api
```

### 1.3 运行时关键约定

- 默认服务端口：`3000`。
- 健康检查接口：`GET /api/status`。
- 本地数据库可使用 SQLite；生产环境建议使用 MySQL 或 PostgreSQL。
- 单机 SQLite 部署必须持久化数据库目录。
- 多节点部署必须共享数据库；建议同时使用 Redis。
- 多节点的 `SESSION_SECRET` 必须一致。
- 使用 Redis 时建议所有节点同时保持一致的 `CRYPTO_SECRET`。
- AI 流式请求经过 Nginx 时应关闭代理缓冲，否则首 token/SSE 可能出现额外延迟。
- 2.0 版本包含模型、限流、性能、DB 等看板能力；DB 看板按当前实现主要面向 MySQL。
- 模型看板使用 Prometheus 查询 vLLM 指标时，需要配置 `PROMETHEUS_URL` 等参数。

---

## 2. 推荐部署架构

### 2.1 测试/验证环境

适用于功能测试、小规模验证：

```text
Browser / API Client
        │
        ▼
   New-API 2.0
        │
        ▼
      SQLite
```

### 2.2 单节点生产环境

```text
Internet / Intranet
        │
        ▼
      Nginx
        │
        ▼
   New-API 2.0
      │      │
      ▼      ▼
    MySQL   Redis
```

### 2.3 多节点生产环境

推荐管理端与 Relay/API 节点分离：

```text
                 ┌──────────────┐
                 │   Nginx/LB   │
                 └──────┬───────┘
                        │
           ┌────────────┴────────────┐
           │                         │
           ▼                         ▼
  new-api-admin               new-api-api-1..N
  管理与看板                    RELAY_ONLY=true
           │                         │
           └────────────┬────────────┘
                        │
               ┌────────┴────────┐
               ▼                 ▼
             MySQL              Redis
```

所有 New-API 节点应共享：

- `SQL_DSN`
- `REDIS_CONN_STRING`
- `SESSION_SECRET`
- `CRYPTO_SECRET`

每个节点应使用唯一的：

```text
NODE_NAME
```

---

# 第一部分：基于 Docker 部署

## 3. Docker 部署前置条件

建议：

- Linux amd64/x86-64。
- Docker 20.10+。
- Docker Compose v2。
- 至少 2 CPU / 4 GB RAM 用于测试；生产按并发和日志规模评估。
- 若采用外置 MySQL/Redis，应先确认网络连通。

验证：

```bash
uname -m
docker version
docker compose version
```

`uname -m` 应输出：

```text
x86_64
```

## 4. 准备 2.0 介质

```bash
mkdir -p /opt/new-api-2.0-docker
cd /opt/new-api-2.0-docker
cp /path/to/new-api-2.0-linux-amd64.tar.gz .
```

校验：

```bash
sha256sum new-api-2.0-linux-amd64.tar.gz
```

应得到：

```text
3d37a143a5ac6715b9270112fe9f71b9cf190beb070efa06f5b24580130f2a6d
```

解压：

```bash
tar -xzf new-api-2.0-linux-amd64.tar.gz
cd new-api-2.0-linux-amd64
chmod +x new-api
./new-api --version
```

## 5. 使用已编译二进制构建 Docker 镜像

本次 2.0 tar.gz 已经包含嵌入前端的生产二进制，无需在目标服务器重新安装 Go/Bun，也无需重新构建源码。

创建运行时 Dockerfile：

```dockerfile
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata wget \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

COPY new-api /new-api
COPY VERSION /VERSION
COPY LICENSE NOTICE THIRD-PARTY-LICENSES.md /licenses/

RUN chmod +x /new-api

EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/new-api"]
```

保存为：

```text
/opt/new-api-2.0-docker/new-api-2.0-linux-amd64/Dockerfile.runtime
```

构建：

```bash
docker build \
  -f Dockerfile.runtime \
  -t new-api:2.0 .
```

验证镜像：

```bash
docker image inspect new-api:2.0 >/dev/null && echo OK
docker run --rm new-api:2.0 --version
```

## 6. Docker 方案 A：SQLite 单机部署

### 6.1 创建目录

```bash
mkdir -p /opt/new-api-2.0-docker/standalone/{data,logs}
cd /opt/new-api-2.0-docker/standalone
```

### 6.2 docker-compose.yml

```yaml
services:
  new-api:
    image: new-api:2.0
    container_name: new-api-2.0
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      TZ: Asia/Shanghai
      SQLITE_PATH: /data/new-api.db
      NODE_NAME: new-api-standalone
      ERROR_LOG_ENABLED: "true"
      BATCH_UPDATE_ENABLED: "true"
    volumes:
      - ./data:/data
      - ./logs:/app/logs
    command: ["--log-dir", "/app/logs"]
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://127.0.0.1:3000/api/status | grep -Eq '\"success\"[[:space:]]*:[[:space:]]*true'"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 20s
```

启动：

```bash
docker compose up -d
```

检查：

```bash
docker compose ps
docker logs --tail=100 new-api-2.0
curl -sS http://127.0.0.1:3000/api/status
```

浏览器访问：

```text
http://<服务器IP>:3000
```

首次启动按页面初始化向导创建管理员和系统配置。本文档不假定存在默认管理员口令。

### 6.3 SQLite 使用边界

SQLite 更适合单节点和测试场景。以下情况建议改用 MySQL/PostgreSQL：

- 多实例/多节点。
- 大量消费日志和看板查询。
- 高并发管理操作。
- 需要数据库高可用、备份、审计。

## 7. Docker 方案 B：MySQL + Redis 生产部署

### 7.1 创建目录

```bash
mkdir -p /opt/new-api-2.0-docker/prod/{data,logs}
cd /opt/new-api-2.0-docker/prod
```

### 7.2 创建 .env

```bash
cat > .env <<'ENV'
MYSQL_ROOT_PASSWORD=CHANGE_ME_ROOT_32CHARS
MYSQL_DATABASE=new_api
MYSQL_USER=newapi
MYSQL_PASSWORD=CHANGE_ME_DB_32CHARS
REDIS_PASSWORD=CHANGE_ME_REDIS_32CHARS
SESSION_SECRET=CHANGE_ME_SESSION_64CHARS
CRYPTO_SECRET=CHANGE_ME_CRYPTO_64CHARS
ENV
chmod 600 .env
```

建议用密码管理系统生成随机字符串，例如：

```bash
openssl rand -hex 32
```

### 7.3 创建 docker-compose.yml

```yaml
services:
  new-api:
    image: new-api:2.0
    container_name: new-api-2.0
    restart: unless-stopped
    ports:
      - "127.0.0.1:3000:3000"
    env_file:
      - .env
    environment:
      TZ: Asia/Shanghai
      NODE_NAME: new-api-node-1
      SQL_DSN: "${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(mysql:3306)/${MYSQL_DATABASE}?charset=utf8mb4&parseTime=true&loc=Local"
      REDIS_CONN_STRING: "redis://:${REDIS_PASSWORD}@redis:6379/0"
      SESSION_SECRET: "${SESSION_SECRET}"
      CRYPTO_SECRET: "${CRYPTO_SECRET}"
      ERROR_LOG_ENABLED: "true"
      BATCH_UPDATE_ENABLED: "true"
      SYNC_FREQUENCY: "60"
      RELAY_IDLE_CONN_TIMEOUT: "90"
      STREAMING_TIMEOUT: "300"
      GLOBAL_WEB_RATE_LIMIT_ENABLE: "false"
    volumes:
      - ./data:/data
      - ./logs:/app/logs
    command: ["--log-dir", "/app/logs"]
    depends_on:
      mysql:
        condition: service_healthy
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://127.0.0.1:3000/api/status | grep -Eq '\"success\"[[:space:]]*:[[:space:]]*true'"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 30s
    networks:
      - new-api-net

  mysql:
    image: mysql:8.2
    container_name: new-api-mysql
    restart: unless-stopped
    env_file:
      - .env
    environment:
      MYSQL_ROOT_PASSWORD: "${MYSQL_ROOT_PASSWORD}"
      MYSQL_DATABASE: "${MYSQL_DATABASE}"
      MYSQL_USER: "${MYSQL_USER}"
      MYSQL_PASSWORD: "${MYSQL_PASSWORD}"
      TZ: Asia/Shanghai
    command:
      - --character-set-server=utf8mb4
      - --collation-server=utf8mb4_unicode_ci
    volumes:
      - mysql_data:/var/lib/mysql
    healthcheck:
      test: ["CMD-SHELL", "mysqladmin ping -h 127.0.0.1 -uroot -p$$MYSQL_ROOT_PASSWORD --silent"]
      interval: 10s
      timeout: 5s
      retries: 10
      start_period: 30s
    networks:
      - new-api-net

  redis:
    image: redis:7-alpine
    container_name: new-api-redis
    restart: unless-stopped
    env_file:
      - .env
    command: ["sh", "-c", "exec redis-server --appendonly yes --requirepass '$$REDIS_PASSWORD'"]
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD-SHELL", "redis-cli -a $$REDIS_PASSWORD ping | grep -q PONG"]
      interval: 10s
      timeout: 5s
      retries: 10
    networks:
      - new-api-net

volumes:
  mysql_data:
  redis_data:

networks:
  new-api-net:
    driver: bridge
```

### 7.4 启动顺序

```bash
docker compose config
docker compose up -d mysql redis
```

确认数据库和 Redis 健康后：

```bash
docker compose ps
docker compose up -d new-api
```

### 7.5 验证

```bash
curl -sS http://127.0.0.1:3000/api/status
docker compose logs --tail=200 new-api
docker compose exec redis sh -lc 'redis-cli -a "$REDIS_PASSWORD" ping'
```

若服务器只通过 Nginx 对外提供服务，建议保持 `127.0.0.1:3000:3000`，不要将 3000 暴露到公网。

## 8. 使用外部 MySQL/Redis

若数据库与 Redis 已由独立集群提供，可仅运行 New-API 容器：

```yaml
services:
  new-api:
    image: new-api:2.0
    restart: unless-stopped
    ports:
      - "127.0.0.1:3000:3000"
    environment:
      TZ: Asia/Shanghai
      NODE_NAME: new-api-node-1
      SQL_DSN: "newapi:DB_PASSWORD@tcp(10.10.10.20:3306)/new_api?charset=utf8mb4&parseTime=true&loc=Local"
      REDIS_CONN_STRING: "redis://:REDIS_PASSWORD@10.10.10.30:6379/0"
      SESSION_SECRET: "REPLACE_WITH_SAME_SECRET_ON_ALL_NODES"
      CRYPTO_SECRET: "REPLACE_WITH_SAME_CRYPTO_SECRET"
      ERROR_LOG_ENABLED: "true"
      BATCH_UPDATE_ENABLED: "true"
      SYNC_FREQUENCY: "60"
    volumes:
      - ./data:/data
      - ./logs:/app/logs
    command: ["--log-dir", "/app/logs"]
```

生产数据库账号建议只授予 New-API 所需库的权限，不要长期使用 root。

## 9. Docker + Nginx 反向代理

AI 模型响应通常包含 SSE/流式传输，必须注意：

- `proxy_buffering off`。
- `proxy_request_buffering off` 可降低上传/流式请求缓存影响。
- `proxy_http_version 1.1`。
- WebSocket 场景传递 Upgrade/Connection。
- 适当增加 `proxy_read_timeout`。
- 对公网部署建议启用 HTTPS。

示例 `/etc/nginx/conf.d/new-api.conf`：

```nginx
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

upstream new_api_backend {
    server 127.0.0.1:3000;
    keepalive 128;
}

server {
    listen 80;
    server_name new-api.example.com;

    client_max_body_size 100m;

    location / {
        proxy_pass http://new_api_backend;
        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;

        proxy_buffering off;
        proxy_request_buffering off;
        proxy_connect_timeout 10s;
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
    }
}
```

检查并重载：

```bash
nginx -t
systemctl reload nginx
curl -sS http://new-api.example.com/api/status
```

## 10. Docker 多节点扩展

推荐结构：

- 1 个管理节点：正常模式。
- 2~N 个 API 节点：`RELAY_ONLY=true`。
- 所有节点：共享 MySQL、Redis、`SESSION_SECRET`、`CRYPTO_SECRET`。
- 每个节点：独立 `NODE_NAME`。
- 前置 Nginx 对 API 节点做负载均衡。

API 节点示例：

```yaml
  new-api-api-1:
    image: new-api:2.0
    restart: unless-stopped
    environment:
      TZ: Asia/Shanghai
      NODE_NAME: api-1
      RELAY_ONLY: "true"
      SQL_DSN: "newapi:DB_PASSWORD@tcp(mysql.example:3306)/new_api?charset=utf8mb4&parseTime=true&loc=Local"
      REDIS_CONN_STRING: "redis://:REDIS_PASSWORD@redis.example:6379/0"
      SESSION_SECRET: "SAME_ON_ALL_NODES"
      CRYPTO_SECRET: "SAME_ON_ALL_NODES"
      SYNC_FREQUENCY: "60"
      BATCH_UPDATE_ENABLED: "true"
```

Nginx：

```nginx
upstream new_api_relay {
    least_conn;
    server 10.0.0.11:3000 max_fails=3 fail_timeout=30s;
    server 10.0.0.12:3000 max_fails=3 fail_timeout=30s;
    server 10.0.0.13:3000 max_fails=3 fail_timeout=30s;
    keepalive 256;
}
```

---

# 第二部分：不基于 Docker 部署

## 11. 非 Docker 部署前置条件

由于二进制为静态链接，目标主机不需要安装 Go、Node、Bun。建议准备：

- Linux amd64/x86-64。
- systemd。
- `ca-certificates`、`tzdata`、`curl` 或 `wget`。
- 生产环境可访问 MySQL/PostgreSQL、Redis。
- Nginx 可选但生产推荐。

Debian/Ubuntu：

```bash
apt-get update
apt-get install -y ca-certificates tzdata curl nginx
```

RHEL/Rocky/AlmaLinux：

```bash
dnf install -y ca-certificates tzdata curl nginx
```

## 12. 创建运行用户和目录

```bash
useradd --system \
  --home-dir /var/lib/new-api \
  --create-home \
  --shell /usr/sbin/nologin \
  newapi

mkdir -p /opt/new-api/releases/2.0
mkdir -p /etc/new-api
mkdir -p /var/lib/new-api
mkdir -p /var/log/new-api

chown -R newapi:newapi /var/lib/new-api /var/log/new-api
chmod 750 /var/lib/new-api /var/log/new-api /etc/new-api
```

解压并安装：

```bash
cd /tmp
sha256sum /path/to/new-api-2.0-linux-amd64.tar.gz
tar -xzf /path/to/new-api-2.0-linux-amd64.tar.gz

install -m 0755 new-api-2.0-linux-amd64/new-api /opt/new-api/releases/2.0/new-api
cp new-api-2.0-linux-amd64/VERSION /opt/new-api/releases/2.0/
cp new-api-2.0-linux-amd64/LICENSE /opt/new-api/releases/2.0/
cp new-api-2.0-linux-amd64/NOTICE /opt/new-api/releases/2.0/
cp new-api-2.0-linux-amd64/THIRD-PARTY-LICENSES.md /opt/new-api/releases/2.0/

ln -sfn /opt/new-api/releases/2.0 /opt/new-api/current
/opt/new-api/current/new-api --version
```

## 13. 非 Docker 方案 A：SQLite 单机部署

创建 `/etc/new-api/new-api.env`：

```bash
cat > /etc/new-api/new-api.env <<'ENV'
TZ=Asia/Shanghai
NODE_NAME=new-api-standalone
SQLITE_PATH=/var/lib/new-api/new-api.db
ERROR_LOG_ENABLED=true
BATCH_UPDATE_ENABLED=true
GLOBAL_WEB_RATE_LIMIT_ENABLE=false
ENV

chown root:newapi /etc/new-api/new-api.env
chmod 640 /etc/new-api/new-api.env
```

SQLite 数据库放在 `/var/lib/new-api`，避免将数据库写入程序安装目录。

## 14. 非 Docker 方案 B：外置 MySQL + Redis 生产部署

创建 `/etc/new-api/new-api.env`：

```bash
cat > /etc/new-api/new-api.env <<'ENV'
TZ=Asia/Shanghai
NODE_NAME=new-api-node-1

SQL_DSN=newapi:REPLACE_DB_PASSWORD@tcp(10.10.10.20:3306)/new_api?charset=utf8mb4&parseTime=true&loc=Local
REDIS_CONN_STRING=redis://:REPLACE_REDIS_PASSWORD@10.10.10.30:6379/0

SESSION_SECRET=REPLACE_WITH_LONG_RANDOM_VALUE
CRYPTO_SECRET=REPLACE_WITH_LONG_RANDOM_VALUE

ERROR_LOG_ENABLED=true
BATCH_UPDATE_ENABLED=true
SYNC_FREQUENCY=60
RELAY_IDLE_CONN_TIMEOUT=90
STREAMING_TIMEOUT=300
GLOBAL_WEB_RATE_LIMIT_ENABLE=false
ENV

chown root:newapi /etc/new-api/new-api.env
chmod 640 /etc/new-api/new-api.env
```

数据库连通性测试：

```bash
mysql -h 10.10.10.20 -P 3306 -u newapi -p new_api
redis-cli -h 10.10.10.30 -p 6379 -a 'REPLACE_REDIS_PASSWORD' ping
```

## 15. 配置 systemd 服务

创建 `/etc/systemd/system/new-api.service`：

```ini
[Unit]
Description=New-API 2.0 LLM Gateway
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=newapi
Group=newapi
WorkingDirectory=/var/lib/new-api
EnvironmentFile=/etc/new-api/new-api.env
ExecStart=/opt/new-api/current/new-api --port 3000 --log-dir /var/log/new-api
Restart=always
RestartSec=5s
TimeoutStopSec=30s
KillSignal=SIGTERM
UMask=0027
LimitNOFILE=1048576

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ProtectKernelTunables=true
ProtectControlGroups=true
RestrictSUIDSGID=true

[Install]
WantedBy=multi-user.target
```

加载并启动：

```bash
systemctl daemon-reload
systemctl enable --now new-api
```

检查：

```bash
systemctl status new-api --no-pager
journalctl -u new-api -n 200 --no-pager
ss -lntp | grep ':3000'
curl -sS http://127.0.0.1:3000/api/status
```

若服务启动失败，优先通过 systemd 日志定位：

```bash
systemctl status new-api --no-pager
journalctl -u new-api -b -n 300 --no-pager
systemctl show new-api -p EnvironmentFiles -p ExecStart
```

不建议使用 `xargs` 将生产密钥拼接到命令行排障，因为可能泄漏到进程列表和 shell 历史。

## 16. 非 Docker + Nginx

可直接复用第 9 章 Nginx 配置。建议 New-API 仅监听/暴露在内网或本机，由 Nginx 提供 80/443。

启用：

```bash
nginx -t
systemctl enable --now nginx
systemctl reload nginx
```

防火墙示例：

```bash
firewall-cmd --permanent --add-service=http
firewall-cmd --permanent --add-service=https
firewall-cmd --reload
```

若 New-API 只给本机 Nginx 使用，不需要放通 3000。

## 17. 非 Docker 多节点部署

每台主机安装相同 2.0 二进制。

管理节点 `/etc/new-api/new-api.env`：

```text
NODE_NAME=admin-1
SQL_DSN=...
REDIS_CONN_STRING=...
SESSION_SECRET=SAME_SECRET
CRYPTO_SECRET=SAME_CRYPTO_SECRET
```

API 节点：

```text
NODE_NAME=api-1
RELAY_ONLY=true
SQL_DSN=...
REDIS_CONN_STRING=...
SESSION_SECRET=SAME_SECRET
CRYPTO_SECRET=SAME_CRYPTO_SECRET
```

第二个 API 节点只需修改：

```text
NODE_NAME=api-2
```

Nginx/LB 将 `/v1/`、模型请求等 AI API 流量发往 API 节点；管理端访问可指向管理节点。

---

# 第三部分：部署后的应用配置

## 18. 首次初始化

1. 浏览器打开 `http://<IP>:3000` 或配置后的 HTTPS 域名。
2. 按首次初始化页面创建管理员账户。
3. 登录后配置上游渠道/模型。
4. 生成访问 Token，进行 `/v1/models` 和模型请求测试。
5. 根据生产需要配置看板、限流和审计。

示例连通性请求：

```bash
curl -i http://127.0.0.1:3000/v1/models \
  -H 'Authorization: Bearer sk-REPLACE_WITH_NEW_API_TOKEN'
```

## 19. 2.0 常用环境变量

| 变量 | 作用 | 生产建议 |
|---|---|---|
| `PORT` | 服务端口 | 通常 3000；也可用 `--port` |
| `SQL_DSN` | MySQL/PostgreSQL 主数据库连接 | 生产建议配置 |
| `LOG_SQL_DSN` | 独立日志数据库 | 大日志量时可评估 |
| `SQLITE_PATH` | SQLite 文件路径 | 单节点时显式设置 |
| `REDIS_CONN_STRING` | Redis 连接 | 多节点/限流/缓存建议配置 |
| `SESSION_SECRET` | 会话密钥 | 多节点必须一致；使用长随机值 |
| `CRYPTO_SECRET` | 加密相关密钥 | 建议设置且多节点一致 |
| `NODE_NAME` | 节点标识 | 每个节点唯一 |
| `RELAY_ONLY` | 仅 Relay/API 模式 | API 节点设置 `true` |
| `SYNC_FREQUENCY` | 配置同步周期（秒） | 多节点常用 60 |
| `BATCH_UPDATE_ENABLED` | 批量更新 | 生产/多节点建议开启 |
| `ERROR_LOG_ENABLED` | 错误日志 | 建议开启 |
| `RELAY_TIMEOUT` | Relay 总超时 | 默认 0 表示不限制；谨慎调整 |
| `RELAY_IDLE_CONN_TIMEOUT` | HTTP 空闲连接超时 | 常用 90 秒 |
| `STREAMING_TIMEOUT` | 流模式无响应超时 | 可按模型响应调整 |
| `TLS_INSECURE_SKIP_VERIFY` | 跳过上游 TLS 校验 | 生产不建议开启 |
| `PROMETHEUS_URL` | 模型看板查询 Prometheus | 需要 vLLM 指标时配置 |
| `VLLM_PROMETHEUS_JOB` | Prometheus 中的 vLLM job 名 | 默认可按部署修改 |
| `PROMETHEUS_QUERY_TIMEOUT_SECONDS` | Prometheus 查询超时 | 默认 15 秒 |
| `PROMETHEUS_BEARER_TOKEN` | Prometheus Bearer Token | Prometheus 启用鉴权时配置 |

### 19.1 模型看板 / Prometheus

若使用 vLLM 指标看板，可增加：

```text
PROMETHEUS_URL=http://prometheus.example:9090
VLLM_PROMETHEUS_JOB=vllm-model-server
PROMETHEUS_QUERY_TIMEOUT_SECONDS=15
# PROMETHEUS_BEARER_TOKEN=...
```

如果模型看板无 vLLM 时序数据，优先检查 Prometheus 地址、job 名称、网络和鉴权。

### 19.2 DB 看板

当前 2.0 的 DB 看板实现针对 MySQL。若主数据库不是 MySQL，DB 看板相关指标不应作为数据库健康判断依据。

---

# 第四部分：运维、备份与升级

## 20. 日志管理

### 20.1 Docker

应用日志目录：

```text
./logs -> /app/logs
```

查看：

```bash
docker compose logs -f --tail=200 new-api
ls -lh ./logs
```

Docker 自身日志建议配置轮转，例如 `/etc/docker/daemon.json`：

```json
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "100m",
    "max-file": "5"
  }
}
```

### 20.2 非 Docker

创建 `/etc/logrotate.d/new-api`：

```text
/var/log/new-api/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
    su newapi newapi
}
```

手工测试：

```bash
logrotate -d /etc/logrotate.d/new-api
```

## 21. 数据备份

### 21.1 SQLite

维护窗口内建议：

```bash
systemctl stop new-api
cp -a /var/lib/new-api/new-api.db /backup/new-api.db.$(date +%F-%H%M%S)
systemctl start new-api
```

Docker：

```bash
docker compose stop new-api
cp -a ./data /backup/new-api-data-$(date +%F-%H%M%S)
docker compose start new-api
```

### 21.2 MySQL

```bash
mysqldump \
  --single-transaction \
  --routines \
  --triggers \
  --hex-blob \
  -h 10.10.10.20 \
  -u newapi -p \
  new_api > /backup/new_api-$(date +%F-%H%M%S).sql
```

重要升级前必须完成数据库备份并验证备份文件可读。

### 21.3 Redis

Redis 在本架构中主要用于缓存、会话、限流等共享状态，不应替代主数据库。生产环境仍建议开启 RDB/AOF，并按 Redis 运维规范备份。

## 22. Docker 升级与回退

升级前：

```bash
docker compose ps
docker image inspect new-api:2.0 >/dev/null
# 备份数据库和配置
tar -czf /backup/new-api-config-$(date +%F-%H%M%S).tar.gz .env docker-compose.yml
```

新版本镜像构建后修改 Compose 镜像 tag，再执行：

```bash
docker compose up -d
curl -sS http://127.0.0.1:3000/api/status
```

回退应用时恢复原镜像 tag，再：

```bash
docker compose up -d
```

数据库发生 schema 变更时，回退应用前必须先确认数据库兼容性，不能只依赖镜像回退。

## 23. 非 Docker 升级与回退

推荐使用 release 目录 + `current` 软链接：

```text
/opt/new-api/releases/2.0/
/opt/new-api/releases/2.1/
/opt/new-api/current -> /opt/new-api/releases/2.0
```

升级：

```bash
systemctl stop new-api
ln -sfn /opt/new-api/releases/2.1 /opt/new-api/current
systemctl start new-api
systemctl status new-api --no-pager
```

回退：

```bash
systemctl stop new-api
ln -sfn /opt/new-api/releases/2.0 /opt/new-api/current
systemctl start new-api
```

若数据库 schema 已发生不兼容变化，需要配套数据库回退方案。

---

# 第五部分：安全与故障排查

## 24. 生产安全建议

1. 不使用示例密码 `123456`，所有数据库、Redis、Session、Crypto 密钥使用独立强随机值。
2. MySQL/Redis 不对公网开放，仅允许业务网段访问。
3. New-API 3000 端口仅对 Nginx/LB 或内网开放。
4. 公网入口使用 HTTPS。
5. 不启用 `TLS_INSECURE_SKIP_VERIFY=true`，除非临时排障且明确承担证书校验风险。
6. `.env`、`/etc/new-api/new-api.env` 权限限制为 600/640。
7. 定期备份主数据库，并实际演练恢复。
8. 管理端与 Relay 节点分离时，所有节点保持相同 `SESSION_SECRET` 和 `CRYPTO_SECRET`。
9. 多节点限流必须使用同一 Redis，否则各节点会形成独立计数。
10. 对 Nginx、数据库、Redis 和主机系统分别配置监控与告警。

## 25. 常见故障定位

### 25.1 `Exec format error`

原因：在 ARM64 或其他非 amd64 系统运行本介质。

```bash
uname -m
file /opt/new-api/current/new-api
```

本介质要求 `x86_64`。

### 25.2 3000 端口被占用

```bash
ss -lntp | grep ':3000'
lsof -iTCP:3000 -sTCP:LISTEN
```

### 25.3 SQLite 无法创建/写入

```bash
ls -ld /var/lib/new-api
ls -l /var/lib/new-api/new-api.db
sudo -u newapi test -w /var/lib/new-api && echo writable
```

### 25.4 MySQL 连接失败

```bash
nc -vz 10.10.10.20 3306
mysql -h 10.10.10.20 -u newapi -p new_api
```

检查 DNS/IP、3306、防火墙、账号权限、DSN。

### 25.5 Redis 认证/连接失败

```bash
nc -vz 10.10.10.30 6379
redis-cli -h 10.10.10.30 -a 'PASSWORD' ping
```

预期：

```text
PONG
```

### 25.6 登录状态在多节点间随机失效

重点检查：

```text
SESSION_SECRET
CRYPTO_SECRET
REDIS_CONN_STRING
```

所有节点应指向同一 Redis，并保持密钥一致。

### 25.7 AI 流式响应卡顿/首 token 被延迟

检查 Nginx：

```nginx
proxy_buffering off;
proxy_http_version 1.1;
proxy_read_timeout 600s;
```

同时检查是否还有上层 LB/CDN 在做响应缓冲。

### 25.8 `/api/status` 不通

```bash
curl -v http://127.0.0.1:3000/api/status
systemctl status new-api --no-pager
journalctl -u new-api -n 200 --no-pager
```

Docker：

```bash
docker compose ps
docker compose logs --tail=200 new-api
```

### 25.9 模型看板无 Prometheus 数据

确认：

```text
PROMETHEUS_URL
VLLM_PROMETHEUS_JOB
PROMETHEUS_QUERY_TIMEOUT_SECONDS
PROMETHEUS_BEARER_TOKEN（如启用鉴权）
```

从 New-API 节点检查：

```bash
curl -sS http://prometheus.example:9090/-/healthy
```

### 25.10 DB 看板无数据

确认当前主数据库是否为 MySQL，再检查 `performance_schema` 是否开启以及监控账号权限。

---

# 第六部分：2.0 功能验收

## 26. 部署验收清单

| 检查项 | 验收方法 | 期望结果 |
|---|---|---|
| 介质版本 | `new-api --version` | 2.0 |
| CPU 架构 | `uname -m` | x86_64 |
| 服务进程 | systemd/docker 状态 | running/healthy |
| 监听端口 | `ss -lntp` | 3000 正常监听 |
| 健康接口 | `GET /api/status` | success=true |
| 管理页面 | 浏览器访问 | 可打开并登录 |
| 主数据库 | MySQL/SQLite | 可读写 |
| Redis | `PING` | PONG |
| API 鉴权 | `/v1/models` | 有 Token 成功、无 Token 被拒绝 |
| 流式请求 | SSE/Chat 测试 | 持续流式输出，无代理缓冲 |
| 日志 | log dir/journal | 正常记录，无持续错误 |
| 数据持久化 | 重启后复核 | 配置、用户、Token 不丢失 |
| 限流看板 | 管理端 | 可查询最近 10/20/40/80 周期 |
| 历史限流查询 | 开始时间 + N 周期 | 无需结束时间即可查询 |
| 模型看板 | Prometheus/vLLM | 模型运行指标可显示 |
| 性能看板 | 管理端 | FRT/Token 速率等可显示 |
| DB 看板 | MySQL 环境 | 指标可显示 |
| 备份 | 数据库备份命令 | 生成有效备份文件 |

## 27. 2.0 限流看板说明

2.0 版本限流看板实时范围不再按 1/2/4/8 小时选择，而按限流周期数选择：

```text
最近 10 周期
最近 20 周期
最近 40 周期
最近 80 周期
```

例如限流周期为 1 分钟：

```text
10 周期 = 最近 10 分钟
20 周期 = 最近 20 分钟
40 周期 = 最近 40 分钟
80 周期 = 最近 80 分钟
```

历史查询采用：

```text
开始时间 + 周期数
```

服务端自动计算：

```text
结束时间 = 开始时间 + 周期数 × 限流周期
```

因此历史查询不再要求用户手工选择结束时间。

## 28. 推荐生产参数模板

```text
TZ=Asia/Shanghai
NODE_NAME=new-api-node-1
SQL_DSN=...
REDIS_CONN_STRING=...
SESSION_SECRET=<64位以上随机值>
CRYPTO_SECRET=<64位以上随机值>
ERROR_LOG_ENABLED=true
BATCH_UPDATE_ENABLED=true
SYNC_FREQUENCY=60
RELAY_IDLE_CONN_TIMEOUT=90
STREAMING_TIMEOUT=300
GLOBAL_WEB_RATE_LIMIT_ENABLE=false
```

若扩展为多个 Relay 节点，在 Relay 节点增加：

```text
RELAY_ONLY=true
```

并为每台节点设置唯一 `NODE_NAME`。

---

## 附录 A：最小化启动命令

### Docker + SQLite

```bash
docker run -d --name new-api-2.0 --restart unless-stopped \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -e SQLITE_PATH=/data/new-api.db \
  -v /opt/new-api/data:/data \
  -v /opt/new-api/logs:/app/logs \
  new-api:2.0 --log-dir /app/logs
```

### 非 Docker + SQLite

```bash
export TZ=Asia/Shanghai
export SQLITE_PATH=/var/lib/new-api/new-api.db
/opt/new-api/current/new-api --port 3000 --log-dir /var/log/new-api
```

## 附录 B：版本与介质信息

```text
Repository : schoIarw/new-api
Tag        : 2.0
Commit     : 829c95ed5fd8e6bb21f5ddc762eddb66ba652f15
Package    : new-api-2.0-linux-amd64.tar.gz
Platform   : linux/amd64
Package SHA256:
3d37a143a5ac6715b9270112fe9f71b9cf190beb070efa06f5b24580130f2a6d
Binary SHA256:
0a8b3c4fe9342351f0ccbf16638123878027006ef7a24a673678008137f34253
```

## 附录 C：资料基线

本手册针对 2.0 发布介质编写，参考 2.0 Tag 中以下工程文件：

```text
Dockerfile
Dockerfile.dev
docker-compose.yml
.env.example
README.md
VERSION
.github/workflows/ci.yml
```

Docker 部署示例针对“已经编译好的 tar.gz 二进制介质”整理，不要求目标环境重新构建源码。
