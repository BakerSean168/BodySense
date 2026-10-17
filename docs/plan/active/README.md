# Active Plans

The coordinated pre-user vNext engineering reset completed through Phase 11 and passed fresh PostgreSQL 18 production-shaped whole-system validation. Its master plan is archived and no longer owns current implementation work.

- [`../archive/2026-09-13-bodysense-vnext-engineering-reset.md`](../archive/2026-09-13-bodysense-vnext-engineering-reset.md) — **COMPLETE / ARCHIVED**; records the cross-language contract/state/runtime/schema reset on `refactor/bodysense-vnext`, including Phase 11 final simplification and `LOCAL_DEPLOY_VALIDATION=PASS`.
- [`../archive/2026-09-28-diagnosis-structured-safety-semantics.md`](../archive/2026-09-28-diagnosis-structured-safety-semantics.md) — **COMPLETE / ARCHIVED**; closes DGS-SAFE-000 through DGS-SAFE-090 with Diagnosis v10 accepted as staging Champion, v3-v7 historical identity/replay preserved, merged-image v10→v3→v10 rollback plus protected-history immutability proven, and production promotion intentionally held pending production-provider qualification.

Current active plans keep their own product/operations acceptance owners:

> **Current execution policy (2026-10-16):** Master §0’s big-bang single designated writer supersedes the historical parallel pilot and scheduler snapshots below. PR6 A–E is complete through #298; the #299 status sync and #274 (TP-PROP-001) are merged; #249 (CI action pin bump) merged; queue/handoff are synchronized at main `6687a9e26`. The code-verified remaining work, order and slices are in the [**2026-10-16 V2 remaining-work plan**](./2026-10-16-v2-remaining-plan.md). This update does not inspect or change live scheduler state.

- [**BodySense V2 Parallel Full-Stack Rebuild · Master Plan**](./2026-10-13-v2-parallel-master.md) — **ACTIVE / PRIMARY V2 PROGRAM**. **2026-10-15: big-bang rebuild mode (Master §0): no data migration/rollback, single baseline, Hark sole writer, one vertical PR per domain.** One domain/Schema foundation, parallel Product Design and Engineering, continuous per-domain Integration/Release. The previous single hourly supervisor is disabled. As observed on 2026-10-15, Track A (Domain/Schema) and Track B (React) hourly automations are **disabled** and Track C (Backend) and Track D (Integration QA) are **enabled**; Track D is currently denied commit/PG18 starts by platform safety control. Do not infer future scheduler state from this static index; the scheduler is authoritative.
  - [A · Domain & Schema](./2026-10-13-v2-domain-schema.md) — domain persistence, presentation/read-model and command/event contracts.
  - [B · Product Design](./2026-10-13-v2-product-design.md) — editable Penpot, window flows, states, contract-based handoff.
  - [C · Engineering](./2026-10-13-v2-engineering.md) — Go/Python/React typed vertical slices and first functional UI.
  - [D · Integration & Release](./2026-10-13-v2-integration-release.md) — continuous domain E2E, visual QA, legacy removal, exact-SHA staging/production.
  - [**2026-10-16 V2 remaining-work plan**](./2026-10-16-v2-remaining-plan.md) — ACTIVE execution plan: per-ticket missing scope verified against main `adaa02607`, S/M/L sizes, mergeable slices, order/tracks, invariants and dev/merge conventions. Raises no gate.
  - [**2026-10-17 V2 overall progress checkpoint**](./2026-10-17-v2-overall-progress-checkpoint.md) — DATED GitHub/source-verified progress of Epic #301 / #302–#317: merged baseline, in-flight Draft PRs, domain readiness, dependency and QA/release blockers, and recovery anchors. This dated snapshot is not live authority or a gate promotion.
  - [**Shared domain handoff readiness**](./v2-handoff-state.json) — five product domains × six independently evidenced gates; validate with `pnpm plan:check`.
  - [**2026-10-15 implementation-documentation alignment audit**](./2026-10-15-v2-implementation-alignment-audit.md) — historical alignment checkpoint: verified main `e4160a9c7`, doc/code drift list (fixed in queue/handoff/index), code-quality review (owner-lock `FOR UPDATE` contention, dormant-feature register, dual JS/Go domain models) and the re-ordered P0/P1 execution wave.
  - [**V2 Execution Reorganization and Integration Control**](./2026-10-14-v2-execution-reorganization.md) — ACTIVE execution ledger; [implementation tickets](../tickets/v2/README.md) with a guarded DAG, exact integration SHA and T3 project admission checks. Subordinate to this Master, not a second business authority.
  - [**Episode explicit course-evidence status admission (v9)**](./2026-10-13-episode-status-admission.md) — bounded source-authorized status admission, migrations, negative-source/retry tests; not whole BodyState readiness or Penpot UI sign-off.
  - [**Episode recurrence relations (v10)**](./2026-10-13-episode-recurrence-relations.md) — source- and exact-user-authorized links between separate Episode histories, independent from clinical status and the final Penpot UI.
  - [**Direct user BodyObservation (v11)**](./2026-10-13-body-observation-direct-report.md) — independently sourced raw measurement/observation ledger with no clinical normalization, legacy dual write or final Penpot UI.
  - [**Exact user Observation amendments (v12)**](./2026-10-13-body-observation-amendments.md) — source-preserving correction and withdrawal of one direct user report with immutable historic evidence and no clinical disposition.
- [**PR6 native BodyState retirement closeout**](../archive/2026-10-16-pr6-e-closeout.md) — **A–E COMPLETE / MERGED through #298 (`af3e1993b`)**, registered as `PR6-001` in queue and `completedPrograms` in handoff. Current exact-main CI passes; full native browser suite 25/25 without skips/retries is recorded. Remaining V2 domains/design/privacy/release are separate work; no PR6 slice remains queued.
- [BodySense V2 Schema-First · SF Ticket Ledger](./2026-10-13-schema-first-full-refactor.md) — **ACTIVE / ENGINEERING IMPLEMENTATION DETAIL**, subordinate to the V2 master above. Preserves historical SF-00–90 tests/commits; the current PR6 completion and remaining-domain queue supersede old “next SF-12” instructions; not a second master or an hourly supervisor.

- [`2026-09-24-full-stack-upgrade-and-remote-dev-modernization.md`](./2026-09-24-full-stack-upgrade-and-remote-dev-modernization.md) — ACTIVE / CHECKPOINTS 1-4 LOCAL ACCEPTED / CHECKPOINT 5 COMPATIBLE SUBSET ACCEPTED / CHECKPOINTS 6-13 LOCAL ACCEPTED / BS-UPG-110 AND BS-UPG-120 PENDING; coordinated toolchain/dependency/runtime modernization, MemoFlow-derived Vite Bundled Dev for remote GCP development, staged Go/Python/React/3D/Redis/LiteLLM/image upgrades, and full local/staging/release acceptance.
- [`2026-09-01-documentation-code-alignment-audit.md`](./2026-09-01-documentation-code-alignment-audit.md) — ACTIVE; repository-wide ADR/current-doc/code alignment audit. Current documentation baseline has been cleaned; remaining P1/P2 evidence/mechanism/code gaps are explicitly tracked here.
- [`2026-09-01-health-document-technology-selection-and-benchmark.md`](./2026-09-01-health-document-technology-selection-and-benchmark.md) — ACTIVE / BENCHMARK-FIRST; defines BodySense-specific health-document corpus, OCR/PDF/layout/parser candidate evaluation, safety/resource gates, immutable mechanism selection, append-only extraction history and rollout. ADR 0013 remains Proposed until the OCR Champion is proven.
- [`2026-08-27-vanatome-3d-body-explorer.md`](./2026-08-27-vanatome-3d-body-explorer.md) — IMPLEMENTED / staging validated / final anatomy-boundary visual audit pending.
- [`2026-08-27-static-asset-cdn.md`](./2026-08-27-static-asset-cdn.md) — STAGING + CLOUDFLARE EDGE COMPLETE / production release validation pending.
- [`2026-08-27-observability-foundation.md`](./2026-08-27-observability-foundation.md) — ACTIVE; converge application/browser diagnostics on structured privacy-bounded logging and an OpenTelemetry-compatible observability path.
- [BodySense product specification](../../product/README.md) — the current business requirements, Agent interactions, UI behavior and acceptance criteria for product implementation.
- [`data-durability-backup-2026-08-25.md`](./data-durability-backup-2026-08-25.md) — ACTIVE / PARKED BY OWNER; real off-host PostgreSQL backup + restore drill and durable private user-upload object storage. Repository implementations are preserved, but cloud provisioning/cutover is intentionally postponed.

The broad post-`v0.5.2` production security/runtime/Knowledge closeout has been archived after the 2026-08-25 owner-approved scope split:

- [`../archive/production-security-runtime-knowledge-closeout-2026-08-23.md`](../archive/production-security-runtime-knowledge-closeout-2026-08-23.md) — archived; security/privacy/session/runtime/Knowledge/supply-chain/documentation implementation is closed, while the two real durability acceptance items were moved into the active plan above. Optional Knowledge production publication and Diagnosis promotion were closed with explicit non-rollout/HOLD dispositions rather than being represented as production acceptance.

## Canonical environment roles

- **GCP-dev** — primary development host and production operations control point.
- **GitHub Actions** — CI/release build plane.
- **Alibaba Cloud ACR** — production image registry.
- **Alibaba Cloud ECS (`body.bakersean.top`)** — sole production runtime.
- **Oracle2** — detached from BodySense.
- **DigitalOcean** — retired historical deployment path.

## Authoritative current architecture

- [`../../architecture/current-longitudinal-system.md`](../../architecture/current-longitudinal-system.md)
- [`../../architecture/model-gateway-routing.md`](../../architecture/model-gateway-routing.md)
- [`../../architecture/agent-platform-role-governance.md`](../../architecture/agent-platform-role-governance.md)
- [`../../architecture/deployment-architecture.md`](../../architecture/deployment-architecture.md)
- [`../../architecture/delivery-platform-v3.md`](../../architecture/delivery-platform-v3.md) — adopted production delivery architecture under ADR 0008; v0.9.0 proved affected PR CI, exhaustive main CI, exact-SHA candidates, canonical staging, Draft-first release publication and explicit production selection.
- [`../../architecture/release-lifecycle-v3.md`](../../architecture/release-lifecycle-v3.md) — explicit Prepare Release / Release Publish / Deploy Production authority boundaries.
- [`../../architecture/knowledge-lifecycle-architecture.md`](../../architecture/knowledge-lifecycle-architecture.md)
- [`../../architecture/body-explorer-3d-anatomy.md`](../../architecture/body-explorer-3d-anatomy.md) — implemented 3D body/anatomy architecture accepted by ADR 0006; remaining acceptance is the final visual/anatomy-boundary audit tracked by the active plan.
- [`../../architecture/body-region-ontology.md`](../../architecture/body-region-ontology.md) — implemented canonical body-region semantic boundary for 3D and durable BodyState integration.

A future model/prompt/tool/schema Challenger, Knowledge production-corpus rollout, or other bounded product change should start its own plan rather than being attached to the parked durability plan.
