# BodySense Repo-Native Postman API Workspace Plan

> 文档状态：PLAN ONLY / 待实施
> 创建日期：2026-09-10
> 目标：让 GCP Dev 上的项目 Agent 以 BodySense 源码为事实来源，自动生成并维护 repo-native Postman Collections / Environments / Tests，再由 Postman CLI 校验、运行并同步到 Postman Cloud；Windows Postman Desktop 主要作为 Cloud View、交互调试与手工验证界面。
> 明确约束：本阶段只产出方案文档；不安装 Postman CLI、不创建/修改 Postman Cloud Workspace、不写真实凭证、不改生产代码、不改 API、不改部署、不提交 Git。

---

## 0. Executive Summary

BodySense 当前不应该采用“在 Postman Desktop 里逐个手工录入 endpoint”的模式，也不应该把 Postman Cloud 当作 API 契约的第一真值。

推荐的目标模型是：

```text
BodySense source code on gcp-dev
          |
          | project AI agent / coding agent
          v
repo-native Postman assets
  .postman/resources.yaml
  postman/collections/*
  postman/environments/*
  postman/documents/* (optional)
          |
          | Postman CLI
          | lint / prepare / run / push
          v
Postman Cloud Workspace
          |
          v
Windows Postman Desktop
  Cloud View / Send / Debug / manual experiments
```

职责必须分开：

```text
AI Agent
= 读代码、抽取 API、补充请求语义、生成/更新 Postman 资产

Postman CLI
= 校验、prepare、运行 collection、同步 Cloud

Postman Cloud
= 已发布 API workspace 的分发与共享视图

Postman Desktop
= GUI 消费、交互测试、临时调试，不作为主要 source-of-truth
```

长期目标不是让 AI 每次“重新猜一遍 API”，而是建立一条可重复、可审计、可增量更新的生成链：

```text
Gin/FastAPI source
   -> deterministic route inventory
   -> AI semantic enrichment
   -> repo-native Postman files
   -> coverage audit
   -> runtime smoke/contract tests
   -> Postman Cloud sync
```

在 OpenAPI/codegen 架构正式落地之后，Postman 的来源再从“源码推导”升级为“canonical OpenAPI 生成”，避免 Postman 成为第二套手工契约。

---

## 1. 为什么这套方案适合 BodySense

### 1.1 开发事实目前集中在 GCP Dev

当前主开发仓库：

```text
/home/dev/projects/bodysense
```

开发 Agent 已经能够直接读取：

- Gin route registration；
- Handler；
- DTO；
- Service；
- Repository；
- Web API wrappers；
- FastAPI routes；
- CI/deployment workflow；
- auth/cookie/origin 配置。

因此没有必要先把完整仓库复制到 Windows，再让 Desktop Agent 做一次重复推导。

### 1.2 BodySense API surface 已经足够大，手工维护不划算

截至本计划创建时，`apps/api/cmd/server/main.go` 中可静态看到约 **96 个 HTTP verb route registration**（包括 health、auth、protected/public、knowledge/operator 路由）。

AI service 源码中可静态看到约 **15 个 FastAPI route decorator**。

这已经超过“手工维护几个 Postman request”最舒服的规模。

如果人工录入：

```text
Route changed
  -> developer remembers to update Postman
  -> maybe updates body
  -> maybe forgets status/error/example
```

很容易产生 drift。

推荐：

```text
Route changed
  -> deterministic inventory changes
  -> CI detects Postman coverage drift
  -> Agent updates Postman artifact in same change
```

### 1.3 BodySense 已有 Dev / Staging / Prod 三个明确运行环境

当前仓库可确认：

```text
Dev API default:
http://127.0.0.1:8080

Staging application:
https://gcp-dev-01.taile92a8e.ts.net:20150

Production application:
https://body.bakersean.top
```

Postman 应该用**同一套 Collection + 多 Environment**表达部署差异，而不是复制三套 Collections。

### 1.4 当前认证结构很适合做自动化 Postman flow

当前 BodySense：

```text
Access Token
- JWT
- JSON response 返回
- 15 min 默认 TTL
- Browser 侧在 Zustand 内存保存

Refresh Token
- opaque random token
- Set-Cookie
- HttpOnly
- /api/v1/auth path
- 通过 Cookie 轮换 session
```

Postman 可以把 login / refresh / protected request 组合成稳定的 API 调试工作流。

---

## 2. 关键架构决策

### Decision A — Repo-first，而不是 Cloud-first

BodySense 的可审查 Postman 资产必须进 Git。

```text
Git = authoring history / review boundary
Postman Cloud = published projection
```

Cloud View 不作为唯一事实来源。

### Decision B — Agent author，CLI executor

不假设 Postman CLI 自己具备“读完整项目代码并理解业务”的能力。

AI authoring 使用 BodySense 已有项目 Agent / coding agent / ForgeFlow 能力；Postman CLI 只承担：

- workspace prepare；
- workspace push/pull（按最终支持能力）；
- collection run；
- lint/validation；
- CI automation。

### Decision C — 先源码推导，后 OpenAPI canonical

当前阶段：

```text
Go/Python source
-> Postman derived artifacts
```

未来 contract-codegen plan 落地后：

```text
Canonical OpenAPI / JSON Schema / Proto
-> generated Postman
-> generated TS/Go/Python clients/models
```

因此 Postman 资产从第一天就必须标记为 **derived/executable API workspace**，不能再产生一套独立手工 schema。

### Decision D — Deterministic inventory + AI enrichment

不要让 AI 纯自由发挥扫描整个仓库后直接覆盖 Collection。

两阶段更稳：

```text
Stage 1: deterministic extraction
- route
- method
- handler symbol
- middleware group
- source file/line

Stage 2: AI enrichment
- request DTO
- response shape
- auth requirement
- cookie/origin behavior
- error taxonomy
- example payload
- tests
- description/source anchors
```

这样 AI 的工作是“解释已知 route”，而不是“猜有哪些 route”。

### Decision E — Prod 默认 read-only

Postman 中所有 production mutation 必须默认阻断。

```text
GET/HEAD prod
-> allowed

POST/PUT/PATCH/DELETE prod
-> blocked by pre-request policy unless explicit override
```

CI 永远不自动对生产执行 destructive/mutating collection。

---

## 3. Scope / Non-Scope

### 3.1 Phase 1 纳入范围

第一阶段纳入：

1. Go public/user API：`/api/v1/*`；
2. auth：`/api/v1/auth/*`；
3. health：`/api/health`；
4. BodyState / HealthWorkspace / Profile / Consultation / Assessment / Treatment / Training / Uploads；
5. knowledge/operator API 单独分类；
6. FastAPI internal AI service 单独 Collection；
7. Dev / Staging / Prod environments；
8. request examples；
9. expected status tests；
10. auth automation；
11. route coverage audit；
12. Postman Cloud sync；
13. Desktop consumption workflow。

### 3.2 第一阶段不做

- 不把 LiteLLM provider API 当 BodySense product API；
- 不自动向 Prod 写测试数据；
- 不把真实用户账号作为 API test fixture；
- 不把 refresh token/password/access token 写入 Git；
- 不把 Postman Collection 设为最终 canonical contract；
- 不在本计划阶段安装任何工具；
- 不为了 Postman 改现有 API 设计；
- 不强行把 SSE/NDJSON 简化成普通 JSON endpoint。

---

## 4. 当前事实基线

### 4.1 Go API registration

主要入口：

```text
apps/api/cmd/server/main.go
```

主要 group：

```text
/api/v1/auth
/api/v1 protected
/api/v1/conversations
/api/v1/consultations
/api/v1/body-state
/api/v1/assessment
/api/v1/training
/public style share route
/knowledge/operator routes
```

当前约 96 个 verb registrations。

### 4.2 FastAPI internal routes

主要路径：

```text
apps/ai-service/src/main.py
apps/ai-service/src/document_main.py
apps/ai-service/src/api/routes/*.py
```

当前静态 decorator 约 15 个，包括：

```text
/health
/extract
/extract-text
/generate
/threads/{thread_id}/turns
/threads/{thread_id}/interrupts/{interrupt_id}/resume
/ingestions/video
/search
/sources
/stats
/analyze
/recommend
```

最终路径前缀必须从 router include/prefix 真实解析，不能只根据 decorator 字符串猜。

### 4.3 Web client wrappers

目前存在：

```text
apps/web/src/features/assessment/services/assessmentService.ts
apps/web/src/features/auth/services/authService.ts
apps/web/src/features/consultation/services/consultationService.ts
apps/web/src/features/profile/services/bodyMetricsService.ts
apps/web/src/features/profile/services/healthHistoryService.ts
apps/web/src/features/profile/services/lifestyleService.ts
apps/web/src/features/profile/services/onboardingContextService.ts
apps/web/src/features/profile/services/privacyService.ts
apps/web/src/features/training/services/trainingService.ts
apps/web/src/features/workspace/api/workspaceApi.ts
```

这些文件不是 route 真值，但非常适合作为：

- 浏览器实际 payload；
- client-visible response；
- query/path parameters；
- authFetch 行为；
- error handling；

的交叉验证来源。

### 4.4 Auth boundary

Protected API：

```go
protected.Use(middleware.AuthMiddleware(jwtConfig, userRepo, sessionCache))
```

Access JWT claims 当前包含：

```text
user_id
email
session_id
sub
exp
iat
```

Middleware 会：

1. 读取 `Authorization: Bearer ...`；
2. 验证签名/有效期；
3. 验证 session cache；
4. 将 user id / email 写入 Gin Context。

因此 Postman protected requests 应继承 collection-level Bearer auth。

---

## 5. 目标 Repository Layout

预期结构：

```text
bodysense/
├── .postman/
│   └── resources.yaml
│
├── postman/
│   ├── collections/
│   │   ├── BodySense API/
│   │   └── BodySense Internal AI API/
│   │
│   ├── environments/
│   │   ├── BodySense Dev.environment.yaml
│   │   ├── BodySense Staging.environment.yaml
│   │   └── BodySense Prod.environment.yaml
│   │
│   └── documents/
│       └── README.md                     # optional workspace usage note
│
├── scripts/
│   └── postman/
│       ├── inventory-routes.*           # deterministic source inventory
│       ├── verify-coverage.*            # source vs collection parity
│       ├── scan-secrets.*               # prevent credential commit
│       └── verify-environments.*         # required variable policy
│
└── .github/workflows/
    └── postman-sync.yml                 # Phase 3/4 only
```

具体 Postman Native Git 文件格式、Collection V3 YAML 目录形态和 `resources.yaml` schema 在实施时以**当前已安装 CLI/官方 schema 实际生成结果**为准，不手工发明私有格式。

---

## 6. Collection Information Architecture

### 6.1 BodySense API

建议目录：

```text
BodySense API
├── 00 System
│   └── Health
├── 10 Auth
│   ├── Register
│   ├── Login
│   ├── Refresh
│   ├── Logout
│   └── Me
├── 20 Profile & Onboarding
├── 30 Uploads & Health Documents
├── 40 Conversations
├── 50 Consultation Runtime
├── 60 Diagnosis
├── 70 Treatment & Outcomes
├── 80 Body State & Workspace
├── 90 Lifestyle / Metrics / History
├── 100 Assessment
├── 110 Training
├── 120 Privacy
├── 130 Public Sharing
└── 140 Knowledge Operator
```

不要为了排序过度依赖数字；如果 Native Git/Postman UI 已有更好的 folder order metadata，就按 tool-native 方式实现。

### 6.2 BodySense Internal AI API

单独 Collection：

```text
BodySense Internal AI API
├── Health
├── Runtime
├── Knowledge
├── OCR / Document
├── Posture
├── Assessment
├── Diagnosis
├── Treatment
└── Title
```

理由：

- 与浏览器/public API 权限不同；
- 运行地址不同；
- request/stream 协议不同；
- 不应该让普通 product API consumer 误用 internal endpoints。

### 6.3 Streaming endpoints

对 SSE / NDJSON / long-lived stream：

- 明确标记 `streaming`；
- 不强行用普通 response JSON test 替代；
- 如 Postman 当前 runner 对流支持不满足需求，保持“手工/专项脚本验证”状态；
- Postman request description 指向真实 protocol source/parser；
- 不为了 Postman 可视化而修改 production wire protocol。

---

## 7. 每个 Request 必须携带的可追溯信息

Agent 生成 request 时，description 至少包含：

```text
Method + route
Auth mode
Source route anchor
Handler symbol
Request DTO/model source
Response shape source
Known error mapping
Environment restrictions
Streaming/non-streaming
```

示例概念：

```text
POST /api/v1/body-state/facts

Route:
apps/api/cmd/server/main.go -> bodyStateHandler.UpsertFact

Handler:
apps/api/internal/handler/body_state_handler.go

Request:
dto.UpsertBodyStateFactRequest

Service:
BodyStateService.UpsertFact

Concurrency:
expected_revision optimistic concurrency
409 BODY_STATE_REVISION_CONFLICT
```

这样 Postman 不只是“能发请求”，同时成为 executable documentation projection。

---

## 8. Environment Design

### 8.1 共享变量 schema

三个 environment 至少统一：

```text
environmentName
baseUrl
origin
accessToken
accessTokenExpiresAt
revision
allowMutation
expectedDeploymentChannel
```

Internal AI collection 可以追加：

```text
aiBaseUrl
documentBaseUrl
```

### 8.2 Dev

默认语义：

```text
environmentName = dev
baseUrl = http://127.0.0.1:8080
origin = http://127.0.0.1:5173
allowMutation = true
```

但有一个必须显式解决的问题：

```text
127.0.0.1 on gcp-dev CLI
!=
127.0.0.1 on Windows Desktop
```

因此 Dev 的 `baseUrl` 必须设计成 **execution-context local override**，不能认为一个共享值同时适用于所有机器。

推荐策略：

```text
Git/shared Environment
- 定义变量名和安全默认值

GCP CLI
- local/CI override -> 127.0.0.1:8080

Windows Desktop
- local override -> SSH/Tailscale tunnel endpoint
  或未来专门的 Tailnet-only dev endpoint
```

不要为了让 Desktop 访问 Dev 就把开发 API 直接公开到公网。

### 8.3 Staging

当前：

```text
environmentName = staging
baseUrl = https://gcp-dev-01.taile92a8e.ts.net:20150
origin = https://gcp-dev-01.taile92a8e.ts.net:20150
allowMutation = true
```

注意：staging 是实际部署 channel，不等价于“开发 branch”。

### 8.4 Prod

当前：

```text
environmentName = prod
baseUrl = https://body.bakersean.top
origin = https://body.bakersean.top
allowMutation = false
```

Production environment 默认禁止 mutation。

---

## 9. Git Branch / Deployment Environment 必须解耦

概念：

```text
Git branch/SHA
= contract/source version

Environment
= request target runtime
```

不建立：

```text
dev branch -> dev
staging branch -> staging
prod branch -> prod
```

的硬映射。

BodySense 当前 staging / production 本身就是部署 pipeline 选定的 SHA/release。

### 9.1 版本偏移问题

真实情况可能是：

```text
main / staging = API vNext
production = previous release
```

如果 API contract 在两者间发生 breaking change，一份“当前 main Collection”不能天然准确描述旧 Prod。

因此 CI test 必须遵守：

```text
Test staging
-> checkout staging deployed SHA
-> run that SHA's Postman assets

Test production
-> checkout production release SHA/tag
-> run that release's Postman assets
```

Cloud View 主要展示“当前已发布稳定 API workspace”，不能替代 exact-SHA regression execution。

未来可增加 `contractVersion` / `deploymentSha` 健康信息进行自动对齐，但本阶段不为了 Postman 增加 production API 字段。

---

## 10. Authentication Automation

### 10.1 Collection-level Bearer

Protected requests 默认：

```text
Authorization: Bearer {{accessToken}}
```

子请求使用 inherit auth。

### 10.2 Login

```text
POST {{baseUrl}}/api/v1/auth/login
Origin: {{origin}}
Content-Type: application/json
```

Body 变量：

```json
{
  "email": "{{email}}",
  "password": "{{password}}"
}
```

Post-response script：

- 校验 2xx；
- 提取 `access_token`；
- 提取 `expires_in`；
- 写入当前 local/runtime variable；
- 不打印 token。

### 10.3 Refresh

```text
POST {{baseUrl}}/api/v1/auth/refresh
Origin: {{origin}}
```

Refresh token 保持 Cookie 语义，不创建 Git tracked `refreshToken` variable。

实施时必须验证当前 Postman App / CLI runner 对 Cookie Jar 的行为与 BodySense `HttpOnly + Path=/api/v1/auth + Secure + SameSite` 配置的兼容情况。

如果 CLI cookie behavior 与浏览器语义不等价：

- Postman 用于 API auth flow test；
- Browser E2E 继续负责 SameSite/CORS/HttpOnly 浏览器安全语义；
- 不修改服务器安全属性去适配 Postman。

### 10.4 Origin

当前 BodySense 在 secure-cookie 环境对 auth-sensitive endpoints 有 trusted-origin check。

因此 staging/prod auth requests 必须显式设置：

```text
Origin: {{origin}}
```

`baseUrl` 和 `origin` 始终作为两个独立变量保留，即使当前同源。

---

## 11. Secret Model

### 11.1 Git 中允许存在

```text
baseUrl
origin
environmentName
allowMutation
placeholder account name
example payload
non-secret test data
```

### 11.2 Git 中禁止存在

```text
real password
real access token
real refresh token
POSTMAN_API_KEY
production user credential
provider key
JWT secret
DB password
```

### 11.3 Cloud shared values

Cloud Workspace 也不应保存真实敏感值作为 shared/default value。

Desktop 使用：

- Postman Vault；或
- local-only value；
- secret variable capability（以当前版本为准）。

GCP/CI 使用：

```text
POSTMAN_API_KEY
BODYSENSE_TEST_EMAIL
BODYSENSE_TEST_PASSWORD
```

从 environment/secret manager 注入。

### 11.4 Secret scanner

计划新增 repo-side audit：

- 检测 JWT-like `xxx.xxx.xxx`；
- 检测 `bodysense_refresh=`；
- 检测 Postman API keys；
- 检测 password variable 非 placeholder；
- 检测常见 token/header 泄漏。

CI 在 `postman/**` 变化时执行。

---

## 12. API Discovery Pipeline

### Phase A — Deterministic Go route inventory

计划实现一个 source extractor，产出稳定 manifest，例如：

```json
{
  "method": "POST",
  "path": "/api/v1/body-state/facts",
  "auth": "protected",
  "handler": "bodyStateHandler.UpsertFact",
  "source": "apps/api/cmd/server/main.go"
}
```

优先使用 AST / structure-aware parser，不长期依赖脆弱 regex。

必须正确解析：

- group prefix；
- nested group；
- public/protected/auth/operator middleware；
- path params；
- route aliases。

### Phase B — Deterministic FastAPI inventory

从：

```text
APIRouter
include_router
prefix
route decorator
```

得到完整 internal path。

如果 FastAPI runtime OpenAPI 可在 dev 安全启动后直接获取，可作为二次事实校验，但不能在计划阶段假设所有 `document_main` 都启用 OpenAPI。

### Phase C — AI semantic enrichment

Agent 对每条 route 追踪：

```text
route
-> handler
-> bind request DTO
-> service call
-> repository/error mapping
-> response
```

同时查看 Web client wrapper 做交叉验证。

Agent 不允许只读 Handler 名称就猜 body。

### Phase D — Generated Postman element

每条 endpoint 生成：

- URL；
- method；
- inherited auth；
- headers；
- path/query params；
- body；
- example；
- tests；
- source references；
- tags/folder；
- safety classification。

### Phase E — Parity verification

```text
source route inventory
       vs
Postman request inventory
```

输出：

```text
missing in Postman
orphan in Postman
method mismatch
path mismatch
auth classification mismatch
```

Hard gate：核心 public API 不允许 missing/orphan。

---

## 13. Mutation Safety Classification

所有 requests 分类：

```text
READ_ONLY
SAFE_AUTH
MUTATION_NON_DESTRUCTIVE
MUTATION_DESTRUCTIVE
OPERATOR_ONLY
INTERNAL_ONLY
STREAMING
```

Prod guard 至少覆盖：

```text
POST
PUT
PATCH
DELETE
```

但不能只按 HTTP method 判断，例如某些 POST replay/analyze/generate 可能不直接修改主状态，却可能产生运行、成本或记录；仍需显式分类。

Production pre-request policy：

```text
environmentName == prod
AND request safety != READ_ONLY
AND allowMutation != true
-> abort request
```

`allowMutation=true` 必须只允许 local 临时值，不能提交为 Prod 默认值。

Operator-only collection folder 必须额外 require operator credential profile，不继承普通用户账号。

---

## 14. Tests 设计

### 14.1 每个 endpoint 的基础 contract test

最少：

- expected success status；
- response JSON/content-type；
- required top-level keys；
- request id/header（如适用）；
- known error status；
- no token echo；
- no unexpected 5xx。

### 14.2 Auth suite

覆盖：

```text
Login -> 200
Access token present
Refresh cookie set
Me with access -> 200
Invalid access -> 401
Refresh -> new access
Logout -> revoke session
Old access/session behavior -> expected unauthorized
```

浏览器专属 SameSite/CORS semantics 继续由 E2E 负责。

### 14.3 BodyState optimistic concurrency suite

把当前学习实验正式放入 Postman：

```text
1. GET current state/revision
2. POST fact with expected_revision=N
   -> 200
3. repeat stale mutation expected_revision=N
   -> 409
4. response error == BODY_STATE_REVISION_CONFLICT
5. GET current state
   -> stale mutation not durable
```

这可同时成为：

- learner L4 evidence；
- API regression test；
- concurrency contract documentation。

### 14.4 Negative boundary suite

重点 endpoints 测：

- invalid JSON；
- missing field；
- malformed UUID；
- stale revision；
- unauthenticated；
- revoked session；
- invalid origin where applicable；
- unknown body region id；
- not found；
- service unavailable where intentionally modeled。

### 14.5 Runtime smoke tiers

```text
Tier 0: file/schema lint, no network
Tier 1: local Dev read/auth/core mutation
Tier 2: Staging full safe regression
Tier 3: Prod read-only smoke only
```

---

## 15. Workspace Bootstrap

### 15.1 One-time prerequisites

需要：

- 一个 BodySense Internal Postman Workspace；
- workspace ID；
- Postman API key（只进 secret）；
- GCP Dev 安装 Postman CLI；
- 当前 Git worktree clean enough to isolate generated files。

### 15.2 Bootstrap preference

推荐顺序：

```text
1. UI 创建一次 BodySense Internal Workspace
2. 获取 workspace ID
3. 在 repo 中建立 Native Git manifest/files
4. CLI prepare/lint
5. 第一次 push
6. Desktop 打开 Cloud View 验证
```

不要为了“全自动”强行把一次性 workspace creation 复杂化。

如果实施时 Postman API 创建 workspace 的方式明显更稳，也可以自动化，但这不是 MVP 的必要条件。

### 15.3 Planned CLI flow

实施时以当前 CLI help/官方 docs 校验参数，目标流程大致：

```bash
postman --version
postman login --with-api-key "$POSTMAN_API_KEY"
postman workspace prepare
# lint command according to installed CLI capabilities
postman workspace push --yes
```

`workspace push` 当前官方文档支持自动 prepare；CI 仍建议先独立 lint 再 push。

---

## 16. Postman Cloud 与 Desktop 的职责

### 16.1 Cloud View

用途：

- 查看稳定 Collection；
- 切换 Dev/Staging/Prod environment schema；
- GUI Send；
- API debugging；
- share within project/team；
- examples/docs。

### 16.2 Desktop Local View

Windows 不强制 checkout BodySense 才能使用 Cloud View。

如果将来 Windows 也有完整 repo checkout，可以选择连接 Native Git Local View，但这属于额外 developer convenience，不是主链必要条件。

### 16.3 编辑原则

推荐：

```text
Persistent contract/test changes
-> edit repo-native files through Agent/IDE/Postman Local View
-> commit/PR
-> CLI sync

Temporary debugging
-> Desktop Cloud View/request clone/scratch
```

如果 Cloud View 的持久编辑会在下一次 force sync 被覆盖，团队约定必须明确，防止“Cloud 手改但没进 Git”。

---

## 17. CI/CD Integration

### 17.1 PR Gate

当以下文件变化：

```text
apps/api/**
apps/ai-service/src/api/**
apps/web/src/**/services/**
apps/web/src/**/api/**
postman/**
.postman/**
```

运行：

```text
route inventory
Postman coverage audit
secret scan
Postman lint/prepare validation
local collection tests where feasible
```

### 17.2 Main / Staging

Main/staging promotion 后：

```text
checkout exact deployed SHA
run Postman staging suite
pass
-> push/sync stable Postman workspace
```

是否“main merge 立即 push Cloud”还是“staging promotion 成功后 push”必须和 BodySense release policy 对齐。

推荐后者：Cloud View 更接近“可部署/已部署 stable API”，而不是未验证的 main 瞬时状态。

### 17.3 Production

Production deployment：

```text
checkout production release SHA
run read-only prod smoke
never run general mutation suite
```

Cloud environment 可以更新 deployment metadata，但不能自动打开 `allowMutation`。

---

## 18. Sync Strategy

### 18.1 默认不用 force-sync 起步

第一次上线先用普通：

```text
prepare
push --yes
```

确认 Cloud entity mapping 正确。

`force-sync` 有删除 Cloud orphan 的语义，只有在：

- repo 已确认是唯一 authoring source；
- workspace 无人工保留对象；
- dry-run/backup 完成；

后才考虑。

### 18.2 `.postman/resources.yaml`

该 manifest 记录：

- workspace mapping；
- local resource path；
- cloud resource ID mapping；

必须进 Git，但不能包含 API key/credential。

### 18.3 Idempotency

同一 commit 连续执行两次生成与 push：

```text
第一次 -> create/update
第二次 -> no semantic diff / no duplicate resources
```

这是验收硬条件。

---

## 19. Agent Generation Rules

Agent 必须遵守：

1. 先生成 deterministic route inventory；
2. 再追 Handler/DTO，不允许凭 endpoint name 猜 body；
3. 每条 request 保存 source anchors；
4. 不读取/复制 `.env` 中真实 secrets 到 Postman artifacts；
5. 不把 generated token 写 example；
6. response examples 必须脱敏；
7. known production data 不作为 fixture；
8. 对 streaming route 标注 protocol；
9. 对 operator/internal route 单独分组；
10. generation 后必须做 route parity；
11. generation 后必须 `git diff` 审查；
12. 未经明确批准不执行 Cloud push；
13. 未经明确批准不对 Prod mutation。

---

## 20. Generated Examples / Sanitization

真实 DevTools/Postman response 中可能含：

```text
access_token
refresh cookie
email
health/user data
conversation text
```

保存 Example 前必须 sanitizer：

```text
access token -> <redacted-access-token>
refresh -> never persist
email -> user@example.com
UUID -> deterministic fake UUID
health content -> synthetic test text
```

禁止把一次手工抓包原样转存成 Git example。

---

## 21. Verification Matrix

| 维度                  | 验证方式                          | Hard Gate                   |
| --------------------- | --------------------------------- | --------------------------- |
| Route completeness    | Gin/FastAPI inventory vs Postman  | core API 0 missing          |
| Orphan requests       | Postman vs source                 | 0 unexplained orphan        |
| Method/path           | deterministic compare             | 0 mismatch                  |
| Auth classification   | middleware group + request config | protected route 100% auth   |
| Secrets               | repo scanner                      | 0 credential                |
| Dev smoke             | GCP localhost                     | pass                        |
| Staging smoke         | exact deployment                  | pass                        |
| Prod safety           | guard tests                       | mutation blocked            |
| Auth flow             | login/refresh/me/logout           | pass                        |
| OCC                   | stale revision                    | 409 + no partial write      |
| Idempotent generation | run twice                         | clean second diff           |
| Cloud sync            | push twice                        | no duplicates               |
| Desktop visibility    | Cloud View                        | elements visible/selectable |

---

## 22. Implementation Phases

### Phase 0 — Tool capability verification

只验证：

- installed/current Postman CLI version；
- Native Git current file format；
- `resources.yaml` schema；
- CLI workspace commands；
- collection runner cookie behavior；
- environment local/shared secret behavior；
- Cloud sync permissions。

产出：compatibility note，不改 API。

### Phase 1 — Bootstrap minimal repo-native workspace

只创建：

```text
System/Health
Auth/Login
Auth/Refresh
Auth/Me
BodyState/Get
BodyState/UpsertFact
```

以及 Dev/Staging/Prod environment schema。

先用最小样本证明：

```text
repo -> CLI -> Cloud -> Desktop
```

完整闭环。

### Phase 2 — Route inventory + automatic expansion

建立 deterministic route inventory。

AI 分批扩展：

```text
Auth/Profile
BodyState
Conversation/Consultation
Assessment/Training
Uploads/Documents
Diagnosis/Treatment
Operator/Knowledge
Internal AI
```

每批必须 coverage audit + diff review。

### Phase 3 — Runtime regression tests

加入：

- auth flow；
- stale revision；
- validation errors；
- read-only smoke；
- staging regression。

### Phase 4 — CI Cloud sync

稳定后才加入自动：

```text
PR validate
staging test
main/release cloud sync
prod read-only smoke
```

### Phase 5 — OpenAPI/codegen convergence

和：

```text
docs/plan/contract-codegen-architecture-spike-plan-2026-09-09.md
```

联动。

一旦 OpenAPI canonical 落地：

- route/request/response schema 从 OpenAPI 生成；
- AI 只负责补业务 examples/tests/source docs；
- Postman 不再从 Handler 自由推导 wire schema；
- source route parity 继续作为 sanity check。

---

## 23. Acceptance Criteria

MVP 完成必须同时满足：

1. GCP Dev 上 Postman CLI 可无 GUI 工作；
2. BodySense Workspace 可由 CLI push 更新；
3. Desktop Cloud View 能看到同一 Workspace；
4. 三个 environments 存在且结构一致；
5. Prod `allowMutation=false`；
6. real secrets 不在 Git / shared Cloud values；
7. collection-level Bearer 工作；
8. login 自动获取 access token；
9. refresh flow 可验证；
10. BodyState stale revision test 稳定得到 409；
11. stale mutation 不产生 partial durable write；
12. core Go API route coverage = 100%；
13. Internal AI API 单独 Collection；
14. route generator 连续运行两次无无意义 diff；
15. Postman workspace push 连续执行不产生重复 entity；
16. CI 能检查 coverage/secrets/lint；
17. production CI 只执行 read-only smoke。

---

## 24. Rollback Strategy

Postman 接入不允许成为 production availability 依赖。

因此：

```text
Postman Cloud outage
-> BodySense product unaffected

Postman CLI failure
-> block Postman publication / optional API test lane
-> 不应自动回滚 production unless explicitly promoted to release gate later

Generated collection broken
-> revert Git commit
-> push previous known-good Postman assets
```

第一阶段 Postman CI 建议先作为独立 quality lane；成熟后再决定哪些 contract tests 升级成 required gate。

---

## 25. 风险与待确认项

### R1 — Native Git format / CLI version drift

Postman 当前产品变化较快。实施必须先执行 Phase 0，不把旧版 JSON-only CLI 文档和新版 Collection V3 YAML 行为混用。

### R2 — Dev baseUrl execution-context mismatch

GCP `127.0.0.1` 与 Windows `127.0.0.1` 不是同一服务。必须依赖 local override/tunnel，而不是把 Dev 共享 URL 写死。

### R3 — Staging/Prod contract version skew

必须 exact-SHA 测试，不能假设最新 Collection 永远描述旧 Prod。

### R4 — Cookie semantics

Postman runner 不是浏览器。HttpOnly/SameSite/CORS 的真正浏览器安全语义继续由 Browser E2E 验证。

### R5 — AI hallucinated API schema

通过 deterministic route inventory、DTO trace、client cross-check、runtime test 降低风险。

### R6 — Production accidental mutation

通过 environment guard + CI policy + local explicit override 多层阻断。

### R7 — Generated artifact noise

要求 deterministic ordering/stable IDs/idempotent generation；否则 Git diff 会失去审查价值。

---

## 26. 与现有 BodySense 学习体系的关系

这套 Postman workspace 还可以成为课程的 learner evidence 工具，但**不能因为 CI 已经有测试就自动认定 learner mastery**。

例如当前：

```text
BS-FSO-0.4 HTTP mutation sequence
```

学习者可以亲自用 Postman 执行：

```text
expected_revision=N -> 200
same N again -> 409
GET verify -> no stale write
```

然后解释：

```text
Browser/API Client
-> Auth Middleware
-> Handler
-> Service
-> Repository transaction
-> SELECT FOR UPDATE
-> expected revision compare
-> commit/rollback
```

这才作为 L4 learner evidence。

自动 collection test 本身只是课程基础设施，不等于学习者完成验证。

---

## 27. 推荐实施顺序（最小风险版本）

```text
A. 创建/确认 BodySense Internal Workspace
   |
B. Phase 0: CLI + Native Git capability audit
   |
C. bootstrap .postman/resources.yaml + minimal collection
   |
D. Dev/Staging/Prod environment schema
   |
E. secret guard + prod mutation guard
   |
F. minimal auth + BodyState 6 requests
   |
G. CLI prepare/lint/run
   |
H. first Cloud push
   |
I. Windows Desktop Cloud View verification
   |
J. stale-revision L4 experiment
   |
K. deterministic full route inventory
   |
L. AI expansion to full collections
   |
M. coverage gate
   |
N. CI sync
   |
O. future OpenAPI convergence
```

不要第一步就一次生成全部 100+ API 并 force-sync Cloud。先验证 6 个核心 request 的完整闭环，确认 file format / auth / environment / sync 都正确，再自动扩张。

---

## 28. 计划中的首个真实验收场景

第一个 end-to-end scenario 就使用当前课程中的 BodyState OCC：

```text
Environment: Staging

1. Login
   -> access token stored locally
   -> refresh cookie captured

2. GET /api/v1/body-state
   -> read current revision N

3. POST /api/v1/body-state/facts
   expected_revision=N
   -> 200
   -> revision N+1

4. Repeat stale POST
   expected_revision=N
   -> 409 BODY_STATE_REVISION_CONFLICT

5. GET /api/v1/body-state
   -> confirm stale write absent
```

这个场景同时证明：

- Environment；
- Bearer auth；
- Cookie/auth integration；
- request body；
- response test；
- repository transaction semantics；
- Postman runner；
- Cloud/Desktop visibility。

---

## 29. 最终目标状态

完成后，开发者不再维护：

```text
代码一份
Postman 手写一份
文档再手写一份
```

而变成：

```text
                Source / Canonical Contract
                         |
                 deterministic facts
                         |
                 AI semantic enrichment
                         |
             repo-native Postman workspace
                         |
              lint / tests / coverage
                         |
                  Postman Cloud
                         |
              Desktop / team consumers
```

未来 OpenAPI 落地：

```text
             Canonical OpenAPI
          /        |         \
         v         v          v
   Web client    Postman     Go/Python
     codegen     workspace    contracts
```

这才是 BodySense 长期更稳的 API 工程化方向。

---

## 30. Official References for Implementation Phase

实施前再次核对当前版本：

- Postman Native Git setup:
  https://learning.postman.com/docs/use/native-git/setup
- Postman Native Git overview:
  https://learning.postman.com/docs/use/native-git/overview
- Develop locally with Native Git:
  https://learning.postman.com/docs/use/native-git/develop-locally
- Automate Native Git with CI/CD:
  https://learning.postman.com/docs/use/native-git/automation
- Postman CLI workspace commands:
  https://learning.postman.com/docs/postman-cli/postman-cli-workspace

这些链接只作为实施时能力核验入口；实际命令参数以实施当天官方文档与本机 `postman --help` 为准。

---

## 31. 当前状态

```text
PLAN ONLY
```

本文件创建不代表：

- Postman CLI 已安装；
- Workspace 已连接；
- Cloud 已同步；
- Collections 已生成；
- Environment 已创建；
- Secret 已配置；
- CI 已改变；
- Production 已被访问或修改。

下一步只有在明确批准实施后，才从 **Phase 0 — Tool capability verification** 开始。
