# BodySense 云原生部署实践方案

> **Retired 2026-08-22:** DigitalOcean is no longer an active BodySense environment. The implementation artifacts were moved to `docs/archive/deployment/digitalocean/`. This document is historical only.


> 在阿里云生产环境之外，搭建一套 DigitalOcean 云原生部署环境用于学习与实践
> 版本: v1.1 · 2026-07-05

---

## 概述

BodySense 当前在阿里云 ECS 上运行着一套稳定的单服务器 Docker Compose 部署（详见第 1 节），这套方案作为**长期生产环境**将持续运行，不做改动。

本文档描述的是在**不影响现有生产环境**的前提下，额外搭建一套基于 DigitalOcean 托管服务的云原生部署方案。目的是在一个月内完成搭建，用于学习和实践云原生架构：托管数据库、对象存储、容器镜像仓库、独立 CI/CD 流水线等。两套方案**并行运行、互不干扰**——阿里云继续承载真实用户流量，DO 环境作为实验场。

| 维度 | 阿里云方案 (生产) | DO 方案 (实践) |
|------|-------------------|----------------|
| 定位 | 正式生产环境 | 学习、实验、技术验证 |
| 生命周期 | 长期运行 | 按需开关，用完可释放 |
| 用户流量 | 承载真实用户 | 仅开发/测试访问 |
| 域名 | `body.bakersean.top` | 独立子域名 `bodydoo.bakersean.top` |
| 改动范围 | **不做任何改动** | 新增文件，不修改已有部署配置 |

---

## 1. 当前架构分析

### 1.1 现状概览

BodySense 目前部署在阿里云一台 2 vCPU / 1.6 GB 的 ECS 上（IP `115.29.222.2`，域名 `body.bakersean.top`），采用单台服务器 + Docker Compose + Watchtower 的模式，共运行 7 个容器：

| 容器 | 角色 | 镜像来源 |
|------|------|----------|
| `postgres` (pgvector/pgvector:pg18) | 主数据库 + pgvector 向量搜索 | Docker Hub |
| `redis` (redis:7-alpine) | 用户会话缓存 | Docker Hub |
| `api` | Go 1.26 后端 REST API | 阿里云 ACR |
| `ai-service` | Python 3.13 AI 推理服务 | 阿里云 ACR |
| `web` | React 19 SPA (nginx 托管) | 阿里云 ACR |
| `caddy` | 反向代理 + TLS 终止 | Docker Hub |
| `watchtower` | 自动拉取 `prod-latest` 镜像更新 | Docker Hub |

CI/CD 流程：`main` 分支 merge → release-please 生成 tag → `docker-deploy.yml` 构建三服务镜像推送到阿里云 ACR → Watchtower 自动滚动重启。

### 1.2 云原生实践动机

阿里云方案运行稳定，在当前的用户规模下完全胜任。搭建 DO 云原生环境的目的不是替代它，而是借 BodySense 这个真实项目来学习和实践一系列云原生技术：

- **托管服务体验**：实践 Managed Database 的自动备份、故障转移、版本升级等能力，对比自管容器的运维成本差异。
- **对象存储集成**：将文件上传从 Docker named volume 迁移到 S3 兼容的对象存储，学习 Pre-signed URL、CDN 加速等模式。
- **容器镜像仓库**：在不同云平台间使用 Container Registry，理解镜像分发、访问控制和地域差异。
- **多环境 CI/CD**：为不同部署目标维护独立的 CI/CD 流水线，学习 GitHub Actions 的多 workflow 编排。
- **网络与安全**：实践 VPC 内网通信、TLS 强制、Trusted Sources 等云安全最佳实践。
- **为未来做准备**：如果项目用户量增长，提前掌握云原生架构的扩展路径。

---

## 2. 目标架构

### 2.1 架构拓扑

```
                        ┌─────────────────────────────┐
                        │     Cloudflare CDN / DNS     │
                        │  (body-do.bakersean.top)     │
                        └──────────────┬──────────────┘
                                       │
                        ┌──────────────▼──────────────┐
                        │  DO Droplet 4H 8G (Ubuntu)   │
                        │                              │
                        │  ┌────────────────────────┐  │
                        │  │    Caddy (反向代理)      │  │
                        │  │  :80 / :443 (auto TLS) │  │
                        │  └──────┬─────────┬───────┘  │
                        │         │         │          │
                        │  ┌──────▼──┐  ┌───▼────────┐ │
                        │  │  web    │  │    api     │ │
                        │  │ (nginx) │  │ (Go 8080)  │ │
                        │  │ :80     │  │            │ │
                        │  └─────────┘  └─────┬──────┘ │
                        │                     │        │
                        │              ┌──────▼──────┐ │
                        │              │ ai-service  │ │
                        │              │ (Py 8100)   │ │
                        │              └─────────────┘ │
                        │                              │
                        │  ┌────────────────────────┐  │
                        │  │ watchtower (自动更新)    │  │
                        │  └────────────────────────┘  │
                        └──────────────────────────────┘
                               │               │
              ┌────────────────┘               └────────────────┐
              │                                                  │
   ┌──────────▼──────────┐   ┌───────────────┐   ┌─────────────▼──────────────┐
   │  DO Managed PG 18   │   │ DO Managed    │   │   DO Spaces Object Storage │
   │  + pgvector 扩展     │   │ Valkey/Redis  │   │   (图片/视频/日志/RAG 文件) │
   │  自动备份 · 高可用    │   │ 自动故障转移   │   │   S3 兼容 API              │
   │  $15.15/月           │   │ $15/月        │   │   $5/月                    │
   └─────────────────────┘   └───────────────┘   └────────────────────────────┘
              ▲
              │
   ┌──────────▼──────────────┐
   │  DO Container Registry  │
   │  存 Docker 镜像          │
   │  $5/月 (Basic)          │
   └─────────────────────────┘
```

### 2.2 服务职责划分

| 层级 | 组件 | 部署位置 | 说明 |
|------|------|----------|------|
| **入口层** | Caddy 2 | Droplet 容器 | 反向代理 + Let's Encrypt 自动 TLS |
| **应用层** | web (nginx + SPA) | Droplet 容器 | 前端静态资源 |
| **应用层** | api (Go/Gin) | Droplet 容器 | REST API |
| **应用层** | ai-service (FastAPI) | Droplet 容器 | AI 推理 + RAG 管线 |
| **数据层** | PostgreSQL 18 + pgvector | DO Managed | 托管数据库，自动备份 |
| **数据层** | Valkey (Redis 兼容) | DO Managed | 托管缓存，自动故障转移 |
| **存储层** | Spaces Object Storage | DO Spaces | 用户上传文件 + 静态资源 |
| **CI/CD** | Container Registry + GHA | DO DOCR + GitHub | 镜像构建与分发 |

### 2.3 月费用预估

| 服务 | 规格 | 月费 | 备注 |
|------|------|------|------|
| Droplet | 4 vCPU / 8 GB (Regular) | $68 | 跑 Caddy + 3 个业务容器 + Watchtower |
| Managed PostgreSQL | db-s-2vcpu-4gb | $15.15 | 自动备份、高可用可选加 $15 |
| Managed Valkey/Redis | db-s-1vcpu-2gb | $15.00 | 会话缓存 + 限流 |
| Spaces Object Storage | 250 GB + 1 TB 流量 | $5.00 | 上传文件、RAG 文件、日志归档 |
| Container Registry | Basic (5 GB) | $5.00 | 3 个镜像仓库 |
| **合计** | | **$108/月** | 不含高可用副本和超额流量 |

> 这是一笔学习投资。通过付费使用托管服务，获得自动备份、故障转移、独立数据库资源等云原生能力的实操经验，同时免去 SSH 运维数据库的繁琐。

---

## 3. 分步实施计划

### Phase 0：准备工作 (Day 1)

**目标**：开通 DigitalOcean 资源，搭建网络基础。

1. 创建 DO 项目 `bodysense`，将所有资源归组。
2. 创建 VPC Network（默认 `default-vpc` 即可），确保 Droplet、Managed PG、Managed Valkey 都在同一 VPC。
3. 开通 Managed PostgreSQL：
   - 引擎版本选 PostgreSQL 18（如果 DO 尚未提供 18，选最高可用版本如 16/17，后续升级）。
   - 节点规格 `db-s-2vcpu-4gb`。
   - 数据库名 `bodysense`，用户名 `bodysense`，记录生成的密码。
   - 启用 pgvector 扩展：连接后执行 `CREATE EXTENSION IF NOT EXISTS vector;`。
4. 开通 Managed Valkey：
   - 节点规格 `db-s-1vcpu-2gb`。
   - 记录连接串（host、port、password）。
5. 创建 Spaces Bucket：
   - 名称：`bodysense-assets`。
   - 区域：与 Droplet 相同（推荐 `sgp1` 新加坡 或 `nyc3`）。
   - 在控制台手动启用 CDN endpoint（DO Spaces CDN 不会自动开启，需要在 Bucket 设置中显式开启，开启后获得 `*.cdn.digitaloceanspaces.com` 子域名）。
   - 生成 Spaces API Key + Secret。
6. 创建 Container Registry：
   - 名称：`bodysense`。
   - 套餐：Basic ($5/月, 5 GB)。
   - 在本地 `docker login registry.digitalocean.com`。
7. 创建 Droplet：
   - 镜像：Ubuntu 24.04 LTS。
   - 规格：4 vCPU / 8 GB Regular。
   - 区域：与 Managed DB 和 Spaces 一致。
   - SSH Key：添加你的公钥。
   - 启用 Monitoring。

### Phase 1：CI/CD 管线搭建 (Day 2)

**目标**：新增一个独立的 GitHub Actions workflow `docker-deploy-do.yml`，用于构建镜像并推送到 DO Container Registry。**现有的 `docker-deploy.yml`（阿里云 ACR）完全不动**。

**新增 `.github/workflows/docker-deploy-do.yml`**：

```yaml
# 与现有 docker-deploy.yml 并行的独立 workflow
# 仅推送到 DO Container Registry，不影响阿里云 ACR 部署

name: Build & Push Docker Images (DO)

on:
  push:
    tags: ['v*']
  workflow_dispatch:
    inputs:
      tag_override:
        description: 'Override image tag'
        required: false
        type: string

concurrency:
  group: docker-deploy-do-${{ github.ref }}
  cancel-in-progress: false

permissions:
  contents: read

env:
  NODE_VERSION: '24'
  REGISTRY: registry.digitalocean.com
  IMAGE_NS: bodysense

# ... (完整 workflow 见第 4.2 节)
```

**GitHub Secrets 新增**（不删除任何现有 Secret）：

| Secret | 操作 | 说明 |
|--------|------|------|
| `DO_API_TOKEN` | 新增 | DO Personal Access Token (需 `read+write` registry 权限) |
| `ACR_REGISTRY` | 保留 | 阿里云 ACR 继续使用 |
| `ACR_USERNAME` | 保留 | 阿里云 ACR 继续使用 |
| `ACR_PASSWORD` | 保留 | 阿里云 ACR 继续使用 |
| `ACR_NAMESPACE` | 保留 | 阿里云 ACR 继续使用 |

> 两个 workflow 在 tag 推送时**同时触发**，各自构建并推送到不同的 Registry，互不干扰。

### Phase 2：DO 专用 Docker Compose (Day 2-3)

**目标**：创建一份 DO 专用的 `docker/docker-compose.do.yml`，使用托管服务连接串和 Spaces 存储。**现有的 `docker-compose.prod.yml`（阿里云）完全不动**。

**新增 `docker/docker-compose.do.yml`**（独立文件，参考 `docker-compose.prod.yml` 编写）：

```yaml
services:
  # ── 移除 postgres 和 redis 容器 ──
  # 它们现在由 DO Managed Services 提供

  ai-service:
    image: registry.digitalocean.com/bodysense/bodysense-ai-service:${AI_TAG:-prod-latest}
    restart: unless-stopped
    init: true
    labels:
      com.centurylinklabs.watchtower.enable: 'true'
    environment:
      AI_SERVICE_PORT: ${AI_SERVICE_PORT:-8100}
      # 改用 DO Managed PostgreSQL 连接串 (VPC 内网)
      DATABASE_URL: ${DATABASE_URL}
      # Spaces 配置
      SPACES_ENDPOINT: ${SPACES_ENDPOINT}
      SPACES_BUCKET: ${SPACES_BUCKET:-bodysense-assets}
      SPACES_ACCESS_KEY: ${SPACES_ACCESS_KEY}
      SPACES_SECRET_KEY: ${SPACES_SECRET_KEY}
      # LLM 配置保持不变
      EMBEDDING_PROVIDER: ${EMBEDDING_PROVIDER:-hashing}
      EMBEDDING_DIMENSIONS: ${EMBEDDING_DIMENSIONS:-384}
      LLM_PROVIDER: ${LLM_PROVIDER:-openrouter}
      LLM_API_KEY: ${LLM_API_KEY:-}
      LLM_MODEL: ${LLM_MODEL:-openai/gpt-oss-120b:free}
      LLM_BASE_URL: ${LLM_BASE_URL:-https://openrouter.ai/api/v1}
      EMBEDDING_API_KEY: ${EMBEDDING_API_KEY:-}
      EMBEDDING_BASE_URL: ${EMBEDDING_BASE_URL:-https://openrouter.ai/api/v1}
      OPENROUTER_API_KEY: ${OPENROUTER_API_KEY:-}
      WHISPER_MODEL: ${WHISPER_MODEL:-ggml-base.bin}
      ASK_USER_ENABLED: ${ASK_USER_ENABLED:-false}
      TZ: ${TZ:-Asia/Shanghai}
    networks:
      - bodysense-network
    healthcheck:
      test: ['CMD', 'python', '-c',
        "import urllib.request; urllib.request.urlopen('http://localhost:8100/health')"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 20s

  api:
    image: registry.digitalocean.com/bodysense/bodysense-api:${API_TAG:-prod-latest}
    restart: unless-stopped
    init: true
    labels:
      com.centurylinklabs.watchtower.enable: 'true'
    depends_on:
      ai-service:
        condition: service_healthy
    environment:
      API_PORT: 8080
      API_HOST: 0.0.0.0
      # DO Managed PostgreSQL (VPC 内网连接)
      DB_HOST: ${DB_HOST}                # 如 private-db-xxx.db.ondigitalocean.com
      DB_PORT: ${DB_PORT:-25061}         # DO 默认 PG 端口 25061
      DB_NAME: ${DB_NAME:-bodysense}
      DB_USER: ${DB_USER:-bodysense}
      DB_PASSWORD: ${DB_PASSWORD}
      DB_SSLMODE: ${DB_SSLMODE:-require} # DO Managed PG 强制 SSL
      # DO Managed Valkey (VPC 内网连接)
      REDIS_HOST: ${REDIS_HOST}          # 如 private-db-xxx.redis.ondigitalocean.com
      REDIS_PORT: ${REDIS_PORT:-25061}   # DO 默认 Redis 端口 25061
      REDIS_PASSWORD: ${REDIS_PASSWORD}
      REDIS_USE_TLS: ${REDIS_USE_TLS:-true}
      # JWT
      JWT_SECRET_KEY: ${JWT_SECRET_KEY}
      JWT_ACCESS_TTL_HOURS: ${JWT_ACCESS_TTL_HOURS:-168}
      JWT_REFRESH_TTL_HOURS: ${JWT_REFRESH_TTL_HOURS:-720}
      CORS_ORIGINS: ${CORS_ORIGINS:-https://body-do.bakersean.top}
      AI_SERVICE_URL: http://ai-service:${AI_SERVICE_PORT:-8100}
      # Spaces (上传文件存储)
      SPACES_ENDPOINT: ${SPACES_ENDPOINT}
      SPACES_BUCKET: ${SPACES_BUCKET:-bodysense-assets}
      SPACES_ACCESS_KEY: ${SPACES_ACCESS_KEY}
      SPACES_SECRET_KEY: ${SPACES_SECRET_KEY}
      TZ: ${TZ:-Asia/Shanghai}
    # 移除 api-uploads volume，改用 Spaces
    networks:
      - bodysense-network
    healthcheck:
      test: ['CMD', 'curl', '-f', 'http://localhost:8080/api/health']
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 30s

  web:
    image: registry.digitalocean.com/bodysense/bodysense-web:${WEB_TAG:-prod-latest}
    restart: unless-stopped
    labels:
      com.centurylinklabs.watchtower.enable: 'true'
    depends_on:
      api:
        condition: service_healthy
    networks:
      - bodysense-network
    healthcheck:
      test: ['CMD', 'wget', '--quiet', '--tries=1', '--spider', 'http://127.0.0.1/']
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    depends_on:
      web:
        condition: service_healthy
    ports:
      - '80:80'
      - '443:443'
      - '443:443/udp'
    environment:
      APP_DOMAIN: ${APP_DOMAIN:-body-do.bakersean.top}
      ACME_EMAIL: ${ACME_EMAIL:-admin@bakersean.top}
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data
      - caddy-config:/config
    networks:
      - bodysense-network

  watchtower:
    image: containrrr/watchtower
    restart: unless-stopped
    environment:
      WATCHTOWER_CLEANUP: 'true'
      WATCHTOWER_POLL_INTERVAL: ${WATCHTOWER_POLL_INTERVAL:-300}
      WATCHTOWER_LABEL_ENABLE: 'true'
      WATCHTOWER_ROLLING_RESTART: 'true'
      WATCHTOWER_INCLUDE_RESTARTING: 'true'
      # Watchtower 需要登录 DO Container Registry
      DOCKER_CONFIG: /config
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /root/.docker/config.json:/config/config.json:ro
    networks:
      - bodysense-network

networks:
  bodysense-network:
    driver: bridge

volumes:
  caddy-data:
  caddy-config:
```

**两套 Compose 文件差异对比**：

| 配置项 | `docker-compose.prod.yml` (阿里云) | `docker-compose.do.yml` (DO) |
|--------|-----------------------------------|------------------------------|
| PostgreSQL | 自管理容器 `pgvector/pgvector:pg18` | 无容器，使用 Managed PG 连接串 |
| Redis | 自管理容器 `redis:7-alpine` | 无容器，使用 Managed Valkey 连接串 |
| `DB_HOST` | `postgres` (Docker 网络名) | `private-db-xxx.db.ondigitalocean.com` |
| `DB_PORT` | `5432` | `25061` (DO 默认端口) |
| `DB_SSLMODE` | 不需要 | `require` |
| `REDIS_HOST` | `redis` (Docker 网络名) | `private-db-xxx.redis.ondigitalocean.com` |
| `REDIS_PORT` | `6379` | `25061` (DO 默认端口) |
| `REDIS_USE_TLS` | 不需要 | `true` |
| 文件存储 | `api-uploads` Docker named volume | Spaces Object Storage |
| 镜像仓库 | `crpi-...aliyuncs.com` (阿里云 ACR) | `registry.digitalocean.com` (DO DOCR) |

### Phase 3：应用层代码适配 (Day 3-4)

> **原则：所有代码改动必须向后兼容。** 通过环境变量控制行为分支——当 `REDIS_USE_TLS`、`SPACES_ENDPOINT` 等变量未设置时，走原有的本地 Redis / 本地文件存储逻辑。这样阿里云生产环境零影响。

#### 3.1 Go API 适配

**数据库 SSL 连接**：`apps/api/internal/database/database.go` 已支持 `DB_SSLMODE` 环境变量且默认值为 `"require"`，无需代码改动。只需在 `docker-compose.do.yml` 或 `.env.production.local` 中设置 `DB_SSLMODE=require` 即可（DO Managed PG 强制 SSL）。

**Redis TLS 连接**：`apps/api/internal/database/redis.go` 中的 `redis.NewClient` 当前未启用 TLS。需要为 DO Managed Valkey 添加 TLS 支持：

```go
// go-redis/v9 支持 TLS — 在 redis.go 中新增环境变量读取
import "crypto/tls"

opts := &redis.Options{
    Addr:     fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
    Password: cfg.RedisPassword,
    TLSConfig: getEnvBool("REDIS_USE_TLS") ? &tls.Config{} : nil,
}
```

**文件上传支持 Spaces（可选路径）**：`apps/api/internal/service/upload_service.go` 中当前的 `os.Create(filePath)` 本地文件写入保持不变（阿里云环境继续使用）。新增一个由环境变量控制的 S3/Spaces 上传路径，当 `SPACES_ENDPOINT` 被设置时走 Spaces，否则走本地文件。需要引入 `aws-sdk-go-v2` 依赖：

```
go get github.com/aws/aws-sdk-go-v2/config \
       github.com/aws/aws-sdk-go-v2/credentials \
       github.com/aws/aws-sdk-go-v2/service/s3
```

核心代码示例：

```go
import "github.com/aws/aws-sdk-go-v2/service/s3"

cfg, _ := config.LoadDefaultConfig(ctx,
    config.WithRegion("sgp1"),
    config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
        os.Getenv("SPACES_ACCESS_KEY"),
        os.Getenv("SPACES_SECRET_KEY"),
        "",
    )),
)
client := s3.NewFromConfig(cfg, func(o *s3.Options) {
    o.BaseEndpoint = aws.String(os.Getenv("SPACES_ENDPOINT"))
})

// Upload
_, err = client.PutObject(ctx, &s3.PutObjectInput{
    Bucket:      aws.String(os.Getenv("SPACES_BUCKET")),
    Key:         aws.String(objectKey),
    Body:        file,
    ContentType: aws.String(contentType),
    ACL:         types.ObjectCannedACLPrivate, // 默认私有，签名 URL 访问
})
```

> **注意：OCR 管线兼容**。`upload_service.go` 中的 `executeOCRCall` 方法从本地文件系统 `os.Open(filePath)` 读取上传文件发送给 AI Service 做 OCR。在 DO 环境中如果文件上传到了 Spaces，此路径需要适配：(a) 上传到 Spaces 后生成 Pre-signed URL 传给 AI Service；(b) 或在上传时同时缓存到临时目录，OCR 完成后清理。阿里云环境不受影响，继续走本地文件路径。

#### 3.2 Python AI Service 适配

AI Service 同样使用 `DATABASE_URL` 连接 PostgreSQL，需要确认：

- `psycopg3` 连接串支持 `sslmode=require`（DO Managed PG 的连接串自带此参数，直接透传即可）。
- LangGraph checkpoint 连接也走同一 `DATABASE_URL`，无需额外改动。
- **注意**：`checkpointing.py` 和 `knowledge_library.py` 中有 `_build_database_url()` 回退方法，会从 `DB_HOST`/`DB_PORT` 等环境变量拼接连接串，但**不含 `sslmode`**。在 DO 环境中必须确保 `DATABASE_URL` 始终被设置（而非走回退路径），否则无 SSL 的连接会被 DO Managed PG 拒绝。
- 如果 AI Service 有本地文件读写（如 RAG 索引文件），考虑迁移到 Spaces 或保持 Droplet 本地存储（视文件大小和访问频率而定）。

**Dockerfile 镜像源说明**：当前 `apps/ai-service/Dockerfile` 中设置了阿里云 PyPI 镜像 (`UV_INDEX_URL=https://mirrors.aliyun.com/pypi/simple/`)，`docker/Dockerfile.web` 中设置了 npmmirror (`registry.npmmirror.com`)。这些是为阿里云环境优化的配置，保持不变。DO 的 CI 构建在 GitHub Actions runner（通常位于美国/欧洲）上运行，国内镜像可能稍慢，但功能上不影响。如果想优化 DO 构建速度，可以在 `docker-deploy-do.yml` 中通过 `build-args` 覆盖镜像源，而不修改 Dockerfile 本身。

#### 3.3 前端 (Web)

前端无需代码改动。`VITE_API_BASE_URL=/api` 仍然通过 nginx 反向代理到 Go API。SSE 流式传输（咨询聊天）走 `/api/` 路径，nginx.conf 已配置 proxy_pass 支持。

> **注**：`.env.production` 中的 `VITE_WS_URL=wss://body.bakersean.top/ws` 是遗留配置，当前代码中无任何消费方（Go API 无 WebSocket handler，前端无 `new WebSocket` 调用），可安全忽略或清理。

### Phase 4：数据同步到 DO (Day 4-5)

#### 4.1 PostgreSQL 数据迁移

```bash
# 1. 从阿里云服务器导出
ssh ali "docker exec bodysense-postgres-1 pg_dump -U bodysense -d bodysense \
  --format=custom --file=/tmp/bodysense_dump.custom"
ssh ali "docker cp bodysense-postgres-1:/tmp/bodysense_dump.custom /root/"
scp ali:/root/bodysense_dump.custom ./

# 2. 导入 DO Managed PostgreSQL
# 先确保 pgvector 扩展已创建
psql "$DO_PG_URL" -c "CREATE EXTENSION IF NOT EXISTS vector;"

# pg_restore 导入
pg_restore --dbname="$DO_PG_URL" --no-owner --no-privileges \
  --clean --if-exists bodysense_dump.custom
```

> **注意**：如果阿里云 PG 版本 (18) 高于 DO Managed PG 版本，`pg_restore` 可能报版本不兼容。解决方案：(a) 使用 `pg_dump --format=plain` 导出 SQL 文本；(b) 或者先在 DO 上用 `pg_dump --no-owner` 配合手动处理。

#### 4.2 Redis 数据迁移

用户会话缓存为临时数据，通常无需迁移。如果确实需要，可以用 `redis-cli --rdb` 导出 RDB 快照。

#### 4.3 上传文件迁移

```bash
# 从阿里云服务器拷贝 api-uploads volume 内容
ssh ali "docker run --rm -v bodysense_api-uploads:/data -v /tmp:/tmp \
  alpine tar czf /tmp/uploads.tar.gz -C /data ."
scp ali:/tmp/uploads.tar.gz ./

# 上传到 Spaces
tar xzf uploads.tar.gz -C ./uploads-extracted/
aws s3 sync ./uploads-extracted/ s3://bodysense-assets/uploads/ \
  --endpoint-url https://sgp1.digitaloceanspaces.com
```

### Phase 4.5：冒烟验证 (Day 5)

**目标**：通过 Droplet IP 直接验证所有服务正常工作。

```bash
# 用 Droplet IP 直接测试
DROPLET_IP="你的 Droplet IP"

# 1. API 健康检查 — 验证 PG + Redis + AI Service 连通性
curl -H "Host: body-do.bakersean.top" http://$DROPLET_IP/api/health

# 2. 前端页面加载
curl -I -H "Host: body-do.bakersean.top" http://$DROPLET_IP/

# 3. 数据库连接验证 — 执行简单查询
curl -H "Host: body-do.bakersean.top" http://$DROPLET_IP/api/auth/check

# 4. 文件上传测试 — 上传一个小文件，确认 Spaces 写入成功
curl -X POST -H "Host: body-do.bakersean.top" \
  -H "Authorization: Bearer <test-token>" \
  -F "file=@test-image.jpg" \
  http://$DROPLET_IP/api/uploads

# 5. AI Service 直连检查
docker exec bodysense-ai-service-1 python -c \
  "import urllib.request; print(urllib.request.urlopen('http://localhost:8100/health').read())"

# 6. pgvector 查询验证
docker exec bodysense-api-1 sh -c \
  'PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME \
  -c "SELECT extname, extversion FROM pg_extension WHERE extname = '\''vector'\''"'
```

> 全部通过后才进入 Phase 5 域名配置。任何失败项应在当前阶段排查修复。

### Phase 5：DO 独立域名上线 (Day 5-6)

DO 环境使用独立的子域名 `body-do.bakersean.top`，与阿里云的 `body.bakersean.top` 完全独立，互不影响。

1. **更新 `.env.production.do.local`**（DO Droplet 上）：填入所有 Managed 服务连接串和 Spaces 密钥。
2. **修改 Caddyfile**（DO Droplet 上）：将 `APP_DOMAIN` 设为 `body-do.bakersean.top`，Caddy 将自动为此域名申请 Let's Encrypt 证书。
3. **设置 CORS**：`CORS_ORIGINS` 设为 `https://body-do.bakersean.top`（DO 环境的前端来源）。
4. **启动服务**：
   ```bash
   docker compose -f docker/docker-compose.do.yml \
     --env-file .env.production --env-file .env.production.do.local up -d
   ```
5. **配置 DNS**：添加 A 记录 `body-do.bakersean.top` → DO Droplet IP。如果走 Cloudflare CDN，添加对应代理规则。
6. **验证 HTTPS 访问**：
   ```bash
   curl https://body-do.bakersean.top/api/health
   # 确认 Caddy TLS 证书自动签发、所有服务正常响应
   ```
7. **观察运行**：确认 Watchtower 正常工作、数据库连接稳定、Spaces 读写正常。

> 阿里云 `body.bakersean.top` 继续运行，不受任何影响。两套环境并行共存。

### Phase 6：备份与监控加固 (Day 6-7)

#### 6.1 数据库备份

DO Managed PostgreSQL 自带每日自动备份（保留 7 天）。额外建议：

```bash
# 在 Droplet 上添加 cron job，每天凌晨备份到 Spaces
0 3 * * * pg_dump "$DATABASE_URL" --format=custom | \
  aws s3 cp - s3://bodysense-assets/backups/pg/$(date +\%Y\%m\%d).custom \
  --endpoint-url https://sgp1.digitaloceanspaces.com
```

#### 6.2 监控告警

- **DO Monitoring**：Droplet 创建时已启用，自动采集 CPU、内存、磁盘指标。在 DO 控制台设置告警阈值（CPU > 80%、内存 > 85%）。
- **DO Managed DB Metrics**：控制台可查看连接数、查询延迟、存储用量。
- **应用层监控**（可选进阶）：
  - 在 Go API 中接入 Prometheus metrics endpoint (`/metrics`)。
  - 在 Droplet 上跑一个 Grafana Agent 或 Datadog Agent 采集指标。
  - 或者使用 DO 的 Observability 平台（基于 Grafana）。

#### 6.3 日志集中化

```bash
# 在 docker-compose.do.yml 中为业务容器添加日志驱动
# 方案 A: 使用 json-file + logrotate (简单)
# 方案 B: 使用 Loki + Promtail (进阶)
# 方案 C: 定期归档到 Spaces
```

---

## 4. CI/CD 双流水线架构

### 4.1 双流水线并行架构

tag 推送时两个 workflow **同时触发**，各自独立构建和推送镜像：

```
feature/* ──PR──▶ dev ──PR──▶ main
                                │
                          release-please
                          生成 release PR
                                │
                          merge release PR
                                │
                          git tag v*.*.*
                                │
                 ┌──────────────┴──────────────┐
                 │                             │
         docker-deploy.yml            docker-deploy-do.yml
         (阿里云 ACR)                 (DO Container Registry)
                 │                             │
          ┌──────┼──────┐               ┌──────┼──────┐
          │      │      │               │      │      │
       web    api   ai-service        web    api   ai-service
          │      │      │               │      │      │
          └──────┼──────┘               └──────┼──────┘
                 │                             │
         push to ACR                  push to DO DOCR
                 │                             │
     Watchtower (阿里云 ECS)        Watchtower (DO Droplet)
     滚动更新 body.bakersean.top    滚动更新 body-do.bakersean.top
```

### 4.2 `docker-deploy-do.yml` (DO 专用)

```yaml
name: Build & Push Docker Images (DO)

on:
  push:
    tags: ['v*']
  workflow_dispatch:
    inputs:
      tag_override:
        description: 'Override image tag'
        required: false
        type: string

concurrency:
  group: docker-deploy-${{ github.ref }}
  cancel-in-progress: false

permissions:
  contents: read

env:
  NODE_VERSION: '24'
  REGISTRY: registry.digitalocean.com
  IMAGE_NS: bodysense

jobs:
  build-and-push:
    name: Build & Push Images
    runs-on: ubuntu-latest
    timeout-minutes: 30

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: ${{ env.NODE_VERSION }}

      - name: Setup pnpm
        run: corepack enable && pnpm --version

      - uses: actions/cache@v4
        with:
          path: ~/.local/share/pnpm/store/v3
          key: pnpm-store-${{ runner.os }}-${{ hashFiles('pnpm-lock.yaml') }}
          restore-keys: pnpm-store-${{ runner.os }}-

      - name: Install deps
        run: pnpm install --frozen-lockfile --reporter=append-only
        env:
          CI: 'true'

      - name: Build web
        run: pnpm nx run web:build
        env:
          CI: 'true'
          VITE_API_BASE_URL: /api

      - name: Resolve tag
        id: tag
        run: |
          VERSION=$(node -p "require('./package.json').version")
          SHA=$(git rev-parse --short=12 HEAD)
          STAMP=$(date -u +%Y%m%d-%H%M%S)
          TAG="${{ inputs.tag_override }}"
          [ -z "$TAG" ] && TAG="v${VERSION}-prod.${STAMP}-${SHA}"
          echo "tag=${TAG}" >> "$GITHUB_OUTPUT"

      - uses: docker/setup-buildx-action@v3

      - name: Login to DO Container Registry
        uses: docker/login-action@v3
        with:
          registry: registry.digitalocean.com
          username: ${{ secrets.DO_API_TOKEN }}
          password: ${{ secrets.DO_API_TOKEN }}

      - name: Build & Push Web
        uses: docker/build-push-action@v6
        with:
          context: .
          file: docker/Dockerfile.web
          push: true
          tags: |
            ${{ env.REGISTRY }}/${{ env.IMAGE_NS }}/bodysense-web:${{ steps.tag.outputs.tag }}
            ${{ env.REGISTRY }}/${{ env.IMAGE_NS }}/bodysense-web:prod-latest
          build-args: |
            BUILD_DATE=${{ github.event.head_commit.timestamp }}
            VCS_REF=${{ github.sha }}
            VERSION=${{ steps.tag.outputs.tag }}
          cache-from: type=gha,scope=web
          cache-to: type=gha,mode=max,scope=web

      - name: Build & Push API
        uses: docker/build-push-action@v6
        with:
          context: apps/api
          file: apps/api/Dockerfile
          push: true
          tags: |
            ${{ env.REGISTRY }}/${{ env.IMAGE_NS }}/bodysense-api:${{ steps.tag.outputs.tag }}
            ${{ env.REGISTRY }}/${{ env.IMAGE_NS }}/bodysense-api:prod-latest
          build-args: |
            BUILD_DATE=${{ github.event.head_commit.timestamp }}
            VCS_REF=${{ github.sha }}
            VERSION=${{ steps.tag.outputs.tag }}
          cache-from: type=gha,scope=api
          cache-to: type=gha,mode=max,scope=api

      - name: Build & Push AI Service
        uses: docker/build-push-action@v6
        with:
          context: apps/ai-service
          file: apps/ai-service/Dockerfile
          push: true
          tags: |
            ${{ env.REGISTRY }}/${{ env.IMAGE_NS }}/bodysense-ai-service:${{ steps.tag.outputs.tag }}
            ${{ env.REGISTRY }}/${{ env.IMAGE_NS }}/bodysense-ai-service:prod-latest
          build-args: |
            BUILD_DATE=${{ github.event.head_commit.timestamp }}
            VCS_REF=${{ github.sha }}
            VERSION=${{ steps.tag.outputs.tag }}
          cache-from: type=gha,scope=ai-service
          cache-to: type=gha,mode=max,scope=ai-service

      - name: Summary
        run: |
          TAG="${{ steps.tag.outputs.tag }}"
          {
            echo "### Images pushed to DO Container Registry"
            echo "| Image | Tags |"
            echo "|-------|------|"
            echo "| \`bodysense-web\` | \`${TAG}\` + \`prod-latest\` |"
            echo "| \`bodysense-api\` | \`${TAG}\` + \`prod-latest\` |"
            echo "| \`bodysense-ai-service\` | \`${TAG}\` + \`prod-latest\` |"
          } >> "$GITHUB_STEP_SUMMARY"
```

---

## 5. Spaces Object Storage 集成方案

### 5.1 存储结构设计

```
bodysense-assets/
├── uploads/                    # 用户上传文件
│   ├── images/                 # 图片
│   ├── videos/                 # 视频
│   └── documents/              # PDF、报告等
├── rag/                        # RAG 管线文件
│   ├── knowledge-packs/        # 知识包
│   └── embeddings/             # 向量索引缓存
├── backups/                    # 数据库备份归档
│   └── pg/
├── logs/                       # 日志归档 (可选)
│   └── api/
└── static/                     # 静态资源 (CDN 友好)
    └── ...
```

### 5.2 访问模式

| 场景 | 访问方式 | 说明 |
|------|----------|------|
| API 上传文件 | S3 SDK (签名写入) | Go API 接收 multipart，写入 Spaces |
| 前端展示图片 | Pre-signed URL 或 CDN URL | 生成临时签名 URL 或直接拼 CDN 地址 |
| RAG 文件读取 | S3 SDK (签名读取) | AI Service 从 Spaces 加载知识文件 |
| 备份写入 | S3 CLI/SDK | cron job 定期 pg_dump 并写入 |

### 5.3 安全策略

- Spaces Bucket 设为 **私有**（默认），所有访问通过 Pre-signed URL。
- 如果需要公开静态资源（如 favicon、logo），可以单独开 CDN endpoint 并限制路径。
- Spaces API Key 权限最小化：只授权 `bodysense-assets` bucket。
- 上传文件大小限制在 API 层控制（如 50 MB），避免 Spaces 存储费用失控。

---

## 6. 安全加固清单

- [ ] **DO Droplet 防火墙**：只开放 80/443 和 SSH (22)，SSH 限制为你的 IP。
- [ ] **Managed DB Trusted Sources**：只允许 Droplet 的 VPC 内网 IP 访问。
- [ ] **Spaces Bucket 私有**：不开启公开列表权限。
- [ ] **TLS 强制**：Caddy 自动 HTTPS，HTTP 301 重定向到 HTTPS。
- [ ] **Secrets 管理**：敏感配置不提交 Git，Droplet 上 `.env.production.local` 权限 600。考虑使用 DO App Platform 的 Encrypted Environment Variables 或 HashiCorp Vault。
- [ ] **镜像扫描**：在 CI 中加入 Trivy 扫描步骤，推送前检查 CVE。

---

## 7. DO 环境管理

由于 DO 环境是独立的实验环境，不存在"回滚"问题。以下是日常管理操作：

**暂停 DO 环境**（节省费用）：
1. 在 Droplet 上 `docker compose -f docker/docker-compose.do.yml down`。
2. 在 DO 控制台将 Droplet 关机（关机后仍收取磁盘存储费，但不收计算费）。
3. Managed PG 和 Valkey 可以保留（$15+15/月）或销毁后需要时重建。

**完全释放 DO 环境**（学习完成后）：
1. 销毁 Droplet 及其磁盘。
2. 销毁 Managed PostgreSQL 和 Managed Valkey 实例。
3. 删除 Spaces Bucket（先清空数据）。
4. 删除 Container Registry 中的镜像，或降级 Registry 到免费套餐。
5. 从 GitHub Secrets 中移除 `DO_API_TOKEN`。
6. 从 GitHub Actions 中禁用或删除 `docker-deploy-do.yml` workflow。
7. 移除 `body-do.bakersean.top` DNS 记录。

> 阿里云生产环境在整个过程中不受任何影响。即使 DO 环境完全释放，`body.bakersean.top` 仍正常服务用户。

---

## 8. 进一步学习方向

完成本次 DO 云原生部署实践后，可以继续探索以下方向：

- **Kubernetes (DOKS)**：在 DO 上搭建 K8s 集群，将 Docker Compose 工作负载迁移到 K8s，学习 Pod、Service、Ingress、ConfigMap 等核心概念。
- **DO App Platform**：尝试将 web 和 api 部署到 App Platform，体验 Serverless 容器和自动扩缩容。
- **CDN 加速实践**：将前端静态资源直接部署到 Spaces CDN，学习 CDN 缓存策略、Cache-Control 配置。
- **蓝绿部署 / 金丝雀发布**：在 GHA 中实践更高级的部署策略，替代当前的全量 `prod-latest` 覆盖。
- **多区域部署**：在多个 DO Region 部署同一套服务，通过 Global Load Balancer 分流，学习多地域架构。
- **基础设施即代码 (IaC)**：用 Terraform 或 Pulumi 管理 DO 资源，学习声明式基础设施管理。
- **经验反哺生产**：将 DO 环境中学到的最佳实践（如 TLS 强制、自动备份策略、对象存储集成）按需应用到阿里云生产环境。
