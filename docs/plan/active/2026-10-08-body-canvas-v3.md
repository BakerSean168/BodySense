# Body Canvas V3 — Penpot to implementation

Status: implementation complete / browser verified / PR-ready · 2026-10-08
Execution owner: ChatGPT Web (direct implementation; no delegated writer)
Branch: `feature/body-canvas-v3`, based on the repository's current `origin/main`. The remote has no `dev` branch and CI/PR workflows target `main`, so integration follows the existing feature-branch → PR → `main` path; the canonical main worktree is never used as a direct writer.

## User-authorized direction

The supplied `bodysense-3d (2).html` is the visual/interaction direction: a full-screen, detailed body; pointer selection, brush selection and questions; movement demonstrations with pause/frame inspection; game-inventory-like training, analysis, equipment and personal-state overlays with keyboard access. The user explicitly authorizes refinement and immediate implementation without an intermediate approval pause. This is direction/implementation authorization, not a claim of human review of the finished candidate.

Preserve the longitudinal BodyState, evidence, safety, proposed/accepted Treatment, corrections versus new changes and Outcome contracts established by ADR 0004 / ADR 0019. The prototype's hardcoded diagnoses, automatic exercise advice, scores and level/XP are not production health data.

## Design baseline

Penpot file: `24d9d841-759d-81bc-8008-b6423250c2b8` / `BodySense · Product Design`.
Create additive V3 pages; do not overwrite V1/V2d or upstream foundations.
Use light mineral/sage canvas as supplied, optional dark mode, restrained green selection, a large body center, narrow command chrome, contextual region card, floating command dock and task overlays. All body selections must have a text alternative. Controls target 44px; keyboard shortcuts ignore text inputs, composition and modifiers. Escape unwinds the top layer first and restores focus.

## Delivery sequence

1. Read current repository, domain contracts and existing Penpot design program; preserve other worktrees and dirty canonical changes.
2. Create V3 foundations/components, desktop/mobile core journeys and critical state boards in Penpot. Bind visible navigation to prototype destinations. Use the actual atlas render for the body, not a screenshot of the whole UI.
3. Verify Penpot layout and persist exact page/board/flow identifiers and design export evidence.
4. Implement the body-first shell, lazy detailed scene, region/brush/box selection, motion playback/inspection, contextual Assistant and real business-panel composition.
5. Preserve structured multi-region/motion context through the message and AI boundary; never convert a demonstration into an observation or diagnosis.
6. Add focused state, semantic mapping, context validation, keyboard and browser interaction tests; run typecheck, lint, production build and relevant regression suites.
7. Capture desktop/mobile implementation, verify against the Penpot baseline, repair identified issues, commit/push the feature branch and integrate through the repository's PR-to-`main` path when verification is green. Do not promote production automatically.

## 3D boundary

Reuse the versioned Vanatome 1.4.0 atlas, build `994e6cc8ffbb212e`, and existing metadata/CDN and CC BY-SA attribution. This atlas has no authored skeleton/animation. V3's procedural pose rig is an educational visualization, not a measured personalized digital twin or validated biomechanical simulator. Anatomical detail remains available independently. No live load/strain/pain inference is made from a pose. Rig limitations must be named in UI and handoff.

## Coverage

Home / selected region / multi-region brush / contextual chat / motion paused / analysis and evidence / proposed and accepted plans / focus training / outcome / equipment / profile / history / keyboard help / empty / loading / errors and 2D fallback / insufficient evidence / safety blocked / correction versus change. Wide and mobile core journeys share terms and state authority.

## Evidence log

- Repository baseline: `301c2888f007c88c4bc6fd5b7c1175e4809e776d`.
- Canonical worktree has pre-existing `apps/web/index.html` modification; untouched.
- Existing design commits integrated: `deea866db`, `a2d42d977` (cherry-picked into this branch).
- Penpot connection reverified after implementation: V3 Foundations = 4 boards, V3 Components = 12 boards, Body Canvas V3 = 30 product boards with 495 prototype hotspots. Seven legacy/current pages remain additive and untouched.
- Atlas GLB inspected: 6,288,144 bytes; no skins. No health data was copied into design artifacts.
- Browser acceptance: Body Canvas V3 desktop/mobile flow passes in Chromium (1/1, 24.3s), including canonical region selection, real BodyState display, region-only versus motion context, shortcuts, inventory dialogs and 390×844 no-horizontal-overflow checks.
- Longitudinal business regression: 2/2 Playwright scenarios pass against the isolated worktree API/Web runtime.
- Contract/runtime verification: contract conformance + generated-artifact check pass; Go `internal/consultation` + `internal/service` pass; Python spatial-context manifest suite = 12 passed.
- Focused Web tests: Body Canvas/reference rig/workbench preference suite = 11 passed; ConsultationPage + Body Explorer workspace/store/adapter/pin suite = 17 passed; BodyRegionStatusSummary = 1 passed; Vite configuration = 4 passed.
- Final Web gates: typecheck, lint, production build and `git diff --check` pass. The lazy `Body3DThreeRuntime` chunk is 718.43 kB raw / 181.54 kB gzip and remains under its explicit Three-only budget; `BodyCanvasScene` itself is 21.40 kB raw / 8.45 kB gzip.
- Worktree dev-font 403 was traced to Vite resolving the shared `node_modules` symlink outside the worktree allow-list. The dev config now permits only the repository root plus the resolved dependency root; the exact Geist font request returns HTTP 200 without widening access to `/home/dev/projects`. The heavy Body Explorer browser flow subsequently passes 1/1 (5.5m) with no console error, and the atlas-metadata fallback passes 1/1 (8.5s).
- Repository integration policy rechecked during implementation: GitHub workflows trigger PR/CI against `main`; there is no remote `dev` branch, so no synthetic branch is created.
