# DigitalOcean 云原生练习环境

> 状态: 当前执行方案 · 2026-07-05 · v2

本文记录 BodySense 在 DigitalOcean 上的一月期云原生练习环境。阿里云生产环境是长期环境，继续使用现有 ACR、`docker-deploy.yml`、`docker-compose.prod.yml` 和 `body.bakersean.top`，本方案不修改其部署链路。

## 目标

- 用 DigitalOcean 练习托管数据库、托管 Redis/Valkey、对象存储、容器镜像仓库和显式 CI/CD。
- 让 4H/8G Droplet 专注远程开发和 AI agent，提供完整的热启动开发体验。
- 让 2C/4G Droplet `DO-bodysense-deploy`（152.42.206.141）专注部署运行，不构建镜像、不跑开发服务。
- 一个月后可完整销毁 DO 资源，不影响阿里云生产。

## 拓扑

```text
开发机 / AI agent
  -> 4H/8G Droplet (远程开发)
     - VS Code Remote SSH
     - pnpm / Go / uv 热启动
     - Docker Compose (Postgres + Redis 基础设施)
     - 不承担公开部署稳定性

GitHub Actions (deploy-do.yml)
  -> DO Container Registry
  -> SSH: DO-bodysense-deploy (2C/4G, 152.42.206.141)

DO-bodysense-deploy (2C/4G)
  - Caddy (HTTPS 反向代理)
  - web (nginx 静态文件)
  - api (Go 后端)
  - ai-service (Python AI 服务)
  - 资源限制 + 日志轮转

DO Managed PostgreSQL + pgvector (端口 25061)
DO Managed Valkey/Redis (端口 25061)
DO Spaces (对象存储/备份)
DO Container Registry (镜像分发)
```

## 资源职责

| 资源 | 规格 | 职责 | 说明 |
| --- | --- | --- | --- |
| 4H/8G Droplet | 远程开发 | 编辑器 server、AI agent、热启动服务 | 跑 Docker (PG+Redis) + 原生构建工具 |
| 2C/4G Droplet | 部署运行 | `docker compose pull/up`，不构建镜像 | 容器资源限制总计 2.25 CPU / ~2G RAM |
| Managed PostgreSQL | 练习数据库 | `sslmode=require`，启用 pgvector | 端口 25061 |
| Managed Valkey/Redis | 缓存/会话 | Go API 用 `REDIS_TLS=true` | 端口 25061 |
| Spaces | 对象存储 | 先做数据库备份；上传迁移后续单独做 | |
| DO Registry | 镜像分发 | GHA 推送 `prod-latest` 和时间戳 tag | |

## 4H/8G 远程开发环境

### 初始化

```bash
scp scripts/setup-dev-server.sh DO-dev:/root/setup-dev-server.sh
ssh DO-dev "chmod +x /root/setup-dev-server.sh && /root/setup-dev-server.sh"
```

脚本自动安装 Docker、Node.js 24 + pnpm、Go 1.26、Python 3.13 + uv、tesseract-ocr，克隆仓库到 `/opt/bodysense-dev`，启动 Postgres + Redis。

### VS Code Remote SSH

1. 安装 `Remote - SSH` 扩展
2. 添加 SSH Host: `ssh root@<DROPLET_IP>`
3. Connect to Host → 打开 `/opt/bodysense-dev`
4. 推荐扩展自动提示安装 (Go、Python、Tailwind CSS、Prettier)

### 开发工作流

```bash
# 启动基础设施 (Postgres + Redis)
docker compose -f docker/docker-compose.yml --profile dev up -d postgres-dev redis-dev

# 热启动各服务 (原生运行，非容器化)
cd apps/web && pnpm dev                    # 前端 Vite dev server
cd apps/api && go run ./cmd/server         # Go API
cd apps/ai-service && uv run uvicorn src.main:app --host 0.0.0.0 --port 8100 --reload
```

VS Code Tasks 已配置好常用命令（`.vscode/tasks.json`），可按 `Ctrl+Shift+B` 快速运行。

## 部署机目录

```text
/opt/bodysense/
  docker/
    docker-compose.prod.do.yml
    Caddyfile
  .env.production.do
  .env.production.do.local
```

`.env.production.do` 是可提交的非敏感配置，`.env.production.do.local` 只在部署机和 GitHub Secret 中保存。

## 部署机资源限制

2C/4G Droplet 上的容器资源分配：

| 服务 | CPU 限制 | 内存限制 | 内存预留 |
| --- | --- | --- | --- |
| ai-service | 1.0 | 1.2G | 512M |
| api | 0.75 | 512M | 128M |
| web | 0.25 | 128M | 32M |
| caddy | 0.25 | 128M | 32M |
| **合计** | **2.25** | **~2G** | **~700M** |

日志轮转: `json-file` driver，`max-size=10m`，`max-file=3`。

## CI/CD

`.github/workflows/deploy-do.yml`：

1. **变更检测**: push 到 `dev` 时，用 `dorny/paths-filter` 判断哪些服务有改动。
2. **构建 web**: `pnpm nx run web:build`。
3. **构建并推送镜像**: web / api / ai-service → DO Registry，附带 `BUILD_DATE` + `VCS_REF` 标签。GHA runner 使用默认镜像源（非中国镜像）。
4. **复制运行时文件**: compose、Caddyfile 和 `.env.production.do` 到部署机。
5. **SSH 部署**: pull + up + 健康检查（重试 5 次）+ `docker image prune -f` 清理旧镜像。
6. **冒烟验证**: `https://bodydoo.bakersean.top/api/health`。

触发方式: push 到 `dev` 分支（路径过滤）或手动 `workflow_dispatch`（可选单服务部署）。

阿里云继续使用 `.github/workflows/docker-deploy.yml`，两条流水线互不覆盖。

### Docker 镜像优化

三个 Dockerfile 的包管理器镜像源已参数化为 `ARG`，默认值是中国镜像（用于国内 CI），GHA runner 通过 `build-args` 覆盖为默认源：

| Dockerfile | ARG | 默认值 | GHA 覆盖 |
| --- | --- | --- | --- |
| `Dockerfile.web` | `NPM_REGISTRY` | `registry.npmmirror.com` | `registry.npmjs.org` |
| `apps/api/Dockerfile` | `GOPROXY` | `goproxy.cn,direct` | `proxy.golang.org,direct` |
| `apps/ai-service/Dockerfile` | `UV_INDEX_URL` | `mirrors.aliyun.com/pypi/simple/` | `pypi.org/simple/` |

Go API 和 AI Service 使用 `--mount=type=cache` 加速增量构建。所有镜像附带 OCI 标准元数据标签。

## GitHub Secrets

| Secret | 用途 |
| --- | --- |
| `DO_API_TOKEN` | 登录 DO Container Registry |
| `DROPLET_HOST` | `DO-bodysense-deploy` 的 IP 或域名 |
| `DROPLET_USER` | SSH 用户，默认 root |
| `DROPLET_SSH_KEY` | SSH 私钥 |
| `ENV_PRODUCTION_DO_LOCAL` | 部署机 `.env.production.do.local` 内容 |

## 部署机环境变量

`.env.production.do.local` 至少包含：

```bash
DB_HOST=
DB_PORT=25061
DB_NAME=bodysense
DB_USER=
DB_PASSWORD=
DB_SSLMODE=require

REDIS_HOST=
REDIS_PORT=25061
REDIS_PASSWORD=
REDIS_TLS=true

JWT_SECRET_KEY=
OPENROUTER_API_KEY=
LLM_API_KEY=
EMBEDDING_API_KEY=

SPACES_ENDPOINT=
SPACES_REGION=
SPACES_BUCKET=
SPACES_ACCESS_KEY_ID=
SPACES_SECRET_ACCESS_KEY=
```

## 部署机初始化

```bash
ssh DO-bodysense-deploy
mkdir -p /opt/bodysense/docker
chmod 700 /opt/bodysense
```

安装 Docker 后，首次部署由 GitHub Actions 完成。手动验证：

```bash
cd /opt/bodysense
docker compose -f docker/docker-compose.prod.do.yml \
  --env-file .env.production.do \
  --env-file .env.production.do.local ps
```

## 安全边界

- DO Firewall 只开放 `22/tcp`、`80/tcp`、`443/tcp`。
- SSH 只允许 key 登录。
- Managed PostgreSQL / Valkey Trusted Sources 只允许部署机 VPC 内网 IP。
- Docker daemon 不开放 TCP。
- `.env.production.do.local` 权限为 `600`。
- 部署机不保存开发依赖、不跑 pnpm/uv/go build。

## 当前取舍

- 暂不引入 Kubernetes、Load Balancer、Terraform 和蓝绿发布。
- 暂不把上传文件迁到 Spaces；当前 Go API 仍依赖本地 `api-uploads` volume，先保持功能可用。
- Spaces 先用于备份练习，上传对象存储迁移作为后续独立任务。

## 销毁清单

1. 备份需要保留的数据。
2. 删除 `bodydoo.bakersean.top` DNS。
3. 销毁 2C/4G 部署 Droplet。
4. 销毁 4H/8G 开发 Droplet。
5. 销毁 DO Managed PostgreSQL、Valkey、Spaces、Registry。
6. 删除 GitHub Secrets 中的 DO 相关密钥。
7. 禁用或删除 `.github/workflows/deploy-do.yml`。
