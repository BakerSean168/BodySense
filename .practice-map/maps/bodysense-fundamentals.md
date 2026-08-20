---
id: bodysense-fundamentals
title: BodySense 边学边改主线
status: active
level: intermediate
language: go, python, javascript, typescript, react
created_at: 2026-07-13
updated_at: 2026-08-18
---

# Goal

直接在当前 production-shaped BodySense 主仓库中学习并推进真实功能，不再维护长期分叉的 learning snapshot。

稳定课程结构见：`docs/learning/00-unified-roadmap.md`。

本文件只记录**当前 checkpoint**，避免同时维护 M1~M7、P1~P9、DX/DMR 多套学习进度编号。

# Current Checkpoint

**L1 · Diagnosis Typed Agent，约 80% 完成。**

已经实际理解、实施并验证：

- Go handler/service/repository 分层与 constructor DI。
- Dependency Inversion / composition root。
- Python `Protocol` / structural typing。
- typed Diagnosis models 与 `0..N candidates` 业务规则。
- `DiagnosisDependencies` 与 run-scoped context。
- PydanticAI `Agent[Deps, Output]`、`deps_type`、`output_type`。
- `RunContext[DiagnosisDependencies]`。
- `@agent.tool search_evidence`。
- Function Tool 与 structured-output output tool 的区别。
- `capture_run_messages()`、ToolCall/ToolReturn 与 `tool_call_id`。
- `EvidenceSearcher` capability 与模型 Tool 入口的分层。
- run-scoped `retrieved_evidence` trail。
- Python 对象引用语义、`default_factory=list`、dict access、append/extend。
- 按稳定 `evidence_id` 去重与 set comprehension。
- Ruff vs Pyright/Pylance。

# Verified State on Oracle Two

2026-08-18 已在 `/home/ubuntu/projects/bodysense` 重建 AI Service 开发环境：

```text
Python 3.13.15
pytest 9.1.1
ruff 0.16.0
pyright 1.1.411
```

环境由项目内：

```text
apps/ai-service/.venv
```

承载。

Diagnosis 学习/回归测试已从旧 learning snapshot 迁入 main，且保留 main 中更成熟的 production Agent 实现，没有用旧快照覆盖新代码。

当前 focused validation：

```text
14 passed
Ruff: clean
Pyright: 0 errors
```

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

完整统一路线剩余有效学习时间：**24~38 小时**。

如果目标是优先达到“能够独立继续推进 BodySense”的实战水平，先完成：

```text
L1 Diagnosis -> L2 Treatment -> L5 独立纵向交付
```

预计约：**14~22 小时**。

# Session Log

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
