# BodySense AI 协作开发流程

本文件是 BodySense 项目的 AI 辅助开发操作手册，描述人与 AI 工具协作的完整日循环。它与 [`AGENT.md`](../AGENT.md) 互补——AGENT.md 定义 AI agent 的边界和约束（what to never do），本文件定义人+AI 如何一起工作（how to do things step by step）。

---

## 1. 开发循环（Development Loop）

每个开发 session 严格遵循以下 6 阶段循环。每个阶段有明确的输入、产出和检查点。

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│   ① 加载     ② 规划     ③ 实现     ④ 验证     ⑤ 提交     ⑥ 交接  │
│   Context → Spec&Plan → TDD Impl → Verify → Commit → Handoff   │
│                                                                 │
│   ◄────────────── 下一个 session 从这里继续 ──────────────────────│
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### ① Context Loading（加载上下文）

**目标**：让 AI 和人同步到相同的认知起点。

每个 session 开始时，按以下顺序加载上下文：

1. **规则层**（每个 session 都加载）：AGENT.md — 项目约束、真值顺序、commit 规范
2. **状态层**（每个 session 都加载）：最近 10 条 git log + 当前 git status + 未合并的 PR 列表
3. **任务层**（按需加载）：要处理的 feature/bug 相关的源文件和配置

加载原则是**渐进式披露（Progressive Disclosure）**——不是一次性把所有信息塞给 agent，而是按阶段需要提供对应的 context。具体策略见第 3 节。

**检查点**：AI 能准确说出"当前在哪个分支、最近在做什么、今天要做什么"，说明 context 加载充分。

### ② Spec & Plan（需求拆解）

**目标**：在写代码之前，先把需求拆成可执行、可验证的任务。

1. **理解需求**：阅读 PRD 或 issue 中的需求描述，用 CodeGraph 了解相关代码的现有结构
2. **写 spec**：对每个 task，在 `docs/plan/` 下写一个简短的 spec，包含：
   - 目标：这个 task 要达成什么
   - DONE 标准：怎么判断做完了（具体的测试用例、页面行为、API 响应）
   - 影响范围：会改哪些文件、影响哪些模块
   - 风险点：有什么可能出问题的地方
3. **拆粒度**：每个 task 应该在一个 session 内能完成。如果一个 task 太大，继续拆分

**检查点**：spec 的 DONE 标准是可执行的（能写成测试或命令），不是模糊的描述（如"用户体验好"）。

### ③ TDD Implement（测试先行实现）

**目标**：先定义"正确"长什么样，再让 AI 填写实现。

遵循 Red-Green-Refactor 循环：

1. **Red**：先写一个会失败的测试，描述期望的行为
2. **Green**：让 AI 实现代码使测试通过
3. **Refactor**：在测试保护下重构，优化代码结构

关键原则：

- **没有失败的测试，就不写产品代码**。如果你不确定"正确"应该是什么样子，先回去写 spec
- **一次只做一个 task**。在同一个 session 里不要同时推进两个不相关的 feature
- **遇到不熟悉的库/API**：先用 Context7 MCP 拿到当前版本的官方文档，再让 AI 生成代码。不要让 AI 凭记忆写可能过时的 API 调用

**检查点**：所有测试通过（`go test`、`pytest`、`pnpm nx run web:test`），且没有为了通过而跳过或禁用的测试。

### ④ Verify（多层验证）

**目标**：用自动化的方式证明代码是正确的。不要靠"看起来没问题"来声明完成。

验证按 5 层质量门逐级执行（详见第 4 节），核心原则：

- **只有端到端执行才算真正的验证**。单元测试通过不够，要能跑起来看到效果
- **每个验证步骤必须产出证据**。测试输出、lint 结果、截图——不是"我检查了，没问题"
- **按改动范围触发对应的验证**。改了什么就验证什么，不做不相关的验证

**检查点**：每个验证步骤都产出了可运行的证据（测试输出、lint 结果、截图），不存在"我看了，没问题"式的口头声明。

### ⑤ Commit（小步提交）

**目标**：用细粒度的 commit 做存档点，防止 AI 生成大量代码后偏离太远。

- 每完成一个有意义的逻辑单元就 commit，不要攒到最后一起提交
- commit message 严格遵循 Conventional Commits 规范（AGENT.md 中有详细格式）
- husky + commitlint 会自动校验格式，不符合规范的 commit 会被拒绝
- 如果一次 AI 交互改动了超过 10 个文件，在中间找合理的断点分多次 commit

**检查点**：`git log --oneline` 显示清晰的原子 commit，`git status` 干净，commitlint 全部通过。

### ⑥ Review & Handoff（评审交接）

**目标**：确保代码被人类审视过，并为下一个 session 留下清晰的入口。

1. **创建 PR**：用 GitHub MCP 创建 PR，附上改动摘要
2. **Review**：自己 review 一遍 AI 生成的代码，重点关注：
   - 安全相关代码（认证、授权、输入校验）
   - 数据库 schema 变更
   - 外部 API 调用
3. **交接准备**：如果当前 task 没有在一个 session 内完成，确保：
   - 当前改动已经 commit 并 push
   - 下一个 session 能直接从 git log 和代码状态恢复上下文

**检查点**：PR 已创建且 CI 全绿，代码中无未 review 的 AI 生成部分，下一个 session 的入口是明确的。

---

## 2. 工具分工（Tool Selection）

不同的 AI 工具擅长不同的阶段。核心原则：**不在同一个 task 中途切换工具**，完成一个逻辑单元再换。

### 各阶段推荐工具

| 阶段 | 推荐工具 | 说明 |
|------|---------|------|
| ① Context Loading | 任意工具 | 主要依赖 MCP server 获取上下文 |
| ② Spec & Plan | Claude Code / QoderWork | 需要 agent 模式的深度分析和跨文件探索能力 |
| ③ TDD Implement | Claude Code（大重构）/ QoderWork（多步骤编排） | agent 模式适合跨文件改动；单文件小改可用任何工具 |
| ④ Verify | 自动化工具链 | lint/typecheck/test 是自动跑的，Playwright MCP 做 E2E 验证 |
| ⑤ Commit | 任意工具 | husky + commitlint 自动校验，工具无关 |
| ⑥ Review & Handoff | GitHub MCP + Copilot | 创建 PR、检查 CI 状态、AI 辅助 review |

### MCP Server 使用决策树

当需要决定"现在该用哪个 MCP"时，按这个决策树走：

```
当前要做什么？
│
├── 用到不熟悉或刚升级的库/API？
│     → Context7（拿当前版本的官方文档，防止 AI 用过时 API）
│
├── 要理解代码关系、调用链、影响范围？
│     → CodeGraph（查符号、入口、依赖关系）
│
├── 涉及 Nx workspace / project / target / affected？
│     → nx-mcp（查项目图、任务依赖、变更影响）
│
├── 做完 UI 改动，要验证渲染效果？
│     → Playwright MCP（浏览器自动化 + E2E 测试）
│
└── 准备提交、创建 PR、查看 CI 状态？
      → GitHub MCP（PR 管理 + 代码搜索）
```

**注意**：活跃 MCP 总数建议控制在 15 个以内。超过这个数量，agent 在"选择用哪个工具"上的决策质量会下降。当前配置了 5 个，在安全范围内。

---

## 3. Context 管理（Context Management）

AI agent 的输出质量直接取决于输入的 context 质量。错误的 context 比没有 context 更危险。

### 渐进式披露策略

不是一次性加载所有信息，而是按需提供：

| 时机 | 加载什么 | 为什么 |
|------|---------|--------|
| Session 开始 | AGENT.md + git log + git status | 知道规则在哪、当前状态是什么 |
| 开始新 task | Spec 文件 + 相关源文件 | 聚焦到具体任务范围 |
| 使用外部库/API | Context7 获取当前版本文档 | 防止 AI 用过时的 API |
| 理解代码关系 | CodeGraph 查调用链 | 精确了解依赖和影响范围 |
| 做完 UI 改动 | Playwright 查看渲染效果 | 用真实浏览器验证，不是靠想象 |
| 准备提交 | git diff + test output | 确认改动范围正确、测试全过 |

### Context 加载原则

1. **先窄后宽**：先用 CodeGraph 定位到具体文件，再读取文件内容。不要直接读整个目录
2. **配置优先于代码**：当需要了解项目结构时，先读 `package.json`、`go.mod`、`pyproject.toml` 等配置文件，它们比源码更紧凑
3. **不重复加载**：如果 AGENT.md 里已经写了技术栈版本，不需要再让 AI 去各个 `package.json` 里查一遍
4. **用 MCP 代替文件搜索**：需要了解符号定义和调用关系时，CodeGraph 比 `grep` 更精确、context 消耗更少

---

## 4. 质量门（Quality Gates）

代码从编写到合并要经过 5 层质量门。每一层都有自动化机制，不依赖人的自觉。

### 5 层质量门

```
Layer 1  ──  Inline 约束（实时）
              AGENT.md 规则 + 工具内置约束
              ↓
Layer 2  ──  Pre-commit（本地提交前）
              husky → commitlint 校验 commit message 格式
              ↓
Layer 3  ──  CI（push/PR 触发）
              web: pnpm nx run web:lint + typecheck + build
              api: go vet ./... + go build ./... + go test ./...
              ai-service: uv run ruff check . + uv run pytest
              commit-lint: pnpm commitlint 校验所有 commit message
              ↓
Layer 4  ──  Skills 验证（按改动范围触发，PR 提交前执行）
              见下方映射表
              ↓
Layer 5  ──  PR Review（人工 + AI 辅助）
              安全代码人工 review
              CI + Skills 全绿后才可合并
```

### 改动范围 → 触发验证映射表

| 改动范围 | 触发验证 | 验证内容 | 成功标准 |
|---------|---------|---------|---------|
| `apps/web/` | CI: lint + typecheck + build | 前端代码质量 | 全部通过 |
| `apps/api/` | CI: vet + build + test | 后端代码质量 | 全部通过 |
| `apps/ai-service/` | CI: ruff + pytest | AI 服务代码质量 | 全部通过 |
| `docker/` 或 Dockerfile | **Skill: validate-local-deploy** | Docker Compose 环境启动 | 容器 healthy + DB/Redis 连通 |
| `apps/ai-service/src/rag/` | **Skill: validate-rag-pipeline** | RAG 管道质量 | 召回率 ≥ 80%，答案准确率 ≥ 70% |
| `docs/` | **Skill: validate-docs-code** | 文档-代码一致性 | 版本号一致，接口定义一致 |
| 跨多个模块 | CI 全量 + 相关 Skills 全部触发 | 端到端一致性 | 全部通过 |

### 扩展 Skills

当项目演进需要新的验证维度时，在 `tools/agent-skills/` 下添加新的 skill 目录，包含 `SKILL.md`，并更新上方映射表。每个 skill 必须定义：触发条件、执行步骤、成功标准、失败处理。

---

## 5. 反模式与红线（Anti-patterns）

以下是 AI 辅助开发中绝对不能做的事情。每一条都来自实际踩坑经验。

### 绝对不做

1. **不跳过测试直接声明"完成"**。AI agent 必须在每个验证步骤产出可运行的证据（测试输出、lint 结果），不允许用"代码逻辑正确"之类的判断代替
2. **不在同一个 session 里推进两个不相关的 feature**。上下文污染会导致两个 task 都做不好。一个 task 一个 session
3. **不用 `@latest` 依赖版本**。`@latest` 意味着每次安装可能拿到不同版本，行为不可复现。所有依赖锁定到具体版本号
4. **不在 agent 没有读最新代码的情况下让它修改那段代码**。先读后改是铁律。"先读代码和配置，再修改"（AGENT.md）
5. **不让 AI 修改质量基础设施本身**。CI 配置文件（`.github/workflows/`）、hook 定义（`.husky/`、`commitlint.config.ts`）、测试框架配置的修改必须由人工完成或至少人工 review 后再合并
6. **不在 context 快满的时候继续往里面塞任务**。当你感觉 agent 开始"忘事"或回答质量下降时，开一个新 session，从 git log 恢复上下文
7. **不做不相关的重构**。一个 PR 做一件事。修 bug 的 PR 里不夹带重构，重构的 PR 里不夹带新功能

### 尽量避免

1. **避免让 AI 一次性生成超过 500 行代码**。大段生成的代码很难 review，也容易引入隐蔽的 bug。拆成多次小改动
2. **避免在 AI 给出方案后直接采用而不审视**。AI 的方案经常有逻辑上可行但不是最优的情况。特别是架构决策，一定要自己判断
3. **避免频繁切换 AI 工具**。工具之间没有共享的 session state，切换意味着丢失上下文。完成一个逻辑单元再换

---

## 6. 完整示例：从 0 实现一个新 Feature

以一个具体例子走一遍完整的 6 阶段循环。假设需求是：**实现体态评估报告的导出功能（PDF 格式）**。

### ① Context Loading

```
session 开始：
1. AI 读取 AGENT.md → 了解项目约束和技术栈
2. AI 执行 git log --oneline -10 → 看到最近在 dev 分支上做评估算法的优化
3. AI 执行 git status → 工作区干净
4. AI 读取 PRD 中关于"评估报告"的描述
5. AI 用 CodeGraph 查找现有的 Report 相关代码 → 发现 apps/api/internal/handler/report.go
   和 apps/web/src/pages/Report.tsx 已经存在
```

**结果**：AI 知道当前在 dev 分支，报告页面和 API 已经有基础结构，需要在此之上加 PDF 导出。

### ② Spec & Plan

```
创建 feature 分支：
  git checkout -b feature/report-pdf-export dev

写 spec 到 docs/plan/report-pdf-export.md：
  目标：用户可以在报告页面点击"导出 PDF"按钮，下载当前评估报告的 PDF 文件
  DONE 标准：
    - 前端有导出按钮，点击后显示 loading 状态
    - 后端新增 POST /api/reports/:id/export 接口
    - 返回 Content-Type: application/pdf 的文件
    - PDF 包含评估分数、体态分析、改善建议
    - 单元测试覆盖 PDF 生成逻辑
  影响范围：
    - apps/api: 新增 export handler + PDF 生成 service
    - apps/web: Report 页面新增导出按钮和下载逻辑
  风险点：
    - PDF 库的选择（wkhtmltopdf vs chromedp vs go-wkhtmltopdf）
    - 中文排版和字体嵌入
```

**结果**：一个明确的 spec，DONE 标准是可测试的。

### ③ TDD Implement

```
Step 1: 先用 Context7 查 Go PDF 库的最新文档
  → 选择 maroto v2（纯 Go，支持中文）

Step 2: 写失败的测试
  → apps/api/internal/service/report_export_test.go
  → 测试用例：生成 PDF 返回有效字节，包含评估分数文本

Step 3: 让 AI 实现 service 代码使测试通过
  → apps/api/internal/service/report_export.go

Step 4: 写 handler 层测试
  → apps/api/internal/handler/report_export_test.go

Step 5: 让 AI 实现 handler 并注册路由

Step 6: 前端：先在 Report.tsx 加一个按钮和 loading 状态的测试

Step 7: 让 AI 实现前端导出逻辑（调用 API + 下载文件）
```

**结果**：每一步都是先写测试再实现，代码在测试保护下逐步生长。

### ④ Verify

```
自动验证链：
1. cd apps/api && go vet ./... && go test ./...        → 后端测试全过 ✓
2. pnpm nx run web:lint                                → 前端 lint 通过 ✓
3. pnpm nx run web:typecheck                           → 类型检查通过 ✓

手动验证（Playwright MCP）：
4. docker compose -f docker/docker-compose.yml --profile dev up -d  启动 dev 环境
5. Playwright 导航到报告页面
6. 点击导出按钮
7. 确认 PDF 文件被下载
8. 确认 loading 状态正确显示和消失
9. docker compose -f docker/docker-compose.yml --profile dev down  清理 dev 环境

Skills 验证（本次改动不涉及 docker/RAG/docs，所以 Layer 4 不触发）
```

**结果**：所有验证通过，有具体的测试输出和截图作为证据。

### ⑤ Commit

```
分 3 次 commit：

commit 1: feat(api): 添加评估报告 PDF 导出的 service 层
  - report_export.go + report_export_test.go

commit 2: feat(api): 添加报告导出 API 接口
  - handler + route registration + handler test

commit 3: feat(web): 报告页面添加 PDF 导出按钮
  - Report.tsx 修改 + 下载逻辑
```

**结果**：3 个细粒度 commit，每个都是可独立 review 的逻辑单元，commitlint 全部通过。

### ⑥ Review & Handoff

```
1. 用 GitHub MCP 创建 PR：feature/report-pdf-export → dev
2. PR 描述中附上 Playwright 截图和测试输出
3. 自己 review AI 生成的代码，重点检查：
   - PDF 生成逻辑是否有资源泄漏（字体文件句柄）
   - 导出接口是否有权限校验（只导出自己的报告）
   - 前端是否有 XSS 风险（文件名注入）
4. CI 全绿后合并到 dev
```

**结果**：代码经过人工审视后合并，PR 有完整的 review 记录。

---

## 7. 参考资源

- 项目约束和规则：[`AGENT.md`](../AGENT.md)
- 技术方案（版本选型）：[`technical-approach.md`](./technical-approach.md)
- PRD：[`PRD-体态健康AI助手.md`](./PRD-体态健康AI助手.md)
- Agent Skills 目录：`tools/agent-skills/`
- MCP 配置：`.mcp.json`
- CI 配置：`.github/workflows/ci.yml`
