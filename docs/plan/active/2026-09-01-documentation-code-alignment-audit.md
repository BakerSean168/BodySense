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

- [x] Separate historical v1 IDs from the mutable/default Champion constants in Go.
- [x] Make Diagnosis v3 and Treatment v2 the repository defaults.
- [x] Introduce explicit rollback configuration identities instead of using the `Challenger` slot as a rollback alias.
- [x] Allow the stable Champion state to have no active Challenger. Require a distinct repository-known Challenger plus a matching future promotion record before any future `shadow/canary/promoted` stage.
- [x] Keep historical v1/v2/v3 manifests immutable for replay.
- [x] Update Compose/dev/staging/prod defaults and deployment validation so clean environments serve the latest Champion without an operator override.
- [x] Update rollout/provenance docs so `Champion`, `Challenger`, rollback target and historical promotion evidence are distinct concepts.
- [x] Re-run qualification/policy tests, Go rollout tests, longitudinal E2E and repository release verification, then validate Staging and Production against immutable release/runtime identities. Control-plane-only commits may advance `main`/Staging without manufacturing another production release when application runtime behavior is unchanged.

### Release-pipeline bookkeeping issue discovered during promotion

The first post-merge `Prepare Release` attempt exposed a delivery-control-plane drift rather than an Agent regression: PR #140 (`chore(main): release 0.9.0`) was still labeled `autorelease: pending` even though the Published `v0.9.0` tag resolves exactly to that PR's merge SHA. release-please therefore correctly refused to create another Release PR because it interpreted #140 as an outstanding untagged release.

The historical GitHub metadata was reconciled to `autorelease: tagged` only after proving tag/SHA identity. Release Publish is also being hardened with an idempotent post-publish reconciliation job: a Release PR may become `autorelease: tagged` only after Published Release state and exact tag SHA are both proven, and a workflow retry can repair bookkeeping after publication without rebuilding or rewriting artifacts.

The first v0.10.0 publication proved the release/tag/manifest boundary but exposed a CLI wiring bug in that reconciliation job: `gh api` does not forward `jq --arg` parameters. The release itself remained Published and immutable; the retry path is corrected to pipe the API response into standalone `jq -r --arg`, then re-run against the existing candidate/release without rebuilding artifacts.

### Acceptance criteria

- [x] a clean `AgentDeploymentPolicy` serves Diagnosis v3 and Treatment v2 in `champion` stage;
- [x] no current config calls Diagnosis v1 or Treatment v1 the active Challenger;
- [x] explicit rollback serves the historical v1 configuration;
- [x] future non-Champion rollout without a distinct qualified Challenger fails closed;
- [x] historical v1 -> latest promotion artifacts remain unchanged and their sync tests use historical IDs rather than current default aliases;
- [x] Dev/Staging/Production serve the latest Diagnosis/Treatment Agent configurations after deployment;
- [x] rollback remains a one-step configuration operation with no schema/data rollback.

---

## 6. Highest-priority remaining code gaps

### P1 — A1. OCR report indicators bypass an explicit admissibility/review gate — RESOLVED

**Classification:** `RESOLVED-CODE-GAP`
**Decision:** ADR 0011.

The original problem was real: `ocr_status=completed` caused every parsed report indicator to enter the Assessment evidence catalog, while BodyState candidates already had explicit review/lifecycle eligibility. Missing indicator confidence also defaulted to `high`, which could fail open.

The implemented contract now separates extraction state from evidence authority:

```text
OCR job completed
  -> persist all extracted indicators
  -> ocr-indicator-admissibility-v1
       high OCR + high indicator + valid name/value -> admissible
       medium/low/unknown                          -> needs_review
       malformed                                   -> rejected
  -> Assessment v4 / assessment-evidence-contract-v3
       only exact admissible provenance enters catalog
  -> Python gate
  -> Go rebuild/revalidation
  -> durable projection
```

Current immutable Assessment identity:

```text
assessment-v4
assess-config-e579030c2b8b540c
output = assessment-output-v2
evidence = assessment-evidence-contract-v3
```

The previous v3 config (`assess-config-c6cfff22aa362fff`) remains repository-known with evidence-contract v2 for historical replay/counterfactual comparison only and cannot serve durable reports. No relational migration/backfill invents admissibility for legacy OCR JSON; old report indicators without the new metadata fail closed under v4 until reprocessed or later reviewed.

Validation evidence:

- Python OCR/admissibility + Assessment focused tests: PASS;
- actual `/api/ocr/extract` route emits `evidence_admissibility`;
- current Assessment deterministic qualification: `assessment-evidence-contract-v3` **9/9 PASS**;
- Python full suite: **407 passed**;
- Ruff: PASS;
- Pyright `src`: PASS;
- Go Assessment focused admissibility/serving/replay tests: PASS;
- Go full suite + `go vet`: PASS;
- Go vertical fixture proves `UserUpload.OCRResult -> assessmentInputsFromUploads -> evidence catalog` preserves review-required metadata but excludes it from current durable evidence;
- Web OCR compatibility tests: historical indicators without admissibility metadata fail closed to `needs_review`, while explicit admissible indicators display `可用于评估`;
- production-shaped Assessment E2E: **2/2 PASS** with durable `assess-config-e579030c2b8b540c` / `assessment-evidence-contract-v3`;
- production-shaped full Playwright suite: **10/10 PASS**;
- `LOCAL_DEPLOY_VALIDATION=PASS`.

This closes **admissibility**, not **mechanism provenance**. OCR engine/parser/PDF-rendering/extractor version identity remains open as A3 below.

---

### P1 — A2. Posture geometric perception mechanism identity — RESOLVED

**Classification:** `RESOLVED-CODE-GAP`
**Decision:** ADR 0012.

The original gap was larger than the initial documentation audit showed. Posture v1 did not fingerprint the geometric mechanism, used a runtime `.../latest/...` pose-model URL and accepted any non-empty cached model. A production-shaped inspection also proved the deployed AI image installed only the OCR optional dependency: both Staging and Production lacked `mediapipe`, so the same v1 configuration could mean VLM-only in deployment and VLM + geometry in another environment.

The first v2 Docker smoke exposed one more native-runtime dependency: `mediapipe==1.0.0` and the pinned artifact were present, but OpenCV could not load because `libGL.so.1` was missing. The final image now includes the minimal `libgl1` dependency, and validation constructs/runs the actual PoseLandmarker rather than merely checking Python package metadata.

The implemented contract is:

```text
Posture v2 / posture-config-efa3a84622818772
  -> geometry required
  -> mediapipe-tasks 1.0.0
  -> versioned pose_landmarker_lite/float16/1 artifact
  -> model SHA256 = 59929e1d1ee95287735ddd833b19cf4ac46d29bc7afddbbf6753c459690d574a
  -> canonical threshold SHA256 = 588917b4a071ee1e249d3930b37769c9c9bd7a4fdebd68eb2a00bfdd13fbb140
  -> build/startup provisioning only; request path never downloads
  -> runtime engine/model/threshold verification before VLM generation
  -> Python mechanism_provenance
  -> Go exact mechanism revalidation
  -> generation_decision_trace
  -> user_uploads.analysis_result
```

Historical v1 remains repository-known as `posture-config-3a774008db422a31`; its fingerprint is unchanged, but it is non-serving and cannot be selected as the current Posture Champion.

No relational migration/backfill is required. Current provenance fits the existing `user_uploads.analysis_result` JSONB and exact `agent_configuration_id`; historical analyses are not given invented geometry provenance.

Validation evidence:

- Python historical v1 identity remains `posture-config-3a774008db422a31`;
- Python current v2 identity is `posture-config-efa3a84622818772`;
- changing model/threshold identity changes the Posture configuration ID;
- artifact SHA mismatch and threshold drift fail closed;
- required geometry failure stops before the VLM call;
- focused Posture Python tests: **27/27 PASS**;
- Python full suite: **414 passed**;
- Ruff: PASS;
- Pyright `src`: PASS;
- Go Posture deployment/response gate tests: PASS;
- Go full suite + `go vet`: PASS;
- historical v1 cannot serve as Champion, but remains repository-known;
- final AI Docker image contains `mediapipe==1.0.0`, the pinned model, `libgl1`, and successfully constructs the real `PoseLandmarker`;
- blank-image geometry smoke executes OpenCV decode + MediaPipe detect and returns zero metrics with exact verified mechanism provenance;
- production-shaped `POSTURE_GEOMETRY_MECHANISM=PASS` with the exact current config/model/threshold identities;
- production-shaped migration/domain validators: PASS;
- production-shaped Playwright: **10/10 PASS**;
- `LOCAL_DEPLOY_VALIDATION=PASS`.

This closes Posture geometry mechanism identity. OCR mechanism provenance remains open as A3 below.

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

**Classification:** `DEPLOYMENT-STATE` — **COMPLETE** under ADR 0010.

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

Final environment evidence:

```text
Published Release: v0.10.0
Runtime release SHA: d25ea0fd8c95bbb5a7c7b8462a5f255e00bcc1f6
Production deploy workflow: 33484845107 PASS
Production .deploy-state: d25ea0fd8c95bbb5a7c7b8462a5f255e00bcc1f6
Production schema_migrations: version=60 dirty=false
Production public /api/health: PASS
Production pre-deploy DB dump: bodysense-pre-d25ea0fd8c95-20260901-080611.dump
Production previous-runtime backup: 61553a40c362597061a6701570c6e8aaf0b62895-20260901-080612
Diagnosis Champion: diag-config-5a4a13627e14b4cf
Treatment Champion: treat-config-f68eec9846664596
Active Challenger: none
```

`main`/Staging later advanced to `b7c68520847fbfeaa251bbbe97828c6c8120a2be`
for the idempotent release-please bookkeeping hotfix. That commit changes the
GitHub Release control plane plus docs/tests only; the application runtime and
Agent baseline are the same as the `v0.10.0` release, so Production correctly
remains on the immutable Published Release rather than receiving a synthetic
patch release with no runtime behavior change.

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

### Validation-infrastructure drift discovered while qualifying A1 — RESOLVED

The first production-shaped A1 validation exposed an unrelated hermeticity bug in `scripts/local-deploy-validate.sh`: disposable runtime credentials such as `DB_PASSWORD=bodysense123` were exported **before** `validate-repo.sh`. The off-host DR unit suite intentionally uses its own fixed secret to prove password argv isolation, so the outer runtime variable overrode the hermetic fixture and produced a false failure.

The validator now runs repository quality before exporting any production-shaped Agent/DB/Redis/JWT variables. A delivery regression test asserts the ordering. Re-running the full validator produced `offhost DR unit tests: PASS=86 FAIL=0`, `REPO_QUALITY=PASS`, 10/10 Playwright and `LOCAL_DEPLOY_VALIDATION=PASS`. This is a test-isolation fix; it does not change production secret precedence or runtime deployment semantics.

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
pnpm nx test web: 42 files / 206 tests PASS

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
- Published Release `v0.10.0`: `d25ea0fd8c95bbb5a7c7b8462a5f255e00bcc1f6`
- Release manifest/tag/Published Release identity: PASS
- Release-please bookkeeping retry: PASS; PR #152 = `autorelease: tagged`; release manifest byte identity unchanged
- Production Deploy workflow `33484845107`: PASS
- Production `.deploy-state` / Web / API / AI OCI revision: `d25ea0fd8c95bbb5a7c7b8462a5f255e00bcc1f6`
- Production migration head: `60`, `dirty=false`
- Production pre-deploy DB dump + previous-runtime backup: PASS
- Production public `/api/health`: PASS
- Production explicit Agent env: Diagnosis v3 Champion / Treatment v2 Champion / Challenger empty / v1 rollback
- OCR indicator admissibility: `ocr-indicator-admissibility-v1`
- Current Assessment: v4 / `assess-config-e579030c2b8b540c` / `assessment-evidence-contract-v3`
- Assessment evidence qualification: 9/9 PASS

OCR admissibility production closeout:

- A1 implementation PR #155 merged as `27c917dadd925f6efc86933cabe4f5a4d865392d`;
- release PR #156 produced Published Release `v0.10.1` at `afae3d3adf9e9584e6a4864468b5fc2d39664525` and is labeled `autorelease: tagged`;
- release tag `v0.10.1` resolves exactly to `afae3d3adf9e9584e6a4864468b5fc2d39664525`;
- main CI, `Publish Main Candidate`, and `Release Publish` for the release SHA: PASS;
- Staging Web / API / AI OCI revision: `afae3d3adf9e9584e6a4864468b5fc2d39664525`;
- production Deploy workflow `33499842273`: PASS;
- production watcher preflight: READY with `active_running=0`;
- production `.deploy-state` / Web / API / AI OCI revision: `afae3d3adf9e9584e6a4864468b5fc2d39664525`;
- production Web / API / AI containers: running + healthy;
- production migration head: `60`, `dirty=false`;
- production pre-deploy database backup: `bodysense-pre-afae3d3adf9e-20260901-110535.dump`, validated at schema `60:false`;
- production previous-runtime backup: `d25ea0fd8c95bbb5a7c7b8462a5f255e00bcc1f6-20260901-110536`;
- production public `/api/health`: PASS; public Web: HTTP 200;
- production Assessment default: v4 / `assess-config-e579030c2b8b540c` / `assessment-evidence-contract-v3`.
```

The Web test target still emits pre-existing localhost connection noise and one React `act(...)` warning to stderr, but the Nx target exits successfully and all 206 tests pass.

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
