# Documentation / ADR / Code Alignment Audit — 2026-09-01

- Status: ACTIVE — documentation baseline cleaned; remaining code gaps triaged below
- Scope: accepted ADRs 0001–0009, current architecture docs, feature specs, active-plan index, executable Web/Go/Python contracts
- Branch: `docs/document-code-alignment-audit`
- Baseline: `7139d2f6`

## 1. Goal

BodySense accumulated three different kinds of drift while the architecture evolved quickly:

```text
A. old design text still presented as current
B. code already moved forward but docs/ADR follow-up still said "pending"
C. newer accepted contract exists, but executable code still has a weaker/older boundary
```

This audit separates those cases. Historical decision context is retained in ADR/archive; **current architecture must describe implemented facts**, and unimplemented stronger behavior must stay explicit in this active audit rather than being written as if it already exists.

## 2. Source-of-truth rule

For this audit:

1. accepted ADRs define intended architectural invariants;
2. later ADRs may narrow/supersede an earlier decision without erasing its history;
3. `current-*`/current architecture docs describe implemented state;
4. executable code, migrations, route registration and configuration manifests prove current mechanics;
5. deployment capability and environment promotion are separate facts;
6. old migration files are immutable historical artifacts and must not be edited merely to fix a stale comment/link.

## 3. Classification

| Code               | Meaning                                                                                                 |
| ------------------ | ------------------------------------------------------------------------------------------------------- |
| `D-CLEANED`        | current doc was stale; fixed in this audit                                                              |
| `HISTORICAL`       | old design kept only as archive/redirect/history                                                        |
| `CODE-GAP`         | accepted/current contract is stronger than executable code                                              |
| `HARDENING`        | current behavior is safe, but obsolete compatibility surface makes regression easier                    |
| `DECISION-GAP`     | current mechanics are valid but product/domain semantics need an explicit decision before changing code |
| `REFERENCE-DRIFT`  | stale/broken doc/code reference fixed without changing business behavior                                |
| `DEPLOYMENT-STATE` | mechanism exists; actual environment promotion remains an operator/release decision                     |

## 4. Accepted ADR alignment matrix

| ADR                                   | Current result                                                                                                          | Alignment                                             |
| ------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------- |
| 0001 Deepen runtime modules           | runtime-module principle remains; transitional Agent runtime ownership was superseded by 0002                           | `D-CLEANED` / aligned                                 |
| 0002 Agent runtime ownership          | Python LangGraph owns Agent Thread runtime; Go owns durable business/event truth; Web consumes projections              | aligned                                               |
| 0003 StreamEvent versioning           | shared TS schema/fixtures/parser + Go/Python contract tests; live/recovery share public v1 semantics                    | aligned                                               |
| 0004 Longitudinal BodyState           | core migration implemented; Diagnosis/Treatment/Training/Outcome use longitudinal durable models                        | aligned, one schema residue below                     |
| 0005 Standalone LiteLLM gateway       | application chooses logical models; physical routing remains behind LiteLLM                                             | aligned, minor helper duplication below               |
| 0006 Vanatome 3D                      | ontology, mapping, viewer, fallback, atlas tooling/CDN path implemented and staging validated                           | aligned; final visual/release acceptance remains      |
| 0007 Stable Profile vs health context | `UserProfile` contains only stable identity; mutable health data is BodyState                                           | aligned; Assessment clause narrowed by 0009           |
| 0008 Delivery Platform V3             | adopted/proven release/deploy control is present                                                                        | aligned                                               |
| 0009 Evidence-grounded Assessment     | selection-only model authority, deterministic rendering, Python+Go evidence gate, no-evidence no-model path implemented | core aligned; remaining evidence/mechanism gaps below |

## 5. Pre-user Agent baseline promotion — owner-approved

**Decision date:** 2026-09-01
**Owner decision:** promote the latest qualified Diagnosis and Treatment configurations to the repository baseline now, while BodySense is still a single-owner/pre-user product.

### Why this is a baseline promotion, not a claim that live rollout finished

The original rollout policies require a live sequence of:

```text
shadow >= 20 observations
  -> canary 5%
  -> canary 25%
  -> canary 50%
  -> promoted 100%
```

That ladder remains the default governance for a product with meaningful external-user traffic. BodySense currently has no external user population from which those samples would provide independent rollout evidence. Repeating one owner's development traffic twenty times would satisfy a counter without adding the statistical meaning the gate was designed for.

Therefore this plan records an explicit **pre-user promotion waiver** rather than pretending the live sample progression has already occurred. The waiver is bounded to these already-qualified immutable pairs:

```text
Diagnosis
  historical v1: diag-config-f492eb1c0c6676ae
  latest v3:     diag-config-5a4a13627e14b4cf

Treatment
  historical v1: treat-config-85718f8e90ac9d80
  latest v2:     treat-config-f68eec9846664596
```

Qualification evidence already exists and remains immutable historical evidence:

- Diagnosis v1 baseline: 100%;
- Diagnosis v1 -> v2 EvidenceGap: non-inferior, zero critical regression;
- Diagnosis v2 -> v3 DecisionAuthority: non-inferior, zero critical regression;
- Diagnosis Evidence policy: 5/5;
- Treatment v1 baseline: 4/4;
- Treatment v2 Challenger: 4/4, non-inferior, zero regression;
- Treatment EvidenceGap policy: 5/5;
- both promotion-readiness reports contain no blocking reason.

### Target repository semantics

Do **not** leave the system permanently encoded as:

```text
v1 Champion + latest Challenger + ROLLOUT_STAGE=promoted
```

That would serve the correct version but leave the repository vocabulary lying about which behavior is the stable baseline. The completed migration must instead converge to:

```text
Diagnosis v3 = repository Champion/default serving baseline
Treatment v2 = repository Champion/default serving baseline

Diagnosis v1 = explicit rollback + historical replay target
Treatment v1 = explicit rollback + historical replay target

active Challenger = none until a future qualified v4/v3 exists
```

The historical `diagnosis_promotion_v1` and `treatment_promotion_v1` records remain evidence that justified the one-time v1 -> latest transition. They must not be rewritten to pretend they describe a future rollout pair.

### Implementation steps

1. Separate historical v1 IDs from the mutable/default Champion constants in Go.
2. Make Diagnosis v3 and Treatment v2 the repository defaults.
3. Introduce explicit rollback configuration identities instead of using the `Challenger` slot as a rollback alias.
4. Allow the stable Champion state to have no active Challenger. Require a distinct repository-known Challenger plus a matching future promotion record before any future `shadow/canary/promoted` stage.
5. Keep historical v1/v2/v3 manifests immutable for replay.
6. Update Compose/dev/staging/prod defaults and deployment validation so clean environments serve the latest Champion without an operator override.
7. Update rollout/provenance docs so `Champion`, `Challenger`, `rollback target`, and `historical promotion evidence` are distinct concepts.
8. Re-run qualification/policy tests, Go rollout tests, longitudinal E2E, repository release verification, then validate Dev -> Staging -> Production with the exact merged SHA.

### Acceptance criteria

- a clean `AgentDeploymentPolicy` serves Diagnosis v3 and Treatment v2 in `champion` stage;
- no current config calls Diagnosis v1 or Treatment v1 the active Challenger;
- explicit rollback serves the historical v1 configuration;
- future non-Champion rollout without a distinct qualified Challenger fails closed;
- historical v1 -> latest promotion artifacts remain unchanged and their sync tests use historical IDs rather than current default aliases;
- Dev/Staging/Production report latest Agent configuration IDs after deployment;
- rollback remains a one-step configuration operation with no schema/data rollback.

---

## 6. Highest-priority remaining code gaps

### P1 — A1. OCR report indicators bypass an explicit admissibility/review gate

**Classification:** `CODE-GAP`
**Contracts:** ADR 0009 + `agent-platform-role-governance.md` + current Assessment feature spec.

Current BodyState evidence has explicit eligibility:

```text
excluded_from_reasoning != true
review_state == confirmed (when present)
lifecycle_state == active (when present)
```

Report indicators do not. Current path:

```text
user_uploads.ocr_status == completed
  -> read result.indicators[]
  -> every indicator copied into reportIndicators
  -> Python/Go Assessment evidence catalog
  -> selectable as report_indicator evidence
```

Code evidence:

- `apps/api/internal/service/assessment_service.go::assessmentInputsFromUploads`
- `apps/ai-service/src/services/assessment_evidence.py::build_assessment_evidence_catalog`
- `apps/api/internal/service/assessment_evidence_contract.go::buildAssessmentEvidenceCatalog`
- `apps/ai-service/src/models/ocr.py::HealthIndicator.confidence`

`HealthIndicator.confidence` is currently only a free string with default `high`; no policy maps `high/medium/low` into evidence admissibility and there is no user/reviewer confirmation identity.

**Risk**

A false-positive or low-confidence deterministic OCR match can become an exact durable Assessment evidence ref even though a BodyState AI observation would have been excluded until confirmation. This creates inconsistent evidence standards across sources.

**Recommended contract**

Choose and version one explicit policy, for example:

```text
OCR indicator
  -> extraction confidence + parser revision
  -> normalized indicator candidate
  -> admissibility policy
       high + strict parse      -> admissible candidate (if product accepts this risk)
       medium/low/ambiguous     -> excluded or review_required
  -> optional user confirmation
  -> Assessment catalog
```

A stronger option is to promote reviewed report indicators into an explicit durable evidence/fact model and let Assessment select only that normalized identity.

**Acceptance tests**

- low-confidence OCR indicator never enters the Assessment catalog;
- malformed/unknown confidence fails closed;
- policy revision is frozen in replay/provenance;
- a confirmed/admissible report indicator is selectable in both Python and Go;
- Python/Go policy fixtures remain identical.

---

### P1 — A2. Posture geometric perception is behavior-significant but not version-pinned in Posture configuration/provenance

**Classification:** `CODE-GAP`
**Contracts:** Agent Platform non-LLM mechanism provenance + Posture immutable behavior identity + ADR 0009 visual evidence chain.

Current geometric path:

```text
MediaPipe Tasks PoseLandmarker
  -> pose_landmarker_lite/float16/latest/pose_landmarker_lite.task
  -> repository threshold constants
  -> geometric findings/metrics
  -> govern_posture_result
  -> user_uploads.analysis_result
  -> Assessment posture evidence
```

Current `posture-v1.yaml` fingerprints prompt/schema/tool/governance/model/generation but contains no:

```text
pose engine revision
pose model artifact hash/version
geometric-threshold revision
```

The model URL uses `latest`, and the resulting numeric metrics are intentionally authoritative over VLM-invented numeric values.

Code evidence:

- `apps/ai-service/src/services/pose_estimator.py`
- `apps/ai-service/src/services/posture_analyzer.py`
- `apps/ai-service/src/configuration/posture_agent_config.py`
- `apps/ai-service/config/agents/posture-v1.yaml`

**Risk**

The same immutable Posture configuration ID can produce different governed numeric evidence after an upstream `latest` model changes or threshold behavior changes. That weakens replay, audit and the meaning of immutable configuration identity for an evidence-producing perception role.

**Recommended contract**

Introduce an explicit mechanism identity, e.g.:

```text
pose_mechanism_revision: posture-geometry-v1
engine: mediapipe-tasks
package_version: ...
model_artifact: pose_landmarker_lite-<pinned-version>.task
model_sha256: ...
threshold_revision: posture-geometry-thresholds-v1
```

The Posture durable result/execution provenance must record it. If geometric behavior affects the Agent output contract, the mechanism revision should also participate in the Posture immutable behavior fingerprint or be a separately pinned subordinate identity referenced by the manifest.

**Acceptance tests**

- no runtime download from an unversioned `latest` artifact;
- artifact hash mismatch fails closed;
- changing threshold/model revision changes the declared behavior identity;
- persisted Posture result exposes exact mechanism identity;
- replay/qualification fixture asserts the identity.

---

### P1/P2 — A3. OCR engine/parser provenance is missing even though OCR output can become Assessment evidence

**Classification:** `CODE-GAP`
**Contracts:** non-LLM mechanism provenance.

Current durable `OCRResult` stores:

```text
raw_text
indicators
confidence
```

but does not store:

```text
engine = tesseract
engine version
language/config
pytesseract wrapper revision
indicator extractor revision
PDF extraction/rendering revision
```

The container installs the OS `tesseract-ocr` package without a durable per-result engine identity. The indicator extractor is deterministic code but has no explicit revision.

Code evidence:

- `apps/ai-service/src/services/ocr.py`
- `apps/ai-service/src/services/indicator_extractor.py`
- `apps/ai-service/src/models/ocr.py`
- `apps/ai-service/Dockerfile`
- `apps/api/internal/service/upload_service.go`

**Risk**

Historical report evidence cannot answer which OCR/parser behavior produced an indicator. A future OCR/parser change can alter evidence while replay/audit sees only the extracted values.

**Recommended contract**

Persist `mechanism_provenance` in OCR result, including an explicit repository-owned extractor revision. Prefer a pinned container/OS package image or at minimum record the runtime-reported Tesseract version.

**Acceptance tests**

- OCR result always contains non-empty mechanism identity;
- missing/unknown extractor revision fails evidence admission for new reports;
- regression export includes the evidence mechanism revision without exposing raw private report text unnecessarily.

## 7. Medium-priority hardening / architecture gaps

### P2 — B1. Assessment v3 still exposes legacy request/dependency fields that the serving contract forbids

**Classification:** `HARDENING`

Current internal Python request type still accepts:

```text
profile
body_state
report_indicators
rag_context
images
posture_analysis
configuration_id
```

`assessment-output-v2` correctly rejects nonempty `images` and `rag_context`, and v3 Agent instructions serialize only the authoritative `evidence_catalog`. So the serving path is currently safe.

However, the API/dependency surface still carries migration-era capabilities that ADR 0009 explicitly removed from serving authority.

Code evidence:

- `apps/ai-service/src/api/routes/assessment.py::AssessmentRequest`
- `apps/ai-service/src/services/assessment_service.py::generate_assessment`
- `apps/ai-service/src/models/assessment.py::AssessmentDependencies`

**Recommended cleanup**

Introduce a serving-v3 request/dependency contract that contains only the frozen health inputs needed to construct the catalog (or the catalog plus explicit trusted metadata), while retaining a clearly separate historical replay adapter for v1/v2.

Do not silently remove historical contracts needed for replay.

**Acceptance tests**

- serving-v3 route cannot represent raw images or free-form RAG context;
- historical replay fixtures still resolve immutable old schemas;
- no change to current v3 deterministic evidence behavior.

---

### P2 — B2. Assessment BodyState projection source key is report-scoped, not evidence/content-addressed

**Classification:** `DECISION-GAP`

Actual source key:

```text
assessment:<report_id>:observation:<index>
```

Database uniqueness is effectively per `(user_id, source_key)`. This guarantees retry/idempotency inside one report transaction path, but does not deduplicate the same evidence selected by two separately generated Assessment reports.

This audit corrected the current architecture doc, which previously claimed content-addressed source keys.

**Decision required**

Two valid domain semantics exist:

1. **report-history semantics** — every Assessment report creates a separate historical unverified candidate, even if based on the same evidence;
2. **evidence-candidate semantics** — the same evidence+kind should not create duplicate active candidate observations across reports.

Do not change the key until the desired longitudinal semantics are explicit.

If option 2 wins, consider a stable key/fingerprint such as:

```text
assessment-evidence:<kind>:<evidence-ref>:<contract-revision>
```

with careful treatment of history/supersession rather than blindly overwriting old artifacts.

---

### P2 — B3. ASR provenance is partial, not a complete mechanism identity

**Classification:** `HARDENING`

Knowledge ingestion records transcript provider/model inputs in source/job metadata, which is materially better than OCR. But local ASR implementations may also depend on runtime binaries/model artifacts whose exact revision/hash is not consistently persisted as one mechanism identity.

Examples:

- whisper.cpp model name is known;
- FunASR has runtime/model constants;
- remote ASR has provider/model env config;
- no single immutable `asr_mechanism_identity` is stored across all providers.

**Recommended cleanup**

Normalize ASR provenance to provider/runtime/model/artifact revision/fingerprint before treating transcripts as reproducible source artifacts.

Priority is lower than OCR/Posture because ASR output enters the reviewed Knowledge lifecycle before publication rather than directly becoming a user health observation.

---

### P2/P3 — B4. ADR 0004 migration left unused `health_features` columns in durable schema

**Classification:** `CODE-GAP` / schema debt

Migration 29 added:

```text
consultation_sessions.health_features
thread_projections.health_features
```

Current executable models/services do not use these fields. Migration 58 removed old mutable health columns from `user_profiles` but did not drop these two migration-era projection/session columns.

**Risk**

Low runtime risk today because application code ignores them, but they are misleading dormant schema authority and can invite accidental reuse of the superseded “session health truth” model.

**Recommended cleanup**

After confirming no production data/compatibility consumer requires them, add a new forward migration to drop the columns. Never edit migration 29 in place.

**Acceptance tests**

- production-baseline upgrade succeeds;
- privacy erasure/schema validators still pass;
- no executable query/model depends on either field;
- down migration semantics are explicitly documented if restoration would be lossy.

---

### P3 — B5. Diagnosis has a duplicate LiteLLM OpenAI-compatible transport helper

**Classification:** `HARDENING`

`apps/ai-service/src/ai/gateway.py::get_gateway_model` is the common gateway constructor. `diagnosis_gateway_model.py` independently creates an `OpenAIProvider` + `OpenAIChatModel` pointed at the same LiteLLM base URL.

This does **not** violate ADR 0005 today: both paths still route exclusively through LiteLLM and neither chooses physical providers. It is duplication that increases drift risk.

**Recommended cleanup**

Unless Diagnosis needs a documented special transport behavior, delegate to the common `get_gateway_model(config.logical_model)` and keep only Diagnosis model-group validation/settings in the role-specific module.

## 8. Explicit non-gaps / deployment states

These were reviewed and must not be reopened as implementation bugs merely because old docs used future tense.

### C1. Diagnosis v3 and Treatment v2 baseline promotion

**Classification:** `DEPLOYMENT-STATE` resolved by ADR 0010 in this lane.

The rollout mechanisms were already implemented; the remaining decision was whether
to keep waiting for live multi-user samples before changing the repository baseline.
The owner approved the bounded pre-user waiver documented in Section 5 and ADR 0010.

Current target/default after this lane:

```text
Diagnosis default Champion: v3 / diag-config-5a4a13627e14b4cf
Diagnosis rollback:         v1 / diag-config-f492eb1c0c6676ae
Treatment default Champion: v2 / treat-config-f68eec9846664596
Treatment rollback:         v1 / treat-config-85718f8e90ac9d80
Active Challenger:          none
```

The old promotion records are preserved as immutable evidence for the completed
v1 -> latest transition; they are not rewritten into future rollout records.

### C2. 3D final visual/anatomy-boundary audit

Implementation is present and staging-validated. The active plan still owns final visual/anatomy-boundary acceptance and production asset-release validation. This is an acceptance/release task, not missing base architecture.

### C3. Generic Python JobWorker

Not a current goal. Go owns durable Job truth and invokes Python as bounded computation. A separate worker process should only be introduced for an actual scaling/availability need.

### C4. Knowledge management UI

The source registry/publication/review backend and operator capability exist. A dedicated management UI is absent, but current architecture does not require one for correctness; it remains optional product/operations work unless promoted to an active feature requirement.

## 9. Documentation drift cleaned in this audit

### D1. Historical architecture removed from current source-of-truth

Old full-text designs were copied into:

```text
docs/plan/archive/architecture-snapshots/2026-09-01/
```

and current paths were rewritten/redirected for:

- `technical-approach.md`
- `ai-output-governance.md`
- `context-engineering-architecture.md`
- `system-engineering-refactor-plan.md`
- `ai-run-job-runtime.md`
- `stream-event-contract-runtime.md`
- `agent-tool-calling-runtime.md`
- `feature_spec_training_schedule.md`

This removes old LangChain/PaddleOCR/PG16/Go ContextBuilder/old ToolExecutor/old Training schedule assumptions from current architecture while preserving history.

### D2. Accepted ADR implementation status corrected

- ADR 0001 now explicitly says its Agent runtime ownership detail was superseded by ADR 0002.
- ADR 0004 stale Follow-up TODOs were converted into an implementation outcome.
- ADR 0006 no longer says Vanatome implementation is pending.
- ADR 0007 explicitly records ADR 0009's later rule that stable Profile is **not** Assessment health evidence.

### D3. Current runtime/docs corrected

- JobRuntime now documents durable OCR/Posture jobs and restart recovery.
- StreamEvent doc now describes the implemented shared v1 contract and recovery path.
- Tool runtime now documents LangGraph checkpoint/interrupt ownership plus durable Go audit.
- Knowledge lifecycle uses `knowledge_sources/segments/units/clips/publications`, not `knowledge_entries`.
- `KnowledgeSourceRegistry` is marked implemented.
- Longitudinal domain/feature spec no longer carries completed migration TODOs.
- Training current spec now uses accepted TreatmentRevision -> execution projection -> Outcome/review semantics.
- 3D architecture/spec/active-plan index no longer say “ready for implementation”.
- Diagnosis phase docs now distinguish implemented rollout mechanics from default Champion serving.
- obsolete `DIAGNOSIS_AGENT_CONFIGURATION_ID` compatibility claims were removed; Treatment's legacy Champion alias remains because code still reads it.
- onboarding copy no longer claims Profile gender directly improves health conclusions, Assessment itself performs multimodal image analysis, or OCR extracts “micronutrients affecting rehabilitation”.

### D4. Reference drift corrected

Source comments pointing to removed active Posture plan now point to archive.

Immutable migrations 20/22 still reference their historical `docs/implementation/...` paths. Redirect documents were restored at those paths rather than mutating checksummed migration files.

## 10. Suggested implementation order

```text
P1 A1 Report indicator admissibility
   -> closes the strongest remaining Assessment evidence hole

P1 A2 Posture geometry mechanism identity
   -> restores reproducibility of authoritative numeric visual evidence

P1/P2 A3 OCR mechanism provenance
   -> makes report evidence auditable/replayable

P2 B1 Assessment v3 request/dependency hardening
   -> remove obsolete authority surface after safety-critical gaps are closed

P2 B3 Normalize ASR provenance

P2/P3 B4 Drop health_features schema residue after compatibility audit

P3 B5 Deduplicate Diagnosis gateway helper

B2 Assessment source-key semantics
   -> decide domain meaning first; implement only after decision
```

## 11. Definition of done for this alignment lane

Documentation cleanup is done when:

- [x] current architecture no longer presents historical Go ContextBuilder/LangChain/PaddleOCR/PG16/old route/schema designs as current;
- [x] accepted ADRs no longer say already-finished core migrations are pending;
- [x] feature specs distinguish current implementation from archived product ideas;
- [x] active-plan index reflects actual implementation status;
- [x] immutable migration references resolve without editing migration history;
- [x] all new/changed relative links pass repository link validation;
- [x] markdown/source formatting checks pass.

Code alignment is **not** complete until the P1 evidence/mechanism gaps above are implemented and qualified. This audit intentionally does not disguise those gaps by weakening the documentation.

### Validation evidence

```text
Markdown relative links: 0 missing
Source docs/...md references: 0 missing (excluding intentional delivery-test fake fixtures)
git diff --check: PASS
Changed TSX Prettier: PASS
Rewritten/new Markdown Prettier: PASS
pnpm nx typecheck web: PASS
pnpm nx test web: 41 files / 204 tests PASS

Pre-user baseline promotion validation:
- Go full suite + go vet: PASS
- Python unit suite: 399 passed
- Diagnosis historical promotion evaluator: PASS / no blocking reasons
- Treatment historical promotion evaluator: PASS / no blocking reasons
- Python default configuration: Diagnosis v3 / Treatment v2
- Dev/Staging/Prod Compose interpolation: latest Champion + empty Challenger + explicit v1 rollback
- Production-shaped Playwright: 10/10 PASS
- DIAGNOSIS_BASELINE_VALIDATION: PASS (latest=3, legacy=0, rollout_observations=0)
- TREATMENT_BASELINE_VALIDATION: PASS (latest=3, legacy=0, rollout_observations=0)
- LOCAL_DEPLOY_VALIDATION: PASS
```

The Web test target still emits pre-existing localhost connection noise and one React `act(...)` warning to stderr, but the Nx target exits successfully and all 204 tests pass.

## 12. Future audit rule

For every behavior-significant change:

```text
ADR / contract changes
  -> implementation
  -> tests/evals
  -> current architecture update in same PR
  -> old current text archived or removed
```

A document that describes a desired but unimplemented design must be labeled target/active-plan explicitly. It must never masquerade as a current implementation source-of-truth.
