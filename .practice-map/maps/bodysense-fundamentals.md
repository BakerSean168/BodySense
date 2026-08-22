---
id: bodysense-fundamentals
title: BodySense 边学边改主线
status: active
level: intermediate
language: go, python, javascript, typescript, react
created_at: 2026-07-13
updated_at: 2026-08-22
---

# Goal

直接在当前 production-shaped BodySense 主仓库中学习并推进真实功能，不再维护长期分叉的 learning snapshot。

稳定课程结构见：`docs/learning/00-unified-roadmap.md`。

本文件只记录**当前 checkpoint**，避免同时维护 M1~M7、P1~P9、DX/DMR 多套学习进度编号。

# Current Checkpoint

**L1 · Diagnosis Production Agent 已完成（100%）。当前下一阶段：L2 · Treatment Typed Vertical Slice。**

L1 已经不再只是“会用 PydanticAI 写一个 Diagnosis Agent”，而是完成了从模型调用到 production governance / runtime authority 的整条学习链：

```text
Typed Agent
  -> LiteLLM logical routing
  -> Immutable AgentConfiguration
  -> Qualification / Non-Inferiority
  -> EvidenceGap / Acquisition / Admissibility
  -> SafetyEnvelope / DecisionAuthority
  -> Durable Diagnosis Domain
  -> DecisionTrace / Provenance
  -> Historical / Counterfactual Replay
  -> Behavioral Contract / Failure Attribution
  -> Shadow / Canary / Promotion
```

已经实际理解并完成知识沉淀：

- Go handler/service/repository 分层、constructor DI、DIP / composition root。
- Python `Protocol`、Pydantic models/dataclass、PydanticAI `Agent[Deps, Output]` 与 `RunContext`。
- ToolCall/ToolReturn、structured output、run-scoped evidence trail。
- logical model 与 physical provider 分层；LiteLLM 独占 provider/retry/fallback mechanism。
- immutable Agent Configuration 是 qualification/promotion unit。
- Capability Policy：`REQUIRED / PREFERRED / OPTIONAL`。
- Pydantic Evals dataset/slices、deterministic evaluators、paired non-inferiority、interaction effect。
- Decision-Relevant EvidenceGap：user information / external knowledge / conflict / safety。
- Evidence Acquisition Policy、EvidenceBudget、typed EvidenceAttempt 与 stopping reason。
- Evidence Admissibility：`retrieved != admissible != resolved`。
- `LLM = proposer/reasoner`、`Runtime = recorder/verifier`、`Policy = authority`。
- SafetyEnvelope 与 deterministic Go DecisionAuthority；deny-overrides；fail closed。
- immutable `DiagnosisAnalysis`、Candidate vs longitudinal Hypothesis、durable Evidence/Gap/Attempt。
- `Past knowledge != current knowledge`；旧 Analysis 不被未来 BodyState 改写。
- Observability Trace vs DecisionTrace。
- Configuration Provenance vs Execution Provenance。
- Historical Replay vs Counterfactual Replay vs Current Re-analysis。
- Behavioral Contract：hard invariants / bounded semantic variation / presentation variation。
- Production failure attribution：沿链寻找第一个 contract violation，而不是默认怪模型。

统一知识入口已经整理到 Thought Forest：`BodySense Diagnosis Agent Architecture`。

# Verified State on Oracle Two

当前主仓库：`/home/ubuntu/projects/bodysense`。

Diagnosis Agent Platform 的 North-Star 治理计划已完成并归档：

```text
docs/plan/archive/2026-08-diagnosis-agent-platform/
  diagnosis-agent-governance-eval-plan-2026-08-19.md
```

已落地并可从归档计划/代码验证的关键 checkpoint：

- Diagnosis general qualification：**7/7**。
- EvidenceGap policy suite：**5/5**。
- v1 -> v2 -> v3 paired non-inferiority：零 critical regression。
- typed/bounded EvidenceGap acquisition 与 `EvidenceAttempt`。
- deterministic Go `DiagnosisDecisionPolicy` / SafetyEnvelope。
- durable DecisionTrace、configuration/execution provenance、evidence acquisition trace。
- frozen-input Historical Replay 与 side-effect-free Counterfactual Replay。
- Champion -> Shadow -> Canary(5% -> 25% -> 50%) -> Promoted / Rollback governance state machine。
- application-owned provider routing stack repository-wide retired；Diagnosis / Treatment / Assessment / Consultation 等统一进入 LiteLLM logical routing boundary。

2026-08-18 建立的 AI Service 开发环境仍位于：

```text
apps/ai-service/.venv
```

L1 当前完成标准已经从早期的“14 条 focused tests 通过”升级为：**能够解释并验证一个 Diagnosis Configuration 为什么有资格上线、一个具体 case 为什么有/没有资格 AUTO，以及怎样从 DecisionTrace/Replay 定位生产失败。**

# Current Routing Boundary

```text
AIService / Typed Agent
  -> BodySense logical model group
  -> internal LiteLLM gateway
  -> provider / retry / fallback
```

Application code no longer owns `ModelRouter`, physical provider construction, or `FallbackModel`. Diagnosis additionally pins immutable AgentConfiguration and Go DecisionAuthority before normal delivery.

# Protected Contracts

Diagnosis 阶段继续保护：

- Go-owned BodyState 与精确 revision。
- Go-owned durable analysis/candidate IDs。
- Safety gate。
- Diagnosis history persistence。
- HTTP `diagnoses` compatibility。
- governance。
- Consultation LangGraph runtime。
- Treatment 暂不提前迁移。

# Remaining Estimate

从 2026-08-22 checkpoint 计算，L1 Diagnosis 已完成；剩余统一路线预计：**20~32 小时**。

```text
L2 Treatment                 5~8 h
L3 Streaming + React/TS/Go   6~10 h
L4 Async / RAG               4~6 h
L5 Independent Delivery      5~8 h
```

优先达到“能够独立继续推进 BodySense”的最短路径：

```text
L2 Treatment -> L5 独立纵向交付
```

预计约：**10~16 小时**。

下一学习任务不是重新实现 Treatment Agent；当前仓库已经存在 typed Treatment Agent、immutable configurations、EvidenceGap challenger、qualification/evidence/promotion evals。L2 应从**阅读并验证现有 production-shaped Treatment vertical slice**开始，重点学习 proposal/action authority、contraindication/human review、durable Treatment identity 与 outcome ownership。

# Session Log

## 2026-08-22 · Diagnosis 学习阶段完成与知识体系收口

- 将 L1 从旧 checkpoint 的 80% 更新为 **100% 完成**。
- 复核 Diagnosis Agent Platform 归档计划，确认 configuration qualification、EvidenceGap runtime、DecisionAuthority、DecisionTrace/provenance、Replay、Shadow/Canary/Promotion 与统一 LiteLLM routing 已形成完整 production-shaped 闭环。
- 完成 EvidenceGap / Evidence Acquisition 深层 ownership 学习：模型负责 proposal/semantic reasoning，runtime 负责事实记录/验证，policy 负责 authority。
- 完成 Evidence Admissibility 学习：`retrieved != admissible != gap resolved`。
- 完成 Durable Diagnosis Domain Model：immutable Analysis、Candidate vs Hypothesis、Evidence/Gap/Attempt durable boundary、historical truth vs current applicability。
- 完成 SafetyEnvelope / deterministic DecisionAuthority：confidence/Judge 不能覆盖 hard blocker；unknown/malformed facts fail closed。
- 完成 DecisionTrace、Configuration/Execution Provenance、Historical/Counterfactual Replay 与 Behavioral Contract。
- 完成 production failure attribution：从 Input/Context 到 Delivery 查找第一个 contract violation。
- Thought Forest 已按原子知识 + 综合 MOC 重新整理；统一入口为 `BodySense Diagnosis Agent Architecture`。
- 下一阶段切换到 **L2 Treatment Typed Vertical Slice**；由于当前代码已经有 production-shaped Treatment Agent 基础，学习目标改为理解/验证 Treatment 的 domain、safety、proposal/action authority 与 durable outcome ownership，而不是从 legacy migration 重新开始。

## 2026-08-18 · Oracle Two 统一学习工作区

- 确认主仓库与 learning snapshot 均已迁到 Oracle Two。
- 对比发现 main 的 Diagnosis production 实现已经比 snapshot 更成熟；因此不复制 snapshot source code。
- 从 snapshot 仅迁移缺失的 Diagnosis Agent/model 学习与回归测试到 main。
- 在 Oracle Two 安装 CPython 3.13.15，并用 uv 建立 `apps/ai-service/.venv`。
- 补齐开发测试需要的 dev + OCR extras。
- 新增 `apps/ai-service/pyrightconfig.json`，让 Pyright 直接识别项目 `.venv`。
- Diagnosis focused tests 扩展到 14 条并全部通过；Ruff clean；Pyright 0 errors。
- 新增 `docs/learning/00-unified-roadmap.md`，从此只使用 L0~L5 一套学习阶段。
- learning snapshot 完成知识迁移后计划退役；主仓库成为唯一学习与实施工作区。
