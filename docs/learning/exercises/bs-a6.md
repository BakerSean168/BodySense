# BS-A6 · decision trace, historical replay and counterfactual replay

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **BodySense Agent Extension A6**

## Concept

Replay is meaningful only when input, configuration, policy and decision provenance are frozen. Historical replay reconstructs what happened; counterfactual replay asks what another qualified configuration would do without rewriting user history.

## Prerequisites

- BS-A1
- BS-A2
- BS-A4
- BS-A5

## BodySense target files

- `apps/api/internal/service/diagnosis_replay_service.go`
- `apps/api/internal/service/diagnosis_replay_service_test.go`
- `apps/api/internal/service/diagnosis_analysis_service.go`
- `apps/ai-service/src/evals/diagnosis_regression_import.py`

## Prediction before reading/running

For a stored Diagnosis run, list the exact facts that must be frozen to reproduce its decision. Predict what happens for a legacy run that predates frozen replay input.

## Task

Trace a persisted Diagnosis artifact into frozen replay input, immutable configuration/decision policy identity and replay output. Compare historical replay with counterfactual replay and prove counterfactual execution does not mutate ordinary durable user state.

## Failure case

Attempt replay with missing frozen input/provenance or choose a counterfactual configuration that was never admitted. Predict fail-closed behavior and which historical facts must remain unchanged.

## Verification command / evidence

- `cd apps/api && go test ./internal/service -run "DiagnosisReplay|PersistAndPublicPayloadUseDedicatedDiagnosisProvenance|FreezesReplayInput" -count=1`
- `cd apps/ai-service && .venv/bin/python -m pytest tests/unit/test_diagnosis_regression_import.py -q`
- Explain why a pre-provenance legacy run must fail closed or degrade explicitly instead of inventing replay inputs.

## Explain-back questions

- Historical replay vs counterfactual replay: which question does each answer?
- Why must replay input be hidden from the normal read model yet preserved durably?
- Why is “rerun the latest prompt/model on current data” not replay?

## Production change

None unless replay identity is incomplete; preserve historical records and add compatibility behavior explicitly instead of backfilling invented facts.

## L4 acceptance

Complete only when the learner can independently reconstruct a replay identity, distinguish historical from counterfactual execution, and prove replay cannot rewrite canonical user history.
