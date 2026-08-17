# Diagnosis → ConfirmedDiagnoses → MedicalRecord Refactor Plan

> **HISTORICAL / SUPERSEDED — 2026-08-15**  
> Superseded by [ADR 0004](../../adr/0004-adopt-longitudinal-body-state-model.md) and the active Longitudinal BodyState migration plan.  
> The DMR-100 / DMR-101 implementation history in this file remains valuable evidence, but the former `Consultation -> MedicalRecord` north star is no longer the target architecture.  
>
> Original status: Active / Ready for manual implementation  
> Created: 2026-08-14  
> Scope: React + Go + Python Diagnosis vertical slice  
> Original non-goal: 本计划不直接实施 Rehabilitation / Treatment / Training，也不重写 Consultation SSE runtime。

## 1. Executive Decision

本轮重构以以下业务主线为唯一 north star：

```text
Consultation
  → DiagnosisReadiness
  → DiagnosisAnalysis
  → user selects 1..N candidates
  → ConfirmedDiagnoses
  → MedicalRecord
```

核心决策：

1. **先修 Diagnosis 领域边界，再大拆 Consultation runtime。**
2. **Go 继续拥有 durable business truth。** Python 负责 Agent reasoning / structured generation，React 负责 presentation / interaction。
3. **PydanticAI 先替换 Diagnosis 的裸 LLM orchestration，不立即替换 LangGraph。**
4. **DiagnosisAnalysis 与 ConfirmedDiagnoses 必须是两个独立对象。** Confirmation 不再覆盖 Analysis。
5. **客户端 confirmation 只提交 `analysisId + selectedCandidateIds`，不提交可信业务内容。**
6. **当前 MVP 的终点是 MedicalRecord，不是 TreatmentPlan。**
7. **现有 Consultation SSE v1、runtime event durability、thread projection、interrupt/resume 均为 protected contracts。**

---

## 2. Verified Current System

以下内容来自当前仓库代码，不是目标设计。

### 2.1 Python Diagnosis

当前路径：

```text
POST /api/diagnosis/analyze
  → DiagnosisService.generate_diagnosis
  → AIService.generate
  → JSON text
  → json.loads
  → Pydantic validation
  → red flags / citations / governance
```

当前 `apps/ai-service/pyproject.toml` 尚未依赖 PydanticAI。

已有保护测试：

- `apps/ai-service/tests/unit/test_diagnosis_api_contract.py`
- `apps/ai-service/tests/unit/test_diagnosis_service.py`

这些测试已经明确把 HTTP request / response 视为 migration characterization contract。

### 2.2 Go Diagnosis

当前：

- `POST /api/v1/consultations/:id/diagnosis` 不强制 `ready_for_analysis`；
- accepted/degraded result 写入 `consultation_sessions.diagnosis`；
- phase 推进到 `analysis_ready`；
- governance rejected 时不会写 diagnosis / 推进 phase。

相关文件：

- `apps/api/internal/handler/diagnosis_handler.go`
- `apps/api/internal/service/ai_client.go`
- `apps/api/internal/service/consultation_service.go`
- `apps/api/internal/service/consultation_phase.go`

### 2.3 Confirmation

当前：

```text
PUT /api/v1/consultations/:id/confirm
body: { diagnosis: <arbitrary JSON> }
```

Handler 直接执行：

```text
UpdateDiagnosis(req.Diagnosis)
UpdatePhase("diagnosis_confirmed")
```

当前没有验证：

- phase 是否为 `analysis_ready`；
- diagnosis 是否来自刚生成的 candidates；
- candidate identity；
- 选择数量；
- analysis 是否 stale。

这属于本计划的首要业务完整性问题。

### 2.4 React

当前 `DiagnosisPanel`：

```text
selectedDiagnosis: Diagnosis | null
```

是明确单选。

当前 ConsultationPage 将 confirmation 与 treatment generation 绑定为一个 mutation：

```text
confirmDiagnosis(single diagnosis)
  → generateTreatment(single diagnosis)
```

同时 Diagnosis analyze 的任何 `onSuccess` 都会本地设置：

```text
phase = analysis_ready
```

因此 governance rejected 可能产生短暂客户端状态分叉。

### 2.5 Persistence / Projection

当前 `consultation_sessions.diagnosis` 同时承担：

```text
DiagnosisAnalysis
OR
ConfirmedDiagnosis
```

confirmation 后会覆盖原始 analysis。

Thread projection 再把 session 的 `diagnosis` / `treatment_plan` 直接投影给 Web。

### 2.6 Protected Consultation Runtime

当前已经稳定且本计划不应破坏：

```text
POST /api/v1/consultation-runs
request_id idempotency
Conversation / Run / Turn / Message identity
StreamEvent v1
runtime_events durability + replay
thread projection
ask_user pending interaction
interrupt / resume
LangGraph checkpoint + thread_id
```

---

## 3. Gaps and Root Causes

| ID | Observed behavior | Desired behavior | Root cause | Priority |
|---|---|---|---|---|
| G-01 | Confirm 接受任意 diagnosis JSON | 仅允许选择当前 Analysis 的 candidate IDs | 缺少独立 Analysis / Candidate identity | P0 |
| G-02 | Confirm 覆盖 DiagnosisAnalysis | Analysis 与 Confirmation 独立持久化 | `diagnosis` 字段多重语义 | P0 |
| G-03 | Diagnosis API 不检查 readiness phase | server authoritative readiness gate | readiness ownership 分散 | P1 |
| G-04 | Web 对 rejected 仍可本地推进 phase | rejected 保持 durable phase | response 未建模为 accepted/rejected union | P1 |
| G-05 | React/Go/Python 全部单选 | 1..N multi-select | singular contract 深入三端 | P1 |
| G-06 | MVP 最终 artifact 是 treatment_plan | MedicalRecord 为最终 artifact | 历史 Rehab spec 驱动旧闭环 | P1 |
| G-07 | `ConsultationPhaseRank` 只允许单调前进 | 允许显式有效 transition / analysis invalidation | rank 不是 state machine | P1 |
| G-08 | Python 手工 orchestration / parsing | typed DiagnosisAgent | Agent / application concerns 混合 | P2 |

---

## 4. Target Ownership Boundaries

### 4.1 React

负责：

- 展示 readiness；
- 触发 analysis；
- 展示 accepted/rejected 状态；
- 多选 candidates；
- 提交 `analysisId + selectedCandidateIds`；
- 展示 MedicalRecord read model；
- 以 Go projection / API 为最终 durable state。

不负责：

- 判断 candidate 是否可信；
- 自己决定 phase truth；
- 构造 ConfirmedDiagnoses 业务对象；
- 重新推导 MedicalRecord。

### 4.2 Go

负责：

- ownership；
- Diagnosis readiness 的 authoritative policy + application enforcement；
- 调用 Python Diagnosis service；
- candidate / analysis durable identity；
- governance persistence；
- DiagnosisAnalysis persistence；
- confirmation validation；
- MedicalRecord transaction；
- phase transition；
- thread projection / journey derivation。

### 4.3 Python

负责：

- `DiagnosisContext` / typed dependencies；
- PydanticAI DiagnosisAgent；
- structured Diagnosis output；
- read-only Agent tools；
- red-flag / response governance integration；
- 保持 Go↔Python HTTP adapter contract 的明确边界。

### 4.4 LangGraph

本轮继续负责 Consultation workflow / HITL，不负责 MedicalRecord durable transaction。

Diagnosis PydanticAI migration 不等于“把 LangGraph 换掉”。

---

## 5. Target Contracts

### 5.1 Python Internal Models

建议新增明确类型：

```text
DiagnosisDependencies
DiagnosisAgentOutput
DiagnosisCandidateDraft
DiagnosisGovernanceResult
```

注意：`candidate_id` 建议由 Go / application layer 分配，因此 Agent output 可以是 CandidateDraft。

### 5.2 Public DiagnosisAnalysis

```text
DiagnosisAnalysis
├─ analysis_id
├─ candidates[]
│   ├─ candidate_id
│   ├─ name
│   ├─ confidence
│   ├─ severity
│   ├─ basis
│   ├─ typical_symptoms?
│   └─ differential?
├─ citations[]
├─ red_flags?
├─ governance
└─ created_at
```

### 5.3 Confirmation Request

```json
{
  "analysisId": "uuid",
  "selectedCandidateIds": ["uuid-a", "uuid-c"]
}
```

### 5.4 MedicalRecord

```text
MedicalRecord
├─ record_id
├─ user_id
├─ consultation_id
├─ diagnosis_analysis_id
├─ consultation_snapshot
├─ diagnosis_analysis
├─ confirmed_diagnoses
├─ safety_snapshot
├─ citations
└─ created_at
```

---

## 6. Protected Contract Matrix

| Contract | Action |
|---|---|
| `POST /api/v1/consultation-runs` | Preserve |
| StreamEvent v1 envelope | Preserve |
| Runtime event replay | Preserve |
| Thread projection mechanism | Preserve, extend fields later |
| interrupt / resume | Preserve |
| Go durable business ownership | Preserve |
| Python `/api/diagnosis/analyze` first migration step | Preserve |
| governance accepted/degraded/rejected | Preserve |
| `PUT /confirm` request body | Migrate |
| `ConsultationSession.diagnosis` overloaded semantics | Retire |
| single `confirmedDiagnosis` | Retire |
| Treatment generation in ConsultationPage | Retire from current MVP |
| phase rank-only transition | Migrate |

---

# Phase 0 — Baseline / Characterization

## Objective

先让错误边界可测试，再动 production behavior。

## Why First

当前 P0 问题就是缺少 server-side invariant。若直接换 PydanticAI，系统会拥有更好的模型层，但仍然允许伪造 confirmation。

## DMR-001 — Freeze current Diagnosis Python HTTP contract

**Goal:** 在替换 `DiagnosisService` 内部实现前，确认现有 FastAPI adapter request / accepted / rejected / 422 行为被测试锁住。

**Scope:**

- `apps/ai-service/tests/unit/test_diagnosis_api_contract.py`
- `apps/ai-service/tests/unit/test_diagnosis_service.py`

**Implementation:**

1. 保留现有 request passthrough characterization。
2. 保留 invalid structured output → 422。
3. 保留 governance rejected → 200 + `safety_fallback`。
4. 增加 request defaults 与 malformed JSON 的 characterization tests，锁定 adapter/service 错误边界。
5. `confidence / severity` 当前只是非空字符串，严格 enum 校验不是现有行为；其 failing test 与 `StrEnum` 实现一起放到 DMR-101，避免 Phase 0 把“目标行为”误写成“当前 contract”。

**Tests:**

```bash
pnpm nx test ai-service -- --tests/unit/test_diagnosis_api_contract.py
pnpm nx test ai-service -- --tests/unit/test_diagnosis_service.py
```

如果 Nx 参数透传与本地版本不兼容，直接在 `apps/ai-service`：

```bash
uv run pytest tests/unit/test_diagnosis_api_contract.py tests/unit/test_diagnosis_service.py
```

**Acceptance:** 旧 HTTP contract 在 Agent replacement 前完全可重复验证。

---

## DMR-002 — Characterize Go readiness / rejection behavior

**Goal:** 为 Diagnosis handler 增加 server application boundary tests。

**Scope:**

- `apps/api/internal/handler/diagnosis_handler.go`
- 新增/扩展对应 handler tests

**Tests to add first:**

1. session 不存在 → 404。
2. phase 不是 `ready_for_analysis` → 409 `INVALID_PHASE`（目标测试；Phase 0 先以 `t.Skip` 固化 contract，DMR-202 激活并转绿）。
3. governance rejected → 不调用 UpdateDiagnosis / UpdatePhase。
4. accepted → persist + phase transition。

**Implementation note (2026-08-14):** 已新增 `apps/api/internal/handler/diagnosis_handler_http_test.go`。404 / governance rejected / accepted persistence 使用当前行为 characterization；`INVALID_PHASE` 目标契约已写成可执行测试，但暂以 `t.Skip` 标记，DMR-202 实现 authoritative readiness gate 时删除该 `t.Skip` 并直接以现有断言验收。这样 Phase 0 保持可重复绿色 baseline，同时不丢失目标行为。

**Acceptance:** readiness 和 rejected invariants 有明确测试；当前行为 characterization 可独立运行，readiness target contract 已显式记录并等待 DMR-202 激活。

---

## DMR-003 — Characterize insecure confirmation before replacing it

**Goal:** 把当前 confirmation 缺口变成明确的 failing target tests。

**Scope:**

- `apps/api/internal/handler/consultation_handler.go`
- consultation handler / service tests

**Target tests:**

- 非 `analysis_ready` confirm → 409；
- 空 selection → 400/422；
- unknown `analysisId` → 409/404；
- unknown candidate ID → 400/409；
- 1 个 candidate → success；
- 多个 candidates → success。

Phase 0 可以先建立测试 skeleton / test cases，production change 在后续 ticket 完成。

---

# Phase 1 — Python Foundation: Dependency Injection → Typed DiagnosisAgent

## Objective

先把 `DiagnosisService` 的依赖装配方式从“类内部写死具体实现”改成显式依赖注入，再替换 Python Diagnosis 的“裸 LLM + JSON parsing”内部实现，同时保持 Go-facing HTTP contract 不变。

这一阶段的学习顺序刻意调整为：

```text
现有 characterization tests
  → constructor injection / dependency inversion
  → composition root
  → typed diagnosis domain models
  → PydanticAI DiagnosisAgent adapter
  → PydanticAI RunContext dependencies / tools
```

这里要区分两种容易混淆的“依赖”：

1. **应用层 Dependency Injection**：`DiagnosisService` 不应该自己 `AIService()` 或自己创建 PydanticAI Agent，而应通过 constructor 接收它所依赖的执行端口；具体实现只在 composition root 组装。
2. **PydanticAI dependencies**：`deps_type` / `RunContext[...]` 是 Agent 运行时向 tools / instructions 提供 typed context 的机制，它不能替代应用层 DI。

先学前者，再学后者。这样 PydanticAI 不会被误用成新的全局单例或新的硬编码依赖。

PydanticAI 当前官方 Agent abstraction 原生支持：

- typed dependencies；
- function tools；
- structured output type；
- model / settings；
- run / stream APIs。

因此这一阶段把 PydanticAI 作为 **Diagnosis execution layer**，而不是 workflow/database framework。

## DMR-100 — Introduce an explicit dependency-injection seam in DiagnosisService

**Goal:** 在引入 PydanticAI 之前，先让 `DiagnosisService` 不再在 `__init__()` 内部写死 `AIService()`，并把“创建具体实现”移到 composition root。

**Why first:**

当前代码：

```python
class DiagnosisService:
    def __init__(self) -> None:
        self._ai = AIService()
```

这导致 service 同时负责“业务编排”和“依赖创建”。现有单元测试只能通过 monkeypatch `src.services.diagnosis_service.AIService` 构造器来替换模型层。若直接迁移到 PydanticAI，只会把硬编码从 `AIService()` 换成另一个具体 Agent。

**Learning targets:**

- constructor injection；
- dependency inversion；
- composition root；
- fake / test double；
- “依赖的使用者不负责创建依赖”；
- interface / `Protocol` 何时值得引入，何时可以先依赖最小 concrete contract。

**Scope:**

- `apps/ai-service/src/services/diagnosis_service.py`
- `apps/ai-service/tests/unit/test_diagnosis_service.py`
- 如需要，新增一个很小的 Diagnosis execution port / `Protocol`
- `get_diagnosis_service()` 继续作为当前 composition root / provider function

**Target shape (first safe step):**

```python
class DiagnosisService:
    def __init__(self, ai):
        self._ai = ai


def get_diagnosis_service() -> DiagnosisService:
    return DiagnosisService(ai=AIService())
```

测试改为显式：

```python
service = DiagnosisService(ai=_FakeAIService(...))
```

而不是：

```python
monkeypatch.setattr("src.services.diagnosis_service.AIService", ...)
```

如果在 DMR-102 前已经看出稳定的最小调用契约，再把 `ai` 收窄为 `Protocol`；不要为了“用了 DI”提前制造大型 abstraction。

**Protected contracts:**

- `/api/diagnosis/analyze` HTTP request / response 不变；
- `DiagnosisService.generate_diagnosis(...)` caller-facing signature 暂不变；
- malformed output / schema validation / governance 语义不变。

**Tests:**

- 现有 `test_diagnosis_service.py` 不再需要 patch constructor；
- fake 显式通过 constructor 注入；
- HTTP characterization tests 保持不变。

**Acceptance:**

- `DiagnosisService.__init__` 不再自行创建 `AIService`；
- concrete `AIService()` 只在 composition root/provider 处组装；
- service tests 可以零 monkeypatch 地注入 Fake execution dependency；
- `DiagnosisService` 的类型依赖收窄为 consumer-owned 最小 `AIExecutor(Protocol)`；
- Fake 不再继承 concrete `AIService`，仅凭兼容的 `generate()` 方法结构满足 Protocol；
- Phase 0 characterization expectations 不改变。

**Implementation note (2026-08-15):** DMR-100 已进一步完成 Dependency Inversion 学习步骤。`diagnosis_service.py` 从真实使用面提取了只包含 `async generate(AiRequest) -> AiResponse` 的 `AIExecutor(Protocol)`；`DiagnosisService.__init__` 依赖该 Protocol，而 composition root 仍注入真实 `AIService()`。单元测试的 `_FakeAIExecutor` 已移除 `AIService` 继承，用 structural typing 满足同一能力契约。此处刻意不把 `generate_stream`、provider/router/config 等未被 DiagnosisService 使用的能力放入 Protocol。

---

## DMR-101 — Add PydanticAI dependency and typed Diagnosis domain models

**Goal:** Python Diagnosis 不再依赖无约束 `dict[str, Any]` 表意。

**Scope:**

- `apps/ai-service/pyproject.toml`
- 新增 `apps/ai-service/src/models/diagnosis.py`
- 新增 `apps/ai-service/src/services/diagnosis_agent.py`
- 保留 `apps/ai-service/src/services/diagnosis_service.py` 作为 application/service adapter。

**Required models:**

```text
DiagnosisConfidence(StrEnum): 高 / 中 / 低
DiagnosisSeverity(StrEnum): 轻度 / 中度 / 重度
DiagnosisCandidateDraft(BaseModel)
DiagnosisAgentOutput(BaseModel)
DiagnosisDependencies(dataclass or BaseModel)
```

**Constraints:**

- CandidateDraft 不承担 durable `candidate_id`；
- `DiagnosisAgentOutput` 不包含 Go database identity；
- governance 不混进 Agent core output，除非现有 reviewer integration 明确需要。

**Tests:** typed model validation tests。

**Acceptance:** 非法 confidence / severity 在 Python typed boundary 失败。

**Implementation note (2026-08-15, first sub-step):** 已新增 `src/models/diagnosis.py`，建立 `DiagnosisConfidence(StrEnum)`、`DiagnosisSeverity(StrEnum)`、`DiagnosisCandidateDraft(BaseModel)` 与 `DiagnosisAgentOutput(BaseModel)`。内部 Agent output 使用 `candidates` 领域词汇，并把 prompt 中的 1-3 candidates 规则编码为 Pydantic list length constraint；对应 `tests/unit/test_diagnosis_models.py` 覆盖合法字符串→enum、非法 confidence/severity、空必填文本、0/4 candidates 拒绝。

**Implementation note (2026-08-15, second sub-step):** 已新增 `DiagnosisDependencies(dataclass, slots=True)`，把一次 Diagnosis reasoning run 当前真正需要的 `extracted_info / profile / conversation_summary / rag_context` 聚合成 request/run-scoped typed context。这里刻意与 DMR-100 的 constructor DI 区分：`AIExecutor` 是 composition root 注入、可跨多次请求复用的 execution capability；`DiagnosisDependencies` 是每一次 run 都重新组装的数据上下文。当前没有把 `use_case` 放入 dependencies，因为它属于 model-routing / execution adapter policy；也没有把 `rag_results` 放入，因为现有代码只在 generation 后用于 citations/governance。nested `dict` 暂不进一步建模，避免同一小步扩大到 Consultation/Profile domain。对应 model tests 覆盖 run context 聚合、默认值与 `slots=True` 拒绝任意附加字段。此时 DMR-101 仍**尚未**安装 PydanticAI、尚未创建 `diagnosis_agent.py`，也尚未把新模型接入 `DiagnosisService`。

---

## DMR-102 — Implement PydanticAI DiagnosisAgent behind existing service interface

**Goal:** `DiagnosisService.generate_diagnosis(...)` 对 caller 保持兼容，但内部改为 PydanticAI Agent。

**Scope:**

- `apps/ai-service/src/services/diagnosis_service.py`
- `apps/ai-service/src/services/diagnosis_agent.py`
- `apps/ai-service/src/models/diagnosis.py`
- `apps/ai-service/src/prompts/diagnosis.py`
- existing model/provider adapter only where PydanticAI integration requires it

**Target shape:**

```text
composition root
  → construct PydanticAI DiagnosisAgent adapter
  → inject into DiagnosisService

DiagnosisService
  → build DiagnosisDependencies
  → injected DiagnosisAgentRunner.run(..., deps=...)
  → result.output: DiagnosisAgentOutput
  → existing red flag / citation / governance adapter
  → existing HTTP payload
```

`DiagnosisService` 不直接 import/instantiate 一个全局 `diagnosis_agent` 作为隐藏依赖。PydanticAI 是 execution adapter；application service 只依赖它需要的最小执行契约。

**Protected contracts:**

- `/api/diagnosis/analyze` request shape；
- accepted response shape（第一阶段仍可保持 `diagnoses` key）；
- rejected governance shape；
- 422 invalid output behavior。

**Do not yet:**

- 改 Go API；
- 改 React；
- 改 candidate identity；
- 删除 LangGraph；
- 迁移 all consultation tools。

**Acceptance:** characterization tests 不改 expected payload 仍通过。

---

## DMR-103 — Move Diagnosis read-only tools to PydanticAI incrementally

**Goal:** Diagnosis-specific tool orchestration 不继续复制 `ToolRegistry/ToolExecutor`。

**First candidates:**

1. knowledge lookup；
2. posture-analysis read；
3. 其他纯查询 context tool。

**Rules:**

- Tool 通过 typed `RunContext[DiagnosisDependencies]` 获取依赖；
- 不允许 Diagnosis Agent tool 直接写 Go durable business tables；
- `ask_user` 不在本 ticket 迁移，它属于 LangGraph HITL；
- `extract_symptom_info` 属于 Consultation collection，不是 Diagnosis confirmation tool。

**Acceptance:** Diagnosis Agent 使用 PydanticAI tools；现有 Consultation ToolRegistry 仍可保留给 consultation runtime，直到单独重构。

---

# Phase 2 — Go Diagnosis Domain Boundary

## Objective

让 Go 拥有正式的 DiagnosisAnalysis identity、readiness gate 和 persistence，而不再把 analysis 当成任意 session JSON。

## DMR-201 — Replace rank-only phase policy with explicit transition policy

**Goal:** 状态机能表达真实业务 transition，包括 analysis invalidation。

**Scope:**

- `apps/api/internal/service/consultation_phase.go`
- `apps/api/internal/service/consultation_service_test.go`

**Target transitions:**

```text
collecting -> ready_for_analysis
ready_for_analysis -> collecting
ready_for_analysis -> analysis_ready
analysis_ready -> ready_for_analysis
analysis_ready -> record_ready
record_ready -> completed (optional)
```

**Important:**

不要允许任意 `ready_for_analysis -> record_ready` 跳跃。

**Acceptance:** 每个允许/拒绝 transition 都有 table-driven tests。

---

## DMR-202 — Enforce readiness at Diagnosis HTTP boundary

**Goal:** 用户不能绕过 Consultation readiness 直接生成 Analysis。

**Scope:**

- `apps/api/internal/handler/diagnosis_handler.go`

**Implementation:**

1. 新增 `apps/api/internal/service/diagnosis_readiness.go`，建立 `DiagnosisReadinessPolicy`，输入来自 durable `ConsultationSession.ExtractedInfo` / `HealthFeatures` 等已持久化状态。
2. `AnalyzeDiagnosis` 每次调用都重新执行该 policy；不要只信任 Python 先前发出的 phase event。
3. readiness=false 时返回 409 `INVALID_PHASE`（或后续统一为更精确的 `DIAGNOSIS_NOT_READY`）。
4. readiness=true 时才调用 Python Diagnosis service。
5. 仅 governance accepted/degraded 才允许 persistence 和 `analysis_ready`。
6. rejected 保持原 durable phase。
7. Python 的 `should_analyze` / phase signal 在完成 Consultation workflow cleanup 前只能作为 advisory，不再是最终业务 gate。

**Acceptance:** DMR-002 tests 全绿。

---

## DMR-203 — Add `diagnosis_analyses` persistence

**Goal:** DiagnosisAnalysis 成为独立 durable aggregate。

**Scope:**

- next sequential migration under `apps/api/migrations/`
- new `apps/api/internal/model/diagnosis_analysis.go`
- new `apps/api/internal/repository/diagnosis_repository.go`
- new `apps/api/internal/service/diagnosis_service.go`
- `consultation_sessions.active_diagnosis_analysis_id`

`DiagnosisService` 负责业务编排/校验；`DiagnosisRepository` 负责 diagnosis domain 的数据库读写和跨表事务。沿用当前 `ConsultationRepository.CreateRunEnvelope` 的 transaction ownership 风格，不在多个 service 之间拼事务。

**Suggested schema:**

```text
diagnosis_analyses
id UUID PK
consultation_id UUID FK
candidates JSONB NOT NULL
citations JSONB NOT NULL DEFAULT []
red_flags JSONB
governance JSONB NOT NULL
context_metadata JSONB NOT NULL DEFAULT {}
created_at TIMESTAMPTZ
```

**Candidate identity:**

- Go 在收到 Python Agent draft 后为每个 candidate 分配 stable UUID / ULID；
- 持久化后的 candidate snapshot 不修改。

**MVP migration:**

不要求迁移旧 `consultation_sessions.diagnosis` 数据。

**Acceptance:** accepted Analysis 有 durable `analysis_id` + candidate IDs；session 指向 active analysis。

---

## DMR-204 — Migrate Go↔Python response adapter to public DiagnosisAnalysis

**Goal:** 在 Python Agent 已稳定后，把 Go-facing payload 从 legacy `diagnoses` 升级为正式 Analysis contract。

**Scope:**

- Python Diagnosis route/service response adapter
- `apps/api/internal/service/ai_client.go`
- `apps/api/internal/handler/diagnosis_handler.go`

**Implementation order:**

1. Python internal output 已 typed；
2. Go 接收 legacy result；
3. Go application layer 分配 IDs 并构造 public Analysis；
4. 最后如需要再把 Python API 自身也改为 `candidates` naming。

**Acceptance:** Web 最终读取的是 Go-persisted Analysis，不依赖 Python 生成 durable IDs。

---

# Phase 3 — Confirmation + MedicalRecord Vertical Slice

## Objective

完成本轮最重要的 end-to-end vertical slice：

```text
analysis_ready
→ select 1..N
→ server validation
→ ConfirmedDiagnoses
→ MedicalRecord
→ record_ready
```

## DMR-301 — Migrate ConfirmDiagnosisRequest

**Goal:** 删除 arbitrary diagnosis JSON trust boundary。

**Scope:**

- `apps/api/internal/dto/consultation.go`
- `apps/api/internal/handler/consultation_handler.go`
- service layer

**Replace:**

```text
Diagnosis json.RawMessage
```

with:

```text
AnalysisID UUID/string
SelectedCandidateIDs []UUID/string
```

**Validation:**

- phase == `analysis_ready`；
- analysis 是 session active analysis；
- selected count >= 1；
- IDs 无重复；
- every selected ID exists in analysis candidates。

**Acceptance:** DMR-003 target tests 全绿。

---

## DMR-302 — Add `medical_records` aggregate and atomic confirmation transaction

**Goal:** confirmation 不再只是改 phase，而是生成 durable business artifact。

**Scope:**

- next sequential migration under `apps/api/migrations/`
- new `apps/api/internal/model/medical_record.go`
- extend `apps/api/internal/repository/diagnosis_repository.go`
- extend `apps/api/internal/service/diagnosis_service.go`

`DiagnosisService` 负责 selection validation 与 MedicalRecord snapshot 构造；`DiagnosisRepository` 提供一个原子 transaction 方法，负责插入 `medical_records` 并推进 session phase。

**Transaction:**

```text
load session
load active analysis
validate selection
build ConfirmedDiagnoses snapshot
build MedicalRecord snapshot
insert medical_record (consultation_id UNIQUE in MVP)
update session phase = record_ready
commit
```

**Invariant:**

不得出现：

```text
phase = record_ready
AND no MedicalRecord
```

也不得出现：

```text
confirmed selection persisted
BUT record insert failed
```

**Acceptance:** transaction tests 覆盖 rollback。

---

## DMR-303 — Extend consultation thread projection with active analysis / medical record

**Goal:** React reload 后从 durable projection 恢复正确 Diagnosis state。

**Scope:**

- `apps/api/internal/model/thread_projection.go`
- `apps/api/internal/service/thread_projection_service.go`
- `apps/api/internal/repository/thread_projection_repository.go`
- `apps/api/internal/handler/thread_projection_handler.go`
- corresponding migration

**Target projection fields:**

```text
diagnosis_readiness
diagnosis_analysis
medical_record
```

旧：

```text
diagnosis
treatment_plan
```

在切换完成后 retire。

**Acceptance:** reload 后 multi-select confirmation result / MedicalRecord 不依赖 client cache。

---

# Phase 4 — React Migration

## Objective

UI 与新的 durable contract 对齐，不再把单选/treatment 逻辑留在 ConsultationPage。

## DMR-401 — Replace single-select DiagnosisPanel with multi-select candidates

**Scope:**

- `apps/web/src/features/consultation/components/DiagnosisPanel.tsx`
- `apps/web/src/features/consultation/components/__tests__/DiagnosisPanel.test.tsx`
- consultation types

**Replace:**

```text
selectedDiagnosis: Diagnosis | null
```

with:

```text
selectedCandidateIds: Set<string>
```

**UI behavior:**

- 0 selected → confirm disabled；
- 1 selected → enabled；
- N selected → enabled；
- candidate card 明确选中状态；
- button: `确认所选判断并生成病历单`。

**Remove from this component:** TreatmentPlan rendering responsibility。

**Acceptance:** tests cover empty / single / multiple selections。

---

## DMR-402 — Model Diagnosis response as accepted/rejected union

**Goal:** 不再把所有 2xx 当作 Analysis ready。

**Scope:**

- `apps/web/src/features/consultation/types/consultation.ts`
- `apps/web/src/features/consultation/services/consultationService.ts`
- `ConsultationPage.tsx`

**Target:**

```text
DiagnosisAnalyzeResponse =
  | DiagnosisAnalysisAccepted
  | DiagnosisAnalysisRejected
```

**Behavior:**

accepted/degraded:

- set/refresh analysis；
- phase -> analysis_ready only from server result/projection。

rejected:

- render safety fallback；
- no local phase promotion；
- invalidate/refetch durable thread。

**Acceptance:** rejected UI regression test。

---

## DMR-403 — Split confirmation from old treatment mutation

**Goal:** ConsultationPage 只完成 confirmation → MedicalRecord。

**Remove current sequence:**

```text
confirmDiagnosis
→ generateTreatment
```

**Replace:**

```text
confirmDiagnoses(analysisId, selectedCandidateIds)
→ response medical_record
→ refresh thread / projection
```

**Acceptance:** successful confirmation ends at `record_ready` and renders record summary。

---

## DMR-404 — Add MedicalRecord presentation

**Goal:** 当前 MVP 有明确完成面。

可以先做轻量 read-only section：

```text
本次咨询摘要
确认的可能性判断
证据 / citations
安全提示 / red flags
生成时间
```

不要在本 ticket 添加 rehabilitation actions。

---

# Phase 5 — Journey / Old Treatment Retirement

## DMR-501 — Update Health Journey derivation

**Goal:** Journey 不再把任意 `session.Diagnosis != nil` 当作完整 Diagnosis 业务完成。

**Scope:**

- `apps/api/internal/workflow/health_journey.go`
- `packages/contracts/src/health-journey.ts`
- `apps/web/src/features/journey/lib/journeyActions.ts`
- journey tests

**New artifacts:**

建议至少区分：

```text
has_diagnosis_analysis
has_confirmed_diagnoses / has_medical_record
```

当前 MVP 终点以 MedicalRecord existence 为准；MVP 中 `medical_records.consultation_id` 为唯一约束，一个 Consultation 只产生一个最终 record。

Treatment / Training actions 暂时不要由本功能线继续推进。

---

## DMR-502 — Retire consultation treatment path from active MVP

**Goal:** 删除或隔离会继续误导开发的旧路径。

**Candidates:**

- `consultationApi.generateTreatment`
- `DiagnosisPanel` TreatmentPlan view
- ConsultationPage treatment mutation
- Go DiagnosisHandler.GenerateTreatment route（是否立即删除取决于其他页面是否仍调用；删除前先搜索引用）
- Python `generate_treatment`（同上，先确认 caller）

**Rule:**

先切断 UI active path，再逐层删除 dead code；不要在同一个 commit 中无证据删除仍有调用的 API。

---

## DMR-503 — Retire overloaded session fields

当新 Analysis / MedicalRecord read path 已稳定后：

```text
consultation_sessions.diagnosis
consultation_sessions.treatment_plan
thread_projections.diagnosis
thread_projections.treatment_plan
```

按 MVP 策略删除。

不做旧数据迁移兼容层。

---

# Phase 6 — Hardening / Agent & Workflow Cleanup

## DMR-601 — Diagnosis eval harness

扩展现有 consultation eval：

- valid structured candidates；
- enum compliance；
- red flag safety；
- RAG citation coverage；
- governance rejected；
- no candidate / malformed output；
- multi-candidate cases。

可考虑后续接 Pydantic Evals，但不是 Phase 1 的 blocker。

---

## DMR-602 — Context engineering cleanup

明确三层：

```text
LangGraph State
≠
DiagnosisContext
≠
PydanticAI Dependencies
```

建议建立一个纯 builder：

```text
build_diagnosis_context(...)
```

它只组装 context，不做模型调用、不持久化、不改 phase。

---

## DMR-603 — Consultation graph routing cleanup

等 Diagnosis vertical slice 完整后，再处理当前：

```text
workflow_action calculated
but graph always classify_intent -> llm_turn
```

目标是让：

- node 单一职责；
- edge / Command 真正表达 workflow；
- ask_user 保持 HITL interrupt；
- side effects 在 interrupt 前具备幂等性。

这一 ticket 明确晚于 Diagnosis domain refactor，避免同时重写两个核心边界。

---

# 7. Recommended Manual Implementation Order

按以下顺序实施，不按 React/Go/Python 分团队并行乱切：

```text
DMR-001
  ↓
DMR-002 + DMR-003 tests
  ↓
DMR-100
  ↓
DMR-101
  ↓
DMR-102
  ↓
DMR-103 (optional before Go domain cutover)
  ↓
DMR-201
  ↓
DMR-202
  ↓
DMR-203
  ↓
DMR-204
  ↓
DMR-301
  ↓
DMR-302
  ↓
DMR-303
  ↓
DMR-401
  ↓
DMR-402
  ↓
DMR-403
  ↓
DMR-404
  ↓
DMR-501
  ↓
DMR-502
  ↓
DMR-503
  ↓
DMR-601/602/603
```

最小可审查 batches：

### Batch 0 — Baseline safety net

```text
DMR-001 + DMR-002 + DMR-003
```

要求：先建立 characterization / target tests；不在这一批做大范围 production refactor。

### Batch A — Python Agent parity

```text
DMR-100 + DMR-101 + DMR-102
```

`DMR-103` 可作为独立小批次紧随其后。

要求：Go / React 无变化，HTTP behavior parity。

### Batch B — Domain truth

```text
DMR-201 + DMR-202 + DMR-203 + DMR-204
```

要求：DiagnosisAnalysis durable identity + server readiness。

### Batch C — Secure confirmation vertical slice

```text
DMR-301 + DMR-302 + DMR-303
```

要求：1..N confirmation + MedicalRecord transaction。

### Batch D — React cutover

```text
DMR-401 + DMR-402 + DMR-403 + DMR-404
```

要求：用户真实主路径完整可用。

### Batch E — Product cleanup

```text
DMR-501 + DMR-502 + DMR-503
```

要求：旧 Treatment semantics 不再污染当前 MVP。

---

# 8. Verification Matrix

## Python

Focused:

```bash
cd apps/ai-service
uv run pytest tests/unit/test_diagnosis_api_contract.py tests/unit/test_diagnosis_service.py
```

Full:

```bash
pnpm nx test ai-service
pnpm nx lint ai-service
pnpm nx typecheck ai-service
```

Eval:

```bash
pnpm nx run ai-service:eval
```

## Go

Focused package tests during implementation：

```bash
cd apps/api
go test ./internal/handler ./internal/service ./internal/repository
```

Full:

```bash
pnpm nx test api
pnpm nx lint api
pnpm nx build api
```

## React

Focused Vitest：

```bash
pnpm exec vitest run apps/web/src/features/consultation/components/__tests__/DiagnosisPanel.test.tsx --config apps/web/vite.config.ts
```

Full:

```bash
pnpm nx test @bodysense/web
pnpm nx typecheck @bodysense/web
pnpm nx lint @bodysense/web
```

## End-to-End Manual Acceptance

1. 新建 Consultation。
2. 信息不足时生成 Analysis 被阻止。
3. 补充足够信息后进入 `ready_for_analysis`。
4. 生成 Analysis accepted → 展示多个 candidates。
5. 选择一个 candidate → 成功 MedicalRecord。
6. 新会话选择多个 candidates → 成功 MedicalRecord。
7. 提交空 selection → 被拒绝。
8. 手工篡改 candidate ID → server 拒绝。
9. governance rejected → 不进入 analysis_ready。
10. 刷新页面 → Analysis / MedicalRecord 从 projection 恢复。
11. Consultation SSE / ask_user interrupt/resume 无回归。

---

# 9. Risk Ledger

## R-01 — Diagnosis Agent migration accidentally changes HTTP payload

Containment：先完成 DMR-001；PydanticAI 第一批只做内部替换。

## R-02 — Go / React candidate ID ownership不一致

Decision：durable candidate ID 由 Go application layer 分配并持久化；React 只消费。

## R-03 — Analysis stale after new user evidence

Decision：后续关键 health context 更新时允许把 current Analysis 标记 stale / phase 回到 `ready_for_analysis`。不要继续使用 rank-only guard。

## R-04 — Confirmation succeeds but MedicalRecord fails

Decision：同 DB transaction。

## R-05 — Removing treatment breaks training pages

Containment：Phase 5 删除前先搜索所有调用者；本计划只要求“退出当前 MVP 主路径”，不是盲删整个 training feature。

## R-06 — PydanticAI and LangGraph responsibilities overlap

Decision：PydanticAI = Diagnosis Agent；LangGraph = long-lived Consultation workflow / HITL。不要引入第二套 durable workflow truth。

## R-07 — Frontend optimistic phase diverges again

Decision：critical business phases 由 server response / thread projection 驱动，client cache 仅作为展示优化。

---

# 10. Review Protocol for Each Batch

每完成一个 batch，至少检查：

### Contract correctness

- request / response schema；
- candidate identity；
- phase transition；
- governance；
- projection。

### Vertical completeness

- UI 是否真的走新 API；
- Go 是否持久化新对象；
- Python 是否输出 typed result；
- reload 是否恢复。

### Failure paths

- rejected；
- invalid selection；
- stale analysis；
- DB rollback；
- AI failure；
- refresh / retry。

### Diff hygiene

- 不混入 consultation runtime 大重写；
- 不在 Diagnosis batch 顺手做 training UI；
- 不保留两套 active confirmation semantics；
- 不把旧 compatibility code 无限期保留。

---

# 11. Definition of Done

本计划完成时，仓库应满足：

```text
Consultation
  → durable collected context
  → server readiness
  → PydanticAI DiagnosisAgent
  → durable DiagnosisAnalysis with candidate IDs
  → React 1..N selection
  → server-validated ConfirmedDiagnoses
  → atomic MedicalRecord
  → durable thread/journey read model
```

并且：

- 不再允许 arbitrary diagnosis JSON confirmation；
- 不再覆盖原始 DiagnosisAnalysis；
- governance rejected 不产生有效 Analysis；
- 当前 MVP 不依赖 TreatmentPlan 才完成；
- Consultation SSE / replay / interrupt-resume contracts 保持稳定；
- 后续 Rehabilitation 可以直接消费 MedicalRecord，而无需重新解释 ConsultationSession 临时 JSON。
