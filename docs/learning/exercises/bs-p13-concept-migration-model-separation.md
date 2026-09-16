# BS-P13-CONCEPT-MIGRATION-MODEL-SEPARATION · historical migrations must remain self-contained instead of importing mutable current model definitions

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P13-CONCEPT-MIGRATION-MODEL-SEPARATION**

## Concept

historical migrations must remain self-contained instead of importing mutable current model definitions

## Prerequisites

- TECH-03

## BodySense target files

- `apps/api/migrations`
- `apps/api/internal/model`

## Prediction before reading/running

Pick an early migration and a current Go model; predict which model fields/relations have evolved while the old migration must remain replayable unchanged.

## Task

Compare BodySense current models with old migration files. Explain why intentional duplication preserves replayability when current field names/types/associations evolve.

## Failure case

Import current model definitions into historical migrations or edit an applied migration, making fresh and existing databases diverge.

## Verification command / evidence

- Compare `apps/api/migrations/000001_vnext_baseline.up.sql` with current user model and later migrations.
- Trace migration runner/checksum behavior in `apps/api/internal/database/migrate.go` and `apps/api/migrations/checksums.sha256`.

Passing an existing test is **not** sufficient for L4. The learner must explain the invariant, the untested boundary and a falsifying observation.

## Explain-back questions

- Why is duplication in migrations intentional?
- What breaks when an applied migration is edited?
- How should a schema change be introduced instead?

## Production change

No production change is required when the current persistence design already satisfies the concept. If a real defect or observability gap is found, first add a focused characterization/regression test, then make the smallest architecture-consistent correction.

## L4 acceptance

Complete only when the learner can predict the schema/query/transaction behavior before reading the answer path, verify it with code/test/SQL evidence, explain the failure mode and identify what would falsify the conclusion.
