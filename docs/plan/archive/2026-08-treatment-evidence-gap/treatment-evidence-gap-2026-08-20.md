# Treatment EvidenceGap Challenger

**Status:** completed
**Date:** 2026-08-20

## Goal

Replace Treatment's model-controlled bare `search_evidence(query)` in a new immutable Challenger with a typed, policy-owned and bounded evidence-acquisition loop. Keep Treatment v1 immutable and the Go deployment pointer on v1 until qualification evidence exists.

## Protected contracts

- Treatment v1 remains resolvable for historical provenance and current serving.
- Go remains the only Treatment acceptance authority.
- User facts can never be synthesized from external RAG.
- External retrieval must have an explicit EvidenceGap rationale and finite budget.
- Every acquisition attempt is auditable and persisted with the TreatmentRevision proposal.

## TEV-100 — Shared bounded evidence acquisition

- Generalize the existing EvidenceGap acquisition engine without changing Diagnosis v2 behavior.
- Add `treatment-evidence-gap-v2` policy support.
- Preserve `user_fact -> no search`, bounded external search, explicit stop reasons, and unresolved critical gaps.

## TEV-110 — Immutable Treatment v2 Challenger

- Add `treatment-v2-evidence-gap.yaml` with new prompt/tool/evidence revisions.
- v1 keeps `search_evidence`; v2 exposes only `acquire_evidence(EvidenceGap)`.
- TreatmentAgentService creates the bounded acquirer only for v2 and emits its trace.
- Go registers v2 as selectable but does not change the default Treatment pointer.

## TEV-120 — Durable trace + qualification

- Persist `evidence_acquisition_trace` on TreatmentRevision.
- Add deterministic Treatment EvidenceGap policy tests/eval coverage.
- Evaluate v1 and v2 on the same Treatment qualification dataset and require zero deterministic regressions before calling the Challenger eligible for later rollout work.

## Verification

focused acquisition/Agent tests -> v1/v2 Treatment qualification -> full repository gate -> migration replay -> prod-like longitudinal validation.

## Implementation checkpoint — 2026-08-20

TEV-100/110/120 are implemented on `feat/treatment-evidence-gap`. The shared bounded acquisition engine preserves Diagnosis v2 behavior and adds `TreatmentEvidenceAcquirer`. Treatment v1 remains `treat-config-85718f8e90ac9d80`; the immutable v2 Challenger is `treat-config-f68eec9846664596` and exposes only `acquire_evidence(EvidenceGap)`. Its per-run budget is two searches with five results per search; `user_fact` never searches; budget exhaustion and unresolved critical gaps are explicit. Migration 000039 persists `evidence_acquisition_trace` on TreatmentRevision. On the exact same Treatment dataset fingerprint, v1 and v2 both pass 4/4 with zero deterministic regressions and delta 0.0; the dedicated Treatment EvidenceGap policy suite passes 5/5. All Compose defaults still point to v1. Final verification passed: repository quality is green with 243 Python tests, 140 Web tests, Go full-suite, Diagnosis qualification 7/7, Diagnosis EvidenceGap 5/5, Treatment v1 qualification 4/4, Treatment v2 qualification 4/4, Treatment EvidenceGap policy 5/5, zero deterministic v1->v2 regressions, Pyright/Ruff, real LiteLLM smoke and builds. Prod-like validation passed migration 39 -> 38 -> 39, API/AI/Web health, 3/3 longitudinal Playwright cases, Treatment activation/outcome atomicity, Diagnosis shadow validation with zero blockers, and explicit v2 Treatment Challenger persistence with 3 revisions. Production Compose defaults remain v1.
