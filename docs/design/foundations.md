# BodySense Design Foundations

> Status: Working foundation for V2d exploration
> Updated: 2026-10-06
> Note: semantic roles are stable; literal visual values may change before V2d human sign-off.

## 1. Foundation architecture

BodySense uses four token layers:

```text
Primitive
  raw color / size / font / motion values
      ↓
Semantic
  bg.canvas / fg.primary / status.warning / action.primary
      ↓
Component
  button.primary.bg / concern.card.border / safety.banner.bg
      ↓
Product
  Body Home / Concern / Why / Plan / Training composition
```

Components consume semantic/component tokens, not arbitrary literals.

## 2. Visual character

The target character is:

- quiet health technology;
- precise rather than clinical-institutional;
- calm, low-noise and body-centered;
- dark graphite as the current working baseline;
- restrained organic warmth;
- high legibility for evidence-heavy states;
- task-specific spatial composition rather than card-everything dashboards.

The product should not look like:

- a generic admin dashboard;
- a neon “AI” interface;
- an EHR table wall;
- a chat app with health widgets attached;
- an over-soft wellness app that obscures evidence and authority.

## 3. Working color roles

The values below are inherited from the validated V1/V2c system and serve as the V2d starting baseline. V2d may tune literal values while preserving semantic roles.

| Semantic role | Working value | Use |
| --- | --- | --- |
| canvas | `#222622` | primary workspace background |
| chrome | `#1B1F1C` | top/global chrome |
| conversation/deep | `#151815` | deep contextual surface |
| surface | `#2B302B` | cards/artifacts/detail surfaces |
| surface-subtle | `#303631` | secondary control surfaces |
| border | `#424A43` | ordinary structural boundary |
| border-subtle | `#343A35` | low-emphasis separation |
| text-primary | `#F1F4F1` | principal text |
| text-secondary | `#BCC6BF` | secondary information |
| text-muted | `#9BA69E` | metadata / supportive text |
| primary | `#9BE2BB` | primary action / positive focus |
| primary-ink | `#142019` | text on primary |
| positive-bg | `#293A31` | improving/confirmed status |
| positive-fg | `#BCE7CB` | improving/confirmed status |
| selected-bg | `#3B2C29` | selected/attention state |
| selected-fg | `#F1B4A1` | terracotta attention |
| warning-bg | `#3B3528` | warning/review state |
| warning-fg | `#EAD08A` | warning/review state |

Danger/safety must have its own high-salience semantic role and must not be represented by the normal terracotta selected state.

## 4. Epistemic colors are secondary to labels

Color must never be the only carrier of epistemic meaning.

Every Fact / Observation / Hypothesis / Evidence / Proposal / Accepted state should also use:

- explicit labels;
- iconography or structural treatment when useful;
- consistent wording;
- provenance/freshness metadata where relevant.

Visual priority:

```text
Safety
> explicit user action / review required
> accepted current state
> analysis / proposed state
> background history / metadata
```

## 5. Typography roles

Typography should support both calm overview and dense evidence review.

Recommended semantic scale for the redesign:

| Role | Working size / line height | Weight |
| --- | --- | --- |
| display | 28 / 34 | 650–700 |
| page/title | 22 / 28 | 650–700 |
| section | 17 / 24 | 600–700 |
| artifact title | 15 / 21 | 600–700 |
| body | 13 / 20 | 400–500 |
| compact body | 12 / 18 | 400–500 |
| label | 11 / 16 | 550–650 |
| metadata/badge | 9–10 / 14 | 550–650 |

Rules:

- normal user-facing prose should not depend on 9–10px text;
- 9–10px is reserved for compact badge/meta usage;
- evidence-heavy layouts may become denser, but hierarchy must remain readable without relying on color;
- use a modern sans-serif family consistently unless a later brand study justifies a change.

## 6. Spacing

Base unit: **4px**.

Preferred semantic steps:

`4 / 8 / 12 / 16 / 20 / 24 / 32 / 40 / 48 / 64`

Rules:

- 8–12px for compact control internals;
- 16–24px for ordinary artifact internals;
- 24–32px between meaningful sections;
- 40–64px for major mode/scene separation;
- dense evidence is achieved through structure and grouping, not by collapsing every gap.

## 7. Radius

Working radius roles:

- 8px — compact controls;
- 10px — buttons/tabs;
- 12px — compact artifacts;
- 16px — primary panel/artifact;
- 20–24px — rare large spatial containers;
- 999px — pills/badges.

Avoid turning every surface into a rounded floating card. A task mode may use large continuous planes.

## 8. Surfaces and elevation

Use surface contrast before shadows.

Suggested hierarchy:

1. Canvas
2. Structural plane / scene
3. Artifact surface
4. Interactive/selected surface
5. Overlay/sheet/dialog

Dark interfaces should preserve visible boundaries without high-contrast “boxed” clutter.

## 9. Body as a first-class visual asset

The body explorer / spatial body representation is not decorative illustration.

It may serve as:

- the primary anchor of Body Home;
- a region selector;
- a spatial index into concerns;
- a summary of current relevant body regions;
- a bridge between body-level and concern-level context.

It must not become a second source of health truth independent of BodyState.

## 10. Task-shaped composition

V2d may use different compositions for different jobs:

- **Body Home** — immersive body atlas / overview;
- **Concern** — narrative/current-state canvas;
- **Why** — evidence studio;
- **Plan / Outcome** — longitudinal journey composition;
- **Training** — focused execution mode;
- **Assistant** — contextual pane/drawer/sheet depending on viewport and task.

A shared visual system does not require a shared page skeleton.

## 11. Motion

Default motion is functional and restrained:

- 120–160ms for local state transitions;
- 180–240ms for pane/sheet transitions;
- use opacity/translate/size changes that preserve spatial continuity;
- no continuous decorative motion;
- honor `prefers-reduced-motion`.

Motion should explain:

- where contextual Assistant came from;
- how a selected body region becomes a Concern;
- how a focused task opens and returns;
- how a review/safety state changes authority.

## 12. Responsive foundations

Breakpoints are implementation details; product modes are semantic:

- **wide** — simultaneous primary + supporting context possible;
- **narrow** — one primary canvas plus overlays/secondary panels;
- **mobile** — one dominant task surface, transient sheets/routes for supporting context.

No responsive mode may silently remove a core capability.

## 13. Accessibility

Minimum expectations:

- WCAG-aware contrast for text/status/control states;
- keyboard reachable primary workflows;
- visible focus treatment;
- icon-only controls have accessible names;
- status never encoded by color alone;
- touch targets generally >= 40px;
- reduced-motion support;
- body-region selection has text/list equivalents;
- loading/error states remain attached to the region they replace;
- safety guidance is announced and remains understandable without visual styling.

## 14. Foundation change policy

Semantic token names and meanings are more stable than literal values.

A V2d visual study may change `#9BE2BB`, surface luminance or radius values. It may not silently change what “primary action”, “warning”, “accepted”, “proposed”, or “safety” mean.

When foundations change:

1. update this document;
2. update the Penpot Foundations page;
3. update Penpot tokens/components;
4. propagate into candidate screens;
5. run visual/accessibility QA;
6. only then update implementation tokens.
