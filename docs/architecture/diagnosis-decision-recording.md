# Diagnosis decision recording

DX-002 / Issue #305, bounded slice 2a. Product authority remains
`docs/product/domain/diagnosis.md`.

## A recorded choice is not adoption

`DiagnosisProposalDecisionService.Record` stores an authenticated user's explicit
selection of one immutable proposal and an ordered `decision_recorded` event in
a single PostgreSQL transaction. It never creates a `ConditionEntry`, changes the
Diagnosis head, or appends a `DiagnosisRevision`. Partial selections survive a
lost response or a later terminal outcome without being presented as adoption.

The caller must supply its authenticated owner separately from the command. The
future HTTP/AskUser transport must establish the actual user interaction and bind
its `toolCallId`; an Agent-generated command cannot impersonate that boundary.
This slice deliberately adds no public decision route or Agent auto-adoption tool.

## Transaction and recovery

The lock order is the owner row, the BodyState head, then the Diagnosis head;
event appends additionally lock the exact owner/run. This is compatible with
TrainingPlan admission. Owner filters are part of each source query rather than
post-fetch comparisons. Source SHARE locks hold availability stable through the
transaction.

A fresh command must match the run, proposal, content fingerprint and expected
Diagnosis revision. Superseded/expired proposals, already-recorded alternate
choices, closed runs, missing user-originated requests, unavailable sources and
incomplete Go safety data are rejected. Supporting and counter-evidence are
qualified against their exact frozen IDs and versions.

An exact idempotent retry returns the original immutable choice receipt. It does
not repeat an analysis or mutation, even if live sources subsequently change.
Reusing the key for a different command fails. Eight concurrent identical calls
are covered by a real PostgreSQL race test.

The receipt and its event either both commit or both roll back. Database guards
make event sequences contiguous and append-only, allow only one terminal event,
and reject further appends after settlement. The event table participates in the
owner erasure inventory and owner cascades; it is not an exemption from erasure.

## Safety and source ownership

The service uses the existing Go BodyState safety projection. Ordinary training
restrictions do not automatically forbid diagnosis decisions. Incomplete or
unavailable safety data fails closed. A valid urgent Go result appends a
`safety_blocked` terminal event while retaining earlier choice receipts as audit;
it does not adopt a diagnosis.

The current approved structured-screen policy only produces `restricted`, not
`urgent`. The urgent terminal regression injects a typed synthetic result through
an internal test seam while retaining real PostgreSQL transactions and audit
constraints. It does not alter the production policy, weaken a database safety
constraint, or claim to validate a new clinical threshold.

`DiagnosisSourceRepository` is shared with TrainingPlan context qualification.
It checks owner-scoped statements, admitted report evidence, available native
UserAssets, current facts/observations and version-bounded Episode evidence. A
source-removed original is not silently treated as re-verifiable evidence.

## Validation

The isolated runner `scripts/plan/test-dx002-decisions-pg18.sh` uses a pinned PG18
image, no container network, a unique synthetic database/socket and cleanup traps.
It never uses an environment-provided production or staging database URL.

- Targeted transaction/race cases cover partial accept/reject, exact retries,
  concurrent retries, conflicting keys, stale identity, foreign evidence,
  supersession, non-user requests, coverage failure and typed urgent closure.
- Injected event-write failure must roll back the already-inserted decision.
- Database tests reject event mutation, sequence gaps, foreign run references and
  post-terminal appends.
- `DX002_FULL_CHECKS=1 bash scripts/plan/test-dx002-decisions-pg18.sh` runs baseline
  up/down/replay, domain-validator, `go vet`, and full API race tests, including
  existing TrainingPlan/native-source and privacy-erasure regressions.

## Remaining DX-002 scope

This slice is not the whole decision lifecycle. The next settlement transaction
must recheck current evidence after all effective proposals are decided (or an
explicit close of the remainder), apply all adopted changes atomically, and
produce exactly one DiagnosisRevision or a no-change terminal receipt. It must
also handle related-evidence additions, confirmation renewal, relation and
Disposition histories, and transaction rollback of partial patches.

Public read/decide APIs, trusted Agent proposal ingestion, direct correction,
legacy DiagnosisAnalysis retirement and end-to-end UI acceptance remain separate
slices. Issue #305 stays open. No Diagnosis proposal TTL is invented here.

## Direct correction and withdrawal (DX-003 slice 2)

`POST /api/v1/diagnosis/conditions/{conditionId}/direct-edits` implements the
lightweight ConditionDirectEdit of diagnosis.md §3.5 without a formal run.

- `withdraw` moves one adopted entry to `inactive` with reason
  `user_withdrawal`; `correct_metadata` replaces only `qualifiers.notes`.
  Medical identity, AI assessment, source claims, evidence links and handling
  never change here; they need a formal analysis and per-proposal decisions.
- The command cites the owner's own available `user_message` statement
  evidence (the same `UserRequest` check formal runs use) and the current
  Diagnosis revision. A foreign or unknown condition/statement is a masked 404.
- One transaction writes the immutable `condition_direct_edits` receipt, one
  `DiagnosisRevision` (`direct_edit_id`, no run), the entry projection and, for
  withdraw, a `condition_disposition_changes` row whose authority is the edit
  (`direct_edit_id`, no decision). The head advance makes every older open run
  stale, so its decisions cannot overwrite the edit.
- Safety is fail-closed: an urgent restriction (409 `SAFETY_BLOCKED`) or
  incomplete coverage (503) changes nothing. A no-op correction or a second
  withdraw is refused, so no empty revision exists. Re-adoption still requires
  a formal run `set_disposition`/`readopted` decision.
- An exact idempotent retry returns the original receipt; a changed body under
  the same key, or a key already used by a settlement revision, is a conflict.

Validation: `bash scripts/plan/test-dx002-decisions-pg18.sh` now includes the
`TestDiagnosisDirectEdit*` PG18 cases (withdraw, metadata, stale/foreign/
replay/concurrency/Safety negatives and receipt immutability).
