# BS-P13-CONCEPT-DIRECT-DATABASE-INSPECTION · direct SQL/database inspection is a diagnostic/admin tool that verifies durable truth independently of application projections

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P13-CONCEPT-DIRECT-DATABASE-INSPECTION**

## Concept

direct SQL/database inspection is a diagnostic/admin tool that verifies durable truth independently of application projections

## Prerequisites

- TECH-03
- TECH-05

## BodySense target files

- `apps/api/migrations`
- `apps/api/internal/database`

## Prediction before reading/running

Given an API/repository discrepancy, write a read-only SQL inspection plan that distinguishes schema mismatch, missing row, wrong FK/owner, and projection bug.

## Task

For a BodySense repository issue, define read-only SQL checks to verify schema/rows/constraints independently of API behavior. Explain privilege and production-safety boundaries.

## Failure case

Repair production data manually before identifying the violated invariant, or use a privileged destructive query as a diagnostic step.

## Verification command / evidence

- Use migration/schema files to write safe `SELECT`, catalog/constraint and count checks for one BodySense table.
- If a local/test Postgres is available, execute read-only checks; otherwise validate the SQL against migration definitions and repository expectations.

Passing an existing test is **not** sufficient for L4. The learner must explain the invariant, the untested boundary and a falsifying observation.

## Explain-back questions

- Why inspect durable truth independently of the API?
- What production privileges should diagnostics have?
- When does direct SQL become an unsafe repair instead of observation?

## Production change

No production change is required when the current persistence design already satisfies the concept. If a real defect or observability gap is found, first add a focused characterization/regression test, then make the smallest architecture-consistent correction.

## L4 acceptance

Complete only when the learner can predict the schema/query/transaction behavior before reading the answer path, verify it with code/test/SQL evidence, explain the failure mode and identify what would falsify the conclusion.
