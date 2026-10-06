# BodySense Product Definition

> Status: Product design source of truth
> Updated: 2026-10-06
> Upstream: ADR 0004 + Longitudinal BodyState Domain Model

## 1. Product thesis

BodySense is a long-lived AI-assisted body-state workspace for understanding physical changes, organizing evidence, exploring possible explanations, following an improvement plan and learning from outcomes over time.

It is **not** a chat transcript with generated reports and it is **not** a sequence of one-off consultation sessions.

One-sentence product promise:

> **BodySense remembers the state and history of your body, helps you understand what may be happening, and continuously adapts what to do next as evidence and outcomes change.**

## 2. Primary user mental model

The default mental model is:

> **My body → the concern I care about → what is known now → why it may be happening → what I can do → what changed.**

The user should not need to think in terms of:

- backend services;
- AI runs or checkpoints;
- BodyState revisions;
- separate consultation documents;
- creating a new chat for every concern;
- Diagnosis / Treatment / Progress as mandatory top-level application modules.

Those may exist internally or as product artifacts, but they do not automatically become navigation.

## 3. Product objects the user can understand

### Body

The long-lived subject of the product. It gives the user one persistent place to return to.

### Concern

A meaningful body problem or question the user wants to understand or improve, such as right scapular discomfort, knee pain after running or persistent neck stiffness.

A Concern is the natural scope for a continuous task canvas.

### Current state

What BodySense currently knows. It is derived from accepted facts, observations, relevant history and safety state.

### Evidence

Information supporting or weakening an interpretation. Evidence must retain provenance and must not be visually conflated with AI inference.

### Why / Analysis

An evidence-bound interpretation of possible explanations. It is probabilistic and revisable, not a definitive medical diagnosis.

### Plan

The current accepted improvement/treatment direction, including interventions, constraints, review conditions and next actions.

### Training / Action

Focused execution of an intervention or self-test. This can use a task-specific mode rather than the default product shell.

### Outcome / Change

What happened after an action or over time. Outcomes may correlate with a plan but do not automatically establish causality.

### History

The temporal record of meaningful changes, corrections, analyses, plans and outcomes. It is available when needed rather than forcing every user through a timeline-first interface.

### Assistant

A contextual capability that can explain, ask, capture, guide and help update product state. It is not necessarily a permanently dominant pane.

## 4. Epistemic model

The UI must preserve these distinctions:

| Concept | Meaning | UI obligation |
| --- | --- | --- |
| Fact | user-confirmed or otherwise accepted durable information | show as current known state |
| Observation | measured/observed information with provenance | show source and verification state |
| Hypothesis | AI/system interpretation that remains uncertain | never style as confirmed fact |
| Evidence | source-grounded support/counterevidence | keep traceable to source |
| Diagnosis candidate | possible explanation | present alternatives, uncertainty and missing information |
| Treatment proposal | proposed action set | distinguish proposal from accepted current plan |
| Outcome | observed change after time/action | avoid automatic causal wording |

## 5. Time semantics

The product must distinguish two different user intents:

**Correction**

> “I said the wrong side earlier; it is the right shoulder.”

The previous record was wrong and should be corrected.

**Temporal change**

> “It used to hurt on the left, but that resolved; now the right side hurts.”

The earlier state was true and belongs in history.

This distinction is a product invariant and must be visible in interaction design whenever an edit could mean either one.

## 6. Safety semantics

Safety is a global guard, not a local card style.

When active safety information changes what the user should do:

- safety takes precedence over normal training or analysis actions;
- the interface must state why the normal flow is interrupted;
- it must provide an understandable next action;
- the product must not hide the safety state merely because the user navigates to a different concern or artifact.

## 7. Primary user jobs

### J1 — Tell BodySense what is happening

Use ordinary language, photos, reports, measurements, self-tests or direct edits without first learning medical terminology.

### J2 — Correct what BodySense got wrong

Change durable body state without corrupting true historical state.

### J3 — Understand the current body picture

See important active concerns, changes and safety information without scanning a full transcript.

### J4 — Understand a concern

Move from “what I feel” to structured current state, evidence and possible explanations.

### J5 — Know what information is missing

See which questions, observations or self-tests would materially improve the analysis.

### J6 — Decide what to do next

Review and accept an appropriate plan or next action with constraints and review conditions.

### J7 — Execute a training/self-test task

Enter a focused, low-distraction task mode when execution requires it.

### J8 — Record what changed

Capture improvement, no change, worsening or another outcome with enough context to support future review.

### J9 — See the longitudinal story

Understand trends, recurrence, plan revisions and how the current state differs from earlier states.

### J10 — Ask for explanation anywhere

Invoke the assistant with relevant current context without losing the product state the user is working on.

## 8. Product capabilities

The design must support these capabilities even if they are not persistent pages:

- Body overview / Body Home;
- Concern discovery and selection;
- Concern intake and clarification;
- body-region exploration;
- structured state review;
- evidence review;
- analysis generation and review;
- missing-information / readiness guidance;
- plan generation and review;
- plan acceptance/rejection/revision;
- training/self-test execution;
- outcome capture;
- timeline/history inspection;
- correction versus new-change choice;
- safety interruption and guidance;
- contextual assistant;
- document/photo upload and provenance;
- empty/loading/error/retry states.

## 9. Experience principles

### Body-first, not module-first

The body and its concerns are the stable product objects. Diagnosis/Treatment/Progress are derived artifacts or capabilities.

### Evidence before confidence

The design should make “why” inspectable before presenting high-confidence conclusions.

### One coherent longitudinal story

Now, why, plan and change must feel like different views of the same concern, not disconnected apps.

### Progressive disclosure

The default surface remains understandable; provenance, revisions and advanced anatomy/evidence details appear when useful.

### Task-shaped space

A Body Home, an evidence-heavy Why view and a focused training task do not need the same spatial skeleton.

### State is durable, chat is interaction

Conversation is a powerful input/explanation surface but not the authoritative storage model.

### Explicit authority

Proposed, accepted, inferred, confirmed, stale, blocked and review-required states must be legible.

## 10. Non-goals

BodySense is not:

- an autonomous medical diagnosis replacement;
- a generic fitness social app;
- a clinical EHR;
- a dashboard exposing every backend aggregate;
- a chat client with model/thread management as the main product;
- a permanent four-tab shell merely because the current implementation has four modules.

## 11. Product invariants

1. BodyState remains the durable center of the domain.
2. Conversation never becomes the only source of current body truth.
3. AI inference never silently becomes confirmed user fact.
4. Safety can interrupt ordinary flows.
5. Correction and temporal change remain different operations.
6. Diagnosis stays evidence-bound and revisable.
7. Treatment proposal and accepted current plan remain different authorities.
8. Outcome does not imply causality unless causality is separately supported.
9. Responsive layouts preserve the same mental model.
10. Navigation follows user jobs, not service/module boundaries.
