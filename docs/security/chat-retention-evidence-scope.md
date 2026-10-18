# Chat Retention: What a Pruned Conversation Keeps

Status: **production contract** (PC-RET-001, #307)
Owner: Go chat retention (`ChatRetentionRepository.PlanPrune` / `CommitPrune`)
Proof: `TestChatRetentionEvidenceScopeCoversEveryChatReferencePostgres`,
`TestChatRetentionPruneKeepsOnlyCitedEvidencePostgres`,
`TestChatRetentionErasedStatementReleasesItsChatSourcePostgres`,
`TestChatRetentionPruneRetainsReferencedEvidencePostgres`,
`TestChatRetentionAskUserExpiryTriggersSettlePostgres`

## 1. Rule

The single Conversation keeps a rolling raw-history window (`context_start_seq`
onward, sized by `CHAT_CONTEXT_TOKEN_BUDGET`). A prune pass removes raw chat
before `min(context_start_seq - 1, processed_through_seq)`, never past an
unsettled Run and never splitting a turn or Run.

Inside a pruned window a chat row survives **only when a durable record cites
it**. Everything else goes, Ask User answers included:

| Kept in place                                                                                                                                                                                                       | Because                                                                                                                                                     |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| User message named by `statement_evidence.source_message_id` of a statement that is not erased                                                                                                                      | It is the source of a BodyFact, Observation, Hypothesis, Safety resolution or Diagnosis request (all funnel through statement evidence)                     |
| `condition_analysis_runs.request_message_id`                                                                                                                                                                        | Diagnosis analysis request receipt                                                                                                                          |
| `condition_direct_edits.request_message_id`                                                                                                                                                                         | Diagnosis direct-edit request receipt                                                                                                                       |
| `safety_screens.source_message_id`                                                                                                                                                                                  | Safety intake screen                                                                                                                                        |
| `safety_resolutions.source_message_id`                                                                                                                                                                              | Safety clearance / reverification                                                                                                                           |
| Cited Ask User: `agent_interactions` row, its `interaction_resumes` receipt, and both receipt messages                                                                                                              | The interaction is named by `statement_evidence.source_interaction_id`, `safety_screens.source_interaction_id` or `safety_resolutions.check_interaction_id` |
| Run-scoped copies (`runtime_events`, `agent_tool_calls`, interactions) of a Run that keeps a cited message or is named by `statement_evidence.source_run_id` / `safety_screens.source_run_id` / a cited interaction | The evidence names that Run's identity                                                                                                                      |
| `runs` rows                                                                                                                                                                                                         | Never pruned; they hold no raw text and anchor ids that evidence and jobs name                                                                              |

### Erased statements cite nothing (AS-002)

An erased statement (`statement_evidence.availability = 'erased'`, set when its
statement erasure finalizes as `erased` or `erased_with_retention`) keeps its
identity and its `source_*` ids as a tombstone, but it **no longer protects chat**:
`protectedMessages`, `citedInteractionsSQL` and `referencedRuns` read only
unerased statement evidence (`citingStatementEvidence`). The ordinary prune then
deletes its source message, the Ask User interaction and resume receipt of an
erased structured answer, and its Run's copies, unless another durable record in
the table above still cites them. Statement erasure itself goes one step
further: its `source_message` target deletes the source message at once when no
other durable record cites it (privacy-erasure-retention §8), so the message does
not wait for the window to move.

The AS-002 statement erasure receipt (`statement_erasure_requests`) references
`statement_evidence` with `ON DELETE CASCADE`; it names the statement only, never a
chat row, so it neither protects nor prunes chat.

| Removed by the pass                                                                                                   |
| --------------------------------------------------------------------------------------------------------------------- |
| Every other message in the window, and its `thread_projection_messages` copy                                          |
| `runtime_events`, `agent_tool_calls`, `thread_projection_tool_calls` of pruned Runs                                   |
| Uncited `agent_interactions` of pruned Runs (raw question and answer JSON)                                            |
| Uncited `interaction_resumes` of pruned Runs, and any uncited receipt whose answer or reply message leaves the window |
| The LangGraph checkpoint copies of pruned Runs (ai-service cleanup receipt first)                                     |

Plans never cite chat rows directly: `training_plan_revision_body_refs` and
`training_plan_revision_conditions` cite BodyFacts, Observations, Episodes and
Diagnosis conditions, which cite statement evidence. A Plan, a BodyState
record and a Diagnosis therefore keep their user source through the same
funnel, and statement evidence itself is never pruned (it also freezes the
excerpt and, for structured answers, the question excerpt).

A BodyState suggestion that is still only a suggestion cites a message by id
and re-reads it under a share lock at commit time; a pruned source rejects
the commit (`ErrBodyStateSuggestionRejected`) instead of trusting a stale
excerpt. That is fail-closed, not a kept reference.

## 2. Why no cascade decides what is kept

`interaction_resumes` references `messages` (answer and reply) and
`agent_interactions` with **no** `ON DELETE CASCADE`. A raw delete of a cited
receipt's message or interaction now fails on
`interaction_resumes_user_message_id_fkey` /
`interaction_resumes_interaction_id_fkey` instead of silently dropping the
receipt a Safety reverification (`safety_resolutions`, deferred FK) or a
structured-answer fact depends on. The prune pass deletes uncited receipts
explicitly, before the messages and interactions they name, inside the same
commit that advances `pruned_through_seq`. Whole-conversation and account
erasure still remove them: the `conversations` and `runs` cascades delete the
receipt in the same statement as its parents.

A late duplicate resume request for a receipt that was pruned finds the
interaction `answered` without a receipt and gets a conflict; the old
authorization is never replayed.

## 3. Settling

Retention reconciles the window whenever a Run settles: every Run-ending path
in the consultation runtime, the lease reconciler for lost Runs, and the Ask
User lifecycle when an interaction closes without an answer (expiry sweep,
soft expiry on a late answer, cancellation). A pending Ask User keeps its Run
unsettled, so nothing at or after it is processed or pruned until it is
answered, cancelled or expires.

## 4. Keeping this proof true

`TestChatRetentionEvidenceScopeCoversEveryChatReferencePostgres` reads the
live catalog: every foreign key into `messages`, `runs`, `agent_interactions`,
`interaction_resumes` or `statement_evidence`, with its delete rule, and every
uuid column named `*message_id`, `*interaction_id` or `source_run_id`. Each one
must be classified in the test's `chatReferenceScope` map. A new reference (or
a changed delete rule) fails until it is classified there and in this
document, and until `protectedMessages` / `referencedRuns` cover it if it
cites chat.

## 5. Legacy public share (PC3-19)

The target product has no public health-conversation share (PC-D8). The share
service, `POST|DELETE /api/v1/conversations/{id}/share` and the public
`GET /api/v1/shared-conversations/{token}` read were removed with the single
Conversation (#321); the vNext baseline has no `conversation_shares` table, and
the vNext reset replaces the database volume, so no old token or snapshot
survives. Fail-closed regressions:
`TestRetiredMultiConversationAndShareRoutesAreGone` (routes are 404 before any
handler), `TestChatRetentionLegacyShareStateIsGonePostgres` (no share table or
token column), and the Chromium `legacy-share-retired.spec.ts` (old API paths
404 with or without a session; old share links land on sign-in or the user's
own workbench).
