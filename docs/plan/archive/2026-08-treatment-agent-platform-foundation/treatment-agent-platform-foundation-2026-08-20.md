# Treatment Agent Platform Foundation

**Status:** completed
**Date:** 2026-08-20

## Outcome

Build Treatment on the completed Agent platform without changing the existing durable Treatment lifecycle. Every AI-generated Treatment proposal must be pinned to exact BodyState and Diagnosis identities and must additionally carry one immutable Treatment Agent configuration identity plus execution provenance that Go verifies and persists.

## Protected contracts

- Go remains the durable owner of Treatment/TreatmentRevision/Intervention identities and acceptance.
- AI output is proposal-only; it can never make a Treatment current.
- Generation and acceptance re-check Diagnosis eligibility/freshness, candidate assessment readiness, and active safety state.
- Accepted Treatment revisions stay immutable and Training is projected only from accepted revisions.
- LiteLLM remains the only physical LLM provider/fallback boundary.

## TREAT-100 — Immutable Treatment Agent configuration

- Add repository-versioned `treatment-v1.yaml` with behavior fingerprint/configuration ID.
- Version prompt/schema/tool/evidence/governance/decision boundaries explicitly.
- Resolve the exact config selected by Go; reject unknown identities.
- Build PydanticAI Treatment runs from that exact config and emit configuration + execution provenance.

Acceptance: Python focused tests prove stable identity, behavior-significant fingerprinting, exact resolution, and provenance on generated proposals.

## TREAT-110 — Go deployment pointer and durable provenance

- Extend the Go-owned Agent deployment policy with one repository-known Treatment configuration pointer.
- Send `configuration_id` on the Treatment AI contract.
- Reject response configuration-role/id mismatch before proposal persistence.
- Add dedicated `agent_configuration_id`, `agent_configuration`, and `execution_provenance` columns to immutable TreatmentRevision.

Acceptance: Go tests prove correct request identity, mismatch fail-closed behavior, and persisted provenance; migration up/down/replay passes.

## TREAT-120 — Treatment qualification baseline

- Add a small deterministic Pydantic Evals Treatment dataset covering normal proposal, candidate-assessment context, and no-side-effect/proposal-only invariants.
- Require exact Agent configuration provenance in qualification.
- Add one repository validation target/command and commit the baseline report.

Acceptance: all cases pass, report fingerprints dataset/config, and release validation invokes it.

## Verification

focused Python/Go tests -> full Python/Go tests -> Treatment eval -> repository quality gate -> migration replay -> prod-like local deploy/E2E.

## Implementation checkpoint — 2026-08-20

TREAT-100/110/120 are implemented on `feat/treatment-agent-platform-foundation`. Treatment v1 is repository-versioned as `treat-config-85718f8e90ac9d80`; Go owns the deployment pointer and sends the exact ID, Python rebuilds the Agent from that manifest, and Go rejects configuration/role/decision-policy/runtime/logical-model drift before proposal persistence. Migration 000038 adds immutable TreatmentRevision configuration and execution provenance. `treatment_qualification_v1` currently passes 4/4 across development/holdout/regression/challenge and is part of repository validation. Final verification passed: repository quality is green with 237 Python tests, 140 Web tests, Go full-suite, Diagnosis qualification 7/7, EvidenceGap 5/5, Treatment qualification 4/4, Pyright/Ruff and real LiteLLM smoke. Prod-like validation passed migration 38 -> 37 -> 38, API/AI/Web health, 3/3 longitudinal Playwright cases, Treatment activation/outcome atomicity, and Diagnosis shadow validation with zero blockers.
