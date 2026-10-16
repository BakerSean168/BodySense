# BS-AGENT-001 · Connect source-bound AI suggestions to Go decisions

Status: QUEUED / NOT ADMITTED. Phase 2. SF cross-reference: SF-12/SF-20.
**Owner role:** Go/Python Owner. **Dependencies:** `BS-OBS-001`, `BS-HYP-001`.
**Exclusive resources:** `agent-tools`, `public-openapi`.

## Current remaining scope · 2026-10-16

**GitHub:** [#308](https://github.com/BakerSean168/BodySense/issues/308) (epic [#301](https://github.com/BakerSean168/BodySense/issues/301)) · Wave 2 · size M · blocked by: BS-HYP-001 #304 · blocks: QA-001 #316, REL-001 #317.

Source-bound BodyFact/AskUser commits and native frozen Agent inputs are implemented; PR6 A–E includes removal of the obsolete intake binding. Remaining scope is unimplemented authorization/command variants, not PR6 input migration or binding cleanup.

The original steps below are historical decomposition; perform only their unfulfilled acceptance work, under the Master’s single-writer policy.

## Goal and why now

Connect source-bound AI suggestions to Go decisions. This is an execution slice under the approved BodySense V2 Master; it does not approve clinical decisions or establish any readiness gate by itself.

## Scoped paths / ownership

- `apps/ai-service/src/**`
- `apps/api/internal/consultation/**`
- `contracts/internal/agent-runtime/**`

**Out of scope:** Other owners' dirty worktrees, unrelated source files, direct production changes, automatic clinical authorization, and any unapproved legacy contract restoration. Public OpenAPI, generated clients and migrations require a serialized designated owner before mutation.

## Ordered implementation

1. Characterize current LangGraph/Go message identity and exact accepted-answer source binding.
2. Implement only approved source-bound proposal/tool envelope; Go validates tenant, revision, provenance and authorization.
3. Cover ambiguous statements, withdrawn source and stale proposals; implement bounded retries without treating model text as commit.

## Verification commands and negative cases

- Existing Python consultation/tool tests and Go consultation -race tests
- Real Ask User / BodyFact browser paths at integration SHA

## Acceptance evidence

- Extraction remains proposal until Go commit receipt and verified read; no false clinical/evidence promotion.
- Sufficient negative auth, replay and cancellation cases are reproducible.

**Required handoff:** execution identity/model; actual cwd/branch; input and output exact SHA; changed file list; tests with exit codes and logs; uncommitted diff disclosure; code review; pending gates/owner. Only an Integrator may accept a new shared schema/contract into the integration candidate.

**Protected invariants:** owner identity and source provenance; commit receipt before success; revision + idempotency; source deletion distinct from erasure; Go-owned medical authority and fail-closed Safety; one writer per checkout. No invented Diagnosis Q-17 TTL, clinical thresholds, or old BodyConcern.

**Scheduling constraint:** even when DAG dependencies pass, do not run in parallel with any other ticket sharing `agent-tools` or any other listed exclusive resource. Ticket does not authorize T3 to use canonical root as a shared writer.
