# BodySense Current Technical Approach

- Status: Current implementation snapshot
- Updated: 2026-09-01
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

Versions below are repository-manifest values, not aspirational selections.

| Layer                   | Current implementation                                                                                                                     |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| Web                     | React `^19.2.8`, TypeScript `6.0.2`, Vite `^8.1.5`, React Router `^8.3.0`, Zustand `^5.0.14`, TanStack Query `^5.101.4`, Tailwind `^4.3.3` |
| Go API                  | Go `1.26.0`, Gin `1.12.0`, GORM `1.31.2`, golang-migrate `4.19.1`                                                                          |
| Python AI               | Python `>=3.13`, FastAPI `>=0.140.13`, LangGraph `>=1.2.10`, PydanticAI/Pydantic Evals `>=2.31.0`                                          |
| LLM routing             | Repository-known Agent manifests -> LiteLLM logical models -> standalone LiteLLM gateway                                                   |
| Database                | PostgreSQL 18 + pgvector                                                                                                                   |
| Cache                   | Redis 7                                                                                                                                    |
| OCR                     | Tesseract via `pytesseract`; PDF rendering/extraction via PyMuPDF                                                                          |
| Package/runtime tooling | Node 24, pnpm `11.17.0`, Nx `23.1.0`, uv                                                                                                   |

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

Mutable health context belongs to longitudinal BodyState:

```text
body_states
body_state_facts
body_state_observations
body_state_hypotheses
body_state_evidence
body_state_revisions
```

Examples:

```text
lifestyle.activity
lifestyle.sleep
lifestyle.exercise
lifestyle.nutrition
lifestyle.substances
lifestyle.recovery
anthropometry.height
anthropometry.weight
history.injury_summary
```

Diagnosis and Treatment use separate durable history rather than writing final truth into `consultation_sessions`:

```text
diagnosis_analyses / diagnosis_candidates / diagnosis_candidate_assessments
treatments / treatment_revisions
interventions / outcomes
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

New Assessment reports use `assessment-output-v2` served by `assessment-v4` (`assess-config-e579030c2b8b540c`) with `assessment-evidence-contract-v3`.

```text
Frozen health inputs
  -> authoritative evidence catalog
  -> model selects: kind + exactly one evidence_ref
  -> Python deterministic evidence governance
  -> deterministic source rendering
  -> Go rebuilds catalog + revalidates + rerenders
  -> unverified/excluded BodyState observation candidate
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

Completed governed Posture findings may later enter Assessment as visual evidence.

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

The executable route registration in `apps/api/cmd/server/main.go` is the route authority. Major protected surfaces include:

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

GET      /api/v1/body-state
POST/PATCH BodyState fact/observation/hypothesis/review surfaces
GET/PUT  /api/v1/lifestyle
GET/PUT  /api/v1/body-metrics
GET/PUT  /api/v1/health-history/injury
GET      /api/v1/health-workspace

POST     /api/v1/assessment/generate
GET      /api/v1/assessment
GET      /api/v1/assessment/:id
POST     /api/v1/assessment/:id/replay
GET      /api/v1/assessment/:id/regression-export

GET/POST /api/v1/diagnosis-analyses...
GET/POST /api/v1/treatments...
GET/POST /api/v1/outcomes...
```

Knowledge administration is protected by the explicit knowledge-operator capability.

## 9. Deployment baseline

Canonical development/staging/production use PostgreSQL 18. Direct development separates persistent infrastructure lifecycle from foreground Web/API/AI hot-reload processes. Production/staging details are owned by [Deployment Architecture](./deployment-architecture.md) and the environment runbooks.

## 10. Source-of-truth rule

When this document, an older feature design, and executable code disagree:

1. accepted ADRs define intended architectural invariants;
2. `current-*` architecture docs describe implemented state;
3. executable route/schema/config code proves current mechanics;
4. implementation gaps are tracked explicitly rather than rewriting aspirational behavior as if it already existed.

Current known mismatches are tracked in [`2026-09-01-documentation-code-alignment-audit.md`](../plan/active/2026-09-01-documentation-code-alignment-audit.md).
