---
id: bodysense-fundamentals
title: BodySense Master Course
status: active
level: intermediate
language: go, python, javascript, typescript, react
created_at: 2026-07-13
updated_at: 2026-09-07
---

# Goal

以当前 production-shaped BodySense 主仓库作为唯一长期练习项目，系统覆盖 Full Stack Open 与 TECH SCHOOL Backend Master Class 的知识/训练目标，并补齐 BodySense 特有的 Agent engineering。

课程唯一入口：`docs/learning/README.md`。

课程编号只用于定位来源与练习：

- `FSO-*`：Full Stack Open source point；`BS-FSO-*`：对应 BodySense exercise；
- `TECH-*`：TECH SCHOOL source lecture；`BS-TECH-*`：对应 BodySense exercise；
- `BS-A*`：BodySense Agent extension exercise。

这些编号不是新的平行项目。Phonebook、Blog List、Simple Bank 不作为长期学习代码库。

# Current Focus

**课程重构已进入可机器验收阶段。当前不做正式 placement；先扩大 `EXERCISE_READY` 前置链并继续补当前 FSO MOOC 来源。**

2026-09-07 本轮后，已可核实的状态：

```text
FSO Parts 0-7 numbered exercises: 158/158 MAPPED
FSO Parts 8-11: historical snapshot indexed; current MOOC pending
FSO Parts 12-14: current source UNVERIFIED_CURRENT_MOOC
TECH public lectures #0-#77: 78/78 title-level MAPPED
Agent A1-A8: 8/8 MAPPED
EXERCISE_READY: 27
LEARNER_VERIFIED: 0 in the new mastery ledger
```

第一条可执行前置链：

```text
BS-FSO-0.4 -> BS-FSO-0.6 -> BS-FSO-2.17

BS-TECH-06 -> BS-TECH-07 -> BS-TECH-09
BS-TECH-11 -> BS-TECH-16
BS-TECH-37
BS-TECH-54
BS-A7
```

下一步优先级：

1. 继续把高价值 prerequisite 节点从 `MAPPED` 提升为 `EXERCISE_READY`；
2. 获取并固定 FSO Parts 8-14 当前 MOOC 可复现来源；
3. 继续 semantic concept audit，而不是把 section heading 数量当成知识点完成率；
4. 当某条依赖链已 ready 后，再对该链执行 placement audit。

# Prior Mastery Evidence

Diagnosis Production Agent 学习已经形成可复用的高阶证据，不因课程重构而重置：

- Go handler/service/repository、constructor DI、DIP / composition root；
- Python `Protocol`、Pydantic/PydanticAI typed boundaries；
- LiteLLM logical routing 与 provider boundary；
- immutable Agent Configuration、qualification、non-inferiority；
- EvidenceGap / acquisition / admissibility；
- SafetyEnvelope / deterministic DecisionAuthority；
- durable Diagnosis domain、DecisionTrace、provenance；
- historical / counterfactual replay；
- behavioral contract 与 production failure attribution；
- shadow / canary / promotion governance。

这些项目将在新的 parity 课程里通过 placement exercise 认定对应 mastery level，而不是重复实现。历史证据入口仍保留：

- `docs/plan/archive/2026-08-diagnosis-agent-platform/diagnosis-agent-governance-eval-plan-2026-08-19.md`；
- Diagnosis general qualification：**7/7**；
- EvidenceGap policy suite：**5/5**；
- v1 -> v2 -> v3 paired non-inferiority：零 critical regression。

这些是历史学习/工程证据，不会自动写成新的 `LEARNER_VERIFIED`；placement 时仍需确认它们是否满足对应 exercise 的 L4/L5 acceptance contract。

# Session Log

## 2026-09-07 · Coverage ledger / executable curriculum hardening

- 接受完整性审计结论：上一版是课程蓝图，不能宣称知识/训练点 100% parity。
- 新增 machine-readable ledgers：FSO、TECH SCHOOL、BodySense Agent，明确 `SOURCE_INDEXED -> MAPPED -> EXERCISE_READY -> LEARNER_VERIFIED` 四阶段。
- FSO pinned snapshot Parts 0~11 建立 262 个不同编号记录；Parts 0~7 的 158 个 source exercise 已逐项语义映射，不再使用范围声明代替逐题关系。
- 修正 Part 1 错位：`1.13` 明确对应不可变 vote-like state update，`1.14` 明确对应最大值/最高票派生；调试作为额外能力而非挤占 source id。
- FSO Parts 8~11 明确为 historical snapshot inventory；Parts 12~14 当前 MOOC source 维持 `UNVERIFIED_CURRENT_MOOC`。
- 单独索引 411 个 snapshot section headings，并新增 Promise/Effect、useMemo/React.memo/useCallback、SQL injection、XSS、dependency security、broken auth/access control 等 semantic concept records；明确不把 section count 当作完整语义覆盖。
- TECH SCHOOL 固定 public README commit `97f000fe58ad01a0774179ffa8884ac7784cf263`，78/78 lecture ID/title 映射完成，但 authority 仅为 `VERIFIED_PUBLIC_README_TITLE_ONLY`。
- 建立 27 个首批可执行 exercise cards（包含高价值节点及其 prerequisite closure），覆盖 HTTP/SPA、stale conflict、transaction/lock/isolation、REST/DB error、refresh race、durable jobs、Agent streaming/replay；每个均有 failure case、verification 与 L4 hard gate。
- 新增 curriculum validator 与 generated coverage view；正式 placement 延后到相关 prerequisite slice 达到 `EXERCISE_READY` 之后。

## 2026-09-07 · 课程体系重建为 source-parity BodySense Master Course

- 退役旧 `docs/learning/00~06` topic tutorials 与第一版 `07-bodysense-open.md`，不再维护本地自创课程和外部课程两套结构。
- 新唯一入口为 `docs/learning/README.md`。
- Full Stack Open 采用 coverage parity：Parts 0~7 建立 `BS-FSO-*` 直接训练序列；Parts 8/9/11/12/13 做直接或比较迁移；React Native/Next.js 明确保留为 optional/compare，不静默跳过。
- TECH SCHOOL Backend Master Class 的 backend lecture #0~#77 已逐条映射到 BodySense，使用 `DIRECT` 或 `COMPARE` 标记，避免为了课程强制引入 sqlc/PASETO/gRPC/Asynq/Kubernetes。
- 新增 BodySense Agent extension，覆盖 typed Agent、runtime ownership、evidence、safety、eval、replay、HITL 与 failure attribution。
- 课程原则从“复制教程项目”改为“source objective -> BodySense prediction/trace/test -> 只有真实缺口才改 production”。
- 旧 Diagnosis 学习成果保留为 prior mastery evidence；这是当时的计划记录。当前规则已升级为：先让对应 prerequisite slice 达到 `EXERCISE_READY`，再做 placement，不机械从零重做已掌握内容。

## 2026-08-22 · Diagnosis 学习阶段完成与知识体系收口

- 将历史路线的 Diagnosis 阶段（旧编号 `L1`）从 80% 更新为 **100% 完成**。
- 复核 Diagnosis Agent Platform 归档计划，确认 configuration qualification、EvidenceGap runtime、DecisionAuthority、DecisionTrace/provenance、Replay、Shadow/Canary/Promotion 与统一 LiteLLM routing 已形成完整 production-shaped 闭环。
- 完成 EvidenceGap / Evidence Acquisition 深层 ownership 学习：模型负责 proposal/semantic reasoning，runtime 负责事实记录/验证，policy 负责 authority。
- 完成 Evidence Admissibility 学习：`retrieved != admissible != gap resolved`。
- 完成 Durable Diagnosis Domain Model：immutable Analysis、Candidate vs Hypothesis、Evidence/Gap/Attempt durable boundary、historical truth vs current applicability。
- 完成 SafetyEnvelope / deterministic DecisionAuthority：confidence/Judge 不能覆盖 hard blocker；unknown/malformed facts fail closed。
- 完成 DecisionTrace、Configuration/Execution Provenance、Historical/Counterfactual Replay 与 Behavioral Contract。
- 完成 production failure attribution：从 Input/Context 到 Delivery 查找第一个 contract violation。
- Thought Forest 已按原子知识 + 综合 MOC 重新整理；统一入口为 `BodySense Diagnosis Agent Architecture`。
- 当时下一阶段切换到历史路线的 **Treatment 阶段（旧编号 `L2`）**；由于当前代码已经有 production-shaped Treatment Agent 基础，学习目标改为理解/验证 Treatment 的 domain、safety、proposal/action authority 与 durable outcome ownership，而不是从 legacy migration 重新开始。

## 2026-08-18 · Oracle Two 统一学习工作区

- 确认主仓库与 learning snapshot 均已迁到 Oracle Two。
- 对比发现 main 的 Diagnosis production 实现已经比 snapshot 更成熟；因此不复制 snapshot source code。
- 从 snapshot 仅迁移缺失的 Diagnosis Agent/model 学习与回归测试到 main。
- 在 Oracle Two 安装 CPython 3.13.15，并用 uv 建立 `apps/ai-service/.venv`。
- 补齐开发测试需要的 dev + OCR extras。
- 新增 `apps/ai-service/pyrightconfig.json`，让 Pyright 直接识别项目 `.venv`。
- Diagnosis focused tests 扩展到 14 条并全部通过；Ruff clean；Pyright 0 errors。
- 当时曾新增 `docs/learning/00-unified-roadmap.md`；该旧路线已于 2026-09-07 随 Master Course 重构退役。
- learning snapshot 完成知识迁移后计划退役；主仓库成为唯一学习与实施工作区。
