# BS-P13-CONCEPT-RELATIONAL-QUERYING · relational queries combine WHERE predicates, joins, ordering, grouping and aggregates with index/query-plan consequences

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P13-CONCEPT-RELATIONAL-QUERYING**

## Concept

relational queries combine WHERE predicates, joins, ordering, grouping and aggregates with index/query-plan consequences

## Prerequisites

- BS-P13-CONCEPT-FOREIGN-KEY-JOIN
- TECH-05

## BodySense target files

- `apps/api/internal/repository`

## Prediction before reading/running

For a selected repository method, write the expected SQL operations (filters/join/order/group/limit) and likely index needs before reading generated SQL/tests.

## Task

Choose a nontrivial BodySense repository query and explain predicates/order/join/aggregate SQL, expected indexes and how to detect an inefficient query or N+1 pattern.

## Failure case

Issue one query per child row or scan/sort on unindexed high-cardinality predicates and assume ORM syntax makes it efficient.

## Verification command / evidence

- Trace `consultation_repository.go` or `agent_interaction_repository.go` and its SQL-mock test.
- Explain how EXPLAIN/query logging would falsify your index/query-count expectation; no production query change without evidence.

Passing an existing test is **not** sufficient for L4. The learner must explain the invariant, the untested boundary and a falsifying observation.

## Explain-back questions

- How do WHERE, JOIN, ORDER/GROUP and LIMIT compose?
- What is an N+1 query?
- Why is an ORM method still concrete SQL behavior?

## Production change

No production change is required when the current persistence design already satisfies the concept. If a real defect or observability gap is found, first add a focused characterization/regression test, then make the smallest architecture-consistent correction.

## L4 acceptance

Complete only when the learner can predict the schema/query/transaction behavior before reading the answer path, verify it with code/test/SQL evidence, explain the failure mode and identify what would falsify the conclusion.
