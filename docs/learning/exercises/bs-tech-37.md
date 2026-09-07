# BS-TECH-37 · refresh rotation, replay detection, single-winner concurrency, and family revocation

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #37 · refresh-token sessions**

## Concept

refresh rotation, replay detection, single-winner concurrency, and family revocation

## Prerequisites

- TECH-20
- TECH-21
- TECH-22

## BodySense target files

- `apps/api/internal/service/auth_service.go`
- `apps/api/internal/service/auth_service_test.go`
- `apps/api/internal/cache`

## Prediction before reading/running

For two simultaneous refresh requests using the same old credential, predict which request wins, what the loser observes, what keys/family state remain, and whether the session stays usable.

## Task

Trace `RefreshToken`, `rotateRefreshToken`, the Lua compare/delete/set sequence, replay marker, refresh family, and session cache. Draw the exact state machine for normal rotation and replay.

## Failure case

Run the existing single-winner/replay test, then explain the security consequence if both requests could mint valid descendants or if replay did not revoke the family.

## Verification command / evidence

- `cd apps/api && go test ./internal/service -run "RotateRefreshTokenSingleWinnerAndReplayRevokesFamily|LogoutWithConsumedRefreshToken" -count=1 -v`

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Why is the raw refresh credential hashed before becoming a Redis key?
- Why is rotation atomic?
- Why does detected replay revoke the family rather than merely reject one token?

## Production change

none unless the existing race test or trace reveals a real authority/revocation gap.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
