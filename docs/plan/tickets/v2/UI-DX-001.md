# UI-DX-001 · Bind the Diagnosis V2 window to exact decision API

Status: BLOCKED (user Q-19 visual sign-off, 2026-10-18) · lane A · #349 #350 #351 merged, last merge `576201e85`. Phase 3. SF cross-reference: SF-80.
**Owner role:** React Diagnosis Owner. **Dependencies:** `DX-003`.
**Exclusive resources:** `diagnosis-ui`.

## Domain closeout · 2026-10-18

Synced on main `3dd993a02`. #312 has 2 of 3 scope checkboxes ticked; the issue stays open.

| Slice                | PR                                                                       | Delivered                                                                                                                       |
| -------------------- | ------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------- |
| 1 · bind window      | [#349](https://github.com/BakerSean168/BodySense/pull/349) (`e86e0ff2d`) | `DiagnosisWindow` reads `GET /diagnosis`; per-proposal decide + whole-run settlement; committed only after a settlement receipt |
| 2 · states + E2E     | [#350](https://github.com/BakerSean168/BodySense/pull/350) (`78f04e997`) | Penpot PD-DX-01 states, 390px layout, Chromium E2E (commit, Safety lock, 390px)                                                 |
| 3 · Penpot alignment | [#351](https://github.com/BakerSean168/BodySense/pull/351) (`576201e85`) | serial stepper per the Penpot "03 App" Diagnosis boards; receipt / no-change / Safety stopped / empty states                    |

**Remaining:** the user's Q-19 visual sign-off. Known follow-ups outside the checkboxes: a start-analysis entry from Chat, Safety → Restricted navigation, and Diagnosis History.

## Current remaining scope · 2026-10-16

**GitHub:** [#312](https://github.com/BakerSean168/BodySense/issues/312) (epic [#301](https://github.com/BakerSean168/BodySense/issues/301)) · Wave 3 · size M · blocked by: DX-003 #310 · blocks: QA-001 #316, REL-001 #317.

Code-verified on main `85b9e68a6`. Big-bang policy (Master §0): edit `000001_vnext_baseline` in place, no numbered migrations, no rollback work. The steps further below are historical decomposition; deliver only this scope.

**Missing:**

- 绑定 V2 read projection + decide command；partial selection UI
- 状态：pending / undecided / changed (stale) / rejected / committed（receipt 后）
- Q-19 visual sign-off（Penpot）+ 390px

**Slices (one PR each):**

1. web: bind DiagnosisWindow to /diagnosis + decide — Vitest VM：stale 409、partial selection、无 receipt 不显示 committed
2. web: Penpot states + E2E — Chromium：proposal → decide → committed；urgent safety 下 decide 被禁用

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
