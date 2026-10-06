# BodySense Penpot Design Program

> Status: Active
> Updated: 2026-10-06

## 1. Purpose

Penpot is the visual and interaction design source of truth for BodySense. Repository documents remain the semantic source of truth.

This separation is intentional:

- **Docs answer:** what the product means and which invariants must hold.
- **Penpot answers:** how those semantics are composed visually and interactively.
- **Code answers:** how the approved design is implemented.

## 2. Page ownership

### BodySense · Product Definition

Human-readable product semantics mirrored from repository design docs:

- product thesis;
- user mental model;
- domain semantics;
- user jobs and capability map;
- product IA;
- design invariants;
- workflow and gates.

### BodySense · Foundations

Visual-system foundations:

- foundation architecture;
- color roles;
- typography;
- spacing/radius;
- surfaces/elevation;
- epistemic/safety semantics;
- responsive modes;
- motion/accessibility.

### BodySense · Design System

Reusable components and composition contracts:

```text
Tokens → Primitives → Shell/Domain → Patterns → Layouts → Product assembly
```

The existing Design System page remains valid and is downstream of Foundations.

### BodySense · Current UI

Current implementation capture/evidence. It is not the desired future design by default.

### BodySense · Prototype V1

Historical validated state/interaction baseline. Preserve for regression evidence.

### BodySense · Exploration

Branching design research and task/responsive experiments. V2b/V2c remain valuable evidence.

### BodySense · Prototype V2d

Current product-design exploration. Its working hypothesis is **Adaptive Body Canvas**:

- Home → Body Atlas;
- Concern → Narrative Canvas;
- Why → Evidence Studio;
- Plan/Outcome → Journey composition;
- Training → Focus mode;
- Assistant → contextual capability.

V2d is not approved merely because it exists.

## 3. Lifecycle

```text
Current / evidence
      ↓
Exploration
      ↓
Candidate
      ↓
Responsive + interaction + state QA
      ↓
Human review
      ↓
Approved / frozen baseline
      ↓
Implementation
      ↓
Browser capture comparison
```

## 4. Version semantics

Use explicit design states instead of overwriting history:

- **LEGACY** — historical product implementation or deprecated idea;
- **CURRENT** — current shipped implementation/evidence;
- **WORKING** — active exploration;
- **CANDIDATE** — technically coherent and ready for human review;
- **APPROVED** — explicitly accepted for implementation;
- **IMPLEMENTED** — implementation parity checked.

V2c currently qualifies as technical evidence/CANDIDATE for task-state coverage, but not as a final visual APPROVED baseline.

V2d is WORKING until explicit review.

## 5. Source-first rules

When reconstructing implementation in Penpot:

1. inspect source structure and domain ownership;
2. capture the live/browser rendering as visual ground truth;
3. rebuild using Penpot foundations/components;
4. compare against the capture;
5. keep raw capture as evidence rather than as editable product structure.

Do not trace screenshots into anonymous groups and call that a design system.

## 6. Exploration rules

An exploration branch must state:

- hypothesis;
- user job;
- semantics it preserves;
- assumptions it intentionally challenges;
- states it covers;
- responsive implications;
- reason to continue or reject it.

Visual variation without a product hypothesis is not a product-design branch.

## 7. Candidate acceptance

A candidate can advance to human review only when:

- core jobs are represented;
- authority and epistemic semantics are preserved;
- safety flow exists;
- correction vs temporal change exists;
- responsive modes preserve mental model;
- critical empty/loading/error states exist;
- action-looking controls are wired or explicitly noninteractive;
- no legacy module IA leaks in without deliberate justification.

## 8. Implementation handoff

The handoff package should include:

- exact Penpot page/board identifiers or immutable snapshot/version;
- selected candidate direction;
- foundation version;
- component/system dependencies;
- responsive matrix;
- state matrix;
- known deviations;
- implementation order;
- visual QA checklist.

Implementation starts only after the user marks the visual/interaction candidate approved.
