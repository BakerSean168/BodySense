# BS-TECH-07 · row locks, wait-for relationships, and deadlock risk

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #7 · transaction locks and deadlocks**

## Concept

row locks, wait-for relationships, and deadlock risk

## Prerequisites

- TECH-06

## BodySense target files

- `apps/api/internal/repository/treatment_repository.go`
- `apps/api/internal/repository/body_state_repository.go`
- `apps/api/internal/repository/consultation_repository.go`

## Prediction before reading/running

Write the row-lock acquisition order for `TreatmentRepository.AcceptRevision`: treatment revision -> BodyState -> Treatment aggregate. Identify at least one other flow that locks any of the same rows and predict whether the orders can conflict.

## Task

Build a lock-order table for the selected Treatment flow and one competing BodySense mutation. Draw a wait-for graph for the worst interleaving. Distinguish ordinary blocking from a cycle/deadlock.

## Failure case

Construct a disposable two-transaction SQL/Go experiment that intentionally acquires two relevant row types in opposite order. Do not run destructive production traffic. Record PostgreSQL's deadlock behavior and which transaction is aborted.

## Verification command / evidence

- `grep -R "clause.Locking" apps/api/internal/repository` as an inventory aid, not completion evidence
- `A focused PostgreSQL-backed concurrency test/experiment showing lock wait or deadlock detection`
- Explain the lock-order rule you would enforce if a real cycle is possible.

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- What is the difference between a lock wait and a deadlock?
- Why can application-level expected revisions coexist with row locks?
- Which lock order is the canonical order for the chosen flow, if one exists?

## Production change

none unless the experiment demonstrates an actual inconsistent lock order; a test documenting the order is acceptable.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
