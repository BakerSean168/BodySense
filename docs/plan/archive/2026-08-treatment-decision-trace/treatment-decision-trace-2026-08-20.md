# Treatment DecisionTrace

**Status:** completed
**Date:** 2026-08-20

## Goal

Make Go's existing Treatment generation and acceptance authority explicit, pure, versioned and durable. The AI Agent remains proposal-only; the trace must be produced from the same deterministic facts that actually authorize the transition, not reconstructed afterward from logs.

## Protected contracts

- Treatment Agent configuration/evidence provenance remains separate from Go business authority.
- Generation requires eligible Diagnosis, fresh analysis when freshness is available, active-safety clearance and confirmed/unsure candidate assessment.
- Acceptance re-runs the same hard gates and additionally requires a proposed revision, non-regressed BodyState revision and no material related BodyState change.
- Acceptance and current-pointer transition remain one transaction with a concurrent BodyState revision guard.

## TDT-100 — Pure TreatmentDecisionPolicy

- Add `treatment-go-acceptance-v1` pure deny-overrides policy for `generation` and `acceptance` phases.
- Encode stable reason codes for diagnosis readiness, freshness, safety, candidate assessment, proposal state and BodyState drift.
- Parse BodyState SafetyState strictly: malformed, unknown or internally inconsistent safety facts fail closed.

## TDT-110 — Durable generation/acceptance trace

- Add `generation_decision_trace` and `acceptance_decision_trace` to TreatmentRevision.
- Persist generation allow trace when a proposal is created.
- Pass the precomputed acceptance allow trace into `AcceptRevision` and persist it in the same transaction that accepts the revision and changes the current Treatment pointer.
- Include exact BodyState/Diagnosis/configuration identities and deterministic policy facts in each trace.

## TDT-120 — Regression and deployment validation

- Prove deny-overrides policy behavior and malformed SafetyState fail-closed behavior with focused tests.
- Prove successful proposal/acceptance traces survive repository persistence.
- Run full repository, migration replay and prod-like longitudinal validation.

## Implementation checkpoint — 2026-08-20

TDT-100/110 are implemented on `feat/treatment-decision-trace`. `TreatmentDecisionPolicyV1` is now the single Go version constant shared by configuration registration and runtime authority. Focused policy tests cover generation/acceptance allow, deny-overrides reasons, unknown policy/phase and malformed/inconsistent/unknown SafetyState fail-closed behavior. TreatmentRevision gains `generation_decision_trace` and `acceptance_decision_trace` through migration 000040. Proposal creation stores the generation allow trace; `AcceptRevision` receives the precomputed acceptance allow trace and writes it in the same transaction as acceptance/current-pointer mutation, while the existing BodyState revision lock remains the final concurrency guard. Go full-suite is green. Final verification passed: repository quality is green with 243 Python tests, 140 Web tests, Go full-suite, all Diagnosis/Treatment qualification and EvidenceGap gates, Pyright/Ruff, real LiteLLM smoke and builds. Prod-like validation passed migration 40 -> 39 -> 40, API/AI/Web health, 3/3 longitudinal Playwright cases, Treatment activation/outcome atomicity, Diagnosis shadow with zero blockers, Treatment v2 Challenger persistence with 3 revisions, and database-level DecisionTrace validation with 2 accepted Treatment revisions carrying both non-empty generation and acceptance traces.
