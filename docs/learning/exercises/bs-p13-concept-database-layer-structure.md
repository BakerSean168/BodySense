# BS-P13-CONCEPT-DATABASE-LAYER-STRUCTURE · database-backed applications benefit from explicit config/connection/model/repository/service/handler boundaries

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P13-CONCEPT-DATABASE-LAYER-STRUCTURE**

## Concept

database-backed applications benefit from explicit config/connection/model/repository/service/handler boundaries

## Prerequisites

- TECH-01
- TECH-11

## BodySense target files

- `apps/api/internal/database`
- `apps/api/internal/model`
- `apps/api/internal/repository`
- `apps/api/internal/service`
- `apps/api/internal/handler`

## Prediction before reading/running

Choose one BodySense write endpoint and predict the call/dependency path handler -> service -> repository/database, including where auth identity, transaction and DTO conversion belong.

## Task

Trace one BodySense endpoint across all persistence layers. State responsibilities and dependency direction, including where transaction/context ownership lives.

## Failure case

Let handlers issue ad-hoc SQL or repositories make transport/business-policy decisions, then explain the coupling and testability failures.

## Verification command / evidence

- Trace one endpoint through `apps/api/internal/handler`, `service`, `repository`, `model` and `database`.
- Run a focused handler/service/repository test for that path and identify which layer each assertion protects.

Passing an existing test is **not** sufficient for L4. The learner must explain the invariant, the untested boundary and a falsifying observation.

## Explain-back questions

- What belongs in handler vs service vs repository?
- Who owns transactions?
- Why should DTOs/models not collapse into one type everywhere?

## Production change

No production change is required when the current persistence design already satisfies the concept. If a real defect or observability gap is found, first add a focused characterization/regression test, then make the smallest architecture-consistent correction.

## L4 acceptance

Complete only when the learner can predict the schema/query/transaction behavior before reading the answer path, verify it with code/test/SQL evidence, explain the failure mode and identify what would falsify the conclusion.
