# BS-P12-CONCEPT-IMAGE-VS-CONTAINER · an image is an immutable filesystem/config template; a container is a runtime instance with an ephemeral writable layer

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P12-CONCEPT-IMAGE-VS-CONTAINER**

## Concept

an image is an immutable filesystem/config template; a container is a runtime instance with an ephemeral writable layer.

## Prerequisites

- None

## BodySense target files

- `apps/api/Dockerfile`
- `apps/web/Dockerfile`
- `docker/docker-compose.yml`

## Prediction before reading/running

For one service, classify Dockerfile/image layers, container runtime writable layer, environment, mounts and external durable data before inspecting Compose.

## Task

Pick a BodySense service and trace Dockerfile -> image -> one or more containers. Classify image layers, runtime env/ports/mounts and data that disappears when the container is replaced.

## Failure case

Store canonical database/application data only inside a container writable layer then recreate the container.

## Verification command / evidence

- Trace `apps/api/Dockerfile` or `apps/web/Dockerfile` into `docker/docker-compose.yml`.
- Draw image -> container(s) -> volume/network relationships and state what replacement destroys/preserves.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Image vs container: template or process instance?
- What state belongs outside the container?
- Why is an image normally immutable?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
