# BS-P12-CONCEPT-DOCKER-NETWORK-DNS · Compose networks provide service-name DNS and container-to-container ports distinct from host-published ports

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P12-CONCEPT-DOCKER-NETWORK-DNS**

## Concept

Compose networks provide service-name DNS and container-to-container ports distinct from host-published ports.

## Prerequisites

- BS-P12-CONCEPT-DOCKER-COMPOSE

## BodySense target files

- `docker/docker-compose.yml`
- `docker/docker-compose.dev-infra.yml`

## Prediction before reading/running

Predict which hostname/port one service uses to reach another on a Compose network and why localhost would be wrong.

## Task

Trace one BodySense service-to-service connection by Compose service name. Distinguish container port, host published port, loopback and why `localhost` inside a container points to itself.

## Failure case

Configure API container to connect to `localhost` for a database running in a different container.

## Verification command / evidence

- Trace one BodySense service-to-service URL/host through Compose environment variables.
- Draw browser/host -> published port and container -> service-name/internal-port paths separately.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What does localhost mean inside a container?
- How does Compose DNS resolve service names?
- When is a host-published port unnecessary?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
