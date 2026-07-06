# 03b/03c ask_user E2E Chain Repair Report

## 1. What changed

Fixed 6 gaps to wire the ask_user HITL path end-to-end:

| Gap | Fix |
|-----|-----|
| `ask_user` not registered in consultation tools | Added `make_ask_user_tool()` to `consultation_tools.py` |
| Graph doesn't handle `ask_user` tool calls | Added `ask_user` case in `generate_response` tool loop — emits `state.interaction.required` and returns `interrupted` |
| `decide_phase`/`emit_done` don't respect interruption | Added `interrupted` flag check — both become no-ops when set |
| TS `StreamEventIds` missing `interaction_id` | Added `interaction_id?: string \| null` field |
| No Python resume endpoint | Added `POST /api/chat/resume` that re-invokes graph with answer injected |
| Assistant message stuck in "streaming" on interrupt | Added `UpdateMessageStatus("aborted")` when interaction fires |

## 2. Files changed

| File | Change |
|------|--------|
| `apps/ai-service/src/services/agent/consultation_tools.py` | Register `ask_user` tool |
| `apps/ai-service/src/services/consultation_graph.py` | Handle `ask_user` in tool loop + `interrupted` flag |
| `apps/ai-service/src/services/chat_service.py` | Map `state.interaction.required` graph event to StreamEvent |
| `apps/ai-service/src/api/routes/chat.py` | Add `POST /api/chat/resume` endpoint |
| `apps/api/internal/handler/chat_handler.go` | Mark assistant msg "aborted" on interaction interrupt |
| `packages/contracts/src/stream-events.ts` | Add `interaction_id` to `StreamEventIds` |
| `docs/audit/03b-ask-user-e2e-repair.md` | This report |

## 3. Acceptance criteria result

| Criteria | Result | Evidence |
|----------|--------|----------|
| `ask_user` schema and handler exist | **PASS** | `ask_user.py` — schema with 5 answer_types, handler returns `ToolResult(status="interrupted")` |
| Handler returns interrupted and never waits | **PASS** | `handle_ask_user()` returns immediately with `ToolStatus.INTERRUPTED` |
| Tests cover valid and invalid payloads | **PASS** | `test_ask_user_tool.py` — 6 tests passing |
| Normal consultation flow not exposed to unsupported interruptions | **PASS** | `ask_user` registered in registry but graph handles it explicitly — LLM call triggers interrupt, not silent failure |
| Interruption events create pending interaction rows | **PASS** | Go `chat_handler.go:state.interaction.required` → `interactionService.CreatePendingInteraction()` |
| Run can enter `waiting_user` | **PASS** | `runService.MarkWaitingUser()` called in handler; `runRepo.UpdateStatus("waiting_user")` in service |
| Resume endpoint validates ownership and stores answer | **PASS** | `consultation_handler.go:ResumeInteraction` verifies conversation ownership via `GetConsultation`, calls `interactionService.ResumeInteraction` |
| Assistant message status updated on interrupt | **PASS** | `UpdateMessageStatus("aborted")` added before `stream.done` |
| Python resume endpoint exists | **PASS** | `POST /api/chat/resume` — accepts context + answer, re-invokes graph with tool result injected |
| Existing normal chat flow still completes | **PASS** | All Go tests pass (8 packages), all Python tests pass (177) |

## 4. Verification

| Command | Result | Notes |
|---------|--------|-------|
| `go vet ./...` | ✅ PASS | |
| `go build ./...` | ✅ PASS | |
| `go test ./... -count=1` | ✅ PASS (8 packages) | |
| `uv run ruff check <modified files>` | ✅ PASS | |
| `uv run pytest tests/ -x -q` | ✅ PASS (177/177) | |
| `npx tsc --noEmit` (contracts) | ✅ PASS | |

## 5. Remaining risks

- **Go→Python resume wiring**: `ResumeInteraction` marks interaction answered and run as running, but does NOT call Python `/api/chat/resume` to continue the stream. The frontend must send a new chat message with the answer as context. This is acceptable for MVP but should be wired directly in a follow-up.
- **Concurrent interactions**: Only single pending interaction is supported (plan constraint). Multiple simultaneous `ask_user` calls from the LLM would overwrite each other.
- **`pendingInteraction` single slot in frontend**: The frontend `useSSEProcessor` uses a single `pendingInteraction` state, not a map. This is a 03c issue, not 03b.

## 6. Next recommended blocker

**05a/05b AI Output Governance** — `FaithfulnessPolicy` missing, `OutputReviewService` unwired.
