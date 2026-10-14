# BodySense V2 · TP-D1 PlanProposal precommit policy slice

**Track:** Training PlanProposal / acceptance boundary.
**Source of truth:** `docs/product/domain/training.md` TP-D1–D5, and
`docs/plan/active/2026-10-14-ds-tp-01-training-safety-handoff.md`.
**Issue:** [TP-D1 cross-track #268](https://github.com/BakerSean168/BodySense/issues/268).

## Implemented, isolated Go decision layer

`apps/api/internal/domain/trainingplan/proposal_admission.go` provides a
deterministic, pure `EvaluateAcceptance` function. It evaluates an
**authoritative proposal snapshot**, a **server-verified explicit user
decision**, and a **current relevant-context fingerprint**. An allowed
result is deliberately named `eligible_for_transactional_commit`, not
`accepted`, `activated`, `safety_clear` or `completed`.

Checks include:

- proposal ID/owner exactly match the authenticated user's decision;
  mismatches return an identity-free unavailable verdict
- only a pending create/update proposal is eligible; an already accepted,
  rejected, expired or unknown state cannot activate by replay
- only an explicit `accept` decision with a durable user-decision ID,
  source ID and idempotency key may pass; an Agent suggestion alone cannot
- the exact presented content fingerprint equals the stored immutable
  proposal fingerprint
- the Training-specific **7×24-hour** maximum expiry is checked by elapsed
  instants, independently from Diagnosis; an earlier explicit expiry is
  respected, a later deadline is invalid
- relevant BodyState/Diagnosis/Safety context must be available,
  explicitly bound to the same owner and have the _same upstream-attested
  fingerprint_; this policy never infers medical safety classification
  or overrides a safety restriction
- `create` may not silently target an existing plan or accepted revision;
  its current context is owner-global, not another plan's revision
- `update` must identify the exact persisted plan in both the proposal
  and the authoritative current context, plus its proposal base
  revision and the authenticated decision's expected revision, with the
  current accepted revision unchanged

Results are identity-free codes; the decision layer performs **zero**
network, database, medical interpretation or writes. Tests exercise
accepted user versus Agent initiation, owner isolation, proposal replay,
content/context changes, cross-owner evidence with coincidentally equal
fingerprints, cross-plan revisions, missing decision authorization, 7-day timeout,
explicit early expiry, future-dated proposals, create/update identity,
revision conflicts and UTC/DST boundaries.

## Non-negotiable future adapter requirements

The typed proposal and user-decision snapshots **must be read and
authenticated server-side**. This helper's `UserAcceptance` struct is not
proof that a request came from the user: the Go service must validate an
actual user-authored decision/receipt and never trust the caller to assert
`ActorUserID`, `DecisionID` or the context fingerprint. Likewise, the
context fingerprint is not computed by this slice. The future adapter
must bind it to authoritative _relevant_ BodyState, Diagnosis and Safety
versions with permitted, available sources and reconcile them in the
same transaction as acceptance.

The next persisted implementation must hold the per-owner transaction
lock, read/update the proposal under the lock, enforce exact
idempotency-key and decision-payload replay consistency, create the
immutable `TrainingPlanRevision`, update the accepted plan pointer, and
write a durable user-authorization/commit receipt **atomically**.
Competing devices must not both accept the same proposal, and a
matching retry must be answered from the persisted receipt rather than
re-applying this precommit eligibility decision. For update, only the
explicitly identified plan may change. Legacy TreatmentRevision
projection and old TrainingLog rows are not PlanProposal authority.

## Formal status

**Precommit rule implemented; TP-001 Safety Domain and TP-002 schema/
Backend queues are NOT complete.** No HTTP/OpenAPI/controller/Provider
construction is wired to this policy, no SQL migration is applied, no
generated DTO or PlanProposal table is created, and no Staging/Production
gate is advanced. Go Race and exact final-source CI receipts must be
attached before merging this bounded prerequisite. Fail-closed clinical
safety verification still belongs to the independent Go SafetyState
authority.
