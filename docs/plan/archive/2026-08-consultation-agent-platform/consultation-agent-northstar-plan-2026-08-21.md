# Consultation Reply Agent Platform North-Star Plan

> Status: Completed (2026-08-21)
> Created: 2026-08-21
> Owner: BodySense AI / Agent Platform program
> Preceded by: Assessment North-Star (Phase 1-6 complete, PR #65/#66/#67 merged).

## 1. Why this plan exists

Assessment North-Star is complete. The gap audit (`docs/plan/active/assessment-gap-audit-2026-08-20.json`)
confirmed **consultation.reply** is the highest-impact remaining gap: it is the primary user-facing
health dialog and it has **no** immutable configuration, **no** Go decision trace/provenance,
**no** replay, **no** rollout governance, and **no** eval/qualification suite.

Consultation is architecturally different from Assessment/Treatment: it is a **multi-turn LangGraph
runtime** with tool loops, interrupts/resumes, and checkpointed state rather than a single-shot
PydanticAI Agent. This plan adapts the North-Star primitive matrix to that runtime.

## 2. North-star rule

Same as Assessment/Treatment. Existing internal AI implementation is not a protected contract.
Protected domain invariants:

- BodyState is Go-owned durable health truth.
- Consultation consumes an exact durable BodyState revision + profile + relevant history.
- Python owns Agent reasoning/runtime; Go owns durable business authority and run envelope.
- Consultation never silently creates Treatment or mutates BodyState outside the defined tool surface.
- Red-flag safety detection is fail-closed.

## 3. Target topology

```text
Go ConsultationRuntime (run envelope, idempotency, SSE)
  | select repository-known Consultation Agent configuration
  | pin BodyState revision + profile + relevant history + run context
  v
Python ConsultationThread (LangGraph runtime)
  | resolve exact immutable manifest
  | build exact prompt + tool surface from manifest revisions
  v
LiteLLM logical model: bodysense-consultation
  v
physical provider/model
```

## 4. Gap audit (consultation.reply vs. North-Star matrix)

| Primitive | Assessment (done) | Consultation |
|---|---|---|
| immutable repo-versioned AgentConfiguration | ✅ | ❌ |
| exact PydanticAI Agent / typed runtime | ✅ | ❌ (LangGraph over raw AIService) |
| LiteLLM logical-model gateway | ✅ | 🟡 (route exists: `consultation.reply`) |
| Go-owned deployment selection / decision authority | ✅ | ❌ |
| durable provenance (config_id + execution_provenance) | ✅ | ❌ |
| Pydantic Evals qualification | ✅ | ❌ |
| frozen-input replay + counterfactual comparison | ✅ | ❌ (multi-turn, needs adaptation) |
| rollout governance (shadow/canary/promote/rollback) | ✅ | ❌ |

## 5. Phase plan

### Phase 1 — Immutable Consultation Agent configuration ✅ (PR #68)

- Add `apps/ai-service/config/agents/consultation-v1.yaml` (role=consultation, prompt/tool/governance
  revisions, logical model, generation settings).
- Add `src/configuration/consultation_agent_config.py` mirroring `assessment_agent_config.py`
  (frozen manifest, canonical behavior JSON, SHA-256 fingerprint, `consult-config-<fingerprint[:16]>`).
- Add Python unit tests: deterministic fingerprint, id stability, unknown-id rejection.

### Phase 2 — Consultation runtime resolution through the manifest ✅ (PR #68)

- Teach `ConsultationThread` to resolve the exact immutable manifest and build its prompt +
  tool surface from manifest revisions.
- Return `agent_configuration` + `execution_provenance` alongside the stream events.
- Add a qualification baseline suite (`consultation_qualification_v1`).

### Phase 3 — Go ownership: deployment policy + provenance + decision trace ✅ (PR #68)

- Extend `agent_deployment_policy.go` with Consultation champion pointer + `knownConsultationConfigurations`.
- Add `ConsultationTurnRequest.configuration_id` and route resolution.
- Persist `agent_configuration_id` / `agent_configuration` / `execution_provenance` on the run record.
- Go `ConsultationRuntime` records a deterministic generation decision trace.

### Phase 4 — Consultation frozen-input replay + comparison ✅ (PR #68)

- Freeze the exact turn input (BodyState revision + profile + relevant history + business context).
- Historical replay recomputes Go generation authority without model call.
- Counterfactual replay runs the frozen input against another immutable Consultation configuration.
- Hard/semantic/presentation drift comparison.

### Phase 5 — Consultation rollout governance (shadow / canary / promotion) ✅ (PR #68)

- Add `consult-promotion-v1` governance, `ConsultationRolloutSelection`, stable per-user bucketing,
  shadow/canary/promoted/rollback, anonymous `consultation_rollout_observations`.
- A Challenger config (`consultation-v2`) gates an iterative refinement.

### Phase 6 — Legacy boundary cleanup for Consultation ✅ (PR #68)

- Retire parallel provider/model construction and any caller-controlled model override.
- Keep `AIService` only as the provider-neutral streaming transport facade; the immutable
  Consultation manifest now pins its LiteLLM logical model and generation settings end-to-end.
- Go remains the configuration/deployment authority; the Python HTTP boundary requires the
  Go-selected `configuration_id`.

## 6. Completion gates

```text
cd apps/ai-service && uv run --extra ocr --extra dev pytest tests/unit -q
go test ./internal/service/ -run 'Consultation' (docker)
```

PR checks: Repository quality gate, commit-lint, PostgreSQL migration replay, Browser longitudinal health E2E.
