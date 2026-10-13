# BodySense V2 · Product Design Workstream

Status: ACTIVE / TRACK B. [Primary program](./2026-10-13-v2-parallel-master.md) · [Readiness register](./v2-handoff-state.json) · [Domain contract track](./2026-10-13-v2-domain-schema.md).

## Design authority and baseline

Product semantics are owned by [requirements](../../product/requirements.md), [interface](../../product/interface.md), [domain](../../product/domain/README.md) and [acceptance](../../product/acceptance.md), **not** by pixels or earlier UI code. Visual baseline is the editable [BodySense · Product Design Penpot Product page](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=2) (file `24d9d841-759d-81bc-8008-b6423250c2b8`, page `ed4c6ec0-8f91-80cb-8008-c1e744bad64b`).

Preserve the 3D body as a persistent underlying canvas and floating Chat/Person/Diagnosis/TrainingPlan/Trainer windows; no return to the old left/right fixed split. [UI PR #244](https://github.com/BakerSean168/BodySense/pull/244), worktree `/home/dev/projects/bodysense-penpot-ui`, contains design-related source work but currently fails Governance and browser E2E. It is **a review candidate, not an accepted implementation**. At this checkpoint no new Penpot edits, screenshots or browser parity checks have been performed by this track.

## Parallel handoff and definition of done

- Track A's accepted `view_model` contract defines **which data/statuses exist**; track B defines **what users see and how they act**. Interaction wireframes may start before that contract, but do not mark a business-flow design ready until mapping is reviewed against it.
- Every display field maps to a named query/read-model field or a documented presentation-only derivation. Every active control maps to a real command, an explicit user action, or a clearly non-persistent local state.
- Data-bound examples for `populated`, `empty`, `loading`, `failed`, `permission_denied`, `stale`, `offline/recovering` and `pending/committed` are required where applicable. AI processing progress != successful persistence.
- The design file must contain reusable editable components (not screenshots pasted as designs), a short flow graph and desktop + narrow-screen review at the actual Viewer viewport. No static frame explosion for every combined theme/state.
- **Draft** plan is not accepted training; proposed Diagnosis condition is not adopted; a symptom Episode is not the same as a diagnosis; internal revision/Run IDs are hidden from default UI; unadmitted OCR is not clinical evidence.
- A visual handoff records a Penpot file/page/frame locator, the relevant product rule/acceptance IDs, typed projection/command names, interaction sequence, open visual exceptions, and screenshot/Viewer evidence. Only then may `design=ready` be recorded.

## UI domains and required flows

| Window       | Required default data and states                                                                                              | Specific interaction risk                                                                             |
| ------------ | ----------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| Chat         | One Conversation, recent rolling timeline, active Run, Ask User replacing composer, saved receipt, interrupted/recovery/error | Choosing Ask User is not a second Chat Run; closing window does not accept/reject                     |
| Person       | Records, Equipment, Media, Reports, Habits; Identity popover; empty/loaded/error                                              | Latest BodyFact display must refresh after receipt; no fabricated profile or duplicate asset identity |
| Diagnosis    | Adopted ConditionEntry list/count, evidence role, alternative/support/contra and info gaps                                    | Multiple proposal confirmations are serial and committed atomically; no suspected=diagnosed inference |
| TrainingPlan | Accepted plan/template, proposal diff, schedule/frequency and safety status                                                   | Proposal preview is not a Trainer session; changes require exact consent                              |
| Trainer      | One running session, exact exercise/set, pause/restart/skip/finish, safety hold                                               | Simply navigating cannot write a completed set; restrictions must remain visible                      |

Cross-window design obligations: active body-region selection; body model remains visible; focus/escape/window restoration; mobile task overlay; keyboard/accessibility labels; meaningful counts (e.g. adopted conditions only); privacy-safe errors.

## Executable tickets

### PD-BASE-01 — Source audit and reusable shell/state inventory

**Inspect:** editable Penpot Product/Components/Foundations, source UI PR #244 and current React shell. **Deliver:** component inventory and gaps for Dock, floating window header, identity popover, toolbar, scroll, dialogs, modal focus, icons and loading/empty/errors. **Acceptance:** each observed behavior has an actual frame/component/source reference; don't claim pixel parity from a mock. Verify #244 failures independently before adopting its code.

### PD-BS-01 — Records/Habits design handoff

**Depends on:** DS-BS-01 `view_model=ready` for final data mapping. **Deliver:** Records/Habits states for self-statement source, temporal Episode, assertion correction/withdrawal, unknown time, conflict/Ask User and real receipt→refresh; editable desktop/mobile components. **Acceptance:** each field/control maps to a real projected field/command; rejected API writes never display 'saved'; visual states inspected in Viewer.

### PD-PC-01 — One Chat and Person identity

**Depends on:** DS-PC-01. **Deliver:** no list/create/share conversation UX, one pending Ask User replacing composer, run/reconnect, title-header time display, identity popover and focused keyboard flow. **Acceptance:** cross-device/pending state diagram and mobile recoverability reviewed.

### PD-DX-01 — Diagnosis and exact proposal confirmation

**Depends on:** DS-DX-01. **Deliver:** adopted-only count/list, ConditionEntry evidence detail, review prompt, serial pending proposal cards, supersession and safety_blocked feedback; resolve [Q-19 presentation](../../product/open-questions.md) within existing authorization rules. **Acceptance:** no pre-commit saved state or medical confirmation-by-click.

### PD-TP-01 — TrainingPlan, Trainer and Safety flows

**Depends on:** DS-TP-01. **Deliver:** draft proposal preview vs accepted plan, per-set activity, pause/recovery, scoped advisory/restricted/urgent displays, narrow-screen layouts. **Acceptance:** every displayed set-status has a user-authorized source; risk states match backend semantics.

### PD-ASSET-01 — Equipment, Media/Reports and two delete flows

**Depends on:** DS-ASSET-01. **Deliver:** shared UserAsset UI with processing/admissibility/source-availability distinction; explicit distinct removal/erasure prompts and failure states. **Acceptance:** no misleading 'erased' completion while controlled copies remain.

## Design QA and checkpoint

Initial status: existing Penpot base available, but **no V2 domain design gate is signed off**. PD-BASE-01 may proceed in parallel with DS-BS-01; PD-BS-01 can prototype early but only close after typed contract review and a real viewer walkthrough. Final visual polish (spacing, motion, icon consistency and broad responsive tuning) belongs to Track D **after** workflow and technical-state behavior are proven, not before.

### Same-Run Ask User checkpoint

The actual React pending question now replaces normal input; the same shared AskUserCard supports retry/draft, accessible labels, separate choice groups and all required fields. Desktop and 390px light/dark sheet screenshots were inspected; canvas tokens and the dark heading were corrected. This is implementation visual evidence, not editable Penpot V2 sign-off or a whole-Chat theme rewrite. See [bounded contract, acceptance and test evidence](./2026-10-13-ask-user-source-contract.md). The latest execution registry is [v2-handoff-state.json](./v2-handoff-state.json); all whole-domain/release gates remain open.

### 2026-10-13 · Explicit Penpot kickoff handoff (not a Viewer sign-off)

[BodyState ViewModel and Episode design brief](./2026-10-13-bodystate-viewmodel-episode-handoff.md) now lists the actual Records/Habits fields and all seven read states, plus Episode uncertain/evidence-backed states. It is sufficient to start editing the existing Product / Components / Foundations page without awaiting all migrations. The React window now consumes the bounded ViewModel and has component regressions for partial Habits history and inconsistent revision pages. **PD-BASE-01 and PD-BS-01 remain OPEN** until actual editable Penpot frames/flows, visual review and contract-to-frame acceptance; this code change did not modify the official Penpot design file.
