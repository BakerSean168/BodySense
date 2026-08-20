# Diagnosis AI Behavioral Baseline — 2026-08-19

This document freezes the business behavior that the north-star Diagnosis AI platform must preserve while internal model routing is replaced.

## Current production path

```text
Go DiagnosisHandler
  -> load exact BodyState revision
  -> durable safety pre-gate
  -> AIClient /api/diagnosis/analyze
  -> Python DiagnosisService
  -> PydanticAI Diagnosis Agent
  -> structured DiagnosisAgentOutput
  -> Python governance
  -> Go immutable DiagnosisAnalysis persistence
```

Current provider selection below the Agent is migration-era implementation and is not a protected contract.

## Protected behavior

- A Diagnosis run requires a positive durable BodyState revision and non-empty BodyState.
- If `body_state.current_revision` is present, it must equal the requested revision.
- Current positive safety/red-flag facts block ordinary candidate generation.
- Historical red-flag text is not itself a current safety fact.
- `completed` Diagnosis output contains at least one candidate.
- `safety_blocked` may contain zero candidates.
- Candidate count is data-driven and has no fixed maximum of three.
- Python candidates are drafts; Go owns durable analysis/candidate identity.
- Governance is forced before the result is persisted.
- Diagnosis does not create Treatment.

## Pydantic Evals qualification set

The original three-case characterization set has now evolved into the repository-versioned qualification dataset at `apps/ai-service/data/evals/diagnosis_qualification.yaml`, validated by `diagnosis_qualification.schema.json`. It keeps the original protected behaviors and adds split/slice coverage without changing the underlying business invariants.

The current deterministic Champion suite covers seven cases across `development`, `holdout`, `regression`, and `challenge`, including four critical-safety cases that must bypass the Agent completely, two temporal/history-isolation cases, and the ordinary no-Treatment-side-effect case. Tool-trace evaluation records whether the Agent ran, which tools were exposed, and whether any tool call occurred.

The task still executes the production-shaped `DiagnosisService -> PydanticAI Agent -> governance` path. Deterministic mode removes provider variance while retaining the real Agent schema/tool surface and exact immutable Agent configuration identity. The committed Champion evidence is `apps/ai-service/data/evals/reports/diagnosis_champion_baseline.json`.

Qualification is deliberately deterministic where possible. A semantic LLM Judge is not used for rules that can be graded exactly; one should only be introduced for future semantic criteria that cannot be expressed as deterministic assertions and only after calibration.

## Validation command

```bash
cd apps/ai-service
uv run python scripts/run_diagnosis_eval.py --stdout-only
```

To compare an immutable Challenger against the committed Champion on the same dataset:

```bash
uv run python scripts/run_diagnosis_eval.py \
  --configuration-id <diag-config-id> \
  --compare-to data/evals/reports/diagnosis_champion_baseline.json \
  --stdout-only
```

The report includes dataset/configuration fingerprints, split and slice results, evaluator totals, critical failures, and paired non-inferiority evidence. Live-provider benchmarking remains separate from this deterministic release qualification so provider availability cannot make the hard business gate flaky.
