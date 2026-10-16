# BS-OBS-001 · Close BodyObservation V12 correction/withdrawal

Status: DONE (PR #276, merged ee4dd2c7f, 2026-10-15). Phase 1. SF cross-reference: SF-20.
**Owner role:** Backend BodyObservation Owner. **Dependencies:** `INT-001`.
**Exclusive resources:** `api-observation`, `public-openapi`, `database-migrations`.

## Goal and why now

Close BodyObservation V12 correction/withdrawal. This is an execution slice under the approved BodySense V2 Master; it does not approve clinical decisions or establish any readiness gate by itself.

## Scoped paths / ownership

- `apps/api/internal/model/body_observation*`
- `apps/api/internal/repository/body_observation*`
- `apps/api/internal/transport/httpapi/body_observation*`
- `apps/api/migrations/000011*`
- `apps/api/migrations/000012*`

**Out of scope:** Other owners' dirty worktrees, unrelated source files, direct production changes, automatic clinical authorization, and any unapproved legacy contract restoration. Public OpenAPI, generated clients and migrations require a serialized designated owner before mutation.

## Ordered implementation

1. Inspect actual V12 HEAD and preserve existing uncommitted amendment test; do not overwrite source writer.
2. Validate exact owner/identity, source provenance, revision conflict, idempotency receipt, rollback and controlled erasure.
3. Complete pending HTTP negative test, rerun native PG18 v11/v12 rollback and Go race, commit in owning worktree.
4. Hand off tested SHA; Integrator reviews and merges it into the integration candidate.

## Verification commands and negative cases

- go test -race -count=1 ./internal/repository ./internal/transport/httpapi -run 'BodyObservation' (cwd apps/api)
- apps/web/e2e/body-observation-amendments.spec.ts against isolated real services

## Acceptance evidence

- Exact committed V12 SHA passes owner/revision/idempotency and rollback tests.
- No dirty source writer changes are silently picked up; Design/React UI still prevents stale revision saves.

**Required handoff:** execution identity/model; actual cwd/branch; input and output exact SHA; changed file list; tests with exit codes and logs; uncommitted diff disclosure; code review; pending gates/owner. Only an Integrator may accept a new shared schema/contract into the integration candidate.

**Protected invariants:** owner identity and source provenance; commit receipt before success; revision + idempotency; source deletion distinct from erasure; Go-owned medical authority and fail-closed Safety; one writer per checkout. No invented Diagnosis Q-17 TTL, clinical thresholds, or old BodyConcern.

**Scheduling constraint:** even when DAG dependencies pass, do not run in parallel with any other ticket sharing `api-observation` or any other listed exclusive resource. Ticket does not authorize T3 to use canonical root as a shared writer.
