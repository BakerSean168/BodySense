# React Desktop Workbench Refactor Plan

> Status: Batch C implemented locally; awaiting reviewed PR and merge
> Started: 2026-08-17
> Target branch sequence: one reviewed PR per batch, each merged only after CI is green
> UI source of truth: `docs/architecture/web-desktop-workbench-ui-ux.md`

## 0. Implementation record

| Batch                          | Commit         | PR         | Merge / validation state                                                    |
| ------------------------------ | -------------- | ---------- | --------------------------------------------------------------------------- |
| A — shell foundation           | `2ed1f6c`      | #45        | merged as `690b508`; PR checks and main CI run `32027181859` passed         |
| B — consultation workbench     | `fd71cb0`      | #46        | merged as `b2c27a1`; PR checks and main CI run `32038310048` passed         |
| C — data engineering / quality | pending commit | pending PR | implementation and local release/deploy gates passed; remote review pending |

Batch C local validation on 2026-08-17:

- repository lint/type/test/build gate: passed;
- Web: 29 test files, 140 tests passed;
- Python: 182 tests passed;
- Go full test suite: passed;
- PostgreSQL full-up/latest-down/replay-up: passed at migration version 34;
- domain semantics validator: BodyState, Treatment activation and Outcome feedback passed;
- browser longitudinal E2E: 3 passed;
- `LOCAL_DEPLOY_VALIDATION=PASS`.

## 1. Why this refactor exists

The Go and Python services now expose a coherent longitudinal model, but the React application still carries the shape of the earlier product:

- a permanent 256px navigation sidebar consumes desktop space;
- `ConsultationPage.tsx` owns querying, cache edits, mutations, responsive layout and all domain panels in one file;
- BodyState, Diagnosis and Treatment appear as a narrow right-side document rather than a primary workspace;
- chat cannot be completely collapsed into a focused mode;
- route-level UI state and local preferences are not modeled explicitly;
- data types, API calls, query configuration and component mutation behavior are mixed across feature files;
- many components use literal colors instead of semantic design tokens.

The target is not a cosmetic reskin. It is a presentation-layer refactor that aligns React with the durable business model.

## 2. Architectural constraints

1. Go remains the durable business truth.
2. Python remains the Agent/runtime reasoning service.
3. React consumes projections and never reconstructs business truth from transcript text.
4. Existing stream event and assistant-ui runtime behavior must remain intact.
5. Backend capabilities are advisory for presentation; mutation responses remain authoritative.
6. The refactor must preserve current HTTP contracts.
7. Every batch must be releasable and independently green.

## 3. Target frontend boundaries

```text
src/
├── app/
│   ├── navigation/
│   ├── providers/
│   └── routing/
├── components/
│   ├── layout/              # application chrome only
│   └── ui/                  # reusable headless/styled primitives
├── features/
│   ├── consultation/
│   │   ├── api/
│   │   ├── model/           # query options, selectors, view models
│   │   ├── hooks/
│   │   ├── runtime/         # existing active-turn projection
│   │   └── components/
│   │       └── workbench/
│   └── workspace/
│       ├── api/
│       ├── model/
│       ├── hooks/
│       ├── components/
│       └── types/
└── stores/                  # persisted client-only preferences/auth only
```

Rules:

- server state lives in TanStack Query;
- local durable preferences live in small Zustand stores with persistence;
- ephemeral form state remains local to the owning component;
- query keys and query functions are colocated through `queryOptions` factories;
- mutation hooks own invalidation and user-facing error mapping;
- components receive view models and callbacks, not raw cache-manipulation responsibilities;
- feature directories export a deliberate public API through `index.ts`.

## 4. Library review

### Adopt

#### react-resizable-panels

Use the upstream v4 primitives:

- `Group` for the desktop split;
- `Panel` for chat and workspace;
- `Separator` for accessible pointer/keyboard resizing;
- `panelRef.collapse/expand` for the explicit toolbar toggle;
- `onLayoutChanged` for persistence after user interaction.

Why not hand-roll: pointer capture, keyboard behavior, constraints and ARIA separator semantics are easy to get subtly wrong.

### Keep and deepen

- `@assistant-ui/react`: already owns chat runtime primitives and streaming presentation;
- `@tanstack/react-query`: server state, cache and mutation lifecycle;
- `@base-ui/react`: accessible headless dialogs/menus/buttons;
- `zustand`: client-only shell preferences;
- `lucide-react`: coherent icon set;
- Tailwind CSS 4 + semantic CSS variables: styling and theme system.

### Do not add

- a second full design system;
- Redux for server state;
- a general-purpose dashboard/grid layout library;
- a new form framework during the shell refactor;
- schema validation dependency until generated/shared contracts are ready.

## 5. Batch plan

### Batch A — Foundation and application shell ✅

Branch: `feature/web-workbench-foundation`

Deliverables:

- UI/UX architecture document;
- implementation plan;
- open-source library decisions;
- `react-resizable-panels` dependency;
- global theme provider and semantic theme foundation; theme switching is enabled only after all active surfaces are tokenized;
- centralized navigation model;
- compact application navigation rail and user menu;
- semantic shell tokens and reduced hard-coded colors;
- layout unit tests;
- no changes to backend contracts.

Acceptance:

- all existing routes remain reachable;
- the released shell uses one coherent theme; no route renders mixed pane themes;
- standard pages use the new shell;
- consultation route remains functional;
- `pnpm verify:release` passes;
- PR checks pass before merge.

### Batch B — Consultation workbench cutover ✅

Branch after Batch A merge: `feature/web-consultation-workbench`

Deliverables:

- extract workbench orchestration from the page component;
- route-addressable workspace mode (`state`, `diagnosis`, `treatment`, `progress`);
- resizable/collapsible chat dock;
- immersive workbench toolbar matching the supplied wireframe;
- conversation history drawer/overlay instead of a permanent third column;
- responsive mobile chat/workspace switch;
- body overview visual with accessible concern markers;
- domain-specific workspace panels;
- preserve assistant-ui runtime instance while resizing/collapsing;
- workbench component and selector tests;
- Playwright coverage for collapse/focus mode.

Acceptance:

- chat state and active stream survive panel collapse;
- full-width focused mode works after reload;
- each workspace mode renders from the same HealthWorkspace projection;
- current longitudinal E2E remains green;
- PR checks pass before merge.

### Batch C — Data engineering, quality and completion ✅ locally validated

Branch after Batch B merge: `feature/web-workbench-data-quality`

Deliverables:

- split workspace API, query options, mutations and selectors;
- centralize cache invalidation policy;
- remove direct API mutation orchestration from BodyState/Treatment components;
- replace raw record traversal with typed view-model selectors;
- error boundaries and scoped empty/loading states;
- accessibility audit and keyboard behavior tests;
- visual token cleanup for workbench components;
- final responsive and reduced-motion pass;
- archive this plan with real validation results;
- update current architecture documentation.

Acceptance:

- no feature component directly edits TanStack cache except dedicated model/hooks;
- no duplicated health-workspace query key literals;
- tests cover selectors and mutation invalidation;
- local deploy validation passes;
- PR and post-merge `main` CI pass.

## 6. Review checklist for every batch

### Domain correctness

- no UI state silently mutates domain meaning;
- capabilities are honored;
- safety gates remain visible and deterministic;
- unverified observations remain excluded;
- proposal/accepted revision distinction remains explicit;
- causality language remains correct.

### Engineering

- public feature exports are intentional;
- no circular feature dependency;
- server and client state are not mixed;
- route identity remains the source of truth for active conversation;
- components do not duplicate query keys;
- loading, error, empty and stale states are represented;
- no new console errors or React key warnings.

### UX/accessibility

- keyboard focus is visible;
- icon buttons have accessible names;
- panel separator is keyboard operable;
- mobile controls work without hover;
- theme is consistent across panes;
- reduced motion is respected.

### Validation

```bash
pnpm lint
pnpm typecheck
pnpm test
pnpm build
git diff --check
pnpm validate:local-deploy
```

The local deploy validator is required before the final batch; focused checks may be used during intermediate implementation.

## 7. Rollback strategy

- each batch merges independently;
- existing HTTP and stream contracts are retained;
- the assistant runtime adapter is not rewritten in the layout batch;
- new shell/workbench components are introduced behind route composition boundaries;
- if a batch fails, revert that merge commit without rolling back the longitudinal backend domain.

## 8. Completion definition

The refactor is complete only when:

1. all three batches are merged to `main`;
2. PR checks and post-merge main checks are green;
3. the active plan is archived with actual commit/PR/check results;
4. the application uses the chat + workspace layout from the supplied wireframe;
5. React data boundaries match the documented target architecture;
6. the working tree is clean and local `main` matches `origin/main`.
