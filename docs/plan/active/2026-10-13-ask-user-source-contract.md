# V2 Ask User resume and source handoff

Status: IMPLEMENTED / LOCAL VERIFICATION; exact-head CI pending. Bounded handoff under the [V2 primary program](./2026-10-13-v2-parallel-master.md), DS-PC-01 / ENG-BS-01 / IR-PC-01. Product authority remains [Person/Chat](../../product/domain/person-chat.md) and [BodyState](../../product/domain/bodystate.md).

## Required behavior

V3 Ask User resumes its original Go Run, not a new run. The accepted answer, its user/assistant message pair, execution lease, metadata-only resume identity, and answered/resumed milestones commit atomically. Duplicate same answers return the original resume identity; changed answers, reused request IDs, foreign owners, expired/cancelled interactions and non-waiting runs cannot create a second execution. Ordinary sends remain blocked while an interaction is pending. Closing Chat does not answer or cancel it.

The database resume record binds the original run, interaction, source user message and response message. Python cannot submit this source identity. A resumed `record_body_state` proposal is bound server-side to that exact accepted answer; durable statement evidence retains the minimal user excerpt plus question context and original interaction/tool identities before raw-message pruning. It cannot use another turn, an unanswered option, an AI-authored statement or the initial ambiguous statement as if the user had authored the answer. No legacy BodyStateFact dual-write.

The first extraction scope is complete literal self-statements inside accepted answers. Short contextual answers and measurements must not be reconstructed as medical facts without a separate typed, source-preserving interpretation contract. Answer acceptance and domain persistence are distinct: a failed extraction or commit never reports a successful health save. Direct Fact amendments retain their existing exact-user-command boundary; this slice does not authorize automatic Episode or Diagnosis edits.

## Implementation sequence

1. Native migration 000006 for one resume receipt per interaction and structured-answer source references, privacy inventory and loss-refusing downgrade.
2. Go question-bound answer validation and transaction coordinator: owner/run/interaction CAS, original run lease renewal, message sequences and durable milestones; exact retry without model execution.
3. V3 runtime integration and sequence continuation; persist native LangGraph interrupt identity and validate it before resumption. Leave explicit historical v1/v2 behavior separate.
4. Actual Python answer extraction and Go-owned source validation using the existing proposal/receipt boundary. Person invalidation stays receipt-driven.
5. PostgreSQL concurrency/rollback/source/pruning tests, generated/Go/Python/UI tests, real-server browser Ask User answer/reload/duplicate/foreign/cancel paths, then exact-SHA CI and checkpoint.

## Acceptance and limits

PC3-02..05/19 and BS-03/29, INV-29/34: one active execution, exact answer identity, no premature saved feedback, no cross-user source reuse. Do not claim singleton Conversation migration, complete retention, short-answer interpretation, Episode migration, final Penpot visual parity or production release from this bounded slice.

Checks and evidence are appended only after execution. Existing checks on 167ebfd725fd73f2f682e92448b3b2ad706c6684 (12/12) establish the starting baseline, not this batch's acceptance.

## Implemented owners and authority

| Surface                               | Concrete owner                                                                                                                          | Result                                                                                                                                                                                                                                                                                                   |
| ------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Accepted answer and same-Run identity | `apps/api/internal/model/interaction_resume.go`, `repository/interaction_resume_repository.go`, `service/interaction_resume_service.go` | Owner/conversation/interaction/run locks; one metadata receipt, one message pair and three ordered milestones commit in the same transaction. Same content replays; changed content, a rebound ordinary-turn request, expiry or cancellation cannot execute again.                                       |
| Native continuation                   | `apps/api/internal/consultation/interaction_resume.go`, `apps/ai-service/src/runtime/consultation_thread.py`                            | Stored native LangGraph interrupt identity and original Run must match the checkpoint. Stream sequence continues; duplicate replay begins at the relevant answer rather than an earlier `stream.done`. Cancellation is registered before dispatch.                                                       |
| Persisted answer facts                | `apps/api/internal/repository/body_fact_answer_repository.go`, `apps/ai-service/src/runtime/body_fact_capture.py`                       | Literal full self-statement proposal, exact Go-accepted answer/message/source fingerprints, actual committed receipt; no old BodyStateFact dual-write. Ordinary source paths reject answer messages without resume binding.                                                                              |
| Preservation and erasure inventory    | Migration `000006_interaction_answer_sources`, `StatementEvidence`, privacy repository                                                  | Minimal excerpt plus question context/identities survive raw message or interaction pruning. Resume receipts are in account-erasure inventory; new writes from pruned sources fail. Downgrade cannot silently drop accepted history. This is not whole-system retention or selective erasure acceptance. |
| Pending composer                      | Shared `AskUserCard`, `AssistantChatPanel`, `StreamingAssistantTurn`, runtime hook/reducer                                              | One pending card replaces ordinary input. No optimistic answered mutation. Failed HTTP submission retains the draft and complete retry identity. New same-Run assistant segments do not reuse earlier text/tool output.                                                                                  |
| Bounded visual integration            | `body-canvas.css`, `ask-user-body-facts.spec.ts`                                                                                        | Existing canvas colors, light/dark card contrast, accessible question label and distinct radio groups, keyboard focus, scroll and 390px bottom-sheet geometry. The shared title uses canvas ink in dark mode. No new design system or pixel-parity claim.                                                |

Go validates the exact stored question: unadvertised custom choices, unknown/duplicate options, inconsistent text/selected/field summaries, invalid dates, non-finite numbers, missing required fields and unrecognized answer properties are rejected before accepting the answer. The UI does not truncate the question to three fields or invent custom-input permission.

### Safety and revision order

An accepted answer contains new user input; the existing red-flag detector runs again before answer extraction/advice. A negative screen does not clear an existing independent restriction. A positive screen is submitted through the existing Go safety authority first. Fact capture may follow only a **real, contiguous, same-owner safety revision receipt from this execution**, not an arbitrary current revision read. A different device's intervening revision still causes conflict. This repairs same-stream ordering without weakening ordinary direct-entry/amendment concurrency or claiming complete scoped Safety V2.

## Executed evidence

Evidence root: `.runtime/schema-first-v2-evidence/`. All model inputs here are synthetic, not patient records. No model credentials are written into the evidence.

- `ask-postgres-final.log`: **34 top-level real PostgreSQL/race cases** (20 BodyFact, 4 thread projection, 10 new interaction-resume cases, with additional parameterized negatives). Native migrations up/down/up pass at 6 / 5 / 6. New cases cover 12-way identical races, competing answers, final-event rollback, tenant/expired/cancelled/rebound/forged answers, tampered sources, raw-source pruning, unsafe downgrade and own-safety versus external-revision ordering.
- `ask-python-all.log`, `ask-pyright.log`, `ask-ruff.log`: **705 tests passed**, Pyright 0 errors/warnings, Ruff passed. Ten new native LangGraph checkpoint/answer tests use actual StateGraph + InMemorySaver + native interrupts and the production resume adapter with a controlled TestModel; wrong Run/native IDs fail before model or checkpoint mutation.
- `ask-web-accepted.log`: **408 tests / 68 files**, lint, typecheck and production build passed. This includes ordinary records/amendment regressions and the new reducer/card constraints.
- `ask-user-e2e-final.log`: **14/14 real-server Chromium cases passed**, including four new Ask User paths. The final follow-up `ask-user-e2e-accepted.log` passed the same **14/14 in 3.4 minutes** after the dark-title assertion and pre-dispatch cancellation guard. Its temporary services/database were cleaned; `vertical-e2e-1232577/` contains detailed logs.
- `ask-go-accepted.log`: final full Go packages and vet passed after the cancellation-registration improvement. Pinned Staticcheck `v0.8.1` (`SA*,S1*,QF*`) also completed with exit 0 (`ask-staticcheck.log`).
- `ask-contracts-final.log`: public/internal lint, regeneration equality, breaking/mutation/conformance probes, 99-route Postman verification and all three language architecture boundaries passed. Existing canonical OpenAPI/Proto operations are reused; no alternate API generator.

The browser tests run actual React, Go, Python, Proto and PostgreSQL; controlled synthetic model output is not a live-LLM or medical-quality certification. Initial foreign-owner browser failure returned 500: the owner lookup now returns the deliberate non-disclosing 404 and the complete test was rerun. A screenshot found an inherited black title on the dark sheet; the shared heading color and mobile assertion were repaired rather than treating a successful click as visual acceptance.

## Remaining handoff

Next: DS-BS-02 / ENG-BS-02 for Episode identity, links, deterministic replay and full evidence/source projections; DS-PC-01 / ENG-PC-01 for singleton Conversation, complete recovery and controlled retention. Short answer interpretation, Agent-authorized corrections, final editable Penpot states and the other domain gates remain open. `PC3-02..05/19` denotes the targeted sub-behaviors above, not a blanket pass of the entire product acceptance matrix. Hourly automation remains off. No staging or production promotion/reset was performed by this slice.

### Immutable code checkpoint

Implementation commit: `6d4fd8100776d688d5f112d688cc8c9598590cb9`. The follow-up registry commit changes documentation only. Local evidence above was executed against this code content. Required GitHub checks must match the final published branch head; their accepted result is recorded in PR #247 after completion, not inferred from the earlier baseline.
