# Batch Review Report: 16 Implementation Plans

## Summary Table

| Phase | Status | Criteria Pass Rate | Key Gaps |
|-------|--------|-------------------|----------|
| 01a-context-builder | PARTIAL | 4/5 PASS, 1 PARTIAL | Missing history filtering unit tests |
| 01b-stream-runtime | PASS | 5/5 PASS | None |
| 01c-stream-event-reducer | PARTIAL | 3/5 PASS, 2 PARTIAL | AskUserCard + layout redesign scope violations; missing reducer interaction tests |
| 02a-python-tool-registry | PARTIAL | 3/5 PASS, 2 FAIL | consultation_graph.py modified; ask_user.py added out of scope |
| 02b-migrate-search-knowledge | PARTIAL | 4/5 PASS, 1 PARTIAL | extract_symptom_info also migrated; ask_user.py present; missing dedicated test file |
| 02c-migrate-extract-symptom-info | PASS | 5/5 PASS | None |
| 02d-agent-tool-calls-audit | PARTIAL | 3/5 PASS, 1 PARTIAL, 1 FAIL | Full agent_interactions + ask_user + resume implemented; no tests |
| 03a-ask-user | PARTIAL | 4/5 PASS, 1 FAIL | Full Go persistence + resume endpoint implemented out of scope |
| 03b-agent-interactions-resume-api | PARTIAL | 4/5 PASS, 3 PARTIAL, 1 FAIL | No Python resume stub; TS contract missing interaction_id; incomplete idempotency |
| 03c-ask-user-card-ui | PARTIAL | 2/5 PASS, 3 PARTIAL | No file type placeholder; no answered state; zero tests |
| 04a-job-runtime-schema | FAIL | 3/5 PASS, 2 FAIL | OCR migrated to JobRuntime out of scope |
| 04b-migrate-ocr-to-job-runtime | PARTIAL | 4/5 PASS, 1 PARTIAL | Idempotency gap; no integration tests |
| 05a-ai-output-guard-skeleton | PARTIAL | 3/5 PASS, 2 PARTIAL | Go ai_output_review persistence out of scope; missing faithfulness policy |
| 05b-governance-review-persistence | PARTIAL | 3/5 PASS, 1 PARTIAL, 1 FAIL | OutputReviewService never wired; missing schema columns; no Go tests |
| 06a-health-journey-readonly | PASS | 5/5 PASS | None |
| 07a-knowledge-lifecycle-schema | PARTIAL | 3/5 PASS, 2 PARTIAL | Schema diverges from plan (FK cardinality, lifecycle_status, defaults) |

**Overall: 3 PASS, 12 PARTIAL, 1 FAIL**

---

## Detailed Findings

### phase-01a-context-builder (PARTIAL)

**What is partial:**
- Previous-turn completed-message filtering has no unit tests. The filtering logic at `context_builder.go:155` is correct but untested -- `getMessageTextContent` and `loadHistory` filtering behavior is only asserted via field-level struct tests, not behavioral tests.

**Elegance issues:**
- `profileService` field is stored on `ChatHandler` struct (line 35, 60) but never accessed via `h.profileService` -- dead code.
- `ContextBuilder` is instantiated inside `NewChatHandler` (line 53) rather than injected, coupling the handler to the concrete type.

**Recommended next steps:**
1. Add unit tests for `loadHistory` filtering (exclude current turn, exclude non-completed, exclude empty text).
2. Remove dead `profileService` field from `ChatHandler` struct.
3. Consider injecting `Builder` interface for better handler testability.

---

### phase-01b-stream-runtime (PASS)

No action required. Clean extraction of sequence allocation and ID enrichment from `ChatHandler` into `stream.Runtime` / `stream.StreamWriter`. All 6 tests pass. Minor dead code path in `enrichEvent` (seq==0 guard unreachable from `SendNew`) is harmless.

---

### phase-01c-stream-event-reducer (PARTIAL)

**What acceptance criteria failed or are partial:**
- **AskUserCard implemented** -- `AssistantChatPanel.tsx` imports `AskUserCard` (line 8), manages `pendingInteraction` state, handles interaction submission via `consultationApi.resumeInteraction`. Plan explicitly stated "Do not implement AskUserCard."
- **AssistantChatPanel layout redesigned** -- Refactored from manual `displayMessages` to `ThreadPrimitive`/`MessagePrimitive` with `MarkdownTextPrimitive` and `remarkGfm`. Plan stated "Do not redesign AssistantChatPanel layout."
- **useSSEProcessor extended** with `onInteractionRequired`/`onInteractionAnswered` handlers beyond stated scope.

**Invariants violated:**
- Current visible chat streaming behavior: rendering path was redesigned, may change visible behavior.
- No backend or contract changes: holds true.

**Recommended next steps:**
1. Confirm the `ThreadPrimitive`-based layout produces equivalent visual output to the old manual mapping.
2. Add reducer tests for `state.interaction.required` and `state.interaction.answered` events.
3. Document the AskUserCard as pre-work for Phase 03c, or revert if it was not intentional.

---

### phase-02a-python-tool-registry (PARTIAL)

**What acceptance criteria failed:**
- **consultation_graph.py modified** to import from `agent.consultation_tools` and route both `search_knowledge` and `extract_symptom_info` through `executor.execute()`. Plan explicitly prohibited this.
- **ask_user.py added** at `tools/ask_user.py` with full handler, schema, and 6 passing tests. Plan explicitly stated "Do not add ask_user."

**Invariants violated:**
- Existing consultation streaming behavior was changed (tools now execute through registry/executor rather than inline).
- `ToolValidationError` is defined but never raised (dead code).

**Elegance issues:**
- `ToolDuplicateError` defined and used but NOT exported from `__init__.py`.

**Recommended next steps:**
1. Confirm that the migrated graph behavior is functionally identical to the original inline execution.
2. Either raise `ToolValidationError` in `_validate` or remove the dead export.
3. Export `ToolDuplicateError` from `__init__.py`.

---

### phase-02b-migrate-search-knowledge (PARTIAL)

**What acceptance criteria partial:**
- **extract_symptom_info also migrated** into `consultation_tools.py:20` despite explicit prohibition.
- **ask_user.py exists** but is not registered -- still outside scope.
- **Missing `test_search_knowledge_tool.py`** -- plan expected dedicated tests for success, no-results, and repeated-query scenarios through the registry/executor path.

**Recommended next steps:**
1. Add `test_search_knowledge_tool.py` covering the handler through the registry/executor path.
2. Document that extract_symptom_info was migrated early (pre Phase 02c work).

---

### phase-02c-migrate-extract-symptom-info (PASS)

No action required. Clean migration with proper handler/orchestration separation. Schema single source of truth maintained in `prompts/consultation.py`. One minor concern: when `executor.execute` fails, `generate_response` still emits an `extracted_info` event with raw `tc_args` (line 434), which could surface malformed data.

---

### phase-02d-agent-tool-calls-audit (PARTIAL)

**What acceptance criteria partial/failed:**
- **tool.result error detection hardcoded to `isError: false`** (chat_handler.go:318-322). Failed tool calls are recorded as "succeeded" with the error payload in the result field. The stream event contract has no `is_error` field.
- **Full agent_interactions + ask_user + resume implemented** -- migration 000016, model, repository, service, handler wiring, and `ResumeInteraction` endpoint. All explicitly listed as "Not Allowed."
- **No tests** for `AgentToolCallRepository` or `AgentToolService`.

**Invariants violated:**
- No interaction/resume endpoint should be added (FAIL -- `ResumeInteraction` endpoint exists at `consultation_handler.go:122-178`).

**Recommended next steps:**
1. Add `is_error` field to `ToolResultEvent` in the stream contract, or document the limitation.
2. Add repository/service tests for the audit persistence layer.
3. Acknowledge the agent_interactions code as pre-work for Phase 03a/03b.

---

### phase-03a-ask-user (PARTIAL)

**What acceptance criteria failed:**
- **Full Go persistence implemented** -- agent_interactions model, migration 000016, repository with full CRUD, service with `CreatePendingInteraction` (sets `waiting_user` status), and resume HTTP endpoint. Plan explicitly prohibited all three.

**Elegance issues:**
- `answer_type` enum missing `file` (plan says `text | single_choice | multi_choice | number | date | file`).
- Extra `context` field not in plan schema.
- Content shape is flat dict instead of plan's nested `{"type": "ask_user", "payload": {...}}` wrapper.
- No prompt policy file created.

**Recommended next steps:**
1. Add `file` to `answer_type` enum or document its deferral.
2. Align content shape with plan's nested structure, or document the deviation.
3. Create the prompt policy file.

---

### phase-03b-agent-interactions-resume-api (PARTIAL)

**What acceptance criteria failed/partial:**
- **No Python resume endpoint** -- plan step 9 requires a Python route stub. Without it, after user answers, the run goes back to "running" but nothing re-invokes the AI. Conversation is effectively stuck.
- **TypeScript `StreamEventIds` missing `interaction_id`** -- Go and Python have it, but `packages/contracts/src/stream-events.ts:11-17` and the JSON schema lack the field.
- **Idempotent resume incomplete** -- spec requires returning existing answer on same-key replay and 409 on different-payload conflict. Current implementation returns generic error for any already-answered interaction.

**Invariants violated:**
- A pending interaction belongs to one user-owned conversation: resume handler does not verify `interaction.conversationID == URL conversationId`.
- Repeating resume must not duplicate user answers: returns error instead of existing answer.

**Elegance issues:**
- Assistant message stays in "streaming" status after interruption -- no status update to "waiting" or "interrupted."
- Resume handler does not trigger a new AI stream to continue the conversation.
- Error string comparison at `consultation_handler.go:164` is fragile.

**Recommended next steps:**
1. **Critical**: Implement Python resume endpoint that re-invokes the AI stream with the user's answer.
2. Add `interaction_id` to TypeScript `StreamEventIds` and JSON schema.
3. Implement proper idempotency (return existing answer on same-key replay).
4. Verify `interaction.conversationID` matches URL in resume handler.
5. Update assistant message status on interruption.

---

### phase-03c-ask-user-card-ui (PARTIAL)

**What acceptance criteria partial:**
- **No file answer_type handling** -- not even a disabled placeholder. Plan says "show disabled placeholder if not supported."
- **No visible answered state** -- card disappears on success rather than showing disabled/answered state.
- **Zero tests** for interaction events in reducer, `resumeInteraction` in service, or `AskUserCard` component.

**Invariants violated:**
- Duplicate submit guard is weak -- `isSubmitting` prop disables button but no request-level dedup (ref guard or abort controller).

**Recommended next steps:**
1. Add disabled placeholder for `file` answer type.
2. Render answered/confirmed state before card disappears.
3. Add reducer tests for interaction events.
4. Add `resumeInteraction` service test.
5. Add `AskUserCard` component test.

---

### phase-04a-job-runtime-schema (FAIL)

**What acceptance criteria failed:**
- **OCR migrated to JobRuntime** -- `UploadService` now depends on `JobRuntime`, `processOCR` was replaced with `processOCRWithJob` that creates durable job records, `main.go` wiring changed. Plan explicitly stated "Do not migrate OCR in this ticket."
- **No tests** for `AgentToolCallRepository` or `AgentToolService`.

**Invariants violated:**
- Existing upload/OCR behavior changed.
- Migration schema diverges from design notes (missing `idempotency_key`, `attempts`, `max_attempts`, `updated_at`, `completed_at`; status `succeeded` instead of `completed`; `waiting_user` and `timed_out` omitted from transition map).
- Repository methods diverge (no user-scoped access control).

**Recommended next steps:**
1. Revert OCR migration or explicitly accept it as pre-work for Phase 04b.
2. Add missing schema fields (`idempotency_key`, `attempts`, `updated_at`) if they are needed for future phases.
3. Add `GetByIDForUser` method for user-scoped access control.

---

### phase-04b-migrate-ocr-to-job-runtime (PARTIAL)

**What acceptance criteria partial:**
- **Idempotency mechanism** -- plan specified job-level idempotency key (`upload_ocr:<upload_id>`). Implementation checks `upload.ocr_status` instead, which has a TOCTOU race window and no database-level enforcement.

**Elegance issues:**
- Stale comment references nonexistent `processOCR` function.
- Job type `"ocr"` instead of plan's `"upload.ocr_extract"`.
- Error handling silently logs on failure to create/transition job -- upload can stay in "pending" forever.

**Recommended next steps:**
1. Add job-level idempotency key to prevent duplicate OCR work.
2. Add integration tests with a fake AI OCR server.
3. Fix stale comment.

---

### phase-05a-ai-output-guard-skeleton (PARTIAL)

**What acceptance criteria partial/failed:**
- **No `GovernanceContext` type defined** -- context is passed as bare `dict[str, Any]`.
- **No `FaithfulnessPolicy` adapter** -- plan step 6 requires it explicitly.
- **Go ai_output_review persistence exists** (model, repository, service, migration 000018) -- explicitly "Not Allowed."

**Recommended next steps:**
1. Define `GovernanceContext` typed dataclass.
2. Implement `FaithfulnessPolicy` adapter around `FaithfulnessChecker`.
3. Acknowledge Go persistence as pre-work for Phase 05b, or revert.

---

### phase-05b-governance-review-persistence (PARTIAL)

**What acceptance criteria failed:**
- **No integration path implemented** -- `OutputReviewService` is never instantiated in `main.go`, never injected, never called. `RecordReview` is dead code. Python `AIOutputGuard` is never imported by any endpoint.

**Invariants violated:**
- Raw output storage has no guard against storing secrets or full prompts.
- Review statuses have no Go-side validation to match Python `GovernanceStatus`.
- `user_id` column missing from migration (plan says NOT NULL).
- `prompt_version`, `model`, `provider`, `business_ref_type`, `business_ref_id` columns missing.

**Recommended next steps:**
1. Wire `OutputReviewService` into `main.go` and inject into at least one handler.
2. Add missing columns to migration (especially `user_id`).
3. Add Go tests for repository and service.
4. Document raw output storage policy.

---

### phase-06a-health-journey-readonly (PASS)

No action required. Clean implementation with 10 test cases covering all 7 stages. Pure `determineStage` function is well-designed. Minor gap: `MissingRequirements` field in DTO is never populated.

---

### phase-07a-knowledge-lifecycle-schema (PARTIAL)

**What acceptance criteria partial:**
- **`lifecycle_status` column missing** -- replaced by `lifecycle_metadata` JSONB (different name, type, and semantics). Loses ability to index and filter by status.
- **`publication_id` FK missing** -- replaced by `published_version` INT (version counter, not FK).
- **`knowledge_publications` FK cardinality inverted** -- plan has units referencing publications, implementation has publications referencing units. Fundamentally changes data model.
- **`knowledge_publications` defaults** -- status defaults to `published` instead of `draft`; missing `created_at`/`updated_at` columns.

**Recommended next steps:**
1. Add `lifecycle_status` column with index, or document `lifecycle_metadata` as the chosen approach and add GIN index.
2. Clarify FK cardinality intent -- if publications should bundle multiple units, add `publication_id` FK to `knowledge_units`.
3. Change default status to `draft`.
4. Add `created_at`/`updated_at` to `knowledge_publications`.

---

## Priority Gap List

### Blocking (must fix before merge)

| # | Phase | Issue |
|---|-------|-------|
| 1 | 03b | No Python resume endpoint -- conversation is stuck after user answers |
| 2 | 03b | TypeScript `StreamEventIds` missing `interaction_id` -- contract inconsistency |
| 3 | 03b | Resume handler does not verify `interaction.conversationID` matches URL |
| 4 | 07a | `lifecycle_status` column missing -- cannot filter by lifecycle state |
| 5 | 07a | `knowledge_publications` FK cardinality inverted from plan design |

### Quality (should fix before release)

| # | Phase | Issue |
|---|-------|-------|
| 6 | 03c | Zero tests for interaction reducer events, resume service, AskUserCard |
| 7 | 02d | Zero tests for agent tool call audit persistence |
| 8 | 02d | `tool.result` error detection hardcoded to false -- all failures recorded as succeeded |
| 9 | 03b | Idempotent resume returns error instead of existing answer |
| 10 | 05b | `OutputReviewService` never wired -- dead code |
| 11 | 05b | Missing `user_id` NOT NULL column in ai_output_reviews |
| 12 | 04b | No job-level idempotency key -- TOCTOU race on duplicate OCR |
| 13 | 04b | No integration tests for OCR job flow |
| 14 | 03c | No visible answered state -- card disappears instead of confirming |
| 15 | 03c | No file answer_type placeholder |
| 16 | 01a | Missing history filtering unit tests |
| 17 | 02b | Missing dedicated search_knowledge tool test file |
| 18 | 05a | Missing FaithfulnessPolicy adapter |
| 19 | 05a | No GovernanceContext typed dataclass |

### Nice-to-have (improve code quality)

| # | Phase | Issue |
|---|-------|-------|
| 20 | 01a | Dead `profileService` field on ChatHandler struct |
| 21 | 01a | ContextBuilder instantiated rather than injected |
| 22 | 01c | ThreadPrimitive layout redesign may change visible behavior |
| 23 | 02a | `ToolValidationError` dead code (never raised) |
| 24 | 02a | `ToolDuplicateError` not exported from `__init__.py` |
| 25 | 03a | `answer_type` enum missing `file` option |
| 26 | 03a | Content shape deviates from plan's nested structure |
| 27 | 04b | Stale comment references nonexistent `processOCR` |
| 28 | 04b | Job type `"ocr"` vs plan's `"upload.ocr_extract"` |
| 29 | 07a | `knowledge_publications` defaults status to `published` instead of `draft` |
| 30 | 07a | `knowledge_publications` missing `created_at`/`updated_at` |

---

## Overall Assessment

**Implementation completeness: 3/16 phases fully pass their acceptance criteria.** The remaining 12 are PARTIAL and 1 is FAIL. The most common pattern is **scope violation** -- implementations frequently include work from future phases (agent interactions, ask_user, resume, OCR migration, Go persistence). This creates coupling risks and makes it harder to verify that each phase's boundaries are respected.

**Code quality is generally good where scope is correct.** The pure modules (stream reducer, context builder, health journey workflow, tool registry/executor) are clean, well-structured, and properly tested. The effect system in the stream reducer and the transition map in the job runtime are elegant designs. Test coverage is strong for Python modules (tool registry: 13 tests, ask_user: 6 tests, output guard: 7 tests, health journey: 10 tests) but weak for Go modules (no tests for agent tool calls, agent interactions, job runtime integration, or output review).

**The three PASS phases (01b, 02c, 06a) demonstrate that disciplined scope adherence produces clean, testable, verifiable code.** The PARTIAL phases consistently show that scope creep introduces verification complexity -- it becomes harder to confirm that existing behavior is unchanged when unrelated features are bundled in.

**Critical path risk:** The 03b/03c chain (resume API + UI) has a fundamental gap -- no Python resume endpoint exists, meaning the end-to-end ask-user flow cannot actually continue a conversation after the user answers. This is the highest-priority fix. The 07a schema deviations (FK cardinality, missing columns) will also require follow-up migrations if not corrected before dependent phases build on the schema.
