# BodySense Workbench UI V2

Date: 2026-08-26
Status: Phase 1 implemented / visual review pending
Branch: `feature/workbench-ui-v2`

## Product decision

BodySense exposes one continuous AI relationship to the user. The UI does not expose multi-chat management, thread branching, model selection, or conversation-history navigation as primary product concepts.

Desktop shell remains a two-surface workbench:

- left: collapsible AI conversation
- right: business workbench with `状态 / 分析 / 方案 / 进展`

The current two-surface information architecture is retained. This phase focuses on visual language, chat ergonomics, motion, and surface hierarchy rather than another page-structure rewrite.

## Reference direction

- assistant-ui owns chat runtime/state/accessibility and primitive behavior.
- AI Elements is used selectively as source-level visual composition/reference, not as a second chat runtime.
- ChatGPT/Codex Desktop is the visual reference for graphite surfaces, inset rounded geometry, floating composer, restrained chrome, and smooth collapse/expand behavior.
- BodySense green remains a semantic accent rather than the dominant background hue.

## Phase 1 scope

1. Remove user-facing multi-conversation affordances from the workbench shell.
2. Restyle the AI conversation as a dark graphite inset surface.
3. Keep the composer permanently bottom-anchored, including the empty state.
4. Replace the form-like `图片 + textarea + 发送` layout with a single floating composer surface.
5. Render assistant messages mostly flat/full-width and user messages as subtle secondary bubbles.
6. Simplify workbench tabs into quiet tab navigation instead of pill-card navigation.
7. Replace visible engineering vocabulary where touched (`Rxx`, count-as-badge, conversation switching copy) with product-facing language or hide it.
8. Add desktop-like motion tokens for panel collapse/expand and workspace transitions, respecting `prefers-reduced-motion`.
9. Preserve existing BodyState / Diagnosis / Treatment / Progress behavior and backend contracts.
10. Preserve the existing assistant-ui runtime and durable SSE/HITL behavior.

## AI Elements integration boundary

Do not introduce `Conversation` or `PromptInput` as competing owners of thread/composer state.

Phase 1 may add local `components/ai-elements/*` source components only where they are behavior-neutral and useful (for example response typography/source presentation). Existing assistant-ui primitives remain the behavioral owner.

## Non-goals

- no model selector
- no new chat button
- no thread list / history UI
- no response branching
- no 3D body map
- no backend conversation-domain migration in this phase
- no change to AI routing/provider configuration
- no redesign of diagnosis/treatment business semantics

## Acceptance criteria

- Desktop shows only a collapsible left conversation surface plus right workbench.
- No visible consultation-history control exists in the primary shell.
- Empty chat keeps composer at the bottom; starting a conversation does not relocate it.
- Conversation surface is graphite/dark and visually distinct from the workbench without heavy borders.
- Collapse/expand feels continuous and preserves workbench state.
- Workbench tab selection uses a quiet indicator rather than a pill-card group.
- Existing consultation shell and page tests are updated and green.
- `web:typecheck`, `web:lint`, focused web tests, and production build pass.
- `git diff --check` passes.

## Implementation checkpoint — 2026-08-26

Phase 1 is implemented on `feature/workbench-ui-v2` and deployed to the current staging web container for visual review.

Implemented:

- retained the two-surface `Conversation | Workbench` shell and removed the primary consultation-history affordance;
- changed the left conversation into a graphite inset surface with a permanently bottom-anchored floating composer;
- kept assistant-ui as the runtime / message / composer behavioral owner;
- added a project-local `components/ai-elements/*` presentation layer adapted from the AI Elements composition model, intentionally omitting competing conversation runtime, model selector, and response branching;
- changed assistant turns to a mostly flat presentation and user turns to subtle secondary bubbles;
- restyled streaming, tool calls, sources, ask-user, cancellation/failure and safety states for the dark conversation surface;
- simplified the workbench header into quiet `状态 / 分析 / 方案 / 进展` tabs with a lightweight active indicator;
- retained resizable/collapsible panels and added continuous collapse/expand and view-transition motion with reduced-motion support;
- shows the body map only in `状态`, avoiding a permanent three-column layout on analysis/plan/progress views;
- removed visible revision/proposal/Fact/Observation/Outcome engineering vocabulary from the primary status, diagnosis, treatment and progress surfaces where touched;
- reduced nested cards in BodyState, Treatment and Outcome projections and converted their copy to product-facing Chinese.

Validation:

- `git diff --check`: pass
- `pnpm nx run web:typecheck --skip-nx-cache`: pass
- `pnpm nx run web:lint --skip-nx-cache`: pass
- web full test suite: **163 passed / 31 files**
- `pnpm nx run web:build --skip-nx-cache`: pass
- staging web image rebuild: pass
- staging web + API health: healthy, HTTP 200 on local port `20150`

Next checkpoint is visual acceptance in the real browser. Any remaining work should be driven by the actual staging screenshot rather than another top-level layout rewrite.

## Content-density refinement — 2026-08-26

The populated workbench now follows a stricter progressive-disclosure rule: data and actions stay visible; explanatory copy moves to empty, safety, stale/error, or confirmation states.

Changes:

- removed visible `身体状态地图`, its subtitle, normal `当前状态` badge, and the persistent body-map disclaimer; the body SVG keeps accessible non-visible description text;
- normal body-map state now shows the figure and real region summaries only; safety review remains visible because it is actionable;
- body-map guidance appears only when no region data exists;
- removed the duplicated `当前身体状态` heading and explanation above BodyState;
- removed the permanent explanatory subtitle under `身体记录`; zero-record copy is now a compact borderless empty state;
- moved diagnosis generation into the analysis empty state instead of keeping a permanent explanatory header/CTA row;
- normal fresh diagnosis no longer displays a freshness banner; only stale / possibly stale conditions surface that status;
- removed the repeated generic diagnosis disclaimer from populated analysis; safety-specific medical messaging remains in the red-flag state;
- simplified analysis-history heading and hides normal `fresh` status badges;
- removed the duplicate outer headings for Treatment and Progress; their actual content owns the single useful heading;
- removed permanent Treatment explanatory copy and redundant proposal provenance copy; confirmation guidance is shown only in empty/pending-review states;
- simplified Progress to divider-based data rows instead of framed mini-cards and added a real empty state when no trend/outcome data exists;
- introduced `bodysense-workspace-surface` semantic tokens so the business workbench uses a lighter neutral graphite (`#272a27`) while conversation remains `#171717`.

Validation after content-density refinement:

- `git diff --check`: pass
- web typecheck: pass
- web lint: pass
- web full test suite: **163 passed / 31 files**
- production web build: pass
- canonical staging web rebuilt with `COMPOSE_PROJECT_NAME=bodysense-staging`: healthy, HTTP 200 on `127.0.0.1:20150`.

## Visual-density refinement — 2026-08-27

Applied from the latest real staging screenshot, focusing on spatial density rather than adding new chrome:

- stopped the BodyOverview grid item from stretching to the full viewport height, which had vertically centered the body map too low;
- changed the body figure to a fixed intrinsic aspect/width (`252px` max) so it renders larger and closer to the top of the workbench;
- tightened the state split to a slightly stronger body-map column while keeping the record column dominant;
- reduced the max width of non-state workbench views to improve reading measure;
- tightened BodyState heading/icon/button scale and vertical rhythm;
- replaced populated fact, hypothesis, observation and history mini-cards with divider-based rows;
- reduced empty-state vertical padding so sparse state screens do not feel artificially tall;
- slightly increased top workbench tab size/spacing for native-app readability;
- lifted the workbench surface from `#272a27` to `#292c29`, preserving a visible separation from the `#171717` conversation surface without turning it into a light dashboard.

Validation:

- Prettier run on touched files;
- `git diff --check`: pass;
- web typecheck: pass;
- web lint: pass;
- web full test suite: **163 passed / 31 files**;
- production web build: pass;
- canonical staging web rebuilt with `COMPOSE_PROJECT_NAME=bodysense-staging`; HTTP 200 on `127.0.0.1:20150`.

## Visual refinement checkpoint — 2026-08-26

Applied after reviewing the first real staging screenshot:

- conversation surface is now flush to the left and bottom edges, with no outer border/shadow and only the top-right corner rounded;
- removed desktop shell padding around the two panels so conversation and workbench share the same content height;
- reduced the draggable separator's visible gap while keeping the resize affordance;
- changed the workbench base from the conversation's `#171717` graphite to a visibly lighter neutral `#242624` surface;
- removed the workbench's outer border, radius and shadow;
- removed the Body Map card background/border/shadow so the figure sits directly on the workbench surface;
- removed the state content wrapper background/radius so the right-side status content also sits directly on the workbench surface;
- replaced top-level `Card` wrappers in BodyState, Treatment and Progress projections with plain transparent layout containers to avoid nested frame-on-frame presentation;
- simplified body-region summary rows to separators instead of filled mini-cards.

Validation after refinement:

- `git diff --check`: pass
- web typecheck: pass
- web lint: pass
- web full test suite: **163 passed / 31 files**
- production web build: pass
- corrected an initial Compose invocation that used the wrong project name, removed only the accidentally created duplicate `docker-*` staging resources, and then rebuilt the canonical `bodysense-staging` web service with `--no-deps`;
- canonical staging web: healthy, HTTP 200 on `127.0.0.1:20150`.

## Safety / coexistence note

The worktree was already dirty when this plan started, containing prior single-workbench, staging, and provider changes. This task must not revert or overwrite unrelated existing modifications. Validation and eventual staging should be path-focused before any commit.
