# UI-DX-001 · Bind the Diagnosis V2 window to exact decision API

Status: QUEUED / NOT ADMITTED. Phase 3. SF cross-reference: SF-80.
**Owner role:** React Diagnosis Owner. **Dependencies:** `DX-003`.
**Exclusive resources:** `diagnosis-ui`.

## Goal and why now

Bind the Diagnosis V2 window to exact decision API. This is an execution slice under the approved BodySense V2 Master; it does not approve clinical decisions or establish any readiness gate by itself.

## Scoped paths / ownership

- `apps/web/src/features/body-canvas/components/DiagnosisWindow.tsx`
- `apps/web/src/features/consultation/hooks/useDiagnosisActions.ts`

**Out of scope:** Other owners' dirty worktrees, unrelated source files, direct production changes, automatic clinical authorization, and any unapproved legacy contract restoration. Public OpenAPI, generated clients and migrations require a serialized designated owner before mutation.

## Ordered implementation

1. Use approved DX read projection and atomic decision command; no direct SQL-table-to-window mapping.
2. Represent pending, undecided, changed, rejected, consented and committed proposals distinctly.
3. Keep UI read-only until API/gate valid; collaborate with Penpot Owner on Q-19.
4. Test desktop/390px keyboard and interruption/refetch.

## Verification commands and negative cases

- React Diagnosis focused Vitest and Playwright actual Go API
- A11y focus/keyboard and staleness negative tests

## Acceptance evidence

- UI never claims confirmed diagnosis before exact commit receipt.
- Q-19 visual sign-off is separate from business authorization.

**Required handoff:** execution identity/model; actual cwd/branch; input and output exact SHA; changed file list; tests with exit codes and logs; uncommitted diff disclosure; code review; pending gates/owner. Only an Integrator may accept a new shared schema/contract into the integration candidate.

**Protected invariants:** owner identity and source provenance; commit receipt before success; revision + idempotency; source deletion distinct from erasure; Go-owned medical authority and fail-closed Safety; one writer per checkout. No invented Diagnosis Q-17 TTL, clinical thresholds, or old BodyConcern.

**Scheduling constraint:** even when DAG dependencies pass, do not run in parallel with any other ticket sharing `diagnosis-ui` or any other listed exclusive resource. Ticket does not authorize T3 to use canonical root as a shared writer.
