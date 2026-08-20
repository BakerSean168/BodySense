# Treatment Rollout Governance

**Status:** completed
**Date:** 2026-08-20

## Goal

Promote Treatment v2 from qualification/replay-ready Challenger to a governed rollout candidate without changing the production default. Go must own stable route selection, shadow/canary admission, durable rollout observations and stop/progression recommendations. Python only executes the exact immutable configuration selected by Go.

## Protected contracts

- Production defaults remain Treatment v1 / `champion`; merging code never promotes v2.
- The served run is the only path allowed to persist a TreatmentRevision or create downstream Training/Outcome state.
- Shadow/counterfactual runs are read-only and use the exact frozen replay input from the served proposal.
- Go generation/acceptance DecisionPolicy remains authoritative and unchanged.
- Existing `TREATMENT_AGENT_CONFIGURATION_ID` remains a compatibility alias for the Champion pointer.
- Canary assignment is stable per subject and rollout salt; observations never persist user identity.

## TRO-100 — Promotion evidence and runtime admission

- Add repository-versioned `treatment_promotion_v1` from existing v1/v2 qualification + EvidenceGap evidence.
- Add machine verification and an Nx promotion target.
- Add Go/Python policy-sync tests so rollout thresholds cannot drift.
- Add Treatment champion/challenger/stage/canary/salt/promotion-record runtime admission with fail-closed validation.

## TRO-110 — Stable serving and read-only paired shadow

- Add `TreatmentRouteSelection` and stable SHA256 subject bucketing.
- TreatmentService selects the served immutable config for every generation path, including internal Training-driven regeneration.
- Refactor TreatmentReplayService to depend only on a read-only revision source.
- After served proposal persistence, execute the opposite config via frozen-input replay when the stage requires pairing; shadow failure never changes the served proposal result.

## TRO-120 — Durable rollout observations and stop policy

- Add anonymous `treatment_rollout_observations` persistence.
- Record hard/semantic/presentation comparison, forbidden side effect/config mismatch/shadow error signals, stage/bucket/config identities and source TreatmentRevision identity.
- Add deny-first stop rules and predeclared progression: shadow -> 500 -> 2500 -> 5000 -> promoted.
- Add operator status command; evaluator reports only and never mutates deployment state.

## TRO-130 — Deployment proof

- Keep all committed production Compose defaults on Champion.
- Enable Treatment shadow only in the disposable local validator stack.
- Require at least one v1/v2 Treatment rollout observation with zero blockers after longitudinal E2E.
- Run focused tests, full repository quality, migration replay and prod-like validation before merge.

## Final verification

Completed 2026-08-20. Repository release validation is green with 246 Python tests, 140 Web tests, Go full-suite, Diagnosis and Treatment qualification/EvidenceGap/promotion evaluators, Ruff/Pyright, real LiteLLM gateway smoke and builds. Prod-like validation passed migration 42 -> 41 -> 42, API/AI/Web health, 3/3 longitudinal Playwright cases, Treatment activation/outcome atomicity, Diagnosis shadow with zero blockers, and Treatment v1 -> v2 shadow with 3 paired observations, zero blockers, 3 served Champion revisions and zero persisted Challenger revisions. Production/default Compose state remains v1 / champion.
