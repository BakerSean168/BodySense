# Assessment Agent Platform North-Star Refactor & Governance Plan

> Status: Active
> Created: 2026-08-20
> Owner: BodySense AI / Agent Platform program
> Reframes: the Assessment agent hypothesis into the same layered North-Star platform already proven by Diagnosis (Phase 0–10) and Treatment.
> Design stance: north-star first, migration second; existing internal AI routing is migration input, not an architectural constraint.
> Product boundary: this pass completes the Assessment role; it does not stop at "one more PR".

## 0. Why this plan exists

PR #64 (Treatment rollout governance) is merged onto `main` (`6716227`). Treatment now has the
full North-Star primitive matrix. The Diagnosis and Treatment programs are archived. `docs/plan/active/`
has no overarching active plan, so this plan re-establishes the next execution wave for the AI Service /
Agent Platform and drives it to completion rather than ending at the Treatment handoff.

The overall program is **not** finished after PR #64. It is finished when every Agent role / LLM
consumer in the AI Service has converged on the shared layered platform:

```text
Web
  |
  v
Go Domain / Application         (durable truth, authority, policy, provenance)
  |
  v
Python Agent Runtime            (PydanticAI Agents, tools, typed reasoning, evidence)
  |
  v
LiteLLM Gateway Service         (provider normalization, retry/fallback, telemetry)
  |
  v
Physical Providers / Models
```

This plan takes the **Assessment** role and brings it to full parity with Diagnosis/Treatment.

## 1. North-star rule

> Existing internal AI implementation is not a protected contract.

Protected are the domain invariants (from `docs/architecture/current-longitudinal-system.md`):

- Assessment is a **derived report**, not a second health truth and not a Treatment system.
- Assessment consumes Profile, Posture analysis and current BodyState.
- Assessment emits traceable Observation candidates and information gaps.
- Observations are projected into BodyState with content-addressed source keys before the report is stored.
- Assessment **never emits** executable exercise, nutrition, or treatment prescriptions.
- Python owns Agent reasoning/runtime; Go owns durable report identity/immutability and BodyState projection.
- Web remains a projection consumer.

Retirable (superseded path): the legacy `AIService`/`AiRequest` application path and parallel model
construction where Assessment uses it today.

## 2. Gap audit (Assessment vs. North-Star matrix)

Confirmed against `apps/ai-service` + `apps/api/internal/service` (2026-08-20):

| Primitive                                                      | Diagnosis | Treatment | Assessment |
| -------------------------------------------------------------- | :-------: | :-------: | :--------: |
| immutable repo-versioned AgentConfiguration + behavior fingerprint | ✅ | ✅ | ❌ |
| exact PydanticAI Agent                                         | ✅ | ✅ | ✅ (exists) |
| LiteLLM logical-model gateway                                  | ✅ | ✅ | 🟡 (route exists: `assessment.generate`) |
| Go-owned deployment selection / decision authority            | ✅ | ✅ | ❌ |
| durable provenance (config_id + execution_provenance)          | ✅ | ✅ | ❌ |
| Pydantic Evals qualification                                   | ✅ | ✅ | ❌ |
| typed evidence acquisition                                     | ✅ | ✅ | ❌ (n/a for assessment — reuse posture/profile evidence) |
| frozen-input replay + counterfactual comparison                | ✅ | ✅ | ❌ |
| rollout governance (shadow/canary/promote/rollback)            | ✅ | ✅ | ❌ |

Assessment today: has a PydanticAI `Agent` (`.py`) and a `assessment.generate` LiteLLM route, but the
Go `AssessmentService` calls a non-typed `assessmentReasoner` with **no** `configuration_id`, persists
`AssessmentReport` with **no** agent/execution provenance, has **no** decision trace, **no** replay
inputs, and **no** eval/qualification suite.

## 3. Target Assessment topology

```text
Go AssessmentService
  | select repository-known Assessment Agent configuration (Go pointer)
  | pin profile + posture analysis + BodyState facts + user uploads
  v
Python AssessmentAgentService
  | resolve exact immutable manifest
  | build exact PydanticAI Assessment Agent
  v
LiteLLM logical model: bodysense-structured
  v
physical provider/model
```

## 4. Phase plan

### Phase 1 — Immutable Assessment Agent configuration

- Add `apps/ai-service/config/agents/assessment-v1.yaml` (role=`assessment`, prompt/schema/tool/
  governance/decision/logical-model revisions, generation settings) modeled on `treatment-v1.yaml`.
- Add `apps/ai-service/src/configuration/assessment_agent_config.py` mirroring
  `treatment_agent_config.py` (frozen manifest, canonical behavior JSON, SHA-256 fingerprint,
  `assess-config-<fingerprint[:16]>` id, provenance()).
- Add Python unit tests: deterministic fingerprint, id stability, config resolution, unknown-id rejection.
- **Do not** change production serving yet (Assessment remains on today's path) — this phase only
  introduces the artifact and its identity.

### Phase 2 — Assessment runtime resolution through the manifest

- Teach `AssessmentAgentService` (Python) to resolve the exact immutable manifest, build the PydanticAI
  Assessment Agent from it, and return `agent_configuration` + `execution_provenance` alongside the
  typed assessment payload.
- Assessment continues through the existing `assessment.generate` LiteLLM logical route.
- Add a qualification baseline suite (`assessment_qualification_v1`) mirroring `treatment_qualification_v1`:
  typed report contract, observation-only/derived-report boundary, exact config + execution provenance,
  no executable prescription output.

### Phase 3 — Go ownership: deployment policy + provenance + decision trace

- Extend `agent_deployment_policy.go` with Assessment champion pointer + `knownAssessmentConfigurations`.
- Add `AssessmentGenerationRequest.configuration_id` and route resolution.
- Migration `000043_add_assessment_agent_provenance`: add `agent_configuration_id`,
  `agent_configuration`, `execution_provenance`, `generation_decision_trace` to immutable
  `assessment_reports` (report rows remain immutable once persisted).
- Go `AssessmentService` persists provenance and a deterministic generation decision trace atomically
  with the report + BodyState observation transaction.

### Phase 4 — Assessment frozen-input replay + comparison

- Migration `000044_add_assessment_replay_input`: freeze exact serialized profile + posture + BodyState
  facts + uploads-derived inputs + generation-authority facts.
- Add historical replay (no model call) and counterfactual replay against another immutable Assessment
  configuration; hard/semantic/presentation drift comparison; privacy-sanitized regression export.

### Phase 5 — Assessment rollout governance (shadow / canary / promotion)

- Add `assess-promotion-v1` governance, `AssessmentRolloutSelection`, stable per-user bucketing,
  shadow/canary/promoted/rollback, anonymous `assessment_rollout_observations` (migration
  `000045_create_assessment_rollout_observations`).
- A Challenger config (`assessment-v2`) may carry an iterative refinement (e.g. evidence-gap or richer
  observation taxonomy) gated behind the same promotion record and non-inferiority qualification.

### Phase 6 — Legacy boundary cleanup for Assessment

- Retire the non-typed `assessmentReasoner` interface and any legacy `AIService` link for Assessment.
- Remove `use_case` overrides and any parallel model construction left in the Assessment path.

## 5. Completion gates (identical to Treatment/Diagnosis)

```text
cd apps/ai-service && uv run --extra ocr --extra dev pytest tests/unit -q     # Python
go test ./internal/service/ -run 'Assessment|DeploymentPolicy' (docker)      # Go
```

PR checks: Repository quality gate, commit-lint, PostgreSQL migration replay, Browser longitudinal health E2E.
Production/default Assessment must remain serving the existing behavior until every gate passes and the
rollout governance record is approved.

## 6. Out of scope for this plan (later waves, tracked separately)

The same North-Star matrix still needs to be extended to the remaining consumers:
consultation.reply, posture.analyze, conversation.title, knowledge.curate/split, ocr, and the legacy
`AIService`/`AiRequest` application path. Those are separate phases/BPRs under the same program banner
(see README for the program backlog).
