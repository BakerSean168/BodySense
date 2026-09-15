# BS-P9-CONCEPT-TYPE-ERASURE · TypeScript type information is erased at runtime and cannot validate untrusted values by itself

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P9-CONCEPT-TYPE-ERASURE**

## Concept

TypeScript type information is erased at runtime and cannot validate untrusted values by itself.

## Prerequisites

- BS-P9-CONCEPT-STRUCTURAL-TYPING

## BodySense target files

- `packages/contracts/src/stream-event-parser.ts`
- `apps/web/src/lib/api-client.ts`

## Prediction before reading/running

For an interface/type assertion/generic used around JSON.parse/fetch, predict exactly what runtime JavaScript check occurs after compilation.

## Task

Trace a typed BodySense network payload into runtime. Identify what JavaScript actually checks, what exists only in the compiler, and why a type assertion/interface cannot make malformed JSON safe.

## Failure case

Cast malformed network JSON to a trusted interface and assume validation occurred. Predict where the bad value fails later.

## Verification command / evidence

- Trace `apps/web/src/lib/api-client.ts` and `packages/contracts/src/stream-event-parser.ts`.
- Run `pnpm exec vitest run packages/contracts/src/stream-event-parser.test.ts` or the repository-equivalent test command and contrast typed generic parsing with explicit runtime parser behavior.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Which TypeScript constructs disappear at runtime?
- Why can `as T` be unsafe?
- What evidence converts unknown JSON into trusted typed data?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
