# BS-TECH-01 · relational schema, keys, cardinality and durable ownership

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #1 · database schema design**

## Concept

relational schema, keys, cardinality and durable ownership

## Prerequisites

- None

## BodySense target files

- `apps/api/migrations/000001_vnext_baseline.up.sql`
- `apps/api/migrations/000001_vnext_baseline.up.sql`
- `apps/api/internal/model`

## Prediction before reading/running

Choose BodyState -> Diagnosis -> Treatment/Outcome. Before reading all migrations, sketch the entities, primary/foreign keys and one-to-many/one-to-one relationships you expect.

## Task

Reverse-engineer the selected BodySense slice into an ER diagram from migrations/models. Annotate aggregate ownership, immutable history versus current pointers, uniqueness constraints and where user ownership is enforced.

## Failure case

Invent one impossible relationship/state (for example a revision linked to the wrong user/aggregate) and identify whether FK/unique/check constraints or application policy prevents it.

## Verification command / evidence

- Compare the ER diagram against the actual migration DDL and model fields
- `cd apps/api && go test ./internal/repository -count=1` after any schema-characterization test addition.

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Which invariants belong in the database rather than Go?
- Why are immutable revisions separate rows instead of overwriting current state?
- What cardinality does each foreign key imply?

## Production change

none unless a missing relational invariant is demonstrated; schema changes require separate migration design/review.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
