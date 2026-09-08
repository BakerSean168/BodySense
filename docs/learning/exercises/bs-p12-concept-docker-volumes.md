# BS-P12-CONCEPT-DOCKER-VOLUMES · named volumes persist data independently of container lifecycle and require explicit backup/ownership policy

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P12-CONCEPT-DOCKER-VOLUMES**

## Concept

named volumes persist data independently of container lifecycle and require explicit backup/ownership policy.

## Prerequisites

- BS-P12-CONCEPT-DOCKER-COMPOSE

## BodySense target files

- `docker/docker-compose.dev-infra.yml`
- `docker/docker-compose.prod.yml`

## Prediction before reading/running

Identify named/bind volumes and predict what survives `stop`, `rm`, recreate, image rebuild and a volume deletion.

## Task

Identify BodySense persistent services/volumes and explain container replacement, volume lifetime, backup/restore and migration implications.

## Failure case

Run destructive volume cleanup as a generic troubleshooting step on production-like persistent data.

## Verification command / evidence

- Inspect BodySense Compose volume declarations and classify persistent versus disposable data.
- Write a backup/restore/migration responsibility for every persistent volume.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Named volume vs bind mount?
- Why is persistence not the same as backup?
- Who owns schema/data migration when a container image changes?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
