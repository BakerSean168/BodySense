# BS-HYP-001 · Finalize Observation/Hypothesis/ConcernGroup projections

Status: QUEUED / NOT ADMITTED. Phase 1. SF cross-reference: SF-20.
**Owner role:** Domain Contract Owner. **Dependencies:** `INT-001`.
**Exclusive resources:** `domain-contract`.

**2026-10-16 code check:** remaining = hypothesis–observation links (`RelatedObservationIDs` is always empty), hypothesis correct/supersede and DC-032 stable ConcernGroup identity; Web UI moves to UI-001. Size S. See [2026-10-16 remaining-work plan](../../active/2026-10-16-v2-remaining-plan.md).

## Current remaining scope · 2026-10-16

**GitHub:** [#304](https://github.com/BakerSean168/BodySense/issues/304) (epic [#301](https://github.com/BakerSean168/BodySense/issues/301)) · Wave 1 · size S · blocked by: none · blocks: BS-AGENT-001 #308, UI-001 #309.

PR #283 already implemented direct-edit user BodyHypothesis and derived ConcernGroup reads and moved Agent hypotheses into Diagnosis audit. PR6 A–E is complete. This ticket retains only full projection/source variants, stable ConcernGroup identity, presentation and contract acceptance; do not recreate those tables/APIs.

The original steps below are historical decomposition; perform only their unfulfilled acceptance work, under the Master’s single-writer policy.

## Goal and why now

Finalize Observation/Hypothesis/ConcernGroup projections. This is an execution slice under the approved BodySense V2 Master; it does not approve clinical decisions or establish any readiness gate by itself.

## Scoped paths / ownership

- `docs/plan/tickets/v2/BS-HYP-001.md (handoff only; approved domain remains authority)`
- `docs/plan/active/2026-10-13-v2-domain-schema.md`

**Out of scope:** Other owners' dirty worktrees, unrelated source files, direct production changes, automatic clinical authorization, and any unapproved legacy contract restoration. Public OpenAPI, generated clients and migrations require a serialized designated owner before mutation.

## Ordered implementation

1. Compare current BodyFact/Episode/Observation identities with approved bodystate.md and existing DS-BS-02 notes.
2. Write explicit owner/source/evidence/revision ViewModels and query/command/event/error contracts for Hypothesis and ConcernGroup.
3. Identify negative/ambiguous and withdrawn-source handling; separately label unimplemented consumers.
4. Submit the contract for Domain review; do not mark database gate ready or introduce old BodyConcern.

## Verification commands and negative cases

- pnpm plan:check
- Cross-check every claimed field against docs/product/domain/bodystate.md

## Acceptance evidence

- Named projection fields, authorization preconditions, conflict/failed-source states and negative scenarios are reviewable.
- No AI hypothesis becomes a clinical fact or a committed concern without approved authority.

**Required handoff:** execution identity/model; actual cwd/branch; input and output exact SHA; changed file list; tests with exit codes and logs; uncommitted diff disclosure; code review; pending gates/owner. Only an Integrator may accept a new shared schema/contract into the integration candidate.

**Protected invariants:** owner identity and source provenance; commit receipt before success; revision + idempotency; source deletion distinct from erasure; Go-owned medical authority and fail-closed Safety; one writer per checkout. No invented Diagnosis Q-17 TTL, clinical thresholds, or old BodyConcern.

**Scheduling constraint:** even when DAG dependencies pass, do not run in parallel with any other ticket sharing `domain-contract` or any other listed exclusive resource. Ticket does not authorize T3 to use canonical root as a shared writer.
