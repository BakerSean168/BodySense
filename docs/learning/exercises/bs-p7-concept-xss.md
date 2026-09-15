# BS-P7-CONCEPT-XSS · cross-site scripting and safe rendering boundaries

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-XSS**

## Concept

cross-site scripting and safe rendering boundaries.

## Prerequisites

- BS-FSO-0.1

## BodySense target files

- `apps/web/src/features/consultation/components`
- `apps/web/src/features/workspace/components`

## Prediction before reading/running

Trace model/server-provided text into React rendering and predict whether `<script>`/HTML-looking text becomes text or executable markup.

## Task

Trace untrusted server/model text to React rendering and verify it is rendered as data rather than executable markup; flag any deliberate HTML escape hatch.

## Failure case

Introduce `dangerouslySetInnerHTML` for untrusted assistant/user content without sanitization. Predict what data can become active content.

## Verification command / evidence

- Search `apps/web/src` for deliberate HTML escape hatches such as `dangerouslySetInnerHTML` and document each trust boundary.
- Render/inspect one consultation/workspace text path and explain React escaping versus URL/HTML sanitizer responsibilities.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Why does React escaping reduce but not eliminate XSS risk?
- Which APIs bypass escaping?
- Why can unsafe URL schemes or third-party HTML still matter?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
