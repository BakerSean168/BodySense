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

## Pydantic Evals characterization set

`apps/ai-service/data/evals/diagnosis_baseline.yaml` is the first executable characterization dataset. It intentionally starts small and deterministic:

| Case | Protected behavior |
| --- | --- |
| `mild-neck-load` | ordinary BodyState produces a governed completed analysis |
| `current-severe-pain-blocks` | current severe safety signal blocks candidate generation |
| `historical-severe-pain-does-not-block` | old severe history does not become current safety state |

The task executes the same `DiagnosisService -> PydanticAI Agent -> governance` application path used in production while injecting the deterministic PydanticAI test model. Provider/model benchmarking is deliberately excluded from this baseline.

## Validation command

```bash
cd apps/ai-service
uv run python scripts/run_diagnosis_eval.py --stdout-only
```

The baseline is a characterization gate, not a claim that the current architecture is the target architecture. Later phases extend it with slice-aware qualification, tool traces, non-inferiority, and live-provider mode.
