# Phase 07 — Legacy runtime retirement

Status: **COMPLETE**

Branch: `refactor/vnext-07-legacy-retirement`

Canonical parent before Phase 07: `refactor/bodysense-vnext` at `ad27ac8fb`

## Goal

Phase 07 removes pre-vNext compatibility and migration-era runtime branches now that the public REST, public StreamEvent, internal runtime protocol, runtime trust boundaries, and durable state machines have stable canonical forms.

Historical artifacts remain only when they have explicit offline evaluation/research value and cannot be selected by production serving code. Operational rollback/recovery remains a current safety capability and is not treated as compatibility debt.

## Retirement ledger closure

### BS-VNEXT-LEGACY-002 — Assessment v1 runtime/public union

Status: **ARCHIVED OFFLINE / RUNTIME DELETED**

- public OpenAPI `AssessmentReport` now resolves only to `AssessmentReportV2` / `assessment-output-v2`;
- generated Web v1 report/observation/dimension-score schemas were deleted by code generation;
- Web Assessment no longer projects a v1/v2 union;
- Python serving accepts only the current Assessment output/evidence policy;
- Go replay accepts only current v2 reports/current runtime configuration and fails closed on retired source reports;
- `health_grade` was removed from the replay public projection. Historical database columns are schema baggage only and are scheduled for Phase 08 rebaseline rather than used by runtime logic.

### BS-VNEXT-LEGACY-003 — sessionless authentication

Status: **DELETED**

- application access tokens require a canonical `session_id`;
- middleware rejects an otherwise signed token without session identity instead of querying the database as a fallback;
- characterization verifies retired sessionless tokens fail closed.

### BS-VNEXT-LEGACY-004 — upload `file_path` compatibility

Status: **DELETED**

- Upload public/runtime storage authority remains opaque `storage_backend + storage_key`;
- the migration-only validator whose purpose was proving the historical `file_path -> storage identity` upgrade was deleted;
- the current local-to-OSS object migrator remains because it is an active operational storage migration capability, not a pre-vNext response compatibility path;
- Web contract tests explicitly fail closed if a nominal 200 response reintroduces `file_path`/private persistence fields.

### BS-VNEXT-LEGACY-005 — nullable/free-text BodyRegion compatibility

Status: **DELETED**

The BodyRegion ontology JSON remains the source of truth. Its Go validator generator now additionally emits an unambiguous alias -> canonical ID map.

Current durable semantics are:

- a fact/observation with no anatomical region may keep `body_region_id = null`;
- a localized write with an explicit ID must supply a valid canonical ID;
- a localized write with a unique ontology alias may omit the ID at the wire boundary and is canonicalized before persistence;
- an ambiguous or unresolvable localized value fails with `BODY_REGION_ID_REQUIRED` rather than persisting `free-text + null canonical id`;
- internal symptom/outcome projection never manufactures a canonical identity: uniquely resolvable regions are canonicalized, while unresolved source text remains in provenance/source state rather than a localized durable BodyState region field;
- current-context corrections and Assessment observations pass through the same canonicalization boundary.

The domain validator now proves ambiguous rejection, unique alias canonicalization, invalid-ID rejection, replay idempotency, temporal ID retention, laterality correction, historical correction preservation, and absence of localized current facts without a canonical ID.

### BS-VNEXT-LEGACY-007 — historical Agent manifests selectable by serving policy

Status: **ARCHIVED OFFLINE**

Historical manifests were physically moved from `apps/ai-service/config/agents/` to `apps/ai-service/data/evals/agent-configurations/`:

- Assessment v1/v2/v3/v4;
- Consultation v1;
- Diagnosis v1/v2 evidence-gap;
- Posture v1;
- Treatment v1.

The production serving directory now contains only current manifests plus current non-health roles:

```text
assessment-v5.yaml
consultation-v2.yaml
diagnosis-v3-decision-authority.yaml
knowledge-curator-v1.yaml
knowledge-splitter-v1.yaml
posture-v2.yaml
title-v1.yaml
treatment-v2-evidence-gap.yaml
```

Go serving registries likewise recognize only current runtime configurations. Retired Diagnosis/Treatment/Assessment/Consultation/Posture IDs cannot be selected as current/canary/rollback targets.

Historical Diagnosis qualification still reproduces the v1 tool behavior through `src/evals/retired_diagnosis_runtime.py`. Production `src/agents/diagnosis_agent.py` exposes only the current typed `acquire_evidence` tool/evidence-gap policy. This preserves evaluation evidence without leaving the historical tool branch runtime-addressable.

Treatment and Assessment replay reject retired source/target configurations. E2E includes a retired Treatment target negative probe that must return `422 UNKNOWN_AGENT_CONFIGURATION`.

## Additional migration-era runtime branches deleted

Phase 07 closeout found and removed several branches not fully enumerated in the initial ledger:

1. **Diagnosis pre-envelope transient public path** — `/consultations/{id}/diagnosis` is now strongly typed as `DiagnosisWorkspaceProjection`; the Web handwritten transient parser, Go pre-envelope authority branch, and transient `CounterfactualFrozen` comparison path were removed.
2. **Generic model-authored `ask_user` -> BodyState fallback** — only runtime-owned structured symptom binding may project an interaction answer into BodyState. Other answers remain durable in interaction/message events but cannot invent `user_answer` or `negative_finding` health facts.
3. **Lease-less running Run compatibility** — a `running` Run must own a non-expired lease. Missing/expired lease is reclaimed rather than treated as indefinitely live.
4. **Evidence retrieval alias** — persisted `no_results` alias was removed; `no_relevant_results` is canonical.
5. **Old product URL redirects** — explicit `/dashboard`, `/profile`, `/assessment`, `/history`, and `/training/:id` migration redirects were removed. Unknown routes use the normal current application fallback rather than a list of pre-vNext aliases.
6. **Unused Body Explorer adapter shape** — the unused singular `LoadedVanatomeAtlas.atlas` compatibility field was removed; the current composed `atlases` representation remains.
7. **Deployment rollback config envs for historical Diagnosis/Treatment** — retired historical Agent configuration pointers are no longer injected into serving Compose/local-deploy policy.
8. **Workbench BodyRegion submission drift** — the Web add-fact form previously allowed ambiguous free-text such as `颈肩` to reach the stricter durable boundary without a canonical ID. It now resolves through the shared ontology before mutation, rejects ambiguous/unresolved text, canonicalizes unique aliases, and offers canonical region labels through the input datalist.
9. **Diagnosis deterministic E2E stub drift** — the local deterministic Diagnosis stub still emitted the pre-evidence-gap payload. It now resolves the current serving manifest and emits the required `evidence-acquisition-trace-v2`; Go remains fail-closed instead of weakening the evidence availability gate for tests.
10. **HealthWorkspace training action still emitted a retired Web URL** — `open_training` no longer targets `/training/:id`; it targets the canonical `/consultation/<conversation>?view=treatment` workbench view. E2E fixtures were updated to test the current route rather than relying on a removed redirect.

## Explicitly retained and why

The final repository grep is intentionally not zero. Every remaining production marker is classified:

### Qualified Tesseract health-document mechanism — KEEP

The currently qualified Tesseract health-document mechanism remains the canonical safety-qualified baseline under `BS-VNEXT-KEEP-002`. Its immutable `legacy-regex-v1` parser revision string is part of frozen mechanism/provenance identity and cannot be renamed without changing artifact/configuration identity.

Code-facing names were clarified from `legacyTesseract*` / `Legacy` to Tesseract Champion/baseline terminology so the mechanism is not mistaken for migration scaffolding. The algorithm itself was not promoted/replaced because Phase 07 is forbidden from bypassing qualification evidence simply to remove a label.

### Offline retired Diagnosis runtime — ARCHIVE-OFFLINE

`apps/ai-service/src/evals/retired_diagnosis_runtime.py` deliberately contains historical `diagnosis-tools-legacy-v1` / `diagnosis-evidence-legacy-v1` identifiers. It is eval-only and is not imported by the serving Diagnosis factory.

### External/protocol compatibility — KEEP

References such as OpenAI-compatible provider APIs, H.264 codec compatibility, S3-compatible object storage, generated-library `Deprecated` comments, and deployment/recovery rollback are interoperability or operational-safety concerns, not pre-vNext BodySense runtime compatibility branches.

### Current local upload storage backend — KEEP UNTIL INFRASTRUCTURE CUTOVER

`local` upload storage is not retained as an old-client compatibility reader. It remains the currently deployed storage backend: the repository runbook records that the real Alibaba private Upload OSS bucket/ECS RAM Role cutover has not yet been provisioned and production is still configured with `UPLOAD_STORAGE_BACKEND=local`. The durable authority is already `storage_backend + storage_key`, so no host `file_path` leaks back into the public/domain contract. The local-to-OSS migrator remains an operational cutover tool. Phase 07 therefore removes the obsolete `file_path` migration validator and misleading compatibility terminology, but does not fabricate an OSS production cutover without real cloud-side credentials/evidence.

### `health_features` and old migration chain — PHASE 08

Migration-era schema columns and the pre-vNext migration chain are intentionally deferred to the schema rebaseline. Application authority has already moved to BodyState/current domain structures; Phase 08 will remove the schema baggage rather than pretending it must be migrated for users that do not exist.

## Contract and route cleanup

- OpenAPI transitional Phase-02 wording was removed; the generated browser boundary is now described as the single current public REST contract;
- Diagnosis analyze response is a strict generated `DiagnosisWorkspaceProjection`, not generic JSON;
- generated contract artifacts were regenerated deterministically;
- public route coverage remains **95/95 (100%)**, with no retired public routes;
- the two existing Redocly ambiguous-path findings remain warnings only and are unrelated to Phase 07 retirement.

## Verification completed before production-shaped acceptance

- `pnpm lint` — PASS;
- `pnpm typecheck` — PASS;
- `pnpm test` — PASS: AI **507/507**, Web **266/266**, Contracts **18/18**, Go full suite PASS;
- `pnpm build` — PASS;
- `pnpm contracts:verify` — PASS;
- `pnpm contracts:route-coverage:check` — PASS, **95/95**;
- `bash scripts/validate-migration-history.sh` — PASS through migration 63, immutability PASS;
- `go vet ./...` — PASS;
- `git diff --check` — PASS;
- focused retired/runtime-boundary regressions — PASS, including current-only Consultation manifests, Diagnosis eval/runtime separation, Treatment/Assessment retired replay rejection, canonical BodyRegion enforcement, lease-less Run reclamation and session-aware authentication.

## Production-shaped local-deploy acceptance

Status: **PASS**

`pnpm validate:local-deploy` completed successfully from a fresh production-shaped local stack with `LOCAL_DEPLOY_VALIDATION=PASS`. Acceptance evidence includes PostgreSQL 18 full-up/latest-down/replay-up through migration 63, off-host DR unit and restore gates, domain semantics, current Agent baseline checks, service health, and Playwright **10/10 PASS** including API process-restart recovery and the real 3D Body Explorer vertical.

Phase 07 is therefore complete and ready to integrate into `refactor/bodysense-vnext`.
