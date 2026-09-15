# BS-P12-CONCEPT-DOCKER-COMPOSE · Docker Compose declares a reproducible multi-service topology: images/builds, env, networks, ports, volumes and dependencies

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P12-CONCEPT-DOCKER-COMPOSE**

## Concept

Docker Compose declares a reproducible multi-service topology: images/builds, env, networks, ports, volumes and dependencies.

## Prerequisites

- BS-P12-CONCEPT-DOCKERFILE

## BodySense target files

- `docker/docker-compose.yml`
- `docker/docker-compose.dev-infra.yml`
- `docker/docker-compose.prod.yml`

## Prediction before reading/running

Draw each service, build/image, network, environment, port, volume and declared dependency before running Compose.

## Task

Trace BodySense Compose services and draw service/network/volume dependencies. Explain what depends_on does and does not guarantee about readiness.

## Failure case

Assume `depends_on` means the database/application is ready to accept requests immediately.

## Verification command / evidence

- Inspect `docker/docker-compose.yml`, `docker-compose.dev-infra.yml`, and `docker-compose.prod.yml`.
- If Docker is available, run a non-mutating `docker compose ... config` on the relevant file; otherwise validate topology statically.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What does Compose own versus the application?
- Container port vs host port?
- How should readiness be established?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
