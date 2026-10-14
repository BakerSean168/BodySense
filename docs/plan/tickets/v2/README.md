# BodySense V2 · Executable ticket DAG

**Execution intent only.** Master Plan: [../../active/2026-10-13-v2-parallel-master.md](../../active/2026-10-13-v2-parallel-master.md); execution owner plan: [../../active/2026-10-14-v2-execution-reorganization.md](../../active/2026-10-14-v2-execution-reorganization.md); official six-gate status remains [../../active/v2-handoff-state.json](../../active/v2-handoff-state.json).

- [queue.json](./queue.json) is the durable source for execution ticket status, dependencies, exclusive resources, project/worktree bindings, active writer admission and exact accepted SHA. Each ticket has a human-readable `<ID>.md` with steps, ownership and tests.
- Dependency ordering does not override `docs/product`, existing SF-00..90 checkpoints, or the single-writer rule.
- Run `node scripts/plan/validate-v2-ticket-queue.mjs` and `node --test scripts/plan/validate-v2-ticket-queue.test.mjs` before dispatch or updating the queue.
- `INT-001` must first deliver a clean exact-SHA integration candidate; do not automatically mark it done because merge succeeds. Only then start `OPS-001` to prove T3 project cwd and live model health.
- Initial wave: at most three disjoint ticket writers in persistent, explicitly registered T3 projects, never the shared root project. ForgeFlow is a deterministic model/Agent preflight, while T3 hosts the visible Thread; Pi subagents own delegated Pi worktree/worker state.
- Migrations, OpenAPI and generated Go/TS/Zod types, handoff JSON, integration branch and deployment operations are serialized across all ticket owners. A queued ticket with a resource collision waits, even if its DAG prerequisites are complete.
- All project status values reflect **admission**, not business-readiness. A completed ticket does not mark a domain Gate Ready without independent integration evidence.

## Scheduling policy

Write a short execution handoff for each thread: `ticketId, threadId, projectId, absolute worktree, branch, expectedHead, provider/model, run status, changed paths, tests, output exact SHA, blockers`. Never retry an ambiguous T3 create/message blindly; first query its durable thread status. Stop a previous writer before ChatGPT Web assumes the same checkout. Existing hourly Track A/C/D tasks must not race new thread owners.
