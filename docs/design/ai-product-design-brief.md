# BodySense AI Product Design Brief

> Status: Active brief for V2d exploration
> Updated: 2026-10-06
> Inputs: Product Definition + Product UX Semantics + Foundations

## 1. Assignment

Explore the next BodySense product UI as a **body-centered longitudinal health workspace**, not as a restyled version of the current React Workbench.

The current leading direction is **Adaptive Body Canvas**. Different jobs may use different spatial compositions while preserving one semantic and visual system.

## 2. Fixed product semantics

Do not change these during visual exploration:

- BodyState is the durable center of the product.
- The default user mental model is “my body → concern → now → why → what to do → change”.
- Concern is the natural continuous task scope.
- Fact, Observation, Hypothesis, Evidence, Proposal, Accepted Plan and Outcome remain distinct.
- Safety is a global guard.
- Correction is different from recording a new change over time.
- Analysis is evidence-bound and revisable.
- Treatment proposal is not the current plan until accepted.
- Outcome does not automatically prove causality.
- Assistant is contextual capability, not necessarily a permanent pane.

## 3. Assumptions you are allowed to challenge

You may redesign or remove:

- fixed Chat + Workspace split;
- permanent State / Diagnosis / Treatment / Progress top-level tabs;
- card-as-default-container;
- one shared page skeleton for every task;
- module names mapped directly to pages;
- current navigation hierarchy;
- current component proportions and literal token values.

## 4. Required product modes

### P0 — Body Home

Purpose: understand what matters in the body now.

Must make the body a meaningful spatial asset rather than decorative illustration.

Must surface:

- active concerns;
- relevant regions;
- safety state;
- meaningful recent change;
- readiness-driven Next.

Working spatial direction: **Body Atlas**.

### P0 — Concern Canvas

Purpose: work continuously on one concern.

Must make Now / Why / What to do / Change feel like one coherent concern rather than four separate apps.

Working spatial direction: **Narrative Canvas**.

### P0 — Why / Evidence

Purpose: inspect possible explanations without overclaiming.

Must support dense evidence, counterevidence, uncertainty, missing information and provenance.

Working spatial direction: **Evidence Studio**.

### P0 — Plan

Purpose: understand and review what to do.

Must distinguish proposal from accepted current plan and show constraints/review triggers.

Working spatial direction: **Journey**.

### P0 — Training / Self-test

Purpose: execute an action with low distraction.

Must work as a focused task mode and return to the originating concern/context.

Working spatial direction: **Focus Mode**.

### P0 — Outcome

Purpose: capture what changed and trigger appropriate review.

Must support improve / same / worse plus richer change where needed, without implying causality.

Working spatial direction: **Journey / Change**.

## 5. Supporting modes

- contextual Assistant;
- concern History;
- safety interruption;
- correction-vs-change choice;
- new-user / empty Body Home;
- insufficient-information state;
- loading / error / retry;
- stale analysis;
- upload/evidence provenance.

## 6. Visual direction

Aim for:

- quiet health technology;
- precise but not institutional;
- body-centered;
- dark graphite as the current baseline;
- restrained organic warmth;
- high evidence legibility;
- low visual noise;
- task-shaped space.

Avoid:

- generic admin-dashboard composition;
- neon AI styling;
- EHR table wall;
- chat-app-first shell;
- “everything is a floating rounded card”;
- decorative body imagery without interaction meaning.

## 7. Foundation constraints

Use semantic roles defined in [Design Foundations](./foundations.md).

The current V1/V2c palette is a working baseline, not a final brand lock.

A new visual direction may tune literal values, but it must preserve:

- primary action;
- accepted/positive;
- selected/attention;
- warning/review;
- safety/danger;
- primary/secondary/muted text;
- canvas/surface/overlay hierarchy.

## 8. Responsive requirement

Every P0 concept must explain its behavior in:

- wide desktop;
- narrow desktop/tablet;
- mobile.

Do not design desktop first and treat mobile as a collapsed copy. Preserve the same product mental model while changing presentation.

## 9. Review tests

A concept should fail review if any of these are false:

1. With text blurred, Body Home / Concern / Why / Plan / Training still have distinguishable spatial roles.
2. The body is a meaningful product asset on Body Home.
3. Concern feels continuous across Now → Why → Plan → Change.
4. Evidence density is possible without making the default Home dense.
5. Assistant can appear/disappear without losing task context.
6. Safety and authority states remain obvious without relying only on color.
7. Mobile preserves the same mental model.
8. No internal service/module name is required to understand navigation.
9. Proposed and accepted states are not visually conflated.
10. The concept can map back to the V2c validated state/interaction evidence.

## 10. Existing Penpot evidence to reuse

Do not restart from zero.

Use:

- V2c state/interaction coverage as requirements evidence;
- V2d Direction A/B/C/D as spatial hypotheses;
- V2d Core Journey 10–15 as the current product-mode exploration;
- V2d Agent-first 20–25 as an alternative Assistant-dominant hypothesis.

The goal is to **converge**, not to add endless parallel directions.

## 11. Expected next output

Produce one refined candidate family:

- Body Home;
- Concern;
- Why / Evidence;
- Plan;
- Training;
- Outcome;
- contextual Assistant behavior;
- narrow/mobile variants for the same mental model.

Then compare it against this brief and the Product Definition before requesting human sign-off.
