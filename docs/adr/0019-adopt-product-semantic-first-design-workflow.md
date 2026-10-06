# ADR 0019: Adopt a Product-Semantic-First Design Workflow with Penpot

> Status: Accepted
> Date: 2026-10-06

## Context

BodySense has a mature longitudinal health domain and a substantial UI implementation/design history.

The project already contains:

- a BodyState-centered domain model;
- a current React Workbench;
- a Penpot Design System;
- a V1 prototype with broad interaction/state coverage;
- extensive V2b/V2c product exploration;
- a technically validated V2c candidate;
- a V2d design reset exploring a more body-centered product shell.

However, the ordering of authority was incomplete. Product meaning, visual foundations, exploratory prototypes and implementation could be interpreted as peers. That creates a risk that whichever screen was drawn or coded most recently becomes the de facto product model.

DigitalBiome established a more robust workflow: clarify product semantics first, establish foundations, represent those decisions in the design tool, explore product screens, obtain review, then implement and visually validate.

BodySense needs the same discipline, adapted to Penpot and to its more complex epistemic/safety domain.

## Decision

BodySense adopts this authority chain:

```text
Business domain
→ Product Definition
→ Product UX Semantics
→ Design Foundations
→ Penpot Design System
→ Product Exploration
→ Approved Candidate
→ Implementation
```

### Semantic authority

Repository documentation owns product semantics and invariants.

The primary product-design documents are:

- `docs/design/product-definition.md`;
- `docs/design/product-ux-semantics.md`;
- `docs/design/foundations.md`;
- `docs/design/penpot-program.md`.

They remain downstream of ADR 0004 and the Longitudinal BodyState Domain Model.

### Visual/interaction authority

Penpot owns the visual and interaction design representation.

Penpot must provide explicit Product Definition and Foundations surfaces so a reviewer can understand the product before inspecting individual candidate screens.

### Evidence preservation

Existing assets are not discarded:

- Current UI remains current implementation evidence;
- Prototype V1 remains historical interaction/state evidence;
- V2b/V2c remain task/responsive evidence;
- V2c technical validation remains valid as technical evidence;
- V2d remains active visual/product-shell exploration.

### No automatic implementation freeze

Technical/machine validation does not constitute product approval.

A candidate becomes implementation authority only after explicit human visual/interaction sign-off and an immutable Penpot baseline is recorded.

### Navigation is not derived from backend modules

BodyState, Diagnosis, Treatment and Progress may be distinct domain artifacts without becoming permanent top-level navigation.

The leading IA hypothesis is One Body Home + Concern Canvas + contextual capabilities. Body/Today/History remains a control hypothesis during design review.

## Consequences

### Positive

- product semantics no longer drift with the latest screen;
- domain authority remains separate from visual representation;
- AI design exploration receives a bounded, reusable brief;
- Penpot can contain aggressive visual experiments without weakening business invariants;
- implementation can be reviewed against an exact approved baseline;
- future redesigns can reuse the same workflow.

### Cost

- design work has explicit gates and therefore more documentation;
- some existing Penpot pages are evidence rather than immediate implementation targets;
- broad UI implementation must wait for design sign-off when the product shell is being reset.

## Rejected alternatives

### Treat current React UI as the design source of truth

Rejected because it locks historical implementation decisions into future product semantics.

### Treat the latest Penpot prototype as the product definition

Rejected because exploratory layout decisions should remain replaceable.

### Freeze V2c because technical validation passed

Rejected because the V2c audit itself records human visual/interaction sign-off as pending.

### Redesign only by changing tokens and component styling

Rejected because V2d is explicitly testing task-shaped spatial composition and a more body-centered mental model, not merely a theme refresh.
