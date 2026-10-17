# BodySense V2 · Executable ticket DAG

**Execution intent only.** Master Plan: [../../active/2026-10-13-v2-parallel-master.md](../../active/2026-10-13-v2-parallel-master.md); execution owner plan: [../../active/2026-10-14-v2-execution-reorganization.md](../../active/2026-10-14-v2-execution-reorganization.md); official six-gate status remains [../../active/v2-handoff-state.json](../../active/v2-handoff-state.json).

- [queue.json](./queue.json) is the durable source for execution ticket status, dependencies, exclusive resources, project/worktree bindings, active writer admission and exact accepted SHA. Each ticket has a human-readable `<ID>.md` with steps, ownership and tests.
- Dependency ordering does not override `docs/product`, existing SF-00..90 checkpoints, or the single-writer rule.
- Run `node scripts/plan/validate-v2-ticket-queue.mjs` and `node --test scripts/plan/validate-v2-ticket-queue.test.mjs` before dispatch or updating the queue.
- `INT-001` must first deliver a clean exact-SHA integration candidate; do not automatically mark it done because merge succeeds. Only then start `OPS-001` to prove T3 project cwd and live model health.
- Initial wave: at most three disjoint ticket writers in persistent, explicitly registered T3 projects, never the shared root project. ForgeFlow is a deterministic model/Agent preflight, while T3 hosts the visible Thread; Pi subagents own delegated Pi worktree/worker state.
- Migrations, OpenAPI and generated Go/TS/Zod types, handoff JSON, integration branch and deployment operations are serialized across all ticket owners. A queued ticket with a resource collision waits, even if its DAG prerequisites are complete.
- All project status values reflect **admission**, not business-readiness. A completed ticket does not mark a domain Gate Ready without independent integration evidence.

## Current checkpoint · 2026-10-16

PR6 A–E is **COMPLETE / MERGED** through #298 (`af3e1993b`), recorded by done ticket `PR6-001` and the handoff `completedPrograms` entry. Native Hypothesis/ConcernGroup, Safety and Web work already shipped; their remaining tickets cover only the residual scope named in `remainingScope`. Do not dispatch the completed PR6 work again.

The frozen baseline is main `af3e1993b1f62930e3b680b54abab5ade456935b`, with exact-main CI including PG18 baseline/recovery/browser PASS. PR #274 (TP-PROP-001) was then updated onto this baseline, passed exact-head CI and merged as `adaa02607`; dependabot #249 (CI action pins only) followed as `6687a9e26`, the current queue baseline. Residual scope per ticket, sizes and order: [2026-10-16 remaining-work plan](../../active/2026-10-16-v2-remaining-plan.md). Master §0’s big-bang **one designated writer** rule supersedes the historical three-writer pilot and parallel scheduling instructions above/below; this status sync changes no locks, scheduler settings or runtime writer ownership.

## Current checkpoint · 2026-10-17 · parallel lanes

At the user's request, Hark now implements V2 in **parallel lanes** with up to four writers (`maxParallelWriters: 4`), replacing the single-writer rule above for scheduling: docs/phase-0 (sole editor of `docs/plan/**`), lane A Diagnosis (#310 → #312 → #341), lane B Training (#311 → #313 → #315), lane C interleaved (#307, #308, #306 → #314, #309). Queue baseline is main `1abcc9f1634bb6039e00c6f9654817118ae5cf4c`. The queue validator now treats only the shared generated hotspots (`database-migrations`, `public-openapi`, `domain-contract`) as concurrent-safe under the rebase rule; every other exclusive resource still collides. GitHub issue checkboxes are the live status; this queue is synced per domain closeout, not per slice. See [remaining plan §7](../../active/2026-10-16-v2-remaining-plan.md#7-parallel-lanes--2026-10-17).

## Scheduling policy

Write a short execution handoff for each thread: `ticketId, threadId, projectId, absolute worktree, branch, expectedHead, provider/model, run status, changed paths, tests, output exact SHA, blockers`. Never retry an ambiguous T3 create/message blindly; first query its durable thread status. Stop a previous writer before ChatGPT Web assumes the same checkout. Existing hourly Track A/C/D tasks must not race new thread owners.
