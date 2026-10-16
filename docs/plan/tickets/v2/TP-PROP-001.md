# TP-PROP-001 · TrainingPlan PlanProposal pure precommit admission policy

Status: BLOCKED / NOT ADMITTED TO CURRENT BASELINE. Phase 1. SF cross-reference: SF-50.
**Owner role:** Training Domain Owner. **Dependencies:** `TP-D1-001`. **Exclusive resources:** `domain-trainingplan-go`.
**Checkout:** `/home/dev/projects/.chatgpt-worktrees/bodysense-v2-privacy`, branch `feat/v2-training-proposal-admission-20261014`, historical base `e4160a9c7…`, head `681edbd8eef88c504f07df5a2a7e2dadd8be85f9`. **Review PR:** #274.

## Current admission checkpoint · 2026-10-16

PR #274 remains OPEN/BEHIND at head `681edbd8eef88c504f07df5a2a7e2dadd8be85f9`. Its historical CI does not validate integration against current frozen main `af3e1993b1f62930e3b680b54abab5ade456935b`. The queue marks this ticket blocked pending rebase, review and fresh exact-head CI. This is an admission block; it does not change the host PR’s open review state.

## Scope

`apps/api/internal/domain/trainingplan/proposal_admission.go` — deterministic, side-effect-free `EvaluateAcceptance(proposal, decision, currentContext, now)` returning a precommit verdict. Enforces owner identity, pending state, content fingerprint, relevant-context fingerprint, exact plan/base revision for updates, and the product 7-day PlanProposal TTL (`docs/product/domain/README.md` INV-43, acceptance TP-22). It does **not** persist, authorize or activate anything.

## Evidence so far

- Exact-head CI run 38593599317: all executed lanes PASS; PG18 baseline/recovery children SKIPPED (no migration in this PR).
- PR description reports 56 Go race positive/negative cases.

## To accept

1. Rebase and review against the current frozen main; obtain fresh exact-head CI. Optional review fixes: typed constant for `DecisionKind` (`"accept"`); state "no migration, PG18 skip expected" in the merge note.
2. Merge under normal branch protection; record merged SHA here and set `acceptedSha` in `queue.json`.
3. The verdict `eligible_for_transactional_commit` must never be presented as acceptance; the persisted transactional write belongs to TP-002.
