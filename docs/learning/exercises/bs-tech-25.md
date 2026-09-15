# BS-TECH-25 · multi-service dev topology, health, dependency and persistence

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #25 · Docker Compose service startup**

## Concept

multi-service dev topology, health, dependency and persistence

## Prerequisites

- TECH-01

## BodySense target files

- docker
- `scripts/dev-infra.sh`
- package.json

## Prediction before reading/running

Predict which services `pnpm dev:infra:up` starts, their ports, persistent volumes, health dependencies and which app processes intentionally stay on the host.

## Task

Trace the current direct-dev/Compose topology. Explain startup ordering, health checks, restart policy, volumes and why infrastructure lifetime is separate from hot-reload app processes.

## Failure case

Stop one infrastructure dependency in a controlled dev environment and predict which health check/request path fails. Distinguish container restart from persistent-volume deletion.

## Verification command / evidence

- `pnpm dev:infra:status`
- `docker compose` config/ps inspection for the current dev files; do not remove persistent volumes.

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- `Why is `depends_on`/startup order not the same as service readiness?`
- Which data survives a container restart and why?
- `Why are Web/API/AI host processes separate in direct dev?`

## Production change

none unless health/startup/persistence behavior contradicts documented dev contract.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
