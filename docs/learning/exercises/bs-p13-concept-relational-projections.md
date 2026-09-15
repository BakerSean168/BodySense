# BS-P13-CONCEPT-RELATIONAL-PROJECTIONS · joined/eager relational projections should select intentional fields and avoid overfetch/sensitive data leakage

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P13-CONCEPT-RELATIONAL-PROJECTIONS**

## Concept

joined/eager relational projections should select intentional fields and avoid overfetch/sensitive data leakage

## Prerequisites

- BS-P13-CONCEPT-RELATIONAL-QUERYING

## BodySense target files

- `apps/api/internal/repository`
- `apps/api/internal/service/health_workspace_service.go`
- `apps/api/internal/dto`

## Prediction before reading/running

Predict the exact fields a workspace/read model should expose and which database columns/relations must remain internal or sensitive.

## Task

Inspect a BodySense aggregate query/projection. Explain join/preload shape, field selection, ordering and how the transport DTO differs from raw joined rows.

## Failure case

Return raw joined model rows directly to transport, leaking columns and coupling API shape to persistence schema.

## Verification command / evidence

- Trace a repository result through `health_workspace_service.go` into DTO/public projection code.
- Use an existing service/handler test to show the public projection omits or transforms persistence-only fields.

Passing an existing test is **not** sufficient for L4. The learner must explain the invariant, the untested boundary and a falsifying observation.

## Explain-back questions

- Why is a read projection not the same as a database model?
- How does field selection reduce leakage/overfetch?
- Where should sorting/aggregation be performed?

## Production change

No production change is required when the current persistence design already satisfies the concept. If a real defect or observability gap is found, first add a focused characterization/regression test, then make the smallest architecture-consistent correction.

## L4 acceptance

Complete only when the learner can predict the schema/query/transaction behavior before reading the answer path, verify it with code/test/SQL evidence, explain the failure mode and identify what would falsify the conclusion.
