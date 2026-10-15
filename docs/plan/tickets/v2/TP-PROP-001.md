# TP-PROP-001 · TrainingPlan PlanProposal pure precommit admission policy

Status: IN_PROGRESS / IN REVIEW. Phase 1. SF cross-reference: SF-50.
**Owner role:** Training Domain Owner. **Dependencies:** `TP-D1-001`. **Exclusive resources:** `domain-trainingplan-go`.
**Checkout:** `/home/dev/projects/.chatgpt-worktrees/bodysense-v2-privacy`, branch `feat/v2-training-proposal-admission-20261014`, base = frozen main `e4160a9c7…`, head `681edbd8eef88c504f07df5a2a7e2dadd8be85f9`. **Review PR:** #274.

## Scope

`apps/api/internal/domain/trainingplan/proposal_admission.go` — deterministic, side-effect-free `EvaluateAcceptance(proposal, decision, currentContext, now)` returning a precommit verdict. Enforces owner identity, pending state, content fingerprint, relevant-context fingerprint, exact plan/base revision for updates, and the product 7-day PlanProposal TTL (`docs/product/domain/README.md` INV-43, acceptance TP-22). It does **not** persist, authorize or activate anything.

## Evidence so far

- Exact-head CI run 38593599317: all executed lanes PASS; PG18 baseline/recovery children SKIPPED (no migration in this PR).
- PR description reports 56 Go race positive/negative cases.

## To accept

1. Optional review fixes: typed constant for `DecisionKind` (`"accept"`); state "no migration, PG18 skip expected" in the merge note.
2. Merge under normal branch protection; record merged SHA here and set `acceptedSha` in `queue.json`.
3. The verdict `eligible_for_transactional_commit` must never be presented as acceptance; the persisted transactional write belongs to TP-002.
