# Issue 1: 开发环境基础设施

> **目标：** 搭建一键启动的本地开发环境，包含 PostgreSQL、Redis、Go API、React 前端

---

## 1. Docker Compose 多容器编排

### 什么是 Docker Compose？

Docker Compose 是一个工具，用于定义和运行多个 Docker 容器。通过一个 YAML 文件描述所有服务及其关系。

### 项目中的服务

```yaml
services:
  postgres-dev:    # 数据库
  redis-dev:       # 缓存
  api:             # Go 后端
  web:             # React 前端
```

### 关键配置解析

#### 端口映射
```yaml
ports:
  - '5432:5432'    # 宿主机端口:容器端口
  - '6384:6379'    # Redis 用不同端口避免冲突
```

**为什么 Redis 用 6384？**
- 避免和本地已有的 Redis 服务冲突
- 外部访问 `localhost:6384`，容器内部仍是 `6379`

#### 数据卷 (Volumes)
```yaml
volumes:
  - postgres-dev-data:/var/lib/postgresql/data
```

**作用：** 数据持久化。容器删除后数据不丢失。

#### 健康检查 (Healthcheck)
```yaml
healthcheck:
  test: ['CMD-SHELL', 'pg_isready -U bodysense']
  interval: 10s      # 每 10 秒检查一次
  timeout: 5s        # 超时时间
  retries: 5         # 失败 5 次才算不健康
```

**作用：** 确保服务完全启动后再接受连接。其他服务可以依赖健康状态。

#### 依赖关系
```yaml
api:
  depends_on:
    postgres-dev:
      condition: service_healthy    # 等 PostgreSQL 健康后再启动
    redis-dev:
      condition: service_healthy
```

### 常用命令

```bash
# 启动所有 dev 环境服务
docker compose -f docker/docker-compose.yml --profile dev up -d

# 查看运行状态
docker compose -f docker/docker-compose.yml ps

# 查看日志
docker compose -f docker/docker-compose.yml logs -f api

# 停止所有服务
docker compose -f docker/docker-compose.yml down

# 停止并删除数据卷
docker compose -f docker/docker-compose.yml down -v
```

### Profile 的作用

```yaml
postgres-dev:
  profiles: [dev]    # 只在 --profile dev 时启动
```

**好处：** 可以有多个 profile（dev、test、prod），按需启动不同服务组合。

---

## 2. PostgreSQL + pgvector

### 为什么选 PostgreSQL？

| 特性 | 说明 |
|------|------|
| 开源免费 | 无授权费用 |
| 功能丰富 | 支持 JSON、全文搜索、向量搜索 |
| pgvector 扩展 | Issue 3 RAG 需要的向量相似度搜索 |
| 社区活跃 | 文档完善，问题容易找到解决方案 |

### pgvector 是什么？

pgvector 是 PostgreSQL 的扩展，支持存储和查询向量（embedding）。

```sql
-- 启用扩展
CREATE EXTENSION IF NOT EXISTS "vector";

-- 创建带向量的表
CREATE TABLE documents (
    id SERIAL PRIMARY KEY,
    content TEXT,
    embedding vector(1536)    -- OpenAI embedding 维度
);

-- 相似度查询
SELECT * FROM documents
ORDER BY embedding <-> '[0.1, 0.2, ...]'    -- 余弦距离
LIMIT 5;
```

**为什么需要向量搜索？**
- Issue 3 的 RAG 功能需要
- "圆肩"和"上交叉综合征"语义相近，向量距离近

---

## 3. Redis 内存数据库

### 什么是 Redis？

Redis 是一个内存中的数据结构存储，可用作数据库、缓存和消息中间件。

### 项目中 Redis 的用途

| 用途 | 说明 |
|------|------|
| 存储 Refresh Token | 登录后的长期凭证，支持快速查询和过期删除 |
| 未来：会话缓存 | 缓存用户会话，减少数据库查询 |
| 未来：速率限制 | 限制 API 调用频率 |

### Redis vs PostgreSQL

| 特性 | Redis | PostgreSQL |
|------|-------|------------|
| 存储位置 | 内存 | 磁盘 |
| 速度 | 极快 (微秒) | 较快 (毫秒) |
| 数据持久化 | 可选 | 默认持久化 |
| 适用场景 | 缓存、临时数据 | 持久化数据 |

### 数据过期 (TTL)

```go
// 设置 key，720 小时后自动删除
redisClient.Set(ctx, key, value, 720*time.Hour)
```

**为什么用 TTL？**
- Refresh Token 30 天后自动过期
- 不需要手动清理，Redis 自动删除

---

## 4. Go 项目结构

### 标准布局

```
apps/api/
├── cmd/                    ← 可执行文件入口
│   └── server/
│       └── main.go
├── internal/               ← 内部包（不对外暴露）
│   ├── database/           ← 数据库连接
│   ├── model/              ← 数据模型
│   ├── repository/         ← 数据访问层
│   ├── service/            ← 业务逻辑层
│   ├── handler/            ← HTTP 处理层
│   ├── middleware/          ← 中间件
│   ├── dto/                ← 数据传输对象
│   └── auth/               ← 认证工具
├── migrations/             ← SQL 迁移文件
├── pkg/                    ← 可被外部引用的包（本项目未使用）
├── go.mod                  ← 依赖声明
├── go.sum                  ← 依赖校验
└── Dockerfile
```

### 为什么用 `internal/`？

Go 语言规定：`internal/` 包下的代码只能被当前模块引用，其他模块无法导入。

```go
// 可以引用（同项目）
import "github.com/bodysense/api/internal/database"

// 编译错误（其他项目）
import "github.com/bodysense/api/internal/database"  // ❌ 不允许
```

**好处：**
- 强制封装，防止外部依赖内部实现
- 可以随意重构内部结构而不影响外部

### `cmd/` vs `internal/`

| 目录 | 职责 | 示例 |
|------|------|------|
| `cmd/` | 程序入口，初始化和启动 | 解析配置、连接数据库、启动服务器 |
| `internal/` | 具体实现 | 业务逻辑、数据访问、HTTP 处理 |

---

## 5. 数据库迁移 (golang-migrate)

### 什么是数据库迁移？

迁移是一种版本控制数据库结构的方式。每次结构变更都是一个可执行的 SQL 文件。

### 迁移文件命名

```
000001_create_users.up.sql      ← 创建表
000001_create_users.down.sql    ← 撤销创建
000002_add_user_avatar.up.sql   ← 添加字段
000002_add_user_avatar.down.sql ← 撤销添加
```

**规则：**
- `up` = 正向迁移（应用变更）
- `down` = 回滚迁移（撤销变更）
- 数字前缀确保执行顺序

### 迁移文件示例

```sql
-- 000001_create_users.up.sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ
);

CREATE INDEX idx_users_email ON users(email);
```

```sql
-- 000001_create_users.down.sql
DROP TABLE IF EXISTS users;
```

### 为什么用迁移？

| 场景 | 不用迁移 | 用迁移 |
|------|----------|--------|
| 新成员加入 | 手动建表，容易出错 | 自动同步 |
| 回滚问题 | 手动 DROP，可能遗漏 | `migrate down` |
| 多环境同步 | 各环境结构不一致 | 所有环境用同一套迁移 |

### Go 中执行迁移

```go
func RunMigrations(cfg Config) error {
    dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
        cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)

    m, err := migrate.New("file://migrations", dsn)
    if err != nil {
        return err
    }
    defer m.Close()

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return err
    }

    return nil
}
```

### 常用命令

```bash
# 创建新迁移
migrate create -ext sql -dir apps/api/migrations -seq add_user_avatar

# 应用所有迁移
migrate -path apps/api/migrations -database "postgres://..." up

# 回滚最近一次
migrate -path apps/api/migrations -database "postgres://..." down 1

# 查看当前版本
migrate -path apps/api/migrations -database "postgres://..." version
```

---

## 6. Dockerfile 多阶段构建

### 什么是多阶段构建？

在一个 Dockerfile 中使用多个 `FROM` 指令，每个阶段可以独立构建，最终只复制需要的文件。

### Go API Dockerfile 解析

```dockerfile
# ── Build stage ─────────────────────────────────────────
FROM golang:1.26-alpine AS builder    # 阶段 1：编译环境
WORKDIR /app
COPY go.mod go.sum ./                 # 先复制依赖文件
RUN go mod download                   # 下载依赖（利用缓存）
COPY . .                              # 复制源码
RUN CGO_ENABLED=0 go build -o /server # 编译（禁用 CGO）

# ── Runtime stage ──────────────────────────────────────
FROM alpine:3.20                      # 阶段 2：运行环境
RUN apk add --no-cache curl           # 只装必要的
COPY --from=builder /server /server   # 从阶段 1 复制编译结果
HEALTHCHECK CMD curl -f http://localhost:8080/api/health || exit 1
CMD ["/server"]
```

### 为什么用多阶段构建？

```
┌─────────────────┐           ┌─────────────────┐
│  golang:1.26    │           │  alpine:3.20    │
│  ~800MB         │    ───→   │  ~15MB          │
│  + 编译工具      │           │  + 可执行文件   │
│  + 源码 + 依赖   │           │                  │
└─────────────────┘           └─────────────────┘
     编译用镜像                    生产用镜像
```

**好处：**
- 最终镜像小（15MB vs 800MB）
- 不包含源码和编译工具，更安全
- 攻击面更小

### 缓存优化

```dockerfile
COPY go.mod go.sum ./    # 先复制这两个文件
RUN go mod download      # 下载依赖
COPY . .                 # 再复制源码
```

**为什么分两步 COPY？**
- `go.mod` 和 `go.sum` 不常变化
- Docker 会缓存 `go mod download` 这一层
- 只有源码变化时不需要重新下载依赖

---

## 7. 环境变量管理

### .env 文件

```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_NAME=bodysense
DB_USER=bodysense
DB_PASSWORD=bodysense123

# Redis
REDIS_HOST=localhost
REDIS_PORT=6384
REDIS_PASSWORD=bodysense123

# API
API_PORT=8080
JWT_SECRET_KEY=bodysense-dev-secret-key
```

### 为什么用环境变量？

| 问题 | 解决方案 |
|------|----------|
| 密码硬编码在代码里 | 用环境变量，代码不包含敏感信息 |
| 不同环境配置不同 | 每个环境有自己的 `.env` 文件 |
| 配置变更需要改代码 | 只改 `.env`，代码不用动 |

### Go 中读取环境变量

```go
// 直接读取
port := os.Getenv("API_PORT")

// 带默认值
func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}
```

### 使用 godotenv 加载 .env

```go
import "github.com/joho/godotenv"

func main() {
    _ = godotenv.Load("../../.env")  // 从项目根目录加载
    // 之后可以用 os.Getenv() 读取
}
```

### .gitignore 中的 .env

```gitignore
.env              # 不提交到 Git
.env.example      # 提交示例文件
```

**为什么？**
- `.env` 包含密码，不能公开
- `.env.example` 是模板，帮助其他人知道需要哪些配置

---

## 8. Nx Monorepo

### 什么是 Monorepo？

Monorepo 是一种代码管理方式，将多个项目放在同一个仓库中。

```
bodysense/
├── apps/
│   ├── web/          ← React 前端
│   ├── api/          ← Go 后端
│   └── ai-service/   ← Python AI 服务
└── packages/         ← 共享包
```

### Nx 的作用

Nx 是一个 Monorepo 管理工具，提供：
- 统一的命令接口
- 任务缓存
- 依赖图分析
- 只运行受影响的任务

### 项目配置

**文件：** `apps/api/project.json`
```json
{
  "name": "api",
  "targets": {
    "build": {
      "command": "go build -o bin/api ./..."
    },
    "serve": {
      "command": "go run ./cmd/server"
    },
    "test": {
      "command": "go test ./..."
    }
  }
}
```

### 常用命令

```bash
# 运行 API
pnpm nx serve api

# 构建 Web
pnpm nx build web

# 运行测试
pnpm nx test api

# 只运行受影响的任务
pnpm nx affected -t build
```

---

## 9. 关键概念总结

| 概念 | 作用 | 类比 |
|------|------|------|
| Docker Compose | 管理多个容器 | 像一个"容器编排器" |
| PostgreSQL | 关系型数据库 | 像一个"结构化文件柜" |
| Redis | 内存缓存 | 像一个"快速便签本" |
| pgvector | 向量搜索扩展 | 像一个"语义搜索引擎" |
| Migration | 版本控制数据库结构 | 像 Git 管理代码 |
| internal/ | 封装内部实现 | 像"私有方法" |
| 多阶段构建 | 减小镜像体积 | 像"只打包必要行李" |
| 环境变量 | 管理配置和密钥 | 像"外部配置文件" |
| Nx | Monorepo 管理 | 像"项目总管" |

---

## 10. 常见问题

### Q: Docker 容器启动失败怎么办？

```bash
# 查看日志
docker compose -f docker/docker-compose.yml logs postgres-dev

# 检查端口占用
netstat -ano | findstr :5432

# 重启容器
docker compose -f docker/docker-compose.yml restart postgres-dev
```

### Q: 如何进入容器内部？

```bash
# 进入 PostgreSQL 容器
docker exec -it bodysense-postgres-dev-1 psql -U bodysense

# 进入 Redis 容器
docker exec -it bodysense-redis-dev-1 redis-cli -a bodysense123
```

### Q: 数据库迁移失败怎么办？

```bash
# 查看当前版本
migrate -path apps/api/migrations -database "postgres://..." version

# 强制设置版本（慎用）
migrate -path apps/api/migrations -database "postgres://..." force 1

# 手动执行 SQL
psql -U bodysense -d bodysense -f apps/api/migrations/000001_create_users.up.sql
```

---

*Issue 1 知识点整理完成 | 2026-06-21*
