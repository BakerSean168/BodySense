# BodyState → Diagnosis Implementation Checkpoint

> Status: Diagnosis checkpoint implemented and locally validated  
> Date: 2026-08-15  
> Parent plan: [`longitudinal-body-state-migration-plan.md`](./longitudinal-body-state-migration-plan.md)  
> Domain source: [`../../architecture/longitudinal-body-state-domain.md`](../../architecture/longitudinal-body-state-domain.md)

## 1. Checkpoint purpose

This file freezes the implementation hand-off point requested by the user:

```text
Long-lived Conversation
  -> Longitudinal BodyState
  -> Consultation producer/context
  -> durable SafetyState
  -> Diagnosis pinned to BodyStateRevision
  -> immutable Diagnosis history
  -> user candidate assessment

STOP HERE
  -> learn/refactor Diagnosis execution (PydanticAI + targeted RAG)
  -> only then continue to Treatment
```

No Treatment-domain migration is part of this checkpoint.

## 2. Implemented code surfaces

### Go / durable business state

New migrations:

```text
apps/api/migrations/000032_create_body_state.{up,down}.sql
apps/api/migrations/000033_create_diagnosis_analyses.{up,down}.sql
```

New BodyState code:

```text
apps/api/internal/model/body_state.go
apps/api/internal/repository/body_state_repository.go
apps/api/internal/service/body_state_service.go
apps/api/internal/dto/body_state.go
apps/api/internal/handler/body_state_handler.go
```

New Diagnosis durable code:

```text
apps/api/internal/model/diagnosis_analysis.go
apps/api/internal/repository/diagnosis_analysis_repository.go
apps/api/internal/service/diagnosis_analysis_service.go
```

Modified integration surfaces:

```text
apps/api/internal/consultation/runtime.go
apps/api/internal/service/consultation_service.go
apps/api/internal/repository/consultation_repository.go
apps/api/internal/service/ai_client.go
apps/api/internal/handler/consultation_handler.go
apps/api/internal/handler/thread_projection_handler.go
apps/api/internal/handler/diagnosis_handler.go
apps/api/cmd/server/main.go
```

### Python / reasoning runtime

```text
apps/ai-service/src/models/diagnosis.py
apps/ai-service/src/prompts/diagnosis.py
apps/ai-service/src/services/diagnosis_service.py
apps/ai-service/src/api/routes/diagnosis.py
apps/ai-service/src/api/routes/runtime.py
apps/ai-service/src/runtime/consultation_thread.py
apps/ai-service/src/runtime/governance.py
```

### React / product projection

```text
apps/web/src/features/consultation/types/consultation.ts
apps/web/src/features/consultation/services/consultationService.ts
apps/web/src/features/consultation/components/DiagnosisPanel.tsx
apps/web/src/features/consultation/components/DiagnosisHistoryPanel.tsx
apps/web/src/features/consultation/pages/ConsultationPage.tsx
```

## 3. Implemented invariants

1. Conversation is not health truth; BodyState is Go-owned durable truth.
2. Meaningful BodyState mutations create monotonic immutable revisions.
3. Fact and Observation are persisted separately.
4. Explicit correction and temporal-update APIs are separate operations.
5. Current reasoning projection excludes inactive and reasoning-excluded items.
6. Existing `health_features` is now a BodyState-derived compatibility projection once BodyState exists.
7. Compatibility workbench rows carry durable BodyState item IDs; confirm/edit/delete acts on the same durable item rather than duplicating it.
8. Consultation extracted symptoms, ask-user answers and positive safety events can produce BodyState state.
9. Python Consultation context receives current BodyState and bounded revision history; corrected BodyState outranks stale chat text.
10. Positive safety state is durable and can block ordinary Diagnosis.
11. Diagnosis pins an exact input BodyState revision.
12. Diagnosis no longer has a fixed maximum candidate count.
13. `completed` requires at least one candidate; insufficient/safety-blocked analyses can contain zero.
14. Go assigns durable Analysis/Candidate IDs; Python does not invent persistent identity.
15. DiagnosisAnalysis and candidates are immutable historical artifacts.
16. User candidate assessment is stored separately as `confirmed / unsure / not_applicable`; omitted candidates are not deleted.
17. Diagnosis history is queryable and has a basic Web timeline.
18. Diagnosis does not automatically generate Treatment.
19. Positive red flags detected inside Diagnosis stop ordinary AI candidate generation and are promoted back into durable BodyState SafetyState.
20. Negative findings / old history are not fed into the legacy safety keyword detector as positive current symptoms.

## 4. Compatibility intentionally retained

The following remain temporary migration adapters, not new domain truth:

```text
consultation_sessions.extracted_info
consultation_sessions.health_features
consultation_sessions.diagnosis
legacy consultation phase values
legacy POST /consultations/:id/treatment
legacy confirmation/treatment code paths
historical multi-conversation records/sidebar
```

Do not expand these adapters with new business semantics.

## 5. Known intentionally incomplete areas at this checkpoint

### Diagnosis learning/refactor next

- PydanticAI `2.31.0` is now installed through `pydantic-ai-slim[openai]`.
- DX-001 Step A now has a parallel typed `src/agents/diagnosis_agent.py` learning path using `deps_type=DiagnosisDependencies`, `RunContext.deps`, and `output_type=DiagnosisAgentOutput`; focused tests prove typed output without manual JSON parsing.
- `DiagnosisService` production execution still intentionally runs through the existing `AIExecutor -> AIService.generate` seam until provider/fallback routing is migrated deliberately.
- targeted evidence-gap RAG/tools are not implemented yet.
- evidence references exist in the candidate schema, but evidence acquisition is not yet a PydanticAI tool flow.
- broader Diagnosis eval/golden cases still need to be developed during the learning pass.

### BodyState follow-up (not blocking this Diagnosis checkpoint)

- final workbench UI for explicit `correction vs changed later` is not implemented; explicit API semantics exist, while the old InfoPanel is still a compatibility editor.
- automatic time-varying lifestyle/activity extraction is not yet a dedicated producer; the domain can persist those Facts and Diagnosis still receives supporting profile context during migration.
- Hypothesis/Evidence are semantically separated in the domain model, but no full durable Hypothesis/Evidence aggregate was added in this batch.
- long-conversation historical-message retrieval is not implemented; current context uses recent runtime messages + BodyState + bounded revisions.
- safety resolution/clearance requires a later explicit policy; positive safety state is deliberately not auto-cleared by one negative detector result.
- historical multi-conversation UI remains for migration/readability, although new ordinary product flow resolves to one long-lived consultation.

## 6. Validation evidence

`@agent-v2` now exposes shell execution, so this checkpoint has been validated on the oracle3 repository instead of remaining a static-only implementation.

Executed successfully:

```text
Go focused:
  go test ./internal/service ./internal/repository ./internal/handler ./internal/consultation

Go full:
  go test ./...
  PASS

Python Diagnosis focused:
  19 passed

Python full:
  212 passed

Python touched-file quality:
  ruff check -> clean
  pyright on Diagnosis-touched files/tests -> 0 errors

Web:
  DiagnosisPanel target tests -> 9 passed
  full web suite -> 138 passed
  @bodysense/web:typecheck -> pass
  @bodysense/web:build -> pass

Contracts:
  @bodysense/contracts:test -> pass

Database migration smoke:
  disposable PostgreSQL database
  applied every migration from 000001 through 000033 with ON_ERROR_STOP=1
  then replayed 000033 down -> 000032 down -> 000032 up -> 000033 up
  migration smoke/replay -> OK
```

The validation pass found and repaired three migration defects before reaching green:

1. a missing `}` in `bodyStateAnswerText` that initially blocked Go compilation;
2. obsolete `DiagnosisPanel` tests that still encoded the retired “pick one diagnosis -> generate Treatment” contract;
3. local Python Ruff/Pyright issues in the touched Diagnosis files.

Repository-wide Pyright still contains pre-existing debt outside this migration scope, so the evidence claim is intentionally limited to the Diagnosis-touched Python files rather than claiming the whole repository is Pyright-clean.

`git diff --check` is part of the final hygiene gate below and must remain clean before this checkpoint is merged.

## 7. Tests written/updated in this batch

```text
apps/api/internal/service/body_state_service_test.go
apps/api/internal/service/diagnosis_analysis_service_test.go
apps/api/internal/service/consultation_service_test.go
apps/api/internal/repository/consultation_repository_test.go
apps/api/internal/handler/consultation_handler_http_test.go
apps/api/internal/handler/diagnosis_handler_http_test.go

apps/ai-service/tests/unit/test_diagnosis_models.py
apps/ai-service/tests/unit/test_diagnosis_service.py
apps/ai-service/tests/unit/test_diagnosis_api_contract.py
```

The target cases include:

- long-lived conversation reuse;
- durable workbench item identity;
- AI-extracted Fact remains unverified;
- positive safety persistence;
- exact BodyState revision pinning;
- 8-candidate Diagnosis;
- zero-candidate safety/insufficient analysis;
- invalid analysis status;
- candidate ownership validation;
- independent candidate assessments;
- current positive red flag blocks Diagnosis without calling the model;
- negative/historical red-flag words do not create a false current safety block.

## 8. Next learning ticket — DX-001

### Goal

**Progress:** Step A complete locally: PydanticAI dependency + parallel typed Diagnosis Agent + focused tests. Production cutover remains pending.

Replace only the Diagnosis execution core:

```text
AIService.generate
  -> raw JSON
  -> Pydantic model_validate
```

with a PydanticAI `Agent` using the already-defined BodyState-based typed input/output boundary.

### Protected contracts

Do not change:

- Go `DiagnosisRequest` BodyState revision semantics;
- Go-owned analysis/candidate IDs;
- `DiagnosisAgentOutput` 0..N candidate semantics;
- safety business gate;
- Diagnosis history persistence;
- public `diagnoses` compatibility response during this migration;
- LangGraph Consultation runtime.

### Learning objectives

Use this ticket to understand, in order:

1. why application constructor DI and PydanticAI `deps_type` solve different problems;
2. how `Agent(..., deps_type=..., output_type=...)` changes the execution boundary;
3. how `RunContext[DiagnosisDependencies]` exposes per-run BodyState/evidence context;
4. how structured output removes the manual raw-JSON parsing path;
5. how a later targeted RAG tool can be added without moving durable business ownership into Python.

Do not start Treatment until DX-001 plus its focused tests/evals are reviewed.
