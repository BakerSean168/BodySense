# Repair Baseline — 2026-06-30

## Current Branch

`dev` (7 commits ahead of `origin/dev`)

## Uncommitted Changes

- 21 modified files (see `git diff --stat`)
- 40+ untracked files (new modules: agent, governance, stream, context, workflow, migrations, etc.)
- No staged changes

## Latest Review Summary

| Phase | Status | Pass Rate |
|-------|--------|-----------|
| 01a Context Builder | PARTIAL | 3/5 |
| 01b Stream Runtime | FAIL | 2/5 |
| 01c Stream Event Reducer | PARTIAL | 3/5 |
| 02a Python Tool Registry | PARTIAL | 3/5 |
| 02b Migrate search_knowledge | PASS | 5/5 |
| 02c Migrate extract_symptom_info | PASS | 5/5 |
| 02d Agent Tool Calls Audit | PARTIAL | 3/5 |
| 03a Ask User Tool Contract | PASS | 5/5 |
| 03b Agent Interactions Resume | PARTIAL | 2/5 |
| 03c Ask User Card UI | PARTIAL | 1/5 |
| 04a Job Runtime Schema | PARTIAL | 3/5 |
| 04b Migrate OCR to Job | PASS | 4/5 |
| 05a AI Output Guard | PARTIAL | 2/5 |
| 05b Governance Review Persistence | PARTIAL | 2/5 |
| 06a Health Journey Read-Only | PARTIAL | 3/5 |
| 07a Knowledge Lifecycle Schema | PARTIAL | 2/5 |

**Totals:** 4 PASS, 11 PARTIAL, 1 FAIL

## P0 Blocker List

1. ~~01b StreamRuntime~~ — **FALSE POSITIVE** (see repair report below; module exists and tests pass)
2. 07a Knowledge Lifecycle Schema — `lifecycle_status` column missing, FK cardinality inverted
3. 03b/03c ask_user E2E — path broken, Python not registered, no resume route
4. 05a/05b AI Output Governance — FaithfulnessPolicy missing, service unwired
5. 04a/04b Job Runtime — idempotency TOCTOU race, missing fields
6. Test gaps across multiple phases

## Out of Scope (this round)

- Scope creep cleanup (AskUserCard, contract changes, UI redesign bundled in wrong phases)
- Nice-to-have items (file answer type, answered/disabled state, endpoint path alignment)
- Phase 01a test coverage (non-blocking)
- Phase 02d test coverage (non-blocking)

## Verification Commands

| Command | Scope |
|---------|-------|
| `cd apps/api && go vet ./...` | Go static analysis |
| `cd apps/api && go test ./... -count=1` | All API tests |
| `cd apps/api && go test ./internal/stream/ -v -count=1` | StreamRuntime tests |
| `cd apps/api && go build ./...` | Full API build |
| `cd apps/ai-service && uv run ruff check .` | Python lint |
| `cd apps/ai-service && uv run pytest` | Python tests |
| `cd apps/web && pnpm nx run web:typecheck` | TS typecheck |
| `cd apps/web && pnpm nx run web:lint` | TS lint |
