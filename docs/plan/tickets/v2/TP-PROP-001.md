# TP-PROP-001 · TrainingPlan PlanProposal pure precommit admission policy

Status: DONE (dormant pure policy) at main `adaa02607a208a1a7d4cc9ba5b70845d6a77f351` via PR #274. Phase 1. SF cross-reference: SF-50.
**Owner role:** Training Domain Owner. **Dependencies:** `TP-D1-001`. **Exclusive resources:** `domain-trainingplan-go`.
**Checkout:** `/home/dev/projects/.chatgpt-worktrees/bodysense-v2-privacy`, branch `feat/v2-training-proposal-admission-20261014`, historical base `e4160a9c7…`, head `681edbd8eef88c504f07df5a2a7e2dadd8be85f9`. **Review PR:** #274.

## Acceptance · 2026-10-16

PR #274 was reviewed against main `887f74b6b` (after the #299 status sync): the diff is additive (one new package `apps/api/internal/domain/trainingplan`, its tests and a handoff doc), has no schema/OpenAPI/caller dependency, and still matches INV-43 / TP-22 (owner, pending state, content and relevant-context fingerprints, exact base revision, 7-day PlanProposal TTL). The branch was brought up to date with GitHub update-branch (merge, no history rewrite), exact-head CI run [38900112810](https://github.com/BakerSean168/BodySense/actions/runs/38900112810) passed on all executed lanes (PG18 children skipped: no migration), local `go test -race` and `go vet` on the package passed, and it merged as `adaa02607`. Optional review nit (typed `DecisionKind` constant) is carried into TP-002.

## Scope

`apps/api/internal/domain/trainingplan/proposal_admission.go` — deterministic, side-effect-free `EvaluateAcceptance(proposal, decision, currentContext, now)` returning a precommit verdict. Enforces owner identity, pending state, content fingerprint, relevant-context fingerprint, exact plan/base revision for updates, and the product 7-day PlanProposal TTL (`docs/product/domain/README.md` INV-43, acceptance TP-22). It does **not** persist, authorize or activate anything.

## Evidence so far

- Exact-head CI run 38593599317: all executed lanes PASS; PG18 baseline/recovery children SKIPPED (no migration in this PR).
- PR description reports 56 Go race positive/negative cases.

## To accept

1. Rebase and review against the current frozen main; obtain fresh exact-head CI. Optional review fixes: typed constant for `DecisionKind` (`"accept"`); state "no migration, PG18 skip expected" in the merge note.
2. Merge under normal branch protection; record merged SHA here and set `acceptedSha` in `queue.json`.
3. The verdict `eligible_for_transactional_commit` must never be presented as acceptance; the persisted transactional write belongs to TP-002.
