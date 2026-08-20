# BodySense 部署架构

## 概览

BodySense 采用 GitHub Actions + 阿里云 ACR + Docker Compose + Caddy + Watchtower 的全自动化 CI/CD 流水线。代码合并到 `main` 分支后，release-please 自动管理版本发布；版本标签触发镜像构建并推送到 ACR；服务器上的 Watchtower 自动拉取新镜像并滚动重启。

```
开发者 → push to dev → PR to main → merge
                                    ↓
                            release-please 创建 release PR
                                    ↓
                            release PR 合并 → 生成 tag (v0.1.0)
                                    ↓
                            docker-deploy.yml 触发
                                    ↓
                    ┌───────────────┼───────────────┐
                    ↓               ↓               ↓
              构建 web 镜像    构建 api 镜像   构建 ai-service 镜像
                    ↓               ↓               ↓
                    └───────┬───────┴───────┬───────┘
                            ↓               ↓
                    推送到阿里云 ACR (prod-latest + 版本号)
                            ↓
                    服务器 Watchtower 检测到新镜像
                            ↓
                    自动 pull + 滚动重启容器
```

## 基础设施

### 阿里云服务器

- OS: Debian 13 (trixie)
- IP: 115.29.222.2
- CPU: 2 vCPU
- 内存: 1.6 GB
- 磁盘: 40 GB (36 GB 可用)
- 访问: `ssh ali` (root 用户)
- 域名: `body.bakersean.top` → 解析到 115.29.222.2

### 阿里云容器镜像服务 (ACR)

- Registry: `crpi-cv97phwhms6wy4as.cn-hangzhou.personal.cr.aliyuncs.com`
- Namespace: `bodysense`
- 镜像:
  - `bodysense/bodysense-web:prod-latest`
  - `bodysense/bodysense-api:prod-latest`
  - `bodysense/bodysense-ai-service:prod-latest`

## 服务架构

### 容器拓扑

```
                    ┌──────────────────────────┐
                    │      Caddy (80/443)       │  ← 反向代理 + HTTPS (有域名时)
                    │   http://115.29.222.2     │  ← HTTP (当前，无域名)
                    └──────────┬───────────────┘
                               │
                    ┌──────────┴───────────────┐
                    │   Web (nginx:80)         │  ← 静态文件 + SPA 路由
                    │   bodysense-web           │  ← /api/ 反向代理到 api:8080
                    └──────────┬───────────────┘
                               │
                    ┌──────────┴───────────────┐
                    │   API (Go:8080)           │  ← REST API + JWT 认证
                    │   bodysense-api            │  ← /api/health 健康检查
                    └──┬────────┬───────────────┘
                       │        │
              ┌────────┴──┐ ┌──┴──────────────┐
              │ PostgreSQL │ │ AI Service (8100)│
              │   pg18     │ │ bodysense-ai    │  ← FastAPI + RAG
              │ + pgvector │ │ -service         │  ← /health 健康检查
              └────────────┘ └──┬───────────────┘
                              │
                       ┌──────┴──────┐
                       │  Redis (6379)│  ← 缓存 + 会话
                       │  redis:7     │
                       └─────────────┘

          ┌──────────────────────────────┐
          │   Watchtower                │  ← 每 5 分钟轮询 ACR
          │   自动更新 prod-latest 镜像  │  ← 滚动重启业务容器
          └──────────────────────────────┘
```

### 服务清单

| 服务 | 镜像 | 端口 | 健康检查 | 内存预估 |
|------|------|------|----------|----------|
| Caddy | `caddy:2-alpine` | 80, 443 | - | ~30MB |
| Web | `bodysense-web` | 80 (内部) | `wget http://127.0.0.1/` | ~20MB |
| API | `bodysense-api` | 8080 (内部) | `curl http://localhost:8080/api/health` | ~50MB |
| AI Service | `bodysense-ai-service` | 8100 (内部) | `python urllib http://localhost:8100/health` | ~200MB |
| LiteLLM Gateway | `docker.litellm.ai/berriai/litellm:v1.97.0` | 4000 (内部) | `/health/liveliness` | ~200-500MB |
| PostgreSQL | `pgvector/pgvector:pg18` | 5432 (内部) | `pg_isready` | ~100MB |
| Redis | `redis:7-alpine` | 6379 (内部) | `redis-cli ping` | ~10MB |
| Watchtower | `containrrr/watchtower` | - | - | ~20MB |

**业务容器常态内存预估: ~630–930MB**（LiteLLM 实际占用随并发、缓存与 provider client 状态变化；部署时以容器 metrics 为准）

### 网络拓扑

所有容器连接到 `bodysense-network` 桥接网络。仅 Caddy 暴露端口到宿主机 (80/443)。PostgreSQL 和 Redis 端口仅绑定到 `127.0.0.1` 用于调试。

Diagnosis 的 PydanticAI runtime 不再直接持有物理模型 provider 路由；它通过 `http://litellm-gateway:4000/v1` 请求逻辑模型 `bodysense-diagnosis`。LiteLLM 独立负责物理 provider、retry/fallback 与 usage telemetry。

## CI/CD 流水线

### 1. CI 流水线 (`.github/workflows/ci.yml`)

已存在。触发条件: push 到 `main`/`dev`，PR 到 `main`/`dev`。

- web: lint + typecheck + build
- api: go vet + go build + go test
- ai-service: ruff check + pytest
- commit-lint: PR 时校验 commit message

### 2. Release-Please (`.github/workflows/release-please.yml`)

触发条件: push 到 `main`。

使用 `googleapis/release-please-action@v4` 的 manifest 模式:
- 自动收集 Conventional Commits 类型的变更
- 维护一个 release PR，自动更新 CHANGELOG.md 和版本号
- release PR 合并后，自动创建 git tag (如 `v0.1.0`) 和 GitHub Release

配置文件:
- `release-please-config.json`: 发布策略配置
- `.release-please-manifest.json`: 当前版本号

### 3. Docker 镜像构建与推送 (`.github/workflows/docker-deploy.yml`)

触发条件: tag 推送 (`v*`) 或手动触发。

流程:
1. Checkout 代码
2. 安装 Node.js + pnpm
3. 安装依赖 (`pnpm install --frozen-lockfile`)
4. 构建 web 应用 (`pnpm nx run web:build`)
5. 生成镜像标签 (`v{VERSION}-prod.{TIMESTAMP}-{SHA}` + `prod-latest`)
6. 登录阿里云 ACR
7. 构建并推送三个镜像 (web, api, ai-service)
8. 使用 GitHub Actions cache 加速构建

需要的 GitHub Secrets:
- `ACR_REGISTRY`: ACR 注册地址
- `ACR_USERNAME`: ACR 用户名
- `ACR_PASSWORD`: ACR 密码
- `ACR_NAMESPACE`: ACR 命名空间

### 4. 自动部署 (Watchtower)

服务器上运行 Watchtower 容器:
- 每 300 秒轮询 ACR 检查 `prod-latest` 镜像更新
- 检测到新镜像后自动 pull 并滚动重启容器
- 仅更新带有 `com.centurylinklabs.watchtower.enable=true` 标签的容器

## Docker 镜像构建策略

### Web (React/Vite → nginx)

多阶段构建:
1. Builder: `node:24-slim` + pnpm@11，安装依赖并执行 `vite build`
2. Runtime: `nginx:1.27-alpine`，复制静态文件 + nginx.conf

nginx 配置:
- 静态文件服务 + SPA 路由回退
- `/api/` 反向代理到 `api:8080`
- 静态资源缓存 30 天
- Gzip 压缩

### API (Go)

多阶段构建 (已有生产 Dockerfile):
1. Builder: `golang:1.26-alpine`，`go mod download` + `CGO_ENABLED=0 go build`
2. Runtime: `alpine:3.20`，复制二进制 + migrations

### AI Service (Python/FastAPI)

多阶段构建 (已有生产 Dockerfile):
1. Builder: `python:3.13-slim` + uv，`uv sync --no-dev --extra ocr`
2. Runtime: `python:3.13-slim` + tesseract-ocr，复制 .venv + 源码

## 环境变量管理

### 文件层级

| 文件 | 用途 | 是否提交 Git |
|------|------|-------------|
| `.env` | 本地开发基础配置 | 否 (gitignored) |
| `.env.example` | 配置模板 | 是 |
| `.env.production` | 生产非敏感配置 | 是 |
| `.env.production.local` | 生产敏感信息 (密码、密钥) | 否 (gitignored) |

### 敏感变量 (必须在 .env.production.local 中设置)

- `DB_PASSWORD` - PostgreSQL 密码
- `REDIS_PASSWORD` - Redis 密码
- `JWT_SECRET_KEY` - JWT 签名密钥
- `LITELLM_MASTER_KEY` - AI Service → LiteLLM 内部网关认证密钥
- `MIMO_API_KEY` - MiMo provider API 密钥（仅 LiteLLM gateway 使用）
- `OPENROUTER_API_KEY` - OpenRouter API 密钥
- `LLM_API_KEY` - LLM API 密钥
- `EMBEDDING_API_KEY` - Embedding API 密钥

### 非敏感变量 (.env.production)

- 端口号、主机名、服务名
- 数据库名、用户名 (非密码)
- CORS 配置
- 日志级别
- Docker 镜像标签

## 服务器初始化

### 首次部署 (`scripts/setup-server.sh`)

1. 安装 Docker Engine + Docker Compose
2. 登录阿里云 ACR
3. 创建部署目录 `/opt/bodysense`
4. 复制 `docker-compose.prod.yml`、`Caddyfile`、`nginx.conf`
5. 生成 `.env.production.local` (提示用户输入或自动生成密码)
6. `docker compose up -d` 启动全部服务

### 后续更新 (自动)

Watchtower 自动检测 `prod-latest` 镜像更新 → 自动 pull → 滚动重启。无需手动操作。

## HTTPS 配置 (有域名时)

当有域名时，修改 `.env.production.local`:

```
APP_DOMAIN=your-domain.com
ACME_EMAIL=your-email@example.com
```

并修改 `docker/Caddyfile`，移除 `http://` 前缀，Caddy 将自动申请 Let's Encrypt 证书。

## 分支策略与发布流程

```
feature/xxx → dev (PR) → main (PR) → release-please → tag → docker-deploy → auto-deploy
```

1. 功能分支从 `dev` 切出
2. 功能分支通过 PR 合并到 `dev`
3. `dev` 通过 PR 合并到 `main`
4. push 到 `main` 触发 release-please 创建/更新 release PR
5. release PR 合并后自动创建 tag 和 GitHub Release
6. tag 触发 docker-deploy 构建并推送镜像
7. Watchtower 自动拉取新镜像并重启

## 文件清单

| 文件 | 说明 |
|------|------|
| `release-please-config.json` | release-please 发布策略 |
| `.release-please-manifest.json` | release-please 版本号 |
| `.github/workflows/release-please.yml` | release-please 工作流 |
| `.github/workflows/docker-deploy.yml` | 镜像构建与推送工作流 |
| `docker/Dockerfile.web` | Web 生产镜像 (多阶段) |
| `docker/nginx.conf` | Nginx 配置 (静态服务 + API 代理) |
| `docker/Caddyfile` | Caddy 反向代理配置 |
| `docker/docker-compose.prod.yml` | 生产 Docker Compose 编排 |
| `.env.production` | 生产环境非敏感配置 |
| `scripts/setup-server.sh` | 服务器初始化脚本 |
