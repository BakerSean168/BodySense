# BodySense · 文档导航

**更新：2026-10-13。** 本页说明哪些文档是现行的、每个主题在哪里、哪些已归档以及为什么。

## 1. 先读什么

| 你要了解 | 去这里 | 权威性 |
| --- | --- | --- |
| 业务规则（BodyState / Diagnosis / TrainingPlan / Agent / UI / 验收） | [product/](./product/README.md) | **业务规则唯一来源** |
| 术语与 Penpot 旧名对照 | [product/glossary.md](./product/glossary.md) | 现行 |
| 规格中尚未决定的问题 | [product/open-questions.md](./product/open-questions.md) | 现行 |
| 已决定问题的决策记录 | [product/decisions.md](./product/decisions.md) | 现行 |
| 视觉与交互基准 | [design/](./design/README.md) → Penpot「BodySense · Product Design」Product 页 | 视觉基准 |
| 当前代码的技术架构 | [architecture/](./architecture/README.md) | 技术现状（见 §3 说明） |
| 架构决策 | [adr/](#adr架构决策记录) | 历史决策记录 |
| V2 全面重构总计划与四条并行工作线 | [并行重构 Master Plan](./plan/active/2026-10-13-v2-parallel-master.md) → [四线索引](./plan/active/README.md) | **现行实施统筹** |
| 进行中的其他工程计划 | [plan/active/](./plan/active/README.md) | 现行 |
| 运维手册 | [runbooks/](#runbooks-运维手册)、[security/](#security-安全与数据) | 现行 |

**冲突处理**：业务规则以 `product/` 为准；Penpot 设计稿与旧代码中的 Assistant / Assessment / Profile / Training / Treatment / Consultation 等旧名按 [术语表 §3](./product/glossary.md#3-penpot--旧代码名称对照) 换算，旧 UI 架构（问诊会话、单一 Treatment 及修订、按问诊生成的诊断候选、Assessment 报告）不得覆盖新规格。技术实现细节以代码、契约与迁移为准。

## 2. 目录结构

~~~text
docs/
├── README.md                    本页
├── product/                     ★ 现行产品业务规格（业务规则唯一来源）
│   ├── README.md                总览与文档分工
│   ├── requirements.md          业务规则 R-* / G-* 与场景
│   ├── domain/                  领域属性、状态和不变量 INV-*
│   ├── agent-workflows.md       Agent 流程与工具
│   ├── interface.md             窗口与交互
│   ├── acceptance.md            测试矩阵（引用规则编号）与实施顺序
│   ├── glossary.md              术语表与 Penpot / 旧代码名称对照
│   ├── domain-decisions.md      独立重建的 70 项待决（原 83 项未获得）
│   ├── implementation-gaps.md   目标规格与当前代码差异
│   ├── open-questions.md        Q-* 开放问题
│   └── decisions.md             已决定问题的决策记录
├── design/                      设计说明：当前 Penpot 文件/页面、Foundations
├── architecture/                技术与运行架构（34 篇）
├── adr/                         架构决策记录 0001–0019
├── plan/
│   ├── active/                  进行中的工程计划
│   ├── archive/                 已完成/废弃的计划（既有归档，保持原位）
│   └── *.md                     三份 2026-09 工程方案（codegen spike、Postman、StreamEvent）
├── refactor/vnext/              vNext 工程重置各阶段记录（已完成）
├── runbooks/                    运维与发布手册
├── security/                    备份恢复、隐私删除与保留
├── knowledges/                  开发知识库（Issue 实现笔记、测试指南、工程术语）
├── learning/                    BodySense 学习课程与练习（144 篇）
├── agents/                      AI 协作工具读取的 Issue / 标签 / 领域文档约定
├── implementation/              迁移注释引用的历史重定向（不可删除）
├── zh-CN/                       根 README 的简体中文版
├── ai-development-workflow.md   AI 协作开发流程
├── issue-workflow.md            GitHub Issue 开发流程
├── team-collaboration-plan.md   双人协作开发计划
└── archive/                     本次及以往归档（见 §4）
~~~

## 3. 各主题说明

### product/（现行业务规格）

规格按需求、领域、工作流、界面与验收分工，领域文件在 domain/，规则用编号互相引用，详见 [product/README.md](./product/README.md)。核心对象：BodyState 身体状态（一人一份，明确陈述自动记录）、Diagnosis 综合诊断（一人一份，只含用户确认的病情条目）、TrainingPlan 训练计划（多份完整处方，Agent 建议需用户接受）、Person 个人档案、Trainer 训练执行、Chat。

### design/（视觉基准）

- [design/README.md](./design/README.md)：Penpot 文件 `24d9d841-759d-81bc-8008-b6423250c2b8`，当前 Product 页 `ed4c6ec0-8f91-80cb-8008-c1e744bad64b`；旧页面 ID `536d2a99…` 已失效。
- [design/foundations.md](./design/foundations.md)：视觉语言与交互基础。

### architecture/（技术现状）

[architecture/README.md](./architecture/README.md) 是索引。注意：

- `longitudinal-body-state-domain.md`、`longitudinal-health-loop.md`、`health-journey-workflow.md`、`web-desktop-workbench-ui-ux.md` 已按新规格改写；前三篇的病情确认步骤链接到 product/agent-workflows（[决策 Q-11](./product/decisions.md)）。
- `current-longitudinal-system.md`、`technical-approach.md`、`agent-platform-role-governance.md`、`diagnosis-*`、`treatment-*`、`consultation-eval-harness.md` 等描述**当前代码**（问诊会话、Treatment、Assessment），作为工程事实保留，开头注明描述的是重构前实现；业务规则不以它们为准（[决策 Q-12](./product/decisions.md)）。
- `context-engineering-architecture.md`、`system-engineering-refactor-plan.md` 是指向历史快照的重定向页。
- 部署与交付：`deployment-architecture.md`、`delivery-platform-v3.md`、`release-lifecycle-v3.md`、`model-gateway-routing.md`。

### adr/（架构决策记录）

0001–0019 保持原样（ADR 不改写历史）。其中 ADR 0004（Longitudinal BodyState / Treatment 提案）、0007、0009（Assessment 契约）、0010 使用旧的 Treatment / Assessment 命名，业务语义已由 `product/` 取代；ADR 0019 规定的「产品语义优先」设计流程仍有效。

### runbooks/ 运维手册

Anatomy atlas 发布、Delivery Platform V3 上线、Diagnosis 生产资格与回放回滚、生产容量、PostgreSQL 异地容灾、上传存储、静态资源 CDN、vNext schema 切换、[V2 发布切换（staging/production wipe、canary、rollback、exact-SHA 资格清单）](./runbooks/v2-release-cutover.md)。

### security/ 安全与数据

[offhost-backup-restore-runbook.md](./security/offhost-backup-restore-runbook.md)、[privacy-erasure-retention.md](./security/privacy-erasure-retention.md)。

### 其他工程文档

- [knowledges/](./knowledges/README.md)：Issue 实现笔记、测试指南、工程术语表（`knowledges/glossary.md` 收录技术缩写；**业务术语以 [product/glossary.md](./product/glossary.md) 为准**）。`ai-agent-impl/` 两篇描述仍在代码中的 consultation-runs 链路。
- [learning/](./learning/README.md)：学习课程与练习。
- [refactor/vnext/](./refactor/vnext/baseline/README.md)：vNext 工程重置 Phase 00–11 记录（已完成）。
- [agents/](./agents/domain.md)：AI 工具的 Issue 跟踪、标签与领域文档约定。

## 4. 归档

### 本次归档（2026-10-12）

| 位置 | 内容 | 原因 |
| --- | --- | --- |
| [archive/product-legacy/](./archive/product-legacy/) | `PRD-体态健康AI助手.md`、`feature_spec_3d_body_explorer.md`、`feature_spec_health_document_ingestion.md`、`feature_spec_longitudinal_body_health.md`、`feature_spec_training_schedule.md` | 被 `product/` 取代；Trainer 仍有效的规则已并入 requirements R-EX / interface §5.1，上传资料规则见 requirements R-PS-03（[决策 Q-07、Q-08](./product/decisions.md)） |
| [archive/design/body-canvas-v3/](./archive/design/body-canvas-v3/README.md) | V3 交接 README、`penpot-manifest.json`、`penpot-home.png`、`plan-2026-10-08-body-canvas-v3.md` | 基于旧 Treatment / Assessment 架构，引用已失效的 Penpot 页面 `536d2a99…` |
| [archive/reviews/](./archive/reviews/project-review-2026-07-10.md) | `project-review-2026-07-10.md` | 针对重构前实现的历史审查快照 |
| [archive/deployment/digitalocean/](./archive/deployment/digitalocean/README.md) | `cloud-native-migration-plan.md`、`digitalocean-practice-environment.md`（原 `cloud-native-practice/`） | DigitalOcean 练习环境已于 2026-08-22 退役 |

每个归档文件开头都有一行「已归档（2026-10-12）：被 … 取代」。

### 既有归档

- [plan/archive/](./plan/archive/)：已完成或废弃的工程计划、架构快照、审计（126 篇），按 AGENT.md 约定保持原位。
- [archive/deployment/digitalocean/README.md](./archive/deployment/digitalocean/README.md)：DigitalOcean 部署产物说明。

产品文档定义用户体验和业务规则。实际接口、部署能力和任务完成情况以可验证的代码、测试及环境结果为准。
