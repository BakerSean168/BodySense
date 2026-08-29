# Consultation State Acquisition v2 — Product/Project Closure Plan

Status: ACTIVE
Owner: BodySense Consultation / BodyState vertical slice
Date: 2026-08-29
Base: `feature/vanatome-body-explorer@297f8a1b`

## Problem statement

The current workbench looks like the intended one-conversation product, but the
runtime still behaves like a generic chat. In staging evidence from the last 14
days there were completed assistant turns, but no `tool.call`, no
`state.extracted_info.upsert`, and no `state.interaction.required` events. The
right-hand BodyState therefore remains empty while the left-hand model can give a
long answer and merely append a free-text question.

This is not primarily a rendering defect. The tool and HITL infrastructure
exists, but the model is allowed to bypass it. The product has no enforceable
state-acquisition invariant between a user health statement and a completed
assistant answer.

## Product contract

For every consultation turn:

1. Explicitly stated health information is classified before the visible answer.
2. Model-mediated extraction is durably stored as an unverified candidate and is
   excluded from reasoning until confirmed.
3. When a new symptom lacks high-value triage/context fields, the run pauses with
   one structured `ask_user` card (at most three fields). The ordinary composer is
   locked while the interaction is pending.
4. A structured answer is persisted into the same candidate as confirmed BodyState
   before the Agent resumes.
5. General information questions do not create BodyState records or forced forms.
6. The right-hand workspace refreshes when state is acquired, not only after a
   page reload.
7. Safety signals continue to outrank ordinary intake and diagnosis/treatment.

## PM audit findings

- **One durable relationship:** present in routing/workbench shell.
- **Conversation -> living BodyState:** infrastructure present, behavior not
  enforced; this plan closes the gap.
- **Structured interruption:** UI/runtime present, invocation optional; this plan
  makes it policy-driven for material intake gaps.
- **Fact vs inference boundary:** violated for extracted symptoms because
  unverified facts were not always excluded from reasoning; this plan repairs the
  durable invariant and adds defense-in-depth filtering.
- **Visible review queue:** observations are visible, extracted fact candidates
  are not; this plan adds `pending_facts` to the workspace projection.
- **Diagnosis/Treatment:** outside this slice; they consume only confirmed current
  BodyState after this repair.

## Implementation slices

### S1 — Immutable Consultation v2 configuration

- Add a repository-versioned typed intake configuration to the Consultation
  manifest.
- Preserve v1 for replay; make v2 the Champion only after all gates pass.

### S2 — Typed state-acquisition preflight

- Add a PydanticAI intake Agent that extracts only explicit latest-turn symptom
  and lifestyle statements.
- Emit durable state candidate events before any visible answer.
- Add a deterministic gap policy and LangGraph interrupt node.

### S3 — Durable answer promotion

- Use a stable `capture_id` across extraction and structured answer.
- Upsert the same fact from `unverified/excluded` to
  `confirmed/included` before resume.
- Store structured fields rather than a generic `user_answer` blob for symptom
  intake interactions.

### S4 — Workspace projection and interaction UX

- Add `pending_facts` to HealthWorkspace.
- Render reviewable extracted facts separately from confirmed current records.
- Invalidate/refetch workspace immediately on extraction and interaction events.

### S5 — Verification, staging, cleanup

- Unit/contract tests for health statement, general question, structured
  interruption, resume promotion, and unverified exclusion.
- Full Go/Python/Web checks and production build.
- Re-anchor staging Compose from the canonical repository, remove stale deleted
  worktree residue, prune merged local worker branches, deploy, and run a real
  event/DB smoke test.

## Acceptance gates

- A new incomplete symptom turn emits at least one
  `state.extracted_info.upsert` before `state.interaction.required` and emits no
  assistant prose before the interruption.
- The ask card contains 1–3 typed fields and does not require the user to retype a
  free-form response in the ordinary composer.
- Resume updates the captured symptom to `confirmed` and
  `excluded_from_reasoning=false` before Python receives refreshed BodyState.
- An unverified extraction is absent from `GetCurrent()` but present under
  `pending_facts`.
- A general knowledge question completes without BodyState mutation or forced
  interruption.
- Staging containers reference `/home/dev/projects/bodysense`, not a removed
  worktree path.
