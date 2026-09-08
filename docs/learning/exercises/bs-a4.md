# BS-A4 · deterministic safety and decision authority

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **BodySense Agent Extension A4**

## Concept

The model proposes; deterministic Go policy owns whether normal delivery is allowed. Safety blockers, malformed/unknown facts and policy revisions fail closed rather than being overridden by model confidence or a Judge score.

## Prerequisites

- BS-A1
- BS-A2
- BS-A3

## BodySense target files

- `apps/api/internal/service/diagnosis_decision_policy.go`
- `apps/api/internal/service/diagnosis_decision_policy_test.go`
- `apps/api/internal/service/diagnosis_analysis_service.go`

## Prediction before reading/running

Given a high-confidence model candidate plus one hard safety blocker, predict the persisted analysis outcome, whether candidates are exposed for normal delivery, and which policy revision appears in the decision trace.

## Task

Trace model-proposed Diagnosis candidates into deterministic Go decision authority. Enumerate hard blockers, abstain/deny/deliver outcomes and the durable representation of a blocked result; show that confidence/Judge output cannot override policy.

## Failure case

Use an unknown decision-policy revision, malformed safety state, or a Python result that strips/rejects a safety field. Predict the first fail-closed boundary and prove ordinary delivery remains blocked.

## Verification command / evidence

- `cd apps/api && go test ./internal/service -run "DiagnosisDecisionPolicy|DecisionAuthority|PythonRejected" -count=1`
- Explain one fail-closed path from malformed/unknown safety input to suppressed normal delivery.

## Explain-back questions

- Why is confidence not authority?
- What is the difference between model abstention and deterministic policy denial?
- Why must the decision-policy revision be part of provenance/replay identity?

## Production change

None unless a hard blocker can be bypassed or is uncharacterized; write the failing policy test first.

## L4 acceptance

Complete only when the learner can predict and verify at least one allow and one fail-closed decision path and explain the model/runtime/policy authority split.
