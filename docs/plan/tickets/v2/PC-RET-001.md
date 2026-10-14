# PC-RET-001 · Implement three-cursor retention and controlled pruning

Status: QUEUED / NOT ADMITTED. Phase 2. SF cross-reference: SF-70.
**Owner role:** Chat Retention Owner. **Dependencies:** `PC-001`.
**Exclusive resources:** `conversation-schema`, `privacy-erasure`.

## Goal and why now

Implement three-cursor retention and controlled pruning. This is an execution slice under the approved BodySense V2 Master; it does not approve clinical decisions or establish any readiness gate by itself.

## Scoped paths / ownership

- `apps/api/internal/repository/** (retention-specific only)`
- `apps/api/internal/service/** (retention-specific only)`
- `apps/ai-service/src/** (checkpoint cleanup only)`

**Out of scope:** Other owners' dirty worktrees, unrelated source files, direct production changes, automatic clinical authorization, and any unapproved legacy contract restoration. Public OpenAPI, generated clients and migrations require a serialized designated owner before mutation.

## Ordered implementation

1. Reuse committed DS-PC-01 fail-closed model/adversarial specifications.
2. Implement processed/context/pruned cursor ordering and minimum source preservation.
3. Prune raw messages only after durable adopted evidence, outbox, checkpoint/cache/index and linked source references qualify.
4. Test interrupted resume, rollback, changed policy, controlled copy and concurrent new messages.

## Verification commands and negative cases

- node --test scripts/plan/v2-chat-retention-model.test.mjs scripts/plan/v2-chat-retention-adversarial.test.mjs
- Native PG18 -race + Python checkpoint negative tests

## Acceptance evidence

- No cursor skip, unsupported source loss, ghost resume or false erasure receipt.
- Explicit cross-domain evidence and deletion scope proof.

**Required handoff:** execution identity/model; actual cwd/branch; input and output exact SHA; changed file list; tests with exit codes and logs; uncommitted diff disclosure; code review; pending gates/owner. Only an Integrator may accept a new shared schema/contract into the integration candidate.

**Protected invariants:** owner identity and source provenance; commit receipt before success; revision + idempotency; source deletion distinct from erasure; Go-owned medical authority and fail-closed Safety; one writer per checkout. No invented Diagnosis Q-17 TTL, clinical thresholds, or old BodyConcern.

**Scheduling constraint:** even when DAG dependencies pass, do not run in parallel with any other ticket sharing `conversation-schema` or any other listed exclusive resource. Ticket does not authorize T3 to use canonical root as a shared writer.
