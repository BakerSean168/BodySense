# BS-A2 · runtime ownership versus durable domain truth

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **BodySense Agent Extension A2**

## Concept

runtime ownership versus durable domain truth

## Prerequisites

- BS-A1

## BodySense target files

- `docs/architecture/current-longitudinal-system.md`
- `apps/api/internal/service/diagnosis_analysis_service.go`
- `apps/api/internal/service/treatment_service.go`
- `apps/ai-service/src`

## Prediction before reading/running

Pick one Diagnosis/Treatment run and classify BodyState, LangGraph checkpoint, runtime event, Analysis/Revision and Web cache by owner, lifetime and mutability before reading the architecture table.

## Task

Build an ownership matrix for one end-to-end Agent-backed flow. Trace where ephemeral thread state stops and durable Go-owned business artifacts begin, including exact BodyState revision pinning.

## Failure case

Imagine Python directly mutating durable BodyState or Web cache becoming canonical truth. Identify the replay/concurrency/audit failures each ownership violation would create.

## Verification command / evidence

- `Compare the matrix against `current-longitudinal-system.md`, route/service code and at least one persistence test`
- No code change required when ownership is already explicit.

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Durable != aggregate root: what does that mean here?
- Why are historical analyses immutable while current applicability can change?
- Why is a LangGraph checkpoint not the health record?

## Production change

none unless actual code contradicts accepted ownership invariants.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
