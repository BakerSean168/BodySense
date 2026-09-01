# BodySense 架构与设计指南 (Architecture & Design Guide)

本目录为 BodySense 项目的核心架构与设计文档中心。项目采用 Monorepo 单仓管理，技术栈涵盖 React 前端、Go 业务网关及 Python AI 算法微服务。

---

## 1. 当前系统架构

描述系统当前的真实架构、技术选型和部署拓扑：

- 📑 **[技术方案大纲 (Technical Approach)](./technical-approach.md)**
  _项目核心技术选型（React 19, Go 1.26, Python 3.13, PostgreSQL 18）、Monorepo 结构、三端接口契约与安全机制。_

- 🐳 **[部署与运维架构 (Deployment Architecture)](./deployment-architecture.md)**
  _Docker Compose 容器编排（Caddy、Go API、FastAPI、Redis 7、PostgreSQL 18）、当前 CI/CD 流水线、阿里云部署拓扑。_

- 🚚 **[Delivery Platform V3](./delivery-platform-v3.md)** ⭐
  _ADR 0008 接受的目标交付架构：单 `main`、PR affected CI、main full CI、稳定 Oracles、exact-SHA candidate、canonical staging，以及 Prepare Release / Release Publish / Deploy Production 三段解耦。_

- 🏷️ **[Release Lifecycle V3](./release-lifecycle-v3.md)**
  _定义版本准备、不可变 Release 发布与生产部署选择之间的权限边界、exact-SHA / digest contract 与 rollback 语义。_

- 🧠 **[上下文工程架构 (Context Engineering Architecture)](./context-engineering-architecture.md)** — _superseded redirect_
  _旧 Go ContextBuilder 全文已移入 archive；当前由 Python LangGraph checkpoint 拥有 Agent Thread 真值。_

---

## 2. 核心业务领域与工程化架构

### 2.1 业务领域 Source of Truth

BodySense 当前业务领域的最高层设计以以下文档为准：

- ⭐ **[Longitudinal BodyState Domain Model](./longitudinal-body-state-domain.md)**  
  _定义一用户一份长期 BodyState、Fact / Observation / Hypothesis / Evidence、时间语义、Diagnosis / Treatment 生命周期及核心 invariant。_
- 🔁 **[Longitudinal Health Loop](./longitudinal-health-loop.md)**  
  _用持续的 BodyState → Diagnosis → Treatment → Outcome → BodyState 闭环替代旧的线性 Health Journey。_
- 📦 **[Longitudinal Body Health Feature Spec](../feature_spec_longitudinal_body_health.md)**  
  _定义用户看到的长期健康工作台、Diagnosis 历史、当前 Treatment 与趋势交互。_
- 🖥️ **[Desktop Workbench UI/UX](./web-desktop-workbench-ui-ux.md)**
  _定义 React 桌面工作台的 chat + workspace 布局、视觉语言、响应式行为与可访问性。_
- 🧍 **[3D Body Explorer Architecture](./body-explorer-3d-anatomy.md)**
  _采用 Vanatome + versioned anatomy atlas 构建 3D 身体空间界面，定义 viewer adapter、BodyState 双向联动、渐进式 anatomy drill-down、性能/回退/自托管边界。_
- 🗺️ **[BodyRegionOntology](./body-region-ontology.md)**
  _BodySense 自有 canonical BodyRegionId 体系；把长期 BodyState 的身体区域语义与 Vanatome anatomyId 解耦并建立版本化 mapping。_
- 📦 **[3D Body Explorer Feature Spec](../feature_spec_3d_body_explorer.md)**
  _定义用户可见的 3D region mode、anatomy drill-down、BodyState/Chat 联动、空状态、WebGL fallback 与验收标准。_
- ⚖️ **[Anatomy Asset Governance](./anatomy-asset-governance.md)**
  _定义 Vanatome/Z-Anatomy atlas 的 license boundary、immutable release、自托管、mapping 兼容性、升级与回滚。_

如果旧文档与上述领域模型冲突，以 ADR 0004 + Longitudinal BodyState Domain Model 为准。

### 2.2 工程化架构设计文档

以下文档定义系统各核心子模块的目标架构：

| 文档                                                                      | 子模块          | 状态                | 说明                                                                     |
| ------------------------------------------------------------------------- | --------------- | ------------------- | ------------------------------------------------------------------------ |
| [System Engineering Refactor Plan](./system-engineering-refactor-plan.md) | 历史总控计划    | Historical redirect | 旧全文已移至 archive，不再承担当前 backlog                               |
| [Context Engineering Architecture](./context-engineering-architecture.md) | 上下文工程      | Superseded redirect | ADR 0002 已删除 Go ContextBuilder 目标                                   |
| [Agent Tool Calling Runtime](./agent-tool-calling-runtime.md)             | 工具调用运行时  | 部分历史            | 概念可参考；执行所有权以 ADR 0002 为准                                   |
| [AI Run / Job Runtime](./ai-run-job-runtime.md)                           | 任务运行时      | Current / partial   | JobRuntime 已承载 OCR + Posture；其它任务按业务需要迁移                  |
| [Stream Event Contract Runtime](./stream-event-contract-runtime.md)       | 流事件契约      | Current / evolving  | Runtime Event Log、StreamEvent contract 与 projection                    |
| [AI Output Governance](./ai-output-governance.md)                         | AI 输出治理     | Current             | role-specific deterministic governance；generic Guard 不是最终 authority |
| [Knowledge Lifecycle](./knowledge-lifecycle-architecture.md)              | 知识生命周期    | Current / evolving  | source registry、review/publication、published retrieval、observations   |
| [Longitudinal Health Loop](./longitudinal-health-loop.md)                 | 长期健康闭环    | Current             | BodyState 为闭环中心                                                     |
| [Model Gateway Routing](./model-gateway-routing.md)                       | 模型路由        | Current             | LiteLLM 为唯一物理 provider/fallback 边界                                |
| [Treatment Agent Configuration](./treatment-agent-configuration.md)       | Treatment Agent | Current             | Immutable config、Go identity gate、durable provenance、eval baseline    |

---

## 3. 架构决策记录 (ADR)

- **[ADR 0001: 围绕流式 AI 工作流深化运行时模块](../adr/0001-deepen-runtime-modules.md)**
  _决定保留三服务架构（React/Go/Python），深化 StreamRuntime、ToolRuntime、JobRuntime，而非引入新 Agent 框架。_

- **[ADR 0002: Python 拥有 Agent Runtime 真值，Go 拥有 Durable Ledger 真值](../adr/0002-agent-runtime-ownership.md)** ⭐
  _Python 通过 LangGraph checkpoint 拥有 Agent Thread 运行时真值；Go 拥有用户业务真值、Runtime Event Log 和 projection；Web 为 projection consumer。_

- **[ADR 0003: StreamEvent Contract Versioning](../adr/0003-stream-event-versioning.md)**
  _跨 Web/Go/Python 的公共 StreamEvent 必须版本化并以契约测试保持一致。_

- **[ADR 0004: Adopt a Longitudinal BodyState as the Core Health Domain Model](../adr/0004-adopt-longitudinal-body-state-model.md)** ⭐
  _一用户一份长期 BodyState；Conversation 只是交互入口；Diagnosis / Treatment pin 明确 BodyState revision；取消 MedicalRecord 作为核心 aggregate，并用 BodyState / Diagnosis / Treatment 历史表达长期健康变化。_

- **[ADR 0005: Adopt a Standalone LiteLLM Model Gateway for Agent Model Execution](../adr/0005-adopt-standalone-litellm-model-gateway.md)** ⭐
  _Diagnosis Agent 通过独立 LiteLLM Gateway 访问逻辑模型组；Gateway 负责 provider 路由/回退/凭据/遥测，Go 继续拥有业务 authority。_

- **[ADR 0006: Adopt Vanatome as the 3D Anatomy Visualization Engine](../adr/0006-adopt-vanatome-3d-anatomy-engine.md)** ⭐
  _采用 Vanatome 作为默认 3D anatomy engine；BodySense 自有 BodyRegionOntology 保持 durable domain 与 third-party anatomyId 解耦；完整 anatomy 能力一次实现，通过 progressive disclosure 控制用户复杂度。_

- **[ADR 0007: Separate stable Profile from longitudinal Lifestyle and Body Metrics](../adr/0007-separate-stable-profile-from-longitudinal-lifestyle.md)** ⭐
  _Profile 仅保留稳定身份；身高体重、生活方式和伤病史进入 BodyState，并通过统一 current-context/review 语义维护历史。_

- **[ADR 0008: Adopt Delivery Platform V3](../adr/0008-adopt-delivery-platform-v3.md)** ⭐
  _融合 MemoFlow 的 delivery control/artifact/observation 思路与 BodySense 的 coherent production watcher：PR 差分、main 全量、build-once/promote-many，以及 Release 与 Deploy 解耦。_

- **[ADR 0009: Adopt an evidence-grounded Assessment contract](../adr/0009-adopt-evidence-grounded-assessment-contract.md)** ⭐
  _Assessment 模型只选择 `kind + evidence_ref`；应用拥有 evidence catalog、确定性渲染与 durable gate；零证据直接跳过模型。_

- **[ADR 0010: Promote qualified Agent baselines before external users](../adr/0010-promote-qualified-agent-baselines-before-external-users.md)** ⭐
  _在无外部用户阶段，对已完成 qualification/non-inferiority 的 Diagnosis v3 与 Treatment v2 做一次显式 owner-approved baseline promotion；最新版成为 Champion，v1 仅保留为 rollback/replay，未来 Challenger 仍必须重新走标准 rollout governance。_

- **[ADR 0011: Adopt explicit OCR report-indicator evidence admissibility](../adr/0011-adopt-ocr-report-indicator-evidence-admissibility.md)** ⭐
  _把 OCR “处理完成”和“可作为健康证据”拆开；未知/中低置信度 fail closed 为 needs_review，只有带版本化 admissibility provenance 的指标才能进入 Assessment v4 evidence catalog。_

- **[ADR 0012: Adopt a pinned Posture geometric perception mechanism](../adr/0012-adopt-pinned-posture-geometry-mechanism.md)** ⭐
  _Posture v2 把 MediaPipe engine、versioned pose model + SHA256 和 canonical threshold hash 纳入 immutable identity；当前 serving 缺失/漂移时 fail closed，Python/Go 双层验证 mechanism provenance。_

---

## 4. 历史方案与设计文档归档 (Archived Plans)

随着项目迭代，阶段性实施方案已归档至此。可追溯各模块的设计原委与演进历程：

### 核心底座与会话重设计

- **通用会话管理统一重构**：[会话管理系统统一重设计方案](../plan/archive/unified-session-redesign.md)
  _解决了并发读-改-写竞态、缺乏幂等控制、会话消息绑定领域的问题。引入通用 conversations / messages / runs / shares 抽象，PostgreSQL 18 原生 uuidv7() 主键。_
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

## Current source of truth

- [Current Longitudinal System](./current-longitudinal-system.md) — authoritative implemented longitudinal architecture
- [Current Technical Approach](./technical-approach.md) — implemented stack, route and persistence snapshot
- [Agent Platform Role Governance](./agent-platform-role-governance.md) — current LLM/mechanism governance classes
- [Documentation / Code Alignment Audit](../plan/active/2026-09-01-documentation-code-alignment-audit.md) — known remaining mismatches; aspirational behavior must stay here until implemented
