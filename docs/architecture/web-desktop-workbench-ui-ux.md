# BodySense Desktop Workbench UI/UX

> Status: Implemented UI source of truth
> Date: 2026-08-17
> Scope: React web application shell and longitudinal health workbench
> Business source of truth: ADR 0004, Longitudinal BodyState Domain Model, Current Longitudinal System

## 1. Product intent

BodySense is not a linear wizard and not a chat-only product. It is a long-lived health workspace in which conversation is an input and explanation surface while durable health state is the primary working object.

The desktop experience therefore uses a **chat + workspace** composition:

- chat is an optional, collapsible companion;
- the workspace is always the primary surface;
- BodyState, Diagnosis, Treatment and Progress are explicit workspace modes;
- all modes use the same visual theme and application chrome;
- the user can collapse chat to enter a focused, full-width workspace.

This is the implementation interpretation of the supplied wireframe:

```text
Expanded
┌──────────────────────────────────────────────────────────────────────┐
│ [chat toggle]      [State] [Diagnosis] [Treatment] [Progress] [user]│
├──────────────────────────┬───────────────────────────────────────────┤
│                          │                                           │
│ AI conversation          │ Body map + selected workspace             │
│ thread + composer        │ status, reasoning, plan, outcomes         │
│                          │                                           │
└──────────────────────────┴───────────────────────────────────────────┘

Focused
┌──────────────────────────────────────────────────────────────────────┐
│ [chat toggle]      [State] [Diagnosis] [Treatment] [Progress] [user]│
├──────────────────────────────────────────────────────────────────────┤
│                    full-width health workspace                       │
│              body map + multi-column information panels              │
└──────────────────────────────────────────────────────────────────────┘
```

## 2. Experience principles

### 2.1 Workspace first

The body map and durable health objects receive the strongest visual hierarchy. Chat supports the work but does not own the product state.

### 2.2 One coherent health story

Internal aggregates remain separate, but the UI presents one understandable health workspace:

- **State** — what is known now;
- **Diagnosis** — what may explain it and what remains uncertain;
- **Treatment** — what is proposed, accepted or under review;
- **Progress** — what happened after intervention and how the state changed.

### 2.3 Explicit epistemic boundaries

The interface must preserve domain meaning:

- Fact, Observation and Hypothesis are visually distinct;
- unverified observations are clearly marked and excluded from reasoning;
- Diagnosis candidates are possibilities, not final medical diagnoses;
- Treatment output remains a proposal until explicit acceptance;
- outcomes describe association unless attribution is explicitly provided;
- active safety state interrupts normal flows and remains prominent.

### 2.4 Stable spatial memory

Top navigation, chat location and body map position stay stable while the selected workspace changes. Users should not relearn the page for each domain object.

### 2.5 Progressive disclosure

Default views show summaries and next actions. Revision history, provenance, evidence IDs and advanced controls remain available without overwhelming the primary surface.

### 2.6 Theme unity

Chat and workspace always share one global theme. A dark chat panel beside a light workspace is prohibited. The current release deliberately forces one coherent light theme while the remaining non-workbench routes are migrated to semantic tokens; the provider and dark-token foundation stay global so theme switching can be enabled later without pane-local themes.

## 3. Information architecture

### 3.1 Global application shell

The non-workbench application uses a compact navigation rail rather than a permanent 256px sidebar.

```text
ApplicationFrame
├── NavigationRail
│   ├── Brand
│   ├── Dashboard
│   ├── Profile
│   ├── Consultation
│   ├── Assessment
│   └── History
├── GlobalTopBar
│   ├── Current section
│   └── User menu
└── Route content
```

The workbench route uses an immersive variant with its own domain toolbar.

### 3.2 Workbench toolbar

- left: chat expand/collapse control and conversation/history access;
- center: route-addressable workspace tabs;
- right: BodyState revision summary and user menu;
- keyboard: `Cmd/Ctrl+B` toggles chat, number shortcuts may switch workspace tabs later.

### 3.3 Chat dock

The chat dock contains:

- compact conversation identity/history trigger;
- active consultation thread;
- streaming events, citations and human-in-the-loop cards;
- composer with attachments;
- resizable boundary;
- persisted last expanded width;
- complete collapse to 0 width.

Desktop defaults:

- default width: 38%;
- minimum useful width: 360px;
- maximum width: 52%;
- collapsed size: 0px;
- workspace never collapses.

### 3.4 Workspace modes

#### State

- body map with concern markers;
- safety status;
- current facts;
- pending observations;
- active hypotheses;
- revision summary and edit actions.

#### Diagnosis

- analysis freshness;
- candidate cards grouped by concern;
- evidence and missing-information summary;
- independent candidate assessment controls;
- regenerate action only when capability permits.

#### Treatment

- accepted current revision;
- proposed revisions;
- explicit accept/reject controls;
- interventions and constraints;
- review-required state;
- route to training execution.

#### Progress

- outcome timeline;
- trend summaries by concern;
- BodyState revision changes;
- treatment review triggers;
- causality wording preserved.

## 4. Layout behavior

### 4.1 Desktop (>= 1100px)

- workbench fills the viewport;
- toolbar height: 56px;
- chat and workspace are horizontally resizable;
- workspace content uses a 12-column grid;
- body map occupies 4–5 columns;
- detail area occupies 7–8 columns;
- focused mode may use a 4/4/4 or 5/7 layout depending on tab.

### 4.2 Tablet (768–1099px)

- chat remains collapsible but opens as a fixed-width overlay or split at a safe minimum;
- toolbar tabs may horizontally scroll;
- body map stacks above detail panels when space is insufficient.

### 4.3 Mobile (< 768px)

- no draggable split;
- chat and workspace are mutually exclusive views controlled by a segmented switch;
- workspace tabs remain horizontally scrollable;
- actions remain reachable without hover;
- body map is simplified and does not consume the entire viewport.

## 5. Visual direction

### 5.1 Design language

- quiet, professional health technology;
- dense enough for a desktop workbench, not a marketing page;
- soft surfaces, precise hierarchy and restrained motion;
- subtle organic cues without decorative blobs behind operational content;
- modern sans-serif typography; display serif is not used for workbench headings.

### 5.2 Color roles

Semantic tokens, not literal colors, drive components:

- background / surface / elevated surface;
- foreground / muted foreground;
- primary: calm BodySense green;
- accent: terracotta for user attention and selected highlights;
- success: confirmed/improving;
- warning: unverified/review recommended;
- danger: safety state/worsening/destructive;
- information: analysis/freshness.

### 5.3 Shape and spacing

- application frame: square viewport, no fake outer browser card;
- toolbar controls: 8–10px radius;
- primary panels: 14–18px radius;
- compact cards: 10–14px radius;
- 4px spacing base with 8/12/16/24/32 steps;
- borders are low contrast but always visible in both themes.

### 5.4 Motion

- chat collapse/expand: 180–240ms ease-out;
- workspace tab transition: opacity/translate no longer than 160ms;
- avoid continuous decorative animation;
- respect `prefers-reduced-motion`.

## 6. Accessibility

- separators expose the ARIA separator role and keyboard resizing;
- all icon-only controls have accessible names and tooltips;
- focus rings use semantic ring tokens;
- tab navigation uses tab semantics or equivalent accessible navigation;
- color is never the only status indicator;
- body map markers have text equivalents in the adjacent list;
- mobile controls meet at least a 40px target size;
- loading and error states remain within the region they replace.

## 7. Open-source references and adoption decisions

Research is based on upstream project documentation and repositories current on 2026-08-17.

| Project                          | Useful pattern                                                         | Decision                                                             |
| -------------------------------- | ---------------------------------------------------------------------- | -------------------------------------------------------------------- |
| `bvaughn/react-resizable-panels` | accessible resizable groups, collapsible panels, persisted layout APIs | Adopt for desktop chat/workspace split                               |
| `assistant-ui/assistant-ui`      | composable chat primitives, streaming, attachments, accessibility      | Keep; BodySense already uses a custom runtime adapter                |
| TanStack Query                   | `queryOptions`, colocated keys/functions, query/mutation separation    | Adopt query option factories and feature-owned mutation hooks        |
| Base UI                          | accessible headless primitives with styling ownership                  | Keep as the low-level interactive primitive layer                    |
| shadcn/ui                        | copied, composable application shell/sidebar patterns                  | Learn from composition; do not add a second opaque component runtime |
| Twenty                           | feature/package boundaries, shared UI package, named exports           | Learn from feature ownership and explicit public entry points        |
| LobeHub/Lobe UI                  | AI workspace visual hierarchy and theme consistency                    | Visual/interaction reference only; no direct dependency              |

Primary references:

- https://github.com/bvaughn/react-resizable-panels
- https://github.com/assistant-ui/assistant-ui
- https://tanstack.com/query/latest/docs/react/guides/query-options
- https://base-ui.com/react/overview/about
- https://ui.shadcn.com/docs/components/radix/sidebar
- https://github.com/twentyhq/twenty
- https://github.com/lobehub/lobe-ui

## 8. Non-goals

- copying ChatGPT Desktop source or branding;
- rebuilding assistant-ui streaming internals;
- changing Go/Python domain contracts for visual convenience;
- introducing a generic dashboard builder;
- exposing raw revision/evidence identifiers as primary UI content;
- adding animations that obscure durable state transitions.

## 9. Acceptance criteria

1. Desktop chat can be resized, collapsed and restored without losing thread state.
2. Focused mode gives the workspace the full available width.
3. State, Diagnosis, Treatment and Progress are explicit, deep-linkable modes.
4. The same global theme applies to chat and workspace; mixed pane themes are impossible.
5. Mobile uses a deterministic chat/workspace switch rather than a broken compressed split.
6. UI actions remain governed by backend-provided capabilities and mutation-boundary errors.
7. Reload restores conversation, active workspace mode and chat preference.
8. Unit tests cover shell state, selectors and critical action rendering.
9. Playwright covers expanded/collapsed navigation and the longitudinal loop.

## 10. Implemented component map

```text
App.tsx
└── RouteErrorBoundary + route Suspense

MainLayout
├── AppNavigationRail
├── mobile navigation drawer
├── AppUserMenu
└── route content

ConsultationPage
├── useConversationsQuery / useConsultationThreadQuery
├── useConversationActions
├── useThreadProjectionActions
├── useDiagnosisActions
└── ConsultationWorkbenchShell
    ├── resizable/collapsible chat Panel
    ├── ConversationHistoryDrawer
    └── WorkspaceViewport
        ├── BodyOverview
        ├── State: BodyStateWorkbench
        ├── Diagnosis: DiagnosisPanel + history
        ├── Treatment: TreatmentPanel
        └── Progress: OutcomeTrendsPanel
```

Server projections use feature-owned TanStack Query option factories. Mutation hooks own invalidation and structured error handling. Only auth and presentation preferences use persisted client stores; health truth remains server-owned.
