# BS-A5 · evaluation, qualification and controlled rollout

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **BodySense Agent Extension A5**

## Concept

Agent quality is governed at the immutable configuration level through versioned datasets, critical slice gates, paired non-inferiority and predeclared shadow/canary/promotion/rollback policy.

## Prerequisites

- BS-A1
- BS-A3
- BS-A4

## BodySense target files

- `apps/ai-service/src/evals/diagnosis_qualification.py`
- `apps/ai-service/src/evals/diagnosis_promotion.py`
- `apps/ai-service/tests/unit/test_diagnosis_evals.py`
- `apps/api/internal/service/agent_deployment_policy.go`
- `apps/api/internal/service/diagnosis_rollout_service.go`
- `apps/api/internal/service/agent_deployment_policy_test.go`
- `apps/api/internal/service/diagnosis_rollout_service_test.go`

## Prediction before reading/running

Imagine a challenger improves average score but introduces one critical safety regression. Predict qualification, rollout admission and promotion outcome before reading the tests.

## Task

Trace the qualification unit from versioned dataset/slices through critical gates and paired non-inferiority into an explicitly admitted Champion/Challenger rollout. Then trace shadow/canary evidence and predeclared promotion/rollback rules.

## Failure case

Model an unqualified pair, critical regression, unsafe relaxation, configuration mismatch or insufficient rollout sample. Identify which gate blocks promotion and why post-hoc threshold changes would invalidate governance.

## Verification command / evidence

- `cd apps/ai-service && .venv/bin/python -m pytest tests/unit/test_diagnosis_evals.py tests/unit/test_diagnosis_promotion.py -q`
- `cd apps/api && go test ./internal/service -run "AgentDeploymentPolicy|DiagnosisRollout|PromotionPolicy" -count=1`
- Explain which observation blocks promotion even if average quality improves.

## Explain-back questions

- Why is physical model name not the qualification unit?
- Why use paired non-inferiority on the same cases?
- What is the difference between offline qualification, shadow evidence and canary serving evidence?

## Production change

None unless a gate/policy lacks explicit evidence. Do not tune production rollout rules merely to make a candidate pass.

## L4 acceptance

Complete only when the learner can independently audit a Champion/Challenger decision, reconstruct its evidence and predict rollback/promotion under a new synthetic result.
