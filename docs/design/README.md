# BodySense Product Design Program

> Status: Active design authority
> Updated: 2026-10-06
> Decision: [ADR 0019](../adr/0019-adopt-product-semantic-first-design-workflow.md)

BodySense uses a **product-semantic-first** design workflow. UI is not allowed to define the product by accident. Business truth and user-facing semantics are fixed first; visual exploration is then performed in Penpot against those constraints.

## 1. Authority ladder

When sources disagree, use this order:

1. **Business domain authority**
   - [ADR 0004](../adr/0004-adopt-longitudinal-body-state-model.md)
   - [Longitudinal BodyState Domain Model](../architecture/longitudinal-body-state-domain.md)
2. **Product design semantics**
   - [Product Definition](./product-definition.md)
   - [Product UX Semantics](./product-ux-semantics.md)
3. **Design foundations**
   - [Design Foundations](./foundations.md)
4. **Penpot design artifacts**
   - Product Definition / Foundations pages
   - Design System
   - Exploration
   - approved product candidate
5. **Implementation**
   - React code, CSS/Tailwind tokens, component APIs and route composition

Code is evidence of the current product, not authority for future product semantics.

## 2. Design workflow

```text
Business truth
    ↓
Product Definition
    ↓
User mental model + product vocabulary
    ↓
User jobs + capability map
    ↓
Information architecture + task flows
    ↓
Design Foundations
    ↓
Components / patterns
    ↓
Current UI capture and legacy evidence
    ↓
Penpot Exploration
    ↓
Candidate product direction
    ↓
Responsive + state + interaction validation
    ↓
Human visual / interaction sign-off
    ↓
Approved design baseline
    ↓
React implementation
    ↓
Browser visual QA against approved design
```

This is the same reusable pattern established during the DigitalBiome redesign, adapted for Penpot and for BodySense's richer longitudinal health domain.

## 3. AI design handoff

After Product Definition, UX Semantics and Foundations are established, AI exploration must use the constrained [AI Product Design Brief](./ai-product-design-brief.md). The brief permits spatial/visual experimentation while freezing product authority and epistemic/safety invariants.

## 4. Gate model

### G0 — Domain is coherent

BodyState, Concern, Fact, Observation, Hypothesis, Evidence, Safety, Diagnosis, Treatment and Outcome semantics are stable enough to design against.

### G1 — Product semantics are coherent

The user mental model, jobs, vocabulary, product boundaries and authority rules are explicit. Internal service/module names must not leak into the IA by default.

### G2 — Foundations are coherent

Color roles, typography roles, spacing, radius, surfaces, epistemic status, safety status, responsive modes, motion and accessibility rules are documented independently of individual screens.

### G3 — Product exploration is coherent

A candidate must prove core jobs with realistic content and states. It must not merely restyle the current React shell.

### G4 — Responsive and state coverage is coherent

Desktop, narrow desktop and mobile preserve the same mental model. Empty, loading, error, insufficient-information, safety, correction/change and review states are represented.

### G5 — Human design sign-off

The user explicitly accepts the visual and interaction direction. Machine checks or model screenshot review do not satisfy this gate.

### G6 — Implementation parity

Implementation is reviewed against the approved Penpot baseline, not against memory or screenshots from an earlier implementation.

## 5. Existing evidence and its role

BodySense already has substantial design work. It is retained rather than discarded:

- **Current UI** — evidence of the current implementation.
- **Prototype V1** — validated prior interaction/state coverage.
- **Exploration V2b/V2c** — task, responsive and technical evidence.
- **V2c technical freeze candidate** — machine-verifiable interaction/responsive baseline, not a final visual freeze.
- **Prototype V2d** — current visual/product-shell exploration, including the Adaptive Body Canvas hypothesis.

The important change is governance: these artifacts now have explicit upstream Product Definition and Foundations.

## 6. Penpot structure

See [Penpot Design Program](./penpot-program.md) for exact page ownership and lifecycle rules.

## 7. Implementation rule

Do not resume a broad React visual rewrite while the current V2d direction is not human-approved. Small correctness fixes may continue, but a design migration must begin from an immutable approved Penpot candidate.
