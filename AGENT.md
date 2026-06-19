# BodySense Agent Guide

本文件是仓库内 AI 协作的唯一维护入口。`AGENTS.md`、`CLAUDE.md` 和 GitHub/Copilot 相关入口只允许指向这里或补充平台特有说明，不再维护平行规则。

## 真值顺序

1. 当前代码、配置和测试
2. 根配置与项目配置：`pnpm-workspace.yaml`、`package.json`、`tsconfig.json`、`go.mod`、`pyproject.toml`
3. `docs/` 下的正式文档（PRD、技术方案、原型）
4. 历史说明、背景材料和归档计划

文档与代码冲突时，以当前代码、配置和测试为准，然后回收或修正文档。

## 技术栈

- **前端**：React 19 + TypeScript 6 + Vite 8 + shadcn/ui + Tailwind CSS 4
- **后端**：Go 1.26 + Gin 1.12 + GORM
- **AI 服务**：Python 3.13 + FastAPI + LangChain v1 + uv
- **数据库**：PostgreSQL 16 (pgvector) + Redis 7
- **部署**：Docker Compose + Caddy + Watchtower
- **Monorepo**：Nx workspace + pnpm 11

## 项目结构

```
apps/
  web/          ← React 前端（pnpm，Nx 项目名 @bodysense/web）
  api/          ← Go 后端（go mod，Nx 项目名 api）
  ai-service/   ← Python AI 服务（uv，Nx 项目名 ai-service）
packages/       ← 共享 TS 包（@bodysense/*）
docker/         ← Docker Compose 编排
tools/
  agent-skills/ ← 项目专属 Agent Skills
scripts/        ← 开发辅助脚本
docs/           ← 项目文档（PRD、技术方案、原型）
```

根配置文件：`pnpm-workspace.yaml`、`nx.json`、`tsconfig.base.json`、`project.json`、`package.json`

## 文件夹架构与命名规范

### 前端 apps/web/src/（React，Feature-based）

```
src/
  components/          ← 通用 UI 组件（Button、Card 等，PascalCase.tsx）
  features/            ← 按功能模块拆分
    {feature}/         ← 如 assessment、consultation、report、profile
      components/      ← 该功能的专属组件（PascalCase.tsx）
      hooks/           ← 该功能的 hooks（useXxx.ts）
      services/        ← 该功能的 API 调用（xxxService.ts）
      index.ts         ← barrel file
  pages/               ← 页面级组件（PascalCasePage.tsx）
  hooks/               ← 全局共享 hooks（useXxx.ts）
  services/            ← 全局共享 API 层（xxxService.ts）
  stores/              ← Zustand 全局 stores（xxxStore.ts）
  lib/                 ← 工具函数、常量、类型（camelCase.ts）
  App.tsx
  main.tsx
```

**文件命名**：组件 `PascalCase.tsx`，hooks `useXxx.ts`，stores `xxxStore.ts`，services `xxxService.ts`，types `xxx.types.ts`，constants `xxx.constants.ts`，页面 `XxxPage.tsx`，测试 `*.test.tsx`。

### 后端 apps/api/（Go，Standard Layout）

```
cmd/
  server/
    main.go            ← 入口，只做初始化和依赖注入
internal/
  handler/             ← HTTP handler（snake_case.go）
  service/             ← 业务逻辑层（snake_case.go）
  repository/          ← 数据访问层，GORM 封装（snake_case.go）
  model/               ← 数据模型，GORM struct（snake_case.go）
  middleware/           ← 中间件（snake_case.go）
  dto/                 ← 请求/响应结构体（snake_case.go）
pkg/                   ← 可被外部导入的公共包（如有）
migrations/            ← 数据库迁移文件
```

**文件命名**：全部 `snake_case.go`，测试 `*_test.go` 与被测文件同目录。这是 Go 社区标准。

### AI 服务 apps/ai-service/（Python，分层结构）

```
src/
  main.py              ← FastAPI 入口
  config.py            ← 配置管理（Pydantic Settings）
  api/
    routes/            ← 路由处理（snake_case.py）
    dependencies.py    ← FastAPI 依赖注入
  services/            ← 业务逻辑（snake_case.py）
  rag/                 ← RAG 管道（knowledge_base.py、embeddings.py、retriever.py）
  prompts/             ← Prompt 模板（snake_case.py）
  models/              ← Pydantic 数据模型（snake_case.py）
tests/
  conftest.py          ← pytest fixtures
  unit/                ← 单元测试（test_*.py）
  integration/         ← 集成测试（test_*.py）
```

**文件命名**：全部 `snake_case.py`，测试 `test_*.py` 放在 `tests/` 对应子目录。遵循 PEP 8。

### 共享包 packages/（@bodysense/*）

按需创建，当前预留：
- `@bodysense/contracts`：前后端共享类型定义（API DTO、枚举）
- `@bodysense/utils`：通用工具函数

包命名跟目录名 1:1 映射（kebab-case），每个包根有 `index.ts` barrel file。

### 文档 docs/

```
docs/
  PRD-体态健康AI助手.md
  technical-approach.md
  ai-development-workflow.md
  prototype.jsx
  plan/
    active/            ← 进行中的 task spec
    archive/           ← 完成的 plan 归档
```

### Import 路径规范

- **跨包引用**：`@bodysense/contracts`（pnpm workspace 解析）
- **前端包内引用**：`@/` 路径别名指向 `apps/web/src/`
- **相对路径**：同 feature 内的近距离引用
- **Barrel file**：仅在 feature 根（`features/{feature}/index.ts`）和 package 根（`packages/{pkg}/src/index.ts`）设置，内部目录不用 barrel

## 工作方式

- 先读代码和配置，再修改。
- 日常代码探索优先使用 CodeGraph 查询符号、入口、调用链和影响范围；确认具体文件后再读取实现。
- 涉及 Nx workspace、project、target、affected 或任务依赖时使用 nx-mcp。
- 查询技术文档最新版本时使用 Context7 MCP。
- 浏览器自动化和 E2E 测试使用 Playwright MCP。
- GitHub PR 管理和代码搜索使用 GitHub MCP。
- 优先使用 `pnpm` 而非 `npm`。
- 所有 Nx 命令统一使用 `pnpm nx ...`。
- Go 模块使用 `go mod` 管理依赖。
- Python 依赖使用 `uv` 管理（`uv add`、`uv sync`、`uv lock`），不使用 pip。
- 需要 build、lint、test 时，优先运行离改动最近的 Nx target 或对应语言的测试命令。
- 涉及 Docker、运行时、env 注入、部署链路的改动，默认先用 `docker compose -f docker/docker-compose.yml --profile dev` 做本地 prod-like 验证。
- 复杂任务先写计划，再实施。计划统一放在 `docs/plan/active/` 目录下，完成后移至 `docs/plan/archive/`。

## Git 分支策略

采用 3 分支 Git Flow：

- `main`：生产就绪代码，只接受从 `dev` 合并
- `dev`：开发主线，feature 分支合并到此
- `feature/xxx`：功能分支，从 `dev` 切出，完成后合并回 `dev`

## Commit 规范

遵循 [Conventional Commits](https://www.conventionalcommits.org/)：

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**type 类型**：`feat`、`fix`、`docs`、`style`、`refactor`、`test`、`chore`、`perf`、`ci`、`build`

**scope 范围**：`web`、`api`、`ai`、`docker`、`docs`、`deps`

示例：
- `feat(web): 实现信息收集分步表单`
- `fix(ai): 修复 RAG 检索结果的排序逻辑`
- `docs: 更新技术方案中的版本信息`

通过 husky + @commitlint/cli 在 `commit-msg` hook 中强制校验。规则定义在 `commitlint.config.ts`。

## 变更策略

- 项目处于 MVP 活跃开发期，不要求向后兼容。
- 不需要数据迁移路径。
- 优先做根因修复，不引入临时 shim、补丁层或双轨兼容。
- 如果更干净的结构性重构可行，优先于局部修补。
- 保持实现直接、明确、易读。

## 配置与文档边界

- 不在多个文件重复抄同一套配置；配置细节以配置文件本身为准。
- 局部配置允许存在，但必须继承根配置并只保留最小例外。
- `docs/` 下的文档是项目知识的唯一正式来源。

## 协作入口约定

- `AGENT.md`：唯一维护中的协作规范。
- `AGENTS.md`、`CLAUDE.md`：只做 shim，指向 AGENT.md。
- `.github/copilot-instructions.md`：只补 GitHub/Copilot 特有约束，不复制仓库规范。

## Repository Skills

- 项目专属 agent skills 统一放在 `tools/agent-skills/`。
- 当前 skills：
  - `validate-local-deploy`：验证 Docker Compose 本地环境
  - `validate-rag-pipeline`：验证 RAG 知识库管道质量
  - `validate-docs-code`：验证文档与代码的一致性
- 安装示例与目录约定见 `tools/agent-skills/README.md`。

## 最小验证

- 文档和治理相关改动：检查文档格式和链接有效性
- 前端改动：`pnpm nx run web:lint && pnpm nx run web:typecheck`
- 后端改动：`cd apps/api && go vet ./... && go test ./...`
- AI 服务改动：`cd apps/ai-service && uv run ruff check . && uv run pytest`
- Docker 相关改动：`docker compose -f docker/docker-compose.yml --profile dev up -d` 验证
