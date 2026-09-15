# BS-A1 · typed Agent execution boundary and immutable configuration identity

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **BodySense Agent Extension A1**

## Concept

typed Agent execution boundary and immutable configuration identity

## Prerequisites

- None

## BodySense target files

- `apps/ai-service/src`
- `apps/api/internal/service/agent_deployment_policy.go`
- `docs/architecture/agent-platform-role-governance.md`

## Prediction before reading/running

Before tracing, state which layer chooses the immutable Agent configuration, which layer executes typed reasoning, which model identifier is logical versus physical, and which layer may authorize durable business effects.

## Task

Trace one Diagnosis or Treatment execution from Go deployment/config selection into Python PydanticAI typed dependencies/output and through LiteLLM logical routing, then back to Go validation/persistence authority.

## Failure case

Model a returned configuration ID/role/policy mismatch or malformed typed output. Predict where execution is rejected and whether ordinary durable delivery is allowed.

## Verification command / evidence

- `Use focused existing configuration/agent identity tests in Go/Python for the selected role`
- `Record a configuration -> logical model -> execution provenance trace from code/tests.`

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Why is Agent Configuration the qualification unit instead of physical model name?
- What does Python own and what does Go own?
- Why can LiteLLM route providers without owning business safety policy?

## Production change

none unless a configuration/typed-boundary invariant is missing or untested.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
