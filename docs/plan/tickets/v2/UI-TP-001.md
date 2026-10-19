# UI-TP-001 · Bind TrainingPlan/Trainer windows to real safety-gated API

Status: DONE (ticket, 2026-10-19) · #363, #364, #369 merged, last merge `05ed96033`. Issue #315 closed. Domain gates raised at the V2 final closeout with QA-001 evidence.
**Owner role:** React Training Owner. **Dependencies:** `TP-003`.
**Exclusive resources:** `training-ui`.

## Final closeout · 2026-10-19

Synced on main `da14c3435`. [#315](https://github.com/BakerSean168/BodySense/issues/315) is closed with every scope checkbox ticked.

Merged: [#363](https://github.com/BakerSean168/BodySense/pull/363) (`fcdc025ed`), [#364](https://github.com/BakerSean168/BodySense/pull/364) (`b8b7c1740`), [#369](https://github.com/BakerSean168/BodySense/pull/369) (`05ed96033`).

TrainingPlan plans + proposals (#363), Trainer sessions + gate + pause/resume (#364), corrections, exercise skip, Ended feedback, Home chip, Training naming, 390px + Chromium E2E (#369).

Cross-domain re-verification: [QA-001](./QA-001.md) on main `da14c3435`.

## Slice sync · 2026-10-19

Synced on main `f2e52c79c`.

| Slice                              | PR                                                                       | Delivered                                                                                                                                              |
| ---------------------------------- | ------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1 · TrainingPlan plans + proposals | [#363](https://github.com/BakerSean168/BodySense/pull/363) (`fcdc025ed`) | plan picker over accepted revisions, read-only revision content, proposal accept/reject that updates only after the receipt, stale 409 → Expired board |
| 2 · Trainer sessions               | [#364](https://github.com/BakerSean168/BodySense/pull/364) (`b8b7c1740`) | Trainer window: session start, per-set receipts, TrainerGate states, pause/resume, Penpot Trainer layout                                               |

Remaining (slice 3): corrections, exercise skip, Ended feedback, Home trainer state, retire web Treatment naming. #315 stays open.

## Current remaining scope · 2026-10-16

**GitHub:** [#315](https://github.com/BakerSean168/BodySense/issues/315) (epic [#301](https://github.com/BakerSean168/BodySense/issues/301)) · Wave 4 · size M · blocked by: TP-003 #313 · blocks: QA-001 #316, REL-001 #317.

Code-verified on main `85b9e68a6`. Big-bang policy (Master §0): edit `000001_vnext_baseline` in place, no numbered migrations, no rollback work. The steps further below are historical decomposition; deliver only this scope.

**Missing:**

- 多 plan 列表 + plan detail（revision 内容只读）+ proposal accept/reject（receipt 后才更新）
- Trainer：session start、per-set performed/skipped、TrainerGate block/warn 禁用状态与原因
- 删除 Treatment UI（`TreatmentPanel.tsx`）；390px + Penpot states

**Slices (one PR each):**

1. web: plans + proposals — Vitest + Chromium：accept → receipt → 新 revision 显示；stale 409
2. web: trainer per-set actions + gate states — Chromium：block 时按钮禁用、重复点击幂等、390px

## Goal and why now

Bind TrainingPlan/Trainer windows to real safety-gated API. This is an execution slice under the approved BodySense V2 Master; it does not approve clinical decisions or establish any readiness gate by itself.

## Scoped paths / ownership

- `apps/web/src/features/body-canvas/components/TrainingPlanWindow.tsx`
- `apps/web/src/features/body-canvas/components/TrainerEntryGuard.tsx`

**Out of scope:** Other owners' dirty worktrees, unrelated source files, direct production changes, automatic clinical authorization, and any unapproved legacy contract restoration. Public OpenAPI, generated clients and migrations require a serialized designated owner before mutation.

## Ordered implementation

1. Bind separate Plan and Trainer windows to typed query/command projections.
2. Never treat a suggestion as completed; real user action with idempotent receipt required.
3. Disable actionable commands on missing or stale Safety Gate and show meaningful recovery state.
4. Verify persistent Canvas/window lifecycle on desktop and 390px.

## Verification commands and negative cases

- React plan/trainer component and keyboard-focused tests
- Real PG18/Go/Chromium per-set and restriction negative E2E

## Acceptance evidence

- Explicit user action and safety-versioning survive reload; no unsafe allow fallback.

**Required handoff:** execution identity/model; actual cwd/branch; input and output exact SHA; changed file list; tests with exit codes and logs; uncommitted diff disclosure; code review; pending gates/owner. Only an Integrator may accept a new shared schema/contract into the integration candidate.

**Protected invariants:** owner identity and source provenance; commit receipt before success; revision + idempotency; source deletion distinct from erasure; Go-owned medical authority and fail-closed Safety; one writer per checkout. No invented Diagnosis Q-17 TTL, clinical thresholds, or old BodyConcern.

**Scheduling constraint:** even when DAG dependencies pass, do not run in parallel with any other ticket sharing `training-ui` or any other listed exclusive resource. Ticket does not authorize T3 to use canonical root as a shared writer.
