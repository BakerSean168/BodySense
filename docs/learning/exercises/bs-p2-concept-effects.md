# BS-P2-CONCEPT-EFFECTS · Effect lifecycle and dependency-driven synchronization

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P2-CONCEPT-EFFECTS**

## Concept

Effect lifecycle and dependency-driven synchronization

## Prerequisites

- BS-P1-CONCEPT-HOOK-RULES
- BS-P2-CONCEPT-PROMISES

## BodySense target files

- `apps/web/src/features/consultation/hooks/useAssistantChatRuntime.ts`
- `apps/web/src/features/body-explorer/hooks/useBodyExplorerWorkspace.tsx`

## Prediction before reading/running

For one `useEffect`, state the external system being synchronized, its dependency trigger, cleanup behavior and what happens on unmount/re-run.

## Task

Choose one real effect, predict when it runs and cleans up, and verify dependency behavior without adding an effect that merely derives state.

## Failure case

Omit a changing dependency or cleanup and predict stale closure, duplicate subscription/timer, or leaked resource behavior.

## Verification command / evidence

- Trace one effect in `apps/web/src/features/body-explorer/hooks/useBodyExplorerWorkspace.tsx` or consultation runtime.
- Run `pnpm exec vitest run --config apps/web/vite.config.ts apps/web/src/features/body-explorer/hooks/useBodyExplorerWorkspace.test.tsx`.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- What problem are effects for?
- Why are effects not a generic place for derived state?
- When exactly does cleanup run?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
