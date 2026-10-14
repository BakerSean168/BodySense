# UI-TP-001 · Bind TrainingPlan/Trainer windows to real safety-gated API

Status: QUEUED / NOT ADMITTED. Phase 3. SF cross-reference: SF-80.
**Owner role:** React Training Owner. **Dependencies:** `TP-003`.
**Exclusive resources:** `training-ui`.

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
