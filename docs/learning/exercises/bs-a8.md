# BS-A8 · production Agent failure attribution

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **BodySense Agent Extension A8**

## Concept

Production Agent debugging is contract-chain fault localization: find the first violated invariant across Input -> Context -> Evidence -> Proposal -> Decision -> Execution Provenance -> Persistence -> Delivery instead of blaming “the model” generically.

## Prerequisites

- BS-A1
- BS-A2
- BS-A3
- BS-A4
- BS-A5
- BS-A6
- BS-A7

## BodySense target files

- `apps/api/internal/service/diagnosis_analysis_service.go`
- `apps/api/internal/service/diagnosis_rollout_service.go`
- `apps/api/internal/service/diagnosis_replay_service.go`
- `apps/api/internal/service/ai_client_test.go`
- `apps/ai-service/src/services/diagnosis_service.py`
- `apps/ai-service/tests/unit/test_diagnosis_service.py`

## Prediction before reading/running

Pick one symptom such as “unsafe candidate reached UI”, “challenger output differs”, or “replay mismatch”. Before inspecting tests, list the contract checkpoints you would examine in order and the evidence expected at each.

## Task

Debug one synthetic production-shaped Diagnosis failure by walking the contract chain Input -> Context -> Evidence -> Proposal -> Decision -> Execution Provenance -> Persistence -> Delivery. Locate the first violated invariant, distinguish model/runtime/policy/transport faults and identify replayable evidence before proposing a fix.

## Failure case

Use a configuration/provenance mismatch, malformed safety input, evidence-availability inconsistency or delivery/replay discrepancy. Do not stop at the last visible symptom; identify the earliest contract that became false.

## Verification command / evidence

- During iteration, run the smallest relevant slice, for example `cd apps/api && go test ./internal/service -run "Diagnosis|AnalyzeDiagnosisSendsPythonContract" -count=1` and narrow further as soon as the failing contract is known.
- `cd apps/ai-service && .venv/bin/python -m pytest tests/unit/test_diagnosis_service.py tests/unit/test_diagnosis_api_contract.py -q`
- Produce a one-page failure-attribution trace naming the first contract violation and one observation that would falsify the diagnosis.

## Explain-back questions

- Why is the last failing component often not the root cause?
- Which provenance fields let you distinguish model/config/runtime drift?
- When should replay be used, and when is a live reproduction more informative?
- What evidence is required before changing production behavior?

## Production change

Only after the first violated contract is identified and characterized. Fix that boundary or its producer; do not patch downstream symptoms.

## L4 acceptance

Complete only when the learner can independently localize a failure to the first broken contract, justify the attribution with persisted/test evidence and propose a minimal regression-protected correction.
