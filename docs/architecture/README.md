# BodySense 架构与设计指南 (Architecture & Design Guide)

本目录为 BodySense 项目的核心架构与设计文档中心。项目采用 Monorepo 单仓管理，技术栈涵盖 React 前端、Go 业务网关及 Python AI 算法微服务。

---

## 1. 当前系统架构

描述系统当前的真实架构、技术选型和部署拓扑：

- 📑 **[技术方案大纲 (Technical Approach)](./technical-approach.md)**
  *项目核心技术选型（React 19, Go 1.26, Python 3.13, PostgreSQL 18）、Monorepo 结构、三端接口契约与安全机制。*

- 🐳 **[部署与运维架构 (Deployment Architecture)](./deployment-architecture.md)**
  *Docker Compose 容器编排（Caddy、Go API、FastAPI、Redis 7、PostgreSQL 18）、CI/CD 流水线、阿里云部署拓扑。*

- 🧠 **[上下文工程架构 (Context Engineering Architecture)](./context-engineering-architecture.md)**
  *咨询 Agent 的上下文分层、Go ContextBuilder、Python LangGraph 职责边界、结构化问诊状态。*

---

## 2. 工程化架构设计文档

以下文档定义了系统各核心子模块的目标架构。每个文档顶部的 **Implementation Status** 段标注了当前实现进度。

| 文档 | 子模块 | 实现进度 | 关键已实现 |
|---|---|---|---|
| [System Engineering Refactor Plan](./system-engineering-refactor-plan.md) | 总控计划 | — | 汇总七个子模块的系统级重构方案 |
| [Context Engineering Architecture](./context-engineering-architecture.md) | 上下文工程 | ~50% | ask_user 契约、agent_interactions |
| [Agent Tool Calling Runtime](./agent-tool-calling-runtime.md) | 工具调用运行时 | ~40% | ToolRegistry 骨架、2 个工具已迁移 |
| [AI Run / Job Runtime](./ai-run-job-runtime.md) | 任务运行时 | ~50% | jobs schema、Go JobRuntime、OCR 迁移 |
| [Stream Event Contract Runtime](./stream-event-contract-runtime.md) | 流事件契约 | ~30% | Go StreamRuntime、StreamEvent v1 |
| [AI Output Governance](./ai-output-governance.md) | AI 输出治理 | ~35% | AIOutputGuard 骨架、OutputReviewService |
| [Knowledge Lifecycle](./knowledge-lifecycle-architecture.md) | 知识生命周期 | ~25% | lifecycle schema、publication repo |
| [Health Journey Workflow](./health-journey-workflow.md) | 健康旅程 | 0% | 未开始 |

---

## 3. 架构决策记录 (ADR)

- **[ADR 0001: 围绕流式 AI 工作流深化运行时模块](../adr/0001-deepen-runtime-modules.md)**
  *决定保留三服务架构（React/Go/Python），深化 ContextBuilder、StreamRuntime、ToolRuntime、JobRuntime，而非引入新 Agent 框架。Go 为业务真相源，Python 为 AI 推理源。*

---

## 4. 历史方案与设计文档归档 (Archived Plans)

随着项目迭代，阶段性实施方案已归档至此。可追溯各模块的设计原委与演进历程：

### 核心底座与会话重设计
- **通用会话管理统一重构**：[会话管理系统统一重设计方案](../plan/archive/unified-session-redesign.md)
  *解决了并发读-改-写竞态、缺乏幂等控制、会话消息绑定领域的问题。引入通用 conversations / messages / runs / shares 抽象，PostgreSQL 18 原生 uuidv7() 主键。*
  - 子方案：[Schema 设计](../plan/archive/01-schema-design.md) · [API 设计](../plan/archive/02-api-design.md) · [SSE 流式协议](../plan/archive/03-sse-protocol.md) · [Go-Python 契约](../plan/archive/04-python-contract.md) · [前端适配器](../plan/archive/05-frontend.md) · [实施路线图](../plan/archive/06-implementation-roadmap.md) · [方案总览](../plan/archive/00-overview.md)

- **多 Provider AI 服务网关**：[多供应商 AI 路由设计方案](../plan/archive/multi-provider-ai-router.md)

- **安全认证**：[JWT 认证系统设计](../plan/archive/issue-2-auth-jwt.md)

### 咨询工作台与业务功能链路
- **端到端核心功能合集 (Issues 6-12)**：[核心业务功能实施总案](../plan/archive/issues-6-12-implementation.md)

- **咨询工作台深度优化**：[代码质量修复](../plan/archive/consultation-workbench-code-quality-fixes.md) · [核心分析](../plan/archive/consultation-workbench-ai-agent-core-analysis.md)

### AI 算法与 RAG 知识库
- **自动化视频知识库入库**：[头前移自动知识入库方案](../plan/archive/issue-13-forward-head-knowledge-pilot.md)
  - 子方案：[ASR 基准对比](../plan/archive/asr-base-vs-small-benchmark.md) · [实测统计](../plan/archive/asr-candidates-field-test-results.md) · [入库状态](../plan/archive/issue-13-live-ingestion-status.md)

- **RAG 向量检索底座**：[RAG 知识检索基础设施方案](../plan/archive/issue-3-rag-infra.md)

- **文件上传与 OCR 模块**：[图片/文件上传与 OCR 方案](../plan/archive/issue-5-file-upload-ocr.md)

### 实施过程记录
- **Phase 实施计划**：[implementation/](../plan/archive/implementation/) — 16 个 Phase 的详细设计文档
- **审计修复报告**：[audit/](../plan/archive/audit/) — 4 份修复报告和 1 份修复基线快照
