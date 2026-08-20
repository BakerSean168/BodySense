# Treatment Historical / Counterfactual Replay

**Status:** completed
**Date:** 2026-08-20

## Goal

Create a read-only Treatment replay/comparison boundary before rollout governance. Historical replay must deterministically verify the stored artifact and Go generation authority from frozen inputs. Counterfactual replay may execute another immutable Treatment Agent configuration, but only against the exact frozen inputs that produced the source proposal.

## Protected contracts

- Never substitute current BodyState/profile/Diagnosis/evidence for historical inputs.
- Replay never creates/accepts/rejects Treatment, changes current Treatment, creates Training, or mutates BodyState/Diagnosis/Evidence.
- Old revisions without frozen replay input fail explicitly as replay unavailable.
- Treatment v1 remains the committed production default.
- v2 remains a qualified Challenger, not promoted.

## TRP-100 — Frozen Treatment replay input

- Add private `replay_input` JSONB to TreatmentRevision.
- Freeze body_state_revision, body_state, diagnosis_analysis, candidate_assessments, profile, user_constraints and evidence at proposal generation.
- Persist the same generation authority facts used by the Go DecisionPolicy inside the frozen envelope for deterministic historical replay.

## TRP-110 — Historical / counterfactual replay service

- Historical replay validates source artifact/config identity and recomputes Go generation DecisionPolicy without an LLM call.
- Counterfactual replay sends the exact frozen Agent input to an explicitly selected repository-known Treatment configuration.
- Counterfactual results remain transient and never enter Treatment persistence.
- Compare hard / semantic / presentation layers separately.

## TRP-120 — Protected API and regression export

- Add protected Treatment replay and regression-export endpoints.
- Regression export structurally redacts direct user/profile identifiers.
- Add deterministic service/handler tests proving read-only behavior, old-data fail-closed behavior, exact frozen input and v1/v2 comparison.
- Run release, migration replay and prod-like longitudinal validation.

## Implementation checkpoint — 2026-08-20

TRP-100/110/120 are implemented on `feat/treatment-replay-comparison`. Migration 000041 adds private `replay_input`; proposal generation freezes the exact JSON bytes used for BodyState, DiagnosisAnalysis, candidate assessments, profile, constraints and evidence plus generation authority facts. `TreatmentReplayService` distinguishes deterministic historical replay from read-only counterfactual replay, performs pre-Agent Go authority checks, verifies source artifact/configuration/DecisionTrace integrity, and compares hard/semantic/presentation layers. Protected replay/regression-export routes are wired; direct identifiers are structurally redacted and a Python importer validates/appends reviewed exports to the Treatment dataset. Focused Go/Python tests and Go full-suite are green. Final verification passed: repository quality is green with 245 Python tests, 140 Web tests, Go full-suite, all Diagnosis/Treatment qualification and EvidenceGap gates, Pyright/Ruff, real LiteLLM smoke and builds. Prod-like validation passed migration 41 -> 40 -> 41, API/AI/Web health, 3/3 longitudinal Playwright cases including historical replay + opposite-config counterfactual + regression export before acceptance, Treatment activation/outcome atomicity, Diagnosis shadow with zero blockers, Treatment v2 Challenger persistence with 3 revisions, accepted DecisionTrace validation, and database-level replay validation with 3 non-empty frozen replay inputs. v2 remains a Challenger; replay readiness does not change the production pointer.
