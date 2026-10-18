# BodySense Current Technical Approach

> **Implementation boundary (2026-10-16, PR6 through #297):** BodyState persistence, evidence reads and Safety use native V2 records. Diagnosis uses only the native Run/Proposal/Condition path (DX-RETIRE-001 #341 retired DiagnosisAnalysis candidates); Treatment and legacy Training remain transitional implementations; this document does not claim the complete V2 product contract. Product rules remain in [docs/product](../product/README.md).

- Status: Current implementation snapshot
- Updated: 2026-10-16 (PR6 native BodyState retirement)
- Architecture authority: accepted ADRs + executable repository state
- Historical initial design: [`docs/plan/archive/architecture-snapshots/2026-09-01/technical-approach.md`](../plan/archive/architecture-snapshots/2026-09-01/technical-approach.md)

This document intentionally describes the **implemented** platform. Historical stack choices, retired routes and migration-era schemas are kept in the archived snapshot rather than mixed into the current architecture.

## 1. Runtime ownership

```text
React Web
  -> user interaction / projections / command submission

Go API
  -> auth / application & domain authority
  -> durable business state / Runtime Event Log / deployment selection

Python AI Service
  -> Agent runtime reasoning / LangGraph checkpointed thread state
  -> PydanticAI typed role execution / RAG / perception & OCR mechanisms

LiteLLM
  -> the only physical LLM provider/model routing boundary
```

The authoritative ownership rules are defined by:

- [ADR 0002 — Agent runtime ownership](../adr/0002-agent-runtime-ownership.md)
- [ADR 0004 — Longitudinal BodyState](../adr/0004-adopt-longitudinal-body-state-model.md)
- [ADR 0005 — Standalone LiteLLM model gateway](../adr/0005-adopt-standalone-litellm-model-gateway.md)
- [ADR 0007 — Stable Profile vs longitudinal health context](../adr/0007-separate-stable-profile-from-longitudinal-lifestyle.md)
- [ADR 0009 — Evidence-grounded Assessment](../adr/0009-adopt-evidence-grounded-assessment-contract.md)

## 2. Current stack

Dependency versions are owned by the executable manifests rather than copied here:

- [Web manifest](../../apps/web/package.json) and [root toolchain manifest](../../package.json): React, TypeScript, Vite, React Router, Zustand, TanStack Query, Tailwind, pnpm and Nx.
- [Go module](../../apps/api/go.mod): Go, Gin, GORM and migration tooling.
- [AI manifest](../../apps/ai-service/pyproject.toml): Python, FastAPI, LangGraph, PydanticAI and OCR dependencies.
- PostgreSQL 18 + pgvector, Redis and LiteLLM runtime images are owned by [Compose configuration](../../docker/docker-compose.yml).

### Historical stack assumptions

Retired stack/provider/schema choices are preserved only in the archived snapshot linked at the top of this document. They are intentionally not repeated in the current technical baseline.

## 3. Durable health domain

`UserProfile` is intentionally narrow:

```text
user_profiles
  id / user_id
  gender
  birth_date
  timestamps
```

Mutable health context belongs to the native BodyState ledger:

```text
body_states / body_state_revisions / body_state_outbox
statement_evidence / body_facts / body_fact_commits
body_observations / body_observation_commits
body_hypotheses / body_hypothesis_commits
symptom_episodes / episode_fact_links / episode_state_evidence
safety_screens / safety_restrictions / safety_resolutions
```

The baseline and model table names are authoritative for the full inventory. Facts preserve the user's statement and assertion status; raw observations preserve source, units and time precision. Correction/withdrawal are distinct from real symptom course. Episode state is derived from eligible evidence. User hypotheses remain separate from Agent hypotheses in analysis audits.

PR6-C removed the three legacy fact/observation/evidence tables and their mutation/review routes. Aggregate identity, revisions and the repository's native V2 methods remain. Assessment selections persist in reports/replay only; they create no BodyState record or revision. Diagnosis freezes owner-scoped native references in its immutable evidence audit; Treatment uses only the selected analysis's audit, never a global evidence pool.

Safety uses independent owner-bound restrictions and exact-target resolutions, with source/revision/lift-basis checks and conservative source-invalidation handling. No JSON overwrite, global clear or AI-write path exists. Incomplete/unavailable coverage fails closed. A positive five-concept screen creates restricted/all-training/professional-clearance restrictions; it is not an AI urgent classification.

Diagnosis and training plans use separate durable history rather than writing final truth into `consultation_sessions` (TP-002 slice 3 removed the Treatment aggregate; Treatment Agent output is a pending PlanProposal):

```text
diagnoses / condition_analysis_runs / diagnosis_proposals / diagnosis_decisions
diagnosis_settlements / diagnosis_revisions / condition_entries / condition_entry_evidence
plan_proposals / training_plans / training_plan_revisions
outcomes
```

`consultation_sessions` is workflow/session state tied 1:1 to a Conversation; it is not the health aggregate.

## 4. Consultation runtime

Current Consultation uses a checkpointed Python Agent runtime and Go durable ledger/projections:

```text
Web intent
  -> Go creates/pins Run + configuration
  -> Python LangGraph thread/checkpoint executes
  -> internal runtime events
  -> Go validates/persists Runtime Event Log
  -> thread/message projections
  -> public SSE/Web projection
```

Important boundaries:

- Python owns Agent Thread runtime history and interrupt/resume semantics.
- Go owns user/run identity, business state and durable runtime events.
- Web does not synthesize resume turns; it submits explicit interrupt answers.
- Diagnosis/Treatment consume pinned BodyState history, not session-level diagnosis/treatment JSON.

## 5. Assessment contract

New Assessment reports use `assessment-output-v2` served by `assessment-v5` (`assess-config-617534e4b17c512a`) with `assessment-evidence-contract-v4`. Assessment v4 / `assessment-evidence-contract-v3` remains immutable for historical replay. The v5 evidence boundary preserves the existing machine-admissible lane and adds a distinct `human-reviewed` lane for the latest effective confirmed/corrected Health Document review; rejected or superseded review states remain unavailable to Assessment.

```text
Frozen health inputs
  -> authoritative evidence catalog
  -> model selects: kind + exactly one evidence_ref
  -> Python deterministic evidence governance
  -> deterministic source rendering
  -> Go rebuilds catalog + revalidates + rerenders
  -> immutable Assessment report and replay (no BodyState mutation)
```

The model cannot author durable Assessment prose, health grades, numeric dimension scores, recommendations, status, coverage or gaps.

Assessment does **not** interpret raw images. Posture is the visual-perception authority. Assessment consumes completed governed `posture_analysis` only. Unmodeled `rag_context` is rejected for the serving contract.

If the admissible evidence catalog is empty:

```text
model call = skipped
status = insufficient_information
model_executed = false
BodyState mutation = none
```

See [ADR 0009](../adr/0009-adopt-evidence-grounded-assessment-contract.md) and the report-indicator admission amendment in [ADR 0011](../adr/0011-adopt-ocr-report-indicator-evidence-admissibility.md).

## 6. Upload, Posture and OCR

### Posture

```text
POST /api/v1/uploads
  -> durable upload object identity
  -> JobRuntime posture job
  -> Python Posture role
  -> governed analysis_result
  -> user_uploads.analysis_result + configuration identity
```

Completed governed Posture findings may later enter Assessment as visual evidence. Current Posture v2 requires `posture-geometry-v1`: MediaPipe `1.0.0`, the versioned `pose_landmarker_lite/float16/1` artifact with pinned SHA256, and a canonical threshold SHA256 are all part of the immutable Posture identity. The artifact is provisioned before serving, never downloaded in the request path, and Python/Go both fail closed on mechanism identity drift. See [ADR 0012](../adr/0012-adopt-pinned-posture-geometry-mechanism.md).

### Health report OCR

```text
POST /api/v1/uploads
  -> JobRuntime OCR job (idempotency: upload_ocr:<upload_id>)
  -> Python /api/ocr/extract
  -> Tesseract OCR / PyMuPDF for PDF pages
  -> deterministic regex HealthIndicator extraction
  -> user_uploads.ocr_result
```

OCR is a non-LLM mechanism. The current code does not use an LLM prompt to clean/extract report indicators.

OCR indicator admissibility is now an explicit versioned gate (`ocr-indicator-admissibility-v1`) consumed by Assessment evidence-contract v3. The remaining OCR alignment gap is immutable mechanism provenance (engine/parser/rendering/extractor revision), tracked in the active documentation/code alignment audit.

## 7. Knowledge architecture

Canonical knowledge persistence is not `knowledge_entries`. The current lifecycle is built around:

```text
knowledge_sources
knowledge_segments
knowledge_units
knowledge_clips
knowledge_publications
knowledge_publication_observations
```

Knowledge source registration is an explicit operator capability. Generated content must pass review/publication governance before it is eligible for published retrieval.

See [Knowledge Lifecycle Architecture](./knowledge-lifecycle-architecture.md).

## 8. Current API surface — major boundaries

The canonical [OpenAPI](../../packages/contracts/openapi/bodysense.v1.openapi.yaml) and generated transport, registered with security partitions in `apps/api/internal/transport/httpapi/routes.go`, define the public surface; `cmd/server/main.go` wires services and security. Major protected surfaces include:

```text
GET/PUT  /api/v1/profile
PUT      /api/v1/onboarding/context

POST/GET /api/v1/uploads
GET      /api/v1/uploads/posture-analysis
GET/DELETE /api/v1/uploads/:id

POST     /api/v1/consultation-runs
POST     /api/v1/consultation-runs/:id/cancel
POST     /api/v1/consultation-runs/:id/replay
POST     /api/v1/consultation-runs/:id/replay/counterfactual
GET      /api/v1/consultations/:id
GET      /api/v1/consultations/:id/thread
POST     /api/v1/consultations/:id/diagnosis
POST     /api/v1/consultations/:id/interrupts/:interactionId/answers

GET/POST /api/v1/body-state/records
POST     /api/v1/body-state/records/:id/correct|withdraw
GET/POST /api/v1/body-state/observation-records
POST     /api/v1/body-state/observation-records/:id/correct|withdraw
GET/POST /api/v1/body-state/hypothesis-records
POST     /api/v1/body-state/hypothesis-records/:id/withdraw
GET/POST /api/v1/body-state/episodes...
GET      /api/v1/body-state/concern-groups
GET      /api/v1/body-state/safety/restrictions
POST     /api/v1/body-state/safety/restrictions/:id/resolve
GET/PUT  /api/v1/lifestyle
GET/PUT  /api/v1/body-metrics
GET/PUT  /api/v1/health-history/injury
GET      /api/v1/health-workspace

POST     /api/v1/assessment/generate
GET      /api/v1/assessment
GET      /api/v1/assessment/:id
POST     /api/v1/assessment/:id/replay
GET      /api/v1/assessment/:id/regression-export

GET      /api/v1/diagnosis
GET/POST /api/v1/diagnosis/analysis-runs...
POST     /api/v1/diagnosis/conditions/:conditionId/direct-edits
GET/POST /api/v1/treatments...
GET/POST /api/v1/outcomes...
```

`health-workspace` exposes capabilities/actions and transitional Diagnosis/Treatment projections, without a `body_state` field. Web loads native records directly, rejects owner/revision drift and never treats a failed read as empty. Internal Go capability and Safety calculations still use one frozen native snapshot.

Knowledge administration is protected by the explicit knowledge-operator capability.

## 9. Deployment baseline

Canonical development/staging/production use PostgreSQL 18. Direct development separates persistent infrastructure lifecycle from foreground Web/API/AI hot-reload processes. Production/staging details are owned by [Deployment Architecture](./deployment-architecture.md) and the environment runbooks.

## 10. Source-of-truth rule

Business requirements are owned by `docs/product`. For implementation facts, code, canonical contracts, the baseline and runtime manifests take precedence over descriptive architecture text. Accepted ADRs supply architectural decisions and historical intent; current documents must be corrected when the implementation changes. Archived plans and old route examples do not authorize restoring retired paths.

Current known mismatches are tracked in [`2026-09-01-documentation-code-alignment-audit.md`](../plan/active/2026-09-01-documentation-code-alignment-audit.md).
