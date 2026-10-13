# BodySense V2 · Integration & Release Workstream

Status: ACTIVE / TRACK D. [Primary program](./2026-10-13-v2-parallel-master.md) · [Readiness register](./v2-handoff-state.json) · [Existing acceptance](../../product/acceptance.md).

## Mission

Integrate Product Design and Engineering **continuously for each domain**, then retire legacy consumers and qualify a full rollout. Do not postpone all UI/code contact until every backend domain is finished. Do not equate a green build, screenshot, transport response, Penpot mock, or additive migration with a working product.

## Accepted interface boundaries

Use [docs/product](../../product/README.md) for business acceptance, Track A's exact query/command projection for data semantics, Track B's editable Penpot handoff for interaction, and Track C's real Go/Python/PostgreSQL/React path for behavior. Track D verifies them jointly and owns the release gate, not another domain model.

Each domain integration gate requires:

1. **Contract match:** front-end visible fields/derived counts map to authenticated Go projections; source/evidence/absence/expired distinctions survive generated DTOs; no private revision or invented clinical grade displayed.
2. **Stateful user path:** empty/populated/loading/error/stale/forbidden/reconnect, explicit consent, conflict/retry/idempotency, resume/cancel/Ask User and pending vs committed states where applicable.
3. **Negative/security tests:** user isolation, unauthorized source, failed or partial write, unapproved Diagnosis/Training modifications, SafetyState block, erased/copy-retained source, and adverse ordering/replay.
4. **Visual/function:** editable Penpot baseline, real desktop and mobile viewer screenshots, floating window over 3D body, keyboard/focus/scroll responsiveness; final pixel spacing polish follows semantic acceptance.
5. **Proof and cleanup:** reproducible test commands/logs, exact SHA, reviewed legacy route/table/model consumers and a safe retirement/migration plan. A closure requires evidence in handoff JSON, not just a changed checkbox.

## Executable tickets

### IR-BASE-01 — Baseline matrix and UI PR repair audit

**Scope:** Map [product acceptance](../../product/acceptance.md) to SF-domain tests and active routes; inventory old BodyState/Diagnosis/Treatment/Conversation/Training consumers. Read [#244](https://github.com/BakerSean168/BodySense/pull/244) failed Governance and browser E2E logs, reproduce specific failures, review UI worktree at `3597a5f29`. **Acceptance:** fault/contract/evidence matrix with P0–P3 severity and focused repair tickets; no blind PR merge; running checks distinguish from historical checks.

### IR-BS-01 — First BodyState E2E and UI handoff

**Dependencies:** DS-BS-01/02, PD-BS-01, ENG-BS-01/02. **Scope:** user statement in real Chat → typed Python extraction → Go provenance and PostgreSQL commit → receipt/outbox → actual Person Records/Habits refresh, Episode correction/withdrawal. **Acceptance:** valid + ambiguous/negative/third-party + stale/replay + rejected write + mobile/desktop; no false 'saved'. Record exact tested API/AI/Web SHA.

### IR-PC-01 — One Chat and Person continuity

**Dependencies:** PD-PC-01 and ENG-PC-01. **Scope:** one Conversation across devices, pending Ask User recovery, retention/source transfer, no public health conversation sharing. **Acceptance:** no second concurrent Run and no source loss on pruning; no UI list/create/share remnants.

### IR-DX-01 — Diagnosis V2 approval and visual integration

**Dependencies:** PD-DX-01 and ENG-DX-01. **Scope:** patient-facing condition count, source/AI distinctions, exact proposals, stale decisions, safety interrupts, history. **Acceptance:** all decisions applied atomically once; a prior accept never authorizes a changed proposal; Q-19 visuals actually reviewed.

### IR-TP-01 — Training/Trainer/Safety vertical

**Dependencies:** PD-TP-01 and ENG-TP-01. **Scope:** accepted plan vs proposal, multi-plan isolation, per-set user actions, paused/resume recovery, fail-closed safety and timezone. **Acceptance:** no false completion or accidental automatic clearance.

### IR-ASSET-01 — Shared media and erasure

**Dependencies:** PD-ASSET-01 and ENG-ASSET-01. **Scope:** equipment, reports/media processing/admission, removal vs erasure, controlled copies and cross-domain evidence. **Acceptance:** source unavailability and deletions do not misrepresent clinical authority or 'erased' before copies verified.

### IR-FINAL-01 — Legacy retirement and full release

**Dependencies:** all integration gates ready. **Steps:** enumerate exact old tables/URLs/models/events; confirm each target consumer migrated; execute isolated native migrations and prod-like restore/recovery; review negative/security/E2E matrix; run required exact-SHA CI; qualify GCP staging; validate actual BodySense host/storage/database ownership for any authorized destructive reset; promote a tested immutable artifact via existing protected workflow; verify production health and rollback. **Acceptance:** no obsolete domain route/schema still serving target product except an explicitly documented transitional consumer, all blockers closed, independent production evidence. **No production reset is implied by writing a plan.**

## Existing CI and deployment architecture

- PR verification: `.github/workflows/ci.yml` (delivery manifest/oracles, Go/Python/React quality lanes, PostgreSQL 18 baseline/recovery, longitudinal browser E2E).
- Repository gate: `scripts/validate-repo.sh`, `scripts/validate-migration-history.sh`, `scripts/validate-body-fact-ledger.sh`; selected contracts from `scripts/contracts`.
- Release authority: `docs/architecture/delivery-platform-v3.md`, `docs/architecture/release-lifecycle-v3.md`. Staging/dev reside on verified GCP ownership; production is Alibaba ECS; Oracle2 is not BodySense runtime. Do not infer live production from old documentation.

Baseline 2026-10-13: #247 code PR is Draft (audited first two migrations/Go/API and message provenance verified) with exact prior head's CI passed; #244 has failing Governance/Experience tests. **No domain's full V2 integration gate is ready; no V2 staging or production deployment.**

## Continuous review protocol

Review by verified root cause, not vague screenshots. Each finding names severity (P0 security/data or unsafe behavior, P1 missing required behavior, P2 robustness, P3 visual polish), exact file/route/event/flow, violated contract, reproduction, owner A/B/C and target test. Fix in small independent repair passes, rerun focused gate then affected full suite; mark fixed/partially fixed/not reproduced/deferred with reason. Do not create simultaneous writers on the same workspace.

**Next:** IR-BASE-01 may run without waiting for DS-BS-01. UI styling finalization is intentionally deferred; UI behavior and safety gate acceptance is not.
