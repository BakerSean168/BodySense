# BodySense Product Design Reset — Semantic-First Program

> Status: Active
> Started: 2026-10-06
> Branch: `design/bodysense-semantic-foundations`

## 1. Why this program exists

BodySense already has:

- a mature longitudinal BodyState domain model;
- a current React Workbench;
- a componentized Penpot Design System;
- V1 interaction/state coverage;
- extensive V2b/V2c task and responsive exploration;
- a technically validated V2c candidate;
- a V2d visual/product-shell reset.

What it lacked was an explicit upstream design authority comparable to the DigitalBiome redesign workflow.

Without that layer, a prototype can accidentally become the product definition. This program fixes that governance gap.

## 2. North-star workflow

```text
Domain
→ Product Definition
→ UX semantics / IA
→ Foundations
→ Components
→ Product exploration
→ Candidate
→ Human approval
→ Implementation
→ Visual QA
```

## 3. Phase 0 — Preserve evidence

Status: **COMPLETE**

- keep Current UI;
- keep Prototype V1;
- keep V2b/V2c exploration and audit evidence;
- keep V2d exploration;
- do not rewrite React as part of this phase.

## 4. Phase 1 — Semantic source of truth

Status: **IMPLEMENTED IN THIS BRANCH**

Deliverables:

- `docs/design/product-definition.md`;
- `docs/design/product-ux-semantics.md`;
- explicit user jobs;
- product vocabulary;
- authority/invariant rules;
- One Body Home + Concern Canvas leading IA;
- Body/Today/History retained as a control hypothesis.

Acceptance:

- no core IA concept depends on backend module names;
- Fact/Observation/Hypothesis/Evidence remain distinct;
- Safety/correction/time/Treatment authority rules are explicit.

## 5. Phase 2 — Design foundations

Status: **IMPLEMENTED IN THIS BRANCH**

Deliverables:

- `docs/design/foundations.md`;
- token layering;
- working color roles derived from validated current system;
- typography/spacing/radius/surface roles;
- epistemic and safety semantics;
- motion/responsive/accessibility rules;
- task-shaped composition principle.

Acceptance:

- foundations can be discussed independently of an individual screen;
- V2d may change literals without changing semantic token meaning.

## 6. Phase 3 — Penpot governance

Status: **COMPLETE**

Deliverables:

- renamed Penpot file to `BodySense · Product Design`;
- added `BodySense · Product Definition` with 7 review boards;
- added `BodySense · Foundations` with 8 review boards;
- retain Design System / Current UI / V1 / Exploration / V2d pages;
- mirror semantic source material into concise review boards;
- validate page/board presence after write.

Acceptance:

- a new reviewer can understand product meaning before opening prototypes;
- V2d is visibly downstream of product semantics/foundations.

## 7. Phase 4 — V2d product exploration

Status: **WORKING / NOT FROZEN**

Continue the existing Adaptive Body Canvas hypothesis:

- Body Home — Body Atlas;
- Concern — narrative/current state;
- Why — Evidence Studio;
- Plan/Outcome — Journey;
- Training — Focus mode;
- Assistant — contextual capability.

Required next review:

1. compare the core V2d journey against Product Definition;
2. check silhouette/task-mode differentiation;
3. check visual hierarchy and Body prominence;
4. apply Foundations consistently;
5. add narrow/mobile parity before freeze;
6. obtain explicit human visual/interaction approval.

## 8. Phase 5 — Approved baseline

Status: **BLOCKED ON HUMAN DESIGN APPROVAL**

When approved:

- mark the exact Penpot candidate APPROVED;
- save immutable snapshot/version metadata;
- record foundation version;
- freeze responsive/state matrix;
- write implementation change set.

Do not infer approval from technical validation.

## 9. Phase 6 — React implementation

Status: **NOT STARTED BY THIS PROGRAM**

Implementation should be ticketed by vertical product slice rather than by CSS layer:

1. shell/context infrastructure;
2. Body Home;
3. Concern Canvas;
4. Why/Evidence;
5. Plan;
6. Training focus;
7. Outcome/history;
8. Assistant context surfaces;
9. responsive parity;
10. visual QA and accessibility repair.

## 10. Exit criteria

The program is complete when:

- semantic docs are merged;
- Penpot semantic/foundation pages exist;
- one V2d candidate is explicitly approved;
- candidate has desktop/narrow/mobile parity;
- implementation matches the approved baseline;
- browser capture visual QA shows no unexplained divergence.
