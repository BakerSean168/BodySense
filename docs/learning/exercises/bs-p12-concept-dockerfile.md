# BS-P12-CONCEPT-DOCKERFILE · Dockerfile is a versioned declarative recipe for reproducibly building an image layer graph

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P12-CONCEPT-DOCKERFILE**

## Concept

Dockerfile is a versioned declarative recipe for reproducibly building an image layer graph.

## Prerequisites

- BS-P12-CONCEPT-IMAGE-VS-CONTAINER

## BodySense target files

- `apps/api/Dockerfile`
- `apps/web/Dockerfile`
- `apps/ai-service/Dockerfile`
- `docker/Dockerfile.runtime`

## Prediction before reading/running

Predict which Dockerfile instruction invalidates which cache layer when only source code changes versus dependency manifests change.

## Task

Read a BodySense Dockerfile instruction by instruction and explain base image, workdir, copy, install/build, user, expose and entry command effects plus cache invalidation.

## Failure case

Copy the whole repo before dependency install, run as root, or bake secrets into an image layer. Explain performance/security consequences.

## Verification command / evidence

- Read BodySense API/web/AI Dockerfiles instruction-by-instruction.
- Build is optional for the learning card; static evidence must identify base image, dependency layer, build/runtime command, user and copied artifacts.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Why order COPY/install steps deliberately?
- What persists in image history/layers?
- EXPOSE vs published port: what differs?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
