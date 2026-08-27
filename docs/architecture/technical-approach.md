# BodySense — AI 体态健康助手 技术方案文档

> **Historical implementation reference.** Routes and domain ownership in this document may be obsolete. See [Current Longitudinal System](./current-longitudinal-system.md).


> **⚠️ HISTORICAL — 初始设计文档。** 本文档为 2026-06-19 的初稿，描述项目启动时的架构选型和 API 设计。后续架构演进详见：
> - [ADR 0001: 深化运行时模块](../adr/0001-deepen-runtime-modules.md)
> - [ADR 0002: Agent Runtime 所有权模型](../adr/0002-agent-runtime-ownership.md)（runtime 当前真值）
> - [ADR 0004: Longitudinal BodyState](../adr/0004-adopt-longitudinal-body-state-model.md)（business domain 当前真值）
> - [Longitudinal BodyState Domain Model](./longitudinal-body-state-domain.md)
> - [Final Agent Runtime Architecture](../plan/active/final-agent-runtime-architecture.md)

**文档版本**：v1.0
**更新日期**：2026-06-19
**状态**：已归档（初稿，已被后续 ADR 和架构文档取代）
**关联文档**：[PRD-体态健康AI助手.md](../PRD-体态健康AI助手.md)、[prototype.jsx](./prototype.jsx)

---

## 1. 系统架构总览

### 1.1 架构选型

采用三层服务架构：React 前端 + Go 后端 + Python AI 服务。前端负责用户交互和界面展示，Go 后端承担 API 网关、业务逻辑、鉴权和数据持久化，Python AI 服务封装大模型调用、RAG 检索和 OCR 能力。三个服务通过 HTTP/gRPC 通信，统一由 Docker Compose 编排部署。

```
┌─────────────────────┐         HTTPS          ┌─────────────────────┐
│   React 前端 (SPA)  │  ◄──────────────────►  │   Caddy 反向代理     │
│   shadcn/ui + TW    │                        │  (自动 HTTPS 证书)   │
└─────────────────────┘                        └──────────┬──────────┘
                                                          │
                                              ┌───────────▼───────────┐
                                              │    Go 后端 (API)      │
                                              │  鉴权 / 路由 / 业务   │
                                              └──────┬──────────┬─────┘
                                                     │          │
                                          ┌──────────▼──┐  ┌───▼──────────┐
                                          │  PostgreSQL  │  │ Python AI    │
                                          │  + pgvector  │  │ 服务         │
                                          │  + Redis     │  │ LLM + RAG    │
                                          └──────────────┘  └──────────────┘
```

### 1.2 仓库结构（Monorepo）

项目采用 Monorepo 结构，前端、后端、AI 服务统一在同一仓库中管理，方便跨服务协调和版本对齐。

```
BodySense/
├── PRD-体态健康AI助手.md
├── docs/
│   ├── PRD-体态健康AI助手.md
│   ├── architecture/
│   │   ├── README.md                  ← 架构中心索引
│   │   ├── technical-approach.md      ← 本文档
│   │   └── deployment-architecture.md
│   └── plan/
│       ├── active/
│       └── archive/                   ← 历史设计文档
├── apps/
│   ├── web/                           ← React 前端
│   │   ├── src/
│   │   │   ├── components/
│   │   │   ├── pages/
│   │   │   ├── hooks/
│   │   │   ├── services/              ← API 调用封装
│   │   │   ├── stores/                ← 状态管理
│   │   │   └── App.tsx
│   │   ├── package.json
│   │   └── vite.config.ts
│   ├── api/                           ← Go 后端
│   │   ├── cmd/
│   │   │   └── server/
│   │   │       └── main.go
│   │   ├── internal/
│   │   │   ├── handler/               ← HTTP 路由处理
│   │   │   ├── service/               ← 业务逻辑
│   │   │   ├── repository/            ← 数据库访问
│   │   │   ├── model/                 ← 数据模型
│   │   │   └── middleware/            ← 鉴权、日志等中间件
│   │   ├── go.mod
│   │   └── go.sum
│   └── ai-service/                    ← Python AI 服务
│       ├── src/
│       │   ├── llm/                   ← LLM 调用封装
│       │   ├── rag/                   ← RAG 检索
│       │   ├── ocr/                   ← OCR 处理
│       │   └── prompts/              ← Prompt 模板
│       ├── pyproject.toml              ← uv 项目配置
│       ├── uv.lock                     ← 依赖锁定文件
│       └── Dockerfile
├── docker/
│   ├── docker-compose.yml             ← 开发环境
│   ├── docker-compose.prod.yml        ← 生产环境
│   └── nginx/
├── scripts/                           ← 开发辅助脚本
└── README.md
```

---

## 2. 技术选型详解

### 2.1 前端

| 技术 | 版本 | 用途 |
|------|------|------|
| React | 19.2 | UI 框架 |
| TypeScript | 6.0 | 类型安全 |
| Vite | 8.0 | 构建工具 |
| shadcn/ui | CLI 4.8 | UI 组件库 |
| Tailwind CSS | 4.3 | 原子化样式 |
| React Router | 7.6 | 页面路由 |
| Zustand | 5.0 | 全局状态管理 |
| TanStack Query | 5.101 | 服务端状态管理（API 请求缓存） |
| lucide-react | latest | 图标库 |

**选型理由**：shadcn/ui + Tailwind 的组合面向 C 端用户，视觉定制灵活度高，组件可按需引入不增加包体积。相比 Ant Design 更适合需要个性化视觉设计的健康类产品。Zustand 轻量且无 boilerplate，配合 TanStack Query 管理服务端状态，避免在 Redux 上花费过多精力。

**版本亮点**：

- **React 19.2**：React Compiler 已稳定发布，可自动优化组件重渲染，无需手动 `useMemo` / `useCallback`。`useEffectEvent` 稳定化，简化 Effect 中的事件处理逻辑。`use()` API 支持在渲染中直接消费 Promise。
- **Vite 8.0**（2026-03 发布）：基于 Rolldown（Rust 重写的 Rollup）构建，冷启动和 HMR 性能大幅提升。
- **Tailwind CSS 4.3**：全新的 CSS-first 配置方式（通过 `@theme` 指令在 CSS 中定义设计令牌），不再依赖 `tailwind.config.js`。使用 Oxide 引擎（Rust + Lightning CSS），构建速度提升 10 倍以上。
- **React Router 7.6**：同时支持 framework 模式和 library 模式。深度集成 React 19 的并发特性，`useNavigate()` / `useSubmit()` 现在返回 Promise，可配合 `React.use()` 使用。
- **Zustand 5.0**：引入 `useShallow` hook 简化浅比较选择器，API 更简洁。注意 v5 要求 selector 返回稳定引用，否则需搭配 `useShallow` 使用。

### 2.2 后端（Go）

| 技术 | 版本 | 用途 |
|------|------|------|
| Go | 1.26 | 编程语言 |
| Gin | 1.12 | HTTP 框架 |
| GORM | 2.x (latest) | ORM |
| JWT (golang-jwt) | v5 | 用户鉴权 |
| sqlc | latest | 类型安全 SQL 生成（可选替代 GORM） |

**选型理由**：Go 作为后端在并发处理和资源消耗方面优势明显，适合做 API 网关层统一处理鉴权、限流和请求转发。Go 与 Python AI 服务之间通过 HTTP 通信，保持松耦合。GORM 在 MVP 阶段降低数据库操作的开发成本；后续如果对 SQL 性能有更高要求，可以切换到 sqlc 获得更精细的控制。

**版本亮点**：

- **Go 1.26**（2026 最新稳定版）：`updatemaxprocs` 特性支持运行时动态更新 GOMAXPROCS，自动适配 CPU 亲和性和 cgroup 限制变化，对容器化部署更友好。`crypto/rsa` 强制最低 1024 位密钥，增强安全性。
- **Gin 1.12**：要求 Go 1.25+，零分配路由和中间件生态进一步成熟。

### 2.3 AI 层（Python）

| 技术 | 版本 | 用途 |
|------|------|------|
| Python | 3.13 | 编程语言 |
| FastAPI | 0.136 | API 框架 |
| LangChain | v1 | LLM 编排和 RAG 管道 |
| PaddleOCR | latest | 体检报告 OCR |
| pgvector (via psycopg) | latest | 向量检索 |

**选型理由**：Python 是 AI/ML 领域的主流语言，FastAPI 提供异步高性能 API 框架且自动生成 OpenAPI 文档。LangChain 简化了 LLM 调用链和 RAG 管道的构建。PaddleOCR 在中文场景下识别准确率优于 Tesseract。

**版本亮点**：

- **Python 3.13**：引入 Tier 2 解释器（微指令格式），为后续 JIT 编译优化奠定基础。`copy.replace()` 标准库函数简化不可变对象（dataclass、namedtuple）的字段替换操作。
- **FastAPI 0.136**：依赖 Starlette ≥0.46 和 Pydantic ≥2.9，全面拥抱 Pydantic v2 的高性能验证。
- **LangChain v1**：全新的生产就绪版本，提供 `create_agent` 统一接口，基于 LangGraph 构建可靠 Agent。要求 Python 3.10+，已放弃 Python 3.9 支持。

### 2.4 数据库

| 组件 | 版本 | 用途 |
|------|------|------|
| PostgreSQL + pgvector | 16 | 结构化数据存储 + 向量检索 |
| Redis | 7.x | 会话缓存、Token 管理、限流计数 |

**选型理由**：PostgreSQL 的 pgvector 扩展可以在不引入独立向量数据库的前提下支持 RAG 所需的向量相似度检索，显著降低 MVP 阶段的运维复杂度。Redis 用于对话上下文缓存和用户会话管理，保障高频读写场景的性能。

### 2.5 开发工具链

| 工具 | 版本 | 用途 |
|------|------|------|
| Node.js | 24 LTS | JavaScript 运行时（Vite / TypeScript 编译） |
| pnpm | 11 | Node.js / TypeScript 包管理器 |
| uv | 0.11 | Python 包和项目管理器（替代 pip / poetry / pyenv） |
| Go | 1.26 | 内置 `go mod` 模块管理 |
| Docker + Compose | latest | 容器化开发与部署 |
| Caddy | 2-alpine | 反向代理 + 自动 HTTPS |
| Watchtower | latest | 容器镜像自动更新 |

**选型理由**：

- **pnpm** 相比 npm / Yarn，磁盘空间利用率更高（硬链接复用），Monorepo workspace 支持成熟，安装速度快。v11 是当前活跃维护版本，安全补丁支持到 2027 年 4 月。
- **uv** 由 Astral（Ruff 团队）用 Rust 开发，包安装速度比 pip 快 10-100 倍，同时替代 pip-tools、poetry、pyenv 等多个工具，一个命令行搞定 Python 版本管理、虚拟环境、依赖锁定和项目构建。v0.10+ 已稳定全部核心功能（`uv python install`、`uv workspace`、`uv lock` 等），生产就绪。
- **Node.js 24 LTS** 是当前长期支持版本（2025-10 进入 Active LTS），稳定性和安全性有保障。26 为 Current 版本，10 月后将转为下一轮 LTS，届时可按需升级。

**各项目对应的工具配置**：

| 项目模块 | 包管理器 | 锁定文件 | 说明 |
|----------|----------|----------|------|
| `apps/web/`（React 前端） | pnpm 11 | `pnpm-lock.yaml` | 通过 `pnpm-workspace.yaml` 管理 workspace |
| `apps/api/`（Go 后端） | go mod | `go.sum` | Go 内置模块管理，无需额外工具 |
| `apps/ai-service/`（Python AI） | uv 0.11 | `uv.lock` | 使用 `pyproject.toml` + `uv sync` 管理依赖 |

**Dockerfile 中的工具安装示例**：

```dockerfile
# Python AI 服务 — 使用 uv 管理依赖
FROM python:3.13-slim
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/
WORKDIR /app
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev
COPY . .
EXPOSE 8100
CMD ["uv", "run", "uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8100"]
```

```dockerfile
# React 前端 — 使用 pnpm 管理依赖
FROM node:24-alpine AS builder
RUN corepack enable && corepack prepare pnpm@11 --activate
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
```

---

## 3. LLM 选型对比分析

PRD 中提到支持 OpenAI / Claude / 国内大模型 API。以下是从本项目需求角度出发的对比分析：

### 3.1 对比维度

| 维度 | OpenAI (GPT-4o) | Claude (3.5 Sonnet) | 通义千问 (Qwen-Max) | DeepSeek (V3) |
|------|-----------------|---------------------|--------------------|--------------  |
| 中文能力 | 优秀 | 优秀 | 原生中文，最自然 | 优秀 |
| 医学/健康领域 | 知识库丰富 | 推理能力强 | 中文医学语境好 | 性价比高 |
| 流式输出 | SSE 支持完善 | SSE 支持完善 | SSE 支持 | SSE 支持 |
| Function Calling | 成熟 | 成熟 | 支持 | 支持 |
| 价格 (每百万 token) | $2.5-$10 | $3-$15 | ¥0.02-¥0.12 | ¥1-¥4 |
| 国内访问 | 需代理 | 需代理 | 直连 | 直连 |
| RAG 友好度 | 高 | 高 | 高 | 高 |

### 3.2 建议方案

**MVP 阶段推荐：通义千问 (Qwen-Max)**，原因如下：

1. 原生中文模型，在健康领域的中文表达最自然，不需要额外的 prompt 工程来保证中文输出质量。
2. 国内直连，无需配置代理，降低部署复杂度。
3. 价格最低，MVP 阶段控制成本。
4. 支持 Function Calling 和流式输出，满足对话问诊和结构化信息提取的需求。

**备选方案**：如果通义千问在某些健康领域场景下表现不理想，可以切换到 DeepSeek V3 作为 fallback，或后续升级到 GPT-4o（需解决网络问题）。

### 3.3 多模型适配层设计

无论最终选择哪个模型，Python AI 服务内部应封装统一的 LLM 调用接口，通过配置切换底层模型：

```python
# ai-service/src/llm/provider.py（示意）

class LLMProvider(Protocol):
    async def chat(self, messages: list[Message], tools: list[Tool] | None = None) -> AsyncIterator[str]:
        ...

class QwenProvider(LLMProvider):
    ...

class OpenAIProvider(LLMProvider):
    ...

class DeepSeekProvider(LLMProvider):
    ...

# 通过环境变量切换 provider
def create_provider() -> LLMProvider:
    provider_name = os.getenv("LLM_PROVIDER", "qwen")
    ...
```

---

## 4. 数据库设计

### 4.1 核心数据表

#### users — 用户表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID (PK) | 用户唯一标识 |
| email | VARCHAR(255) UNIQUE | 邮箱（登录凭证） |
| password_hash | VARCHAR(255) | 密码哈希（bcrypt） |
| created_at | TIMESTAMPTZ | 注册时间 |
| last_login_at | TIMESTAMPTZ | 最后登录时间 |

#### user_profiles — 稳定身份档案表

> ADR 0007：`user_profiles` 不再承担可变化健康状态。生活方式、身体测量和健康历史由 BodyState 持久化并保留时间语义。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID (PK) | 档案 ID |
| user_id | UUID (FK → users) | 关联用户 |
| gender | VARCHAR(20) | 用户提供的性别背景 |
| birth_date | DATE | 出生日期；年龄按当前日期派生，不持久化 age |
| created_at | TIMESTAMPTZ | 创建时间 |
| updated_at | TIMESTAMPTZ | 最后更新时间 |

可变化健康信息的归属：

```text
height / weight          -> BodyState Observation (anthropometry.*)
activity / sleep / etc.  -> BodyState Fact (lifestyle.*)
injury history summary   -> BodyState Fact (history.injury_summary)
BMI                       -> current height + weight derived projection
```

#### user_uploads — 上传材料表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID (PK) | 材料 ID |
| user_id | UUID (FK → users) | 关联用户 |
| file_type | VARCHAR(20) | posture_photo / health_report |
| file_path | TEXT | 文件存储路径 |
| ocr_result | JSONB | OCR 提取结果（体检报告用） |
| uploaded_at | TIMESTAMPTZ | 上传时间 |

#### consultation_sessions — 咨询会话表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID (PK) | 会话 ID |
| user_id | UUID (FK → users) | 关联用户 |
| messages | JSONB | 完整对话记录（含角色、内容、时间戳） |
| extracted_info | JSONB | AI 提取的结构化症状信息 |
| diagnosis | JSONB | 可能性判断结果 |
| treatment_plan | JSONB | 改善方案 |
| status | VARCHAR(20) | in_progress / completed |
| created_at | TIMESTAMPTZ | 创建时间 |
| ended_at | TIMESTAMPTZ | 结束时间 |

#### assessment_reports — 评估报告表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID (PK) | 报告 ID |
| user_id | UUID (FK → users) | 关联用户 |
| health_grade | VARCHAR(5) | 健康等级（A/B/C/D） |
| dimension_scores | JSONB | 各维度评分 |
| identified_issues | JSONB | 识别的问题列表 |
| improvement_summary | JSONB | 改善方案概要 |
| created_at | TIMESTAMPTZ | 生成时间 |

#### knowledge_entries — 知识库条目表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID (PK) | 条目 ID |
| category | VARCHAR(100) | 问题分类（如：圆肩、骨盆前倾） |
| title | VARCHAR(255) | 标题 |
| content | TEXT | 知识内容 |
| embedding | VECTOR(1536) | 文本向量（pgvector） |
| source_video | TEXT | 来源视频信息 |
| source_timestamp | VARCHAR(50) | 视频时间戳 |
| created_at | TIMESTAMPTZ | 入库时间 |

#### training_plans — 训练计划表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID (PK) | 计划 ID |
| user_id | UUID (FK → users) | 关联用户 |
| consultation_id | UUID (FK → consultation_sessions) | 关联咨询会话 |
| goal | TEXT | 训练目标 |
| duration_weeks | INT | 训练周期（周） |
| current_week | INT | 当前所在周 |
| phases | JSONB | 各阶段详细计划 |
| created_at | TIMESTAMPTZ | 创建时间 |

#### training_logs — 训练日志表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID (PK) | 日志 ID |
| user_id | UUID (FK → users) | 关联用户 |
| plan_id | UUID (FK → training_plans) | 关联训练计划 |
| date | DATE | 训练日期 |
| exercises | JSONB | 当日训练任务及完成状态 |
| notes | TEXT | 用户训练感受记录 |
| is_checked_in | BOOLEAN | 是否打卡 |
| created_at | TIMESTAMPTZ | 记录时间 |

### 4.2 索引设计

```sql
-- 用户表
CREATE UNIQUE INDEX idx_users_email ON users(email);

-- 身体档案
CREATE INDEX idx_profiles_user ON user_profiles(user_id);

-- 上传材料
CREATE INDEX idx_uploads_user ON user_uploads(user_id);
CREATE INDEX idx_uploads_type ON user_uploads(file_type);

-- 咨询会话
CREATE INDEX idx_sessions_user ON consultation_sessions(user_id);
CREATE INDEX idx_sessions_status ON consultation_sessions(status);

-- 评估报告
CREATE INDEX idx_reports_user ON assessment_reports(user_id);

-- 知识库向量索引（pgvector IVFFlat）
CREATE INDEX idx_knowledge_embedding ON knowledge_entries
  USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- 训练计划
CREATE INDEX idx_plans_user ON training_plans(user_id);

-- 训练日志
CREATE INDEX idx_logs_user_date ON training_logs(user_id, date);
```

---

## 5. API 接口设计

### 5.1 接口概览

#### 认证模块

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/auth/register | 用户注册 |
| POST | /api/v1/auth/login | 用户登录 |
| POST | /api/v1/auth/refresh | 刷新 Token |

#### 用户档案

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/profile | 获取身体档案 |
| PUT | /api/v1/profile | 更新身体档案 |
| POST | /api/v1/profile/uploads | 上传体态照片/体检报告 |
| DELETE | /api/v1/profile/uploads/:id | 删除上传材料 |

#### 评估报告

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/assessment/generate | 生成评估报告 |
| GET | /api/v1/assessment/:id | 获取评估报告详情 |
| GET | /api/v1/assessment | 获取评估报告列表 |

#### 咨询会话

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/consultation | 创建咨询会话 |
| POST | /api/v1/consultation/:id/message | 发送消息（SSE 流式返回） |
| GET | /api/v1/consultation/:id | 获取会话详情 |
| GET | /api/v1/consultation | 获取会话列表 |
| PUT | /api/v1/consultation/:id/confirm | 确认诊断 |

#### 训练计划

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/training/generate | 生成训练计划 |
| GET | /api/v1/training/:id | 获取训练计划详情 |
| GET | /api/v1/training/:id/today | 获取今日训练任务 |
| POST | /api/v1/training/:id/checkin | 训练打卡 |
| PUT | /api/v1/training/:id/log | 提交训练日志 |
| GET | /api/v1/training/:id/progress | 获取训练进度统计 |

### 5.2 接口详细设计示例

#### 发送咨询消息（流式返回）

```
POST /api/v1/consultation/:id/message
Authorization: Bearer <token>
Content-Type: application/json

Request:
{
  "content": "我的肩膀主要是酸胀感，按压的时候会舒服一些"
}

Response: text/event-stream (SSE)
event: message
data: {"type": "text", "content": "了解了，酸胀感且按压缓解，这通常提示..."}

event: message
data: {"type": "extracted_info", "field": "symptom_type", "value": "酸胀感，按压缓解"}

event: message
data: {"type": "extracted_info", "field": "body_part", "value": "肩部"}

event: done
data: {"session_id": "xxx", "message_id": "yyy"}
```

#### 生成训练计划

```
POST /api/v1/training/generate
Authorization: Bearer <token>
Content-Type: application/json

Request:
{
  "consultation_id": "uuid-of-consultation",
  "preferences": {
    "daily_time_minutes": 30,
    "equipment": ["foam_roller", "resistance_band"],
    "available_days": ["mon", "tue", "wed", "thu", "fri"]
  }
}

Response (200 OK):
{
  "id": "uuid",
  "goal": "改善圆肩和头前伸症状",
  "duration_weeks": 4,
  "phases": [
    {
      "week": 1,
      "focus": "基础激活",
      "exercises": [...]
    },
    ...
  ],
  "created_at": "2026-06-19T10:30:00+08:00"
}
```

### 5.3 鉴权方案

采用 JWT 双 Token 机制：

- **Access Token**：有效期 7 天，放在 `Authorization: Bearer` header 中。
- **Refresh Token**：有效期 30 天，存放在 Redis 中，用于续期。
- 前端在 Access Token 过期后自动调用 `/auth/refresh` 获取新 Token。

Go 后端中间件统一校验 JWT：

```go
// middleware/auth.go（示意）
func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        token := extractBearerToken(c.GetHeader("Authorization"))
        claims, err := jwt.Validate(token, jwtSecret)
        if err != nil {
            c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
            return
        }
        c.Set("userID", claims.UserID)
        c.Next()
    }
}
```

---

## 6. AI / RAG 架构设计

### 6.1 对话问诊流程

Python AI 服务的对话问诊分为三个阶段，每个阶段使用不同的 Prompt 策略：

**阶段一：问题描述与引导**

AI 结合用户档案信息和当前描述，进行引导式追问。Prompt 中注入用户档案作为上下文，并使用 symptom_extraction tool（Function Calling）实时从对话中提取结构化信息。

**阶段二：可能性分析**

AI 综合所有已收集信息，结合 RAG 检索的知识库内容，生成可能性判断。此阶段 Prompt 中包含知识库检索结果作为参考依据，并要求 AI 输出置信度和匹配依据。

**阶段三：方案生成**

用户确认诊断后，AI 基于确认的诊断和知识库中的改善方案，生成个性化的训练和习惯调整建议。

### 6.2 RAG 检索管道

```
用户输入
    │
    ▼
┌───────────────────────┐
│  1. Embedding 生成     │  ← 使用 text-embedding 模型
│     (用户当前问题)      │
└───────────┬───────────┘
            │
            ▼
┌───────────────────────┐
│  2. pgvector 向量检索   │  ← cosine similarity, top-k=5
│     (知识库条目匹配)    │
└───────────┬───────────┘
            │
            ▼
┌───────────────────────┐
│  3. 重排序 (Reranker)  │  ← 可选：基于相关性分数筛选 top-3
└───────────┬───────────┘
            │
            ▼
┌───────────────────────┐
│  4. Prompt 拼装        │  ← 将检索结果注入 system prompt
│     + LLM 生成回答     │
└───────────────────────┘
```

### 6.3 知识库构建

知识库以 B 站 UP 主的视频内容为核心来源，构建步骤：

1. 视频字幕/语音转文字 → 提取体态知识、动作讲解、案例分析。
2. 按体态问题分类整理为结构化知识条目（约 200-500 条）。
3. 每条知识生成 Embedding 向量（使用 text-embedding-3-small 或 m3e-base），存入 knowledge_entries 表。
4. 基础质量校验：确保信息与主流运动康复知识不矛盾。

### 6.4 对话记忆管理

单次咨询会话维护上下文窗口，控制 token 用量：

- 保留最近 N 轮对话（建议 N=10）作为短期记忆。
- 超过窗口范围的历史对话进行摘要压缩，保留关键信息。
- 用户档案信息和已提取的结构化信息始终注入 system prompt，不占用对话窗口。

---

## 7. 前端架构

### 7.1 页面路由

```
/                    → 首页/引导页
/onboarding          → 信息收集（分步表单）
/assessment          → 评估报告
/assessment/:id      → 评估报告详情
/consultation        → 咨询工作台（新建/最近会话）
/consultation/:id    → 咨询工作台（指定会话）
/training            → 训练计划
/training/:id        → 训练计划详情
/history             → 历史记录
/profile             → 个人档案
/login               → 登录
/register            → 注册
```

### 7.2 状态管理

- **Zustand**：管理用户登录状态、全局 UI 状态（主题、侧边栏等）。
- **TanStack Query**：管理所有 API 请求的缓存、加载状态和错误处理。对话流式消息通过 SSE EventSource 处理，不走 TanStack Query。

### 7.3 咨询工作台实现要点

咨询工作台是前端最复杂的页面，核心挑战在于左右面板的实时联动：

- 左侧对话区使用 SSE (EventSource) 接收 AI 的流式响应，逐字渲染。
- 右侧面板的"结构化信息区"监听 SSE 中的 `extracted_info` 事件类型，实时更新对应字段。
- "身体可视化区"使用 SVG 人体示意图，根据 `extracted_info` 中的部位字段动态高亮对应区域。
- 移动端适配：左右分栏变为上下布局或 Tab 切换，通过 CSS media query 和 React 响应式 hook 实现。

### 7.4 人体可视化

使用 SVG 实现人体示意图，每个可交互部位有独立的 `<path>` 或 `<g>` 元素，通过 CSS class 控制高亮状态。SVG 资源需要包含正面、侧面、背面三个视图。

---

## 8. 部署方案（Docker）

### 8.1 开发环境

参考 dailyuse 项目的 docker-compose 模式，使用 profiles 区分环境：

```yaml
# docker/docker-compose.yml（开发环境示意）

services:
  postgres-dev:
    profiles: [dev]
    image: pgvector/pgvector:pg18
    environment:
      POSTGRES_USER: bodysense
      POSTGRES_PASSWORD: bodysense123
      POSTGRES_DB: bodysense
    ports:
      - '5432:5432'
    volumes:
      - postgres-dev-data:/var/lib/postgresql
    healthcheck:
      test: ['CMD-SHELL', 'pg_isready -U bodysense -d bodysense']
      interval: 10s
      timeout: 5s
      retries: 5

  redis-dev:
    profiles: [dev]
    image: redis:7-alpine
    ports:
      - '6384:6379'
    command: redis-server --appendonly yes --requirepass bodysense123

volumes:
  postgres-dev-data:
```

### 8.2 生产环境

参考 dailyuse 的 docker-compose.prod.yml，使用 Caddy 自动管理 HTTPS 证书，Watchtower 自动更新镜像：

```yaml
# docker/docker-compose.prod.yml（生产环境示意）

services:
  postgres:
    image: pgvector/pgvector:pg18
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${DB_NAME}
      POSTGRES_USER: ${DB_USER}
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres-prod-data:/var/lib/postgresql
    networks:
      - bodysense-network

  redis:
    image: redis:7-alpine
    restart: unless-stopped
    command: redis-server --appendonly yes --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis-prod-data:/data
    networks:
      - bodysense-network

  ai-service:
    image: ${REGISTRY}/bodysense-ai:${AI_TAG:-prod-latest}
    restart: unless-stopped
    labels:
      com.centurylinklabs.watchtower.enable: 'true'
    environment:
      LLM_PROVIDER: ${LLM_PROVIDER:-qwen}
      LLM_API_KEY: ${LLM_API_KEY}
      EMBEDDING_MODEL: ${EMBEDDING_MODEL:-text-embedding-3-small}
    networks:
      - bodysense-network

  api:
    image: ${REGISTRY}/bodysense-api:${API_TAG:-prod-latest}
    restart: unless-stopped
    labels:
      com.centurylinklabs.watchtower.enable: 'true'
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      ai-service:
        condition: service_healthy
    environment:
      DB_HOST: postgres
      REDIS_HOST: redis
      JWT_SECRET: ${JWT_SECRET}
      AI_SERVICE_URL: http://ai-service:8100
    volumes:
      - uploads:/app/uploads
    networks:
      - bodysense-network

  web:
    image: ${REGISTRY}/bodysense-web:${WEB_TAG:-prod-latest}
    restart: unless-stopped
    labels:
      com.centurylinklabs.watchtower.enable: 'true'
    depends_on:
      api:
        condition: service_healthy
    networks:
      - bodysense-network

  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - '80:80'
      - '443:443'
      - '443:443/udp'
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data
    networks:
      - bodysense-network

  watchtower:
    image: containrrr/watchtower
    restart: unless-stopped
    environment:
      WATCHTOWER_CLEANUP: 'true'
      WATCHTOWER_POLL_INTERVAL: 300
      WATCHTOWER_LABEL_ENABLE: 'true'
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    networks:
      - bodysense-network

networks:
  bodysense-network:
    driver: bridge

volumes:
  postgres-prod-data:
  redis-prod-data:
  uploads:
  caddy-data:
```

### 8.3 Dockerfile 示例

**Go 后端 (Dockerfile.api)**：

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /server /server
EXPOSE 3000
CMD ["/server"]
```

**Python AI 服务 (Dockerfile.ai)**：

```dockerfile
# 使用 uv 管理依赖（替代 pip）
FROM python:3.13-slim
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/
WORKDIR /app
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev
COPY . .
EXPOSE 8100
CMD ["uv", "run", "uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8100"]
```

**React 前端 (Dockerfile.web)**：

```dockerfile
FROM node:24-alpine AS builder
RUN corepack enable && corepack prepare pnpm@11 --activate
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

---

## 9. 安全与隐私

### 9.1 传输安全

- 所有 HTTP 通信通过 Caddy 反向代理，自动管理 Let's Encrypt HTTPS 证书。
- 内部服务间通信在 Docker network 内走内网，不暴露到公网。

### 9.2 数据加密

- 用户密码使用 bcrypt 哈希存储（cost factor ≥ 12）。
- 敏感个人信息（PII）字段在数据库中加密存储（PostgreSQL pgcrypto 扩展或应用层 AES 加密）。
- 文件存储（体态照片、体检报告）设置文件权限，仅通过 API 鉴权后可访问。

### 9.3 隐私合规

- 符合《个人信息保护法》要求：用户可查看、修改、删除全部个人数据。
- API 提供 `DELETE /api/v1/profile` 接口，实现用户数据完全清除（包括文件、对话记录、评估报告）。
- AI 对话数据不用于模型训练，除非用户明确授权。
- 上传的文件通过预签名 URL 或直接 API 上传，不暴露存储路径。

---

## 10. 性能保障

### 10.1 目标指标（来自 PRD）

| 指标 | 目标值 |
|------|--------|
| 首屏加载 | ≤ 3 秒 |
| AI 首 token 响应 | ≤ 2 秒 |
| AI 完整响应 | ≤ 5 秒 |
| 右侧面板更新延迟 | ≤ 1 秒 |

### 10.2 实现措施

- **前端**：Vite 构建产物自动代码分割，路由级别 lazy loading，图片懒加载。
- **Go 后端**：连接池管理（数据库、Redis），API 限流（令牌桶算法），响应缓存（静态数据）。
- **Python AI 服务**：流式输出（SSE）减少用户等待感，Embedding 缓存（相同问题不重复计算），RAG 检索结果缓存（Redis，TTL=5 分钟）。
- **数据库**：合理索引（见 4.2 节），向量检索使用 IVFFlat 索引，查询优化。

### 10.3 浏览器兼容性

| 目标 | 支持范围 |
|------|----------|
| 桌面浏览器 | Chrome、Safari、Edge、Firefox 最新两个主要版本 |
| 移动端浏览器 | iOS Safari 15+、Android Chrome 90+ |

**实现策略**：

- Vite 默认 target 设为 `es2020`，覆盖上述浏览器范围。
- Tailwind CSS 的 preflight 样式已包含浏览器重置，减少兼容问题。
- SSE (EventSource) 在所有目标浏览器中原生支持，无需 polyfill。
- 使用 `browserslist` 配置 Autoprefixer，自动添加必要的 CSS 前缀。
- 关键交互组件（对话、表单、可视化）在上线前需在真机上进行兼容性测试。

---

## 11. 技术风险与应对

### 11.1 风险评估

| 风险 | 影响 | 概率 | 应对策略 |
|------|------|------|----------|
| LLM 在健康领域产生不准确建议 | 高 | 中 | RAG 强制引用知识库内容，Prompt 中加入医疗免责声明，关键输出增加人工审核标记 |
| 流式对话在高并发下不稳定 | 中 | 低 | Go 后端作为网关层缓冲 SSE 流，设置超时和重试机制 |
| 知识库内容覆盖不足导致 AI 回答质量差 | 中 | 高 | Prompt 设计 fallback 策略——知识库无匹配时输出通用建议并明确标注"建议咨询专业人士" |
| 用户上传的体检报告 OCR 识别率低 | 中 | 中 | PaddleOCR 预处理优化（图片增强），低置信度字段标记为"待确认"让用户手动修正 |
| pgvector 在知识库规模增长后检索变慢 | 低 | 低 | 知识库条目控制在 1000 条以内足够 MVP，后续可迁移至 Milvus/Qdrant |

### 11.2 关键依赖

- **B 站 UP 主视频内容**：知识库质量直接依赖视频内容的专业性和覆盖范围。需要与 UP 主建立合作关系，获得内容使用授权。
- **LLM API 稳定性**：选择有 SLA 保障的 LLM 服务商，并准备 fallback 模型方案。
- **pgvector 扩展**：确认部署环境的 PostgreSQL 支持 pgvector 扩展（使用 `pgvector/pgvector:pg18` 镜像）。

---

## 12. MVP 版本范围定义

### 12.1 MVP 包含

- 用户注册/登录（邮箱 + 密码）
- 身体信息分步采集
- 健康评估报告生成
- AI 对话问诊（含结构化信息提取和可能性判断）
- 训练计划生成和每日打卡
- 进度追踪面板
- 历史记录查看
- 个人档案管理
- 知识库 RAG（基于 B 站 UP 主视频内容）

### 12.2 MVP 不包含（后续迭代）

- 手机号/微信登录
- AI 视觉分析体态照片（仅存档）
- 社区分享功能
- 云对象存储（MVP 用本地文件系统）
- 邮件/短信通知提醒
- 多语言支持
- 独立向量数据库

---

*文档结束。如有疑问或需要调整，请在评审中提出。*
