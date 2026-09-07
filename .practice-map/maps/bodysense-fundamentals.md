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

- `BS-FSO-*`：Full Stack Open 对应训练点；
- `TS-*`：TECH SCHOOL lecture 对应训练点；
- `A*`：BodySense Agent extension。

这些编号不是新的平行项目。Phonebook、Blog List、Simple Bank 不作为长期学习代码库。

# Current Focus

**BodySense Master Course 已建立。下一步进行 placement audit，再从第一个未达到 L4（Verify）的核心训练点继续。**

placement audit 不要求把已经真正掌握的内容重新做一遍。已有证据可以直接认定 mastery；只有“代码是 AI 写的但自己无法解释/预测/验证”的部分需要补做。

当前建议的第一轮诊断顺序：

```text
BS-FSO-0.4/0.6  HTTP/SPA sequence tracing
-> BS-FSO-2.x    server communication
-> TS-06/07/09   transaction / lock / isolation
-> TS-11/16      REST + DB error boundary
-> BS-FSO-4.x    tests + auth
-> BS-FSO-6.x    state ownership
-> A7            streaming / interrupt / replay
```

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

这些项目将在新的 parity 课程里通过 placement exercise 认定对应 mastery level，而不是重复实现。

# Session Log

## 2026-09-07 · 课程体系重建为 source-parity BodySense Master Course

- 退役旧 `docs/learning/00~06` topic tutorials 与第一版 `07-bodysense-open.md`，不再维护本地自创课程和外部课程两套结构。
- 新唯一入口为 `docs/learning/README.md`。
- Full Stack Open 采用 coverage parity：Parts 0~7 建立 `BS-FSO-*` 直接训练序列；Parts 8/9/11/12/13 做直接或比较迁移；React Native/Next.js 明确保留为 optional/compare，不静默跳过。
- TECH SCHOOL Backend Master Class 的 backend lecture #0~#77 已逐条映射到 BodySense，使用 `DIRECT` 或 `COMPARE` 标记，避免为了课程强制引入 sqlc/PASETO/gRPC/Asynq/Kubernetes。
- 新增 BodySense Agent extension，覆盖 typed Agent、runtime ownership、evidence、safety、eval、replay、HITL 与 failure attribution。
- 课程原则从“复制教程项目”改为“source objective -> BodySense prediction/trace/test -> 只有真实缺口才改 production”。
- 旧 Diagnosis 学习成果保留为 prior mastery evidence；下一步先做 placement audit，不机械从零重做已掌握内容。

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
- 当时曾新增 `docs/learning/00-unified-roadmap.md`；该旧路线已于 2026-09-07 随 Master Course 重构退役。
- learning snapshot 完成知识迁移后计划退役；主仓库成为唯一学习与实施工作区。
