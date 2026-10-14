# DS-PC-01 · Singleton Chat, resumable Run and controlled retention

Status: **TRACK A CONTRACT CANDIDATE / NOT API OR DATABASE READY** (2026-10-14).
Authority: [approved PC-D4–D9 and CD-D1–D4](../../product/domain/person-chat.md), [acceptance PC3-01–10/19, CD3-05/07](../../product/acceptance.md), [Track A](./2026-10-13-v2-domain-schema.md) and [handoff register](./v2-handoff-state.json).
This document is a **handoff/implementation map**, not another source of business decisions or a claim that these types, routes or migrations exist.

## 1. Repository-grounded current → target ownership

| Concern                       | Current implementation (verified paths, not future API)                                                                                                                                                                                                        | DS-PC-01 target / blocking delta                                                                                                                                                                                                             |
| ----------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Conversation                  | `apps/api/internal/model/conversation.go` includes title, pin, archived/deleted, active_run_id; `repository/conversation_repository.go` has Create/List/SoftDelete/GetLastEmptyConversation                                                                    | One durable owner-scoped Conversation per user, lazy creation with a **database uniqueness guarantee**, no new/list/rename/pin/archive/share user flows. Inventory and migrate all old identities before retirement.                         |
| Messages                      | `model/message.go` has `(conversation_id, seq)` uniqueness and optional `run_id`; `repository/message_repository.go` still lists messages                                                                                                                      | Owner-bound monotonically allocated sequence; recent-window paging preserving original seq and message-part/turn boundaries. Never renumber after prune.                                                                                     |
| Runs                          | `model/run.go` has `running/waiting_user/completed/failed/cancelled`; migration `000001_vnext_baseline.up.sql` has `idx_runs_one_active_per_conversation` on running/waiting_user                                                                              | Keep existing active-run uniqueness and binding to the singleton conversation; preserve a stable run, lease and original stream across Ask User interruption.                                                                                |
| Ask User                      | `000006_interaction_answer_sources`, `2026-10-13-ask-user-source-contract.md`, existing consultation start/resume APIs and `AgentInteraction`                                                                                                                  | Reuse original Run and toolCallId/interactionId. No new Run to answer a pending question; revalidate stale/foreign/cancelled answers. Already bounded implementation, not singleton acceptance.                                              |
| Evidence                      | `statement_evidence` (from 000003/000006) stores minimum original user excerpt and stable Go-bound source IDs                                                                                                                                                  | Every persisted long-lived health claim must have a valid versioned source owned by its domain before deletion of the original Chat Message. A summary is not health authority.                                                              |
| Context                       | `repository/message_context_repository.go` searches bounded old text; Conversation has mutable summary                                                                                                                                                         | Use a rolling token/context window plus typed domain read models. Do not reinterpret old global transcript search as a governed retention/pruning system.                                                                                    |
| Sharing                       | OpenAPI currently includes list/delete/title/pin/share and `GET /api/v1/shared-conversations/{token}`; `conversation_shares` exists in baseline                                                                                                                | Retire public health-share tokens/endpoints only after explicit migration, revocation, source-copy and negative authorization tests; hiding UI alone is insufficient.                                                                        |
| Physical erasure              | No three-cursor model or all-controlled-copy cleanup receipt established in the audited target files                                                                                                                                                           | Source transfer, checkpoint/cache/index cleanup and verification precede advancing physical prune cursor. Explicit privacy erasure follows separate, broader PC-D7 protocol.                                                                 |
| Receipt and projection copies | `000006_interaction_answer_sources.up.sql` creates `interaction_resumes` with `user_message_id` and `assistant_message_id` references using `ON DELETE CASCADE`; baseline also stores `thread_projection_messages` and `conversation_shares.snapshot_messages` | Pruning Messages currently risks cascading accepted-answer receipts; retain idempotency/recovery identity after safe pruning. Inventory and verify projection/share snapshots separately, not as if raw Message deletion removes all copies. |

**Existing public transport** (do not label target singleton routes deployed): `packages/contracts/openapi/bodysense.v1.openapi.yaml` contains `StartConsultationRunRequest` with nullable conversationId, clientMessageId and requestId; `ResumeConsultationInteractionRequest` with requestId and answer; legacy `/api/v1/conversations*` and public share route. The precise V2 URL/method replacement is **reserved for canonical OpenAPI review by Track C**, not invented here.

## 2. Proposed named query/read-model contracts (typed target shapes, not generated DTO)

```ts
type CurrentChatView = {
  conversationId: string; // exactly one authenticated user owner; stable after first creation
  window: {
    messages: ChatMessageView[];
    hasOlder: boolean;
    olderBeforeSeq?: number;
  };
  currentRun?: {
    runId: string;
    status: "running" | "waiting_user" | "completed" | "failed" | "cancelled";
    streamId?: string;
    recovery: "live" | "reconnecting" | "recovered" | "unavailable";
  };
  pendingInteraction?: {
    interactionId: string;
    runId: string;
    toolCallId: string;
    questionVersion: string;
    question: unknown;
    responseState: "pending" | "retrying" | "answered";
  };
  composer: "ordinary" | "ask_user" | "busy" | "read_only";
  visibility: "ready" | "loading" | "unavailable" | "stale";
};
type ChatMessageView = {
  id: string;
  seq: number;
  turnId: string;
  role: "user" | "assistant" | "tool";
  renderParts: unknown[];
  delivery: "pending" | "committed" | "failed";
  // Source transcript seq and domain receipt are distinct.
};
type ChatRetentionInternal = {
  conversationId: string;
  processedThroughSeq: number;
  contextStartSeq: number;
  prunedThroughSeq: number;
  updatedAt: string;
};
```

`CurrentChatView` is a **UI/query projection**, not the persistence row. No revision or retention cursor is rendered in normal Chat UX. Required query set: (a) get-or-create singleton current Chat; (b) page most recent committed full interactions / older page by stable sequence; (c) retrieve active Run and exact pending interaction after reconnect; (d) server-internal retention progress/proof only. Distinguish never-created/empty from unavailable; network/recovery from persisted; tool answered from domain committed. All reads must use server-authenticated user ownership, never trust UI-supplied userId. Missing/foreign identifiers are masked without leaking health context.

## 3. Target commands, preconditions and idempotent results

| Command (conceptual, **not endpoint**) | Required preconditions / identity                                                                                                                                                                                                   | Success result / negative outcomes                                                                                                                         |
| -------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GetOrCreateMyConversation`            | authenticated user; single-user unique key, transactional create-or-read                                                                                                                                                            | exactly one stable conversationId under concurrent devices; never return another owner's ID                                                                |
| `SendMessage`                          | singleton conversation; exact `clientMessageId` and `requestId`; no active Run/pending interaction; user-authored validated parts                                                                                                   | persisted user Message + new Run + stable seq/turn/stream; identical retry returns same identity; busy/conflict returns existing recoverable active state  |
| `AnswerInteraction`                    | exact original `runId/interactionId/toolCallId`, content/question version, original accepted option, actor ownership and still-waiting run                                                                                          | durable answer pair + accepted-answer receipt, resume **original** Run; same answer replay exactly once; changed/stale/cancelled/foreign attempts rejected |
| `CancelCurrentRun`                     | exact active run, authenticated owner, stale-state check                                                                                                                                                                            | one cancellation result; close/hide/route navigation alone has no effect                                                                                   |
| `AdvanceProcessedCursor` (server only) | complete contiguous interaction boundary, per-message durable extraction outcome; necessary source evidence already committed and verifiable, or explicit no-long-term-value disposition                                            | monotone `processedThroughSeq`; no value fabricated by inference or chat summary                                                                           |
| `AdvanceContextWindow` (server only)   | token-budget and run-recovery constraints, stable unpruned full-turn boundary                                                                                                                                                       | monotone `contextStartSeq`; no deletion or domain mutation implied                                                                                         |
| `PruneControlledCopies` (server only)  | no running/waiting Run, unfinished proposal, tool/checkpoint dependency or active source reference crosses boundary; relevant durable evidence transferred; all controlled Chat copies cleared or references rewritten and verified | monotone `prunedThroughSeq` only **after** verified cleanup; otherwise retryable incomplete result with blocked re-use, not success                        |

No global atomic transaction is claimed across Go DB, model runtime checkpoint, cache, search and object storage. Implement a recoverable staged cleanup job with durable per-surface proof before the final cursor CAS. This is not the same user operation as “delete file and health data.”

## 4. Cursor invariant and a runnable specification-only oracle

Let `P=processedThroughSeq`, `C=contextStartSeq` (first retained raw model-context seq), `R=prunedThroughSeq`; empty chat begins `P=0,C=1,R=0`. Every committed transition must preserve **`0 <= R <= P`**, **`R < C`**, monotonic `P,C,R`, and original message `seq`. All advances must name a **contiguous completed-interaction boundary**, not an arbitrary message ID. `C` must not exceed the next allocatable message sequence. A pending Ask User, unclosed tool pair, proposal, checkpoint or live Run places a hard protection fence; replay/reconnect may move that fence only after durable resolution.

A **synthetic, specification-only oracle** now exists in `scripts/plan/v2-chat-retention-model.mjs` with two test suites (`v2-chat-retention-model.test.mjs`, `v2-chat-retention-adversarial.test.mjs`). Its previous two fail-open cases were repaired: unknown dependency flags now reject pruning; distinct completed turns require distinct processing dispositions. Validation on 2026-10-13 UTC: **13/13** oracle tests and the V2 Active Plan structural validator passed. All proof flags, boundaries, receipts and inventories are test fixtures; real PostgreSQL/runtime ownership, authorization and complete-copy cleanup remain unverified.

Recommended atomic proof order: snapshot owner+conversation+revision → inventory live dependencies and per-surface copies → persist accepted minimal source evidence → mark contiguous processing → compute complete-turn eligible boundary → block writes/references into candidate deletion scope → execute and verify Message/Checkpoint/cache/index/snapshot cleanup → CAS committed prune cursor. On ambiguous timeout inspect actual receipts/surfaces before retrying. Controlled copies must not be made unavailable to ongoing Run recovery.

## 5. Minimum executable negative acceptance matrix

| Case            | Expected observable result                                                                                                                              | Owner / evidence needed                                            |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| PC3-01 / PC3-02 | Two concurrent first-chat sends create exactly one Conversation and at most one active Run; distinct requests cannot overwrite; retries retain same IDs | Track C: PG18 unique partial/check constraint, `-race` + real HTTP |
| PC3-03 / PC3-04 | Closing/reload/new device restores original Ask User; ordinary send denied; exact identical reply replay does not create a second Run                   | Existing bounded 000006 tests plus new singleton browser E2E       |
| PC3-05          | Old fingerprint/foreign user/stale tool answer cannot change domain revision                                                                            | Go command/answer negative tests                                   |
| PC3-06 / CD3-05 | After message raw prune, original minimal SourceEvidence remains versioned and can support lawful correction; Message ID alone is not a valid source    | PostgreSQL source-transfer/delete/replay                           |
| PC3-07 / PC3-08 | Prune cannot pass pending tool, incomplete turn, active checkpoint or source not yet persisted; it cannot advance after partial copy-cleanup failure    | staged cleanup integration with inject-failure fixtures            |
| PC3-09          | Trivial processed message may be marked no-long-term-value; no gratuitous BodyFact or endless summary                                                   | processing disposition test                                        |
| PC3-10          | Correction after safe prune acts on persisted BodyState + source, does not fabricate Episode chronology                                                 | BodyState correction end-to-end                                    |
| PC3-19          | Legacy public share URL/token is revoked and cannot expose transcript, including any old snapshot/cache                                                 | shared-token invalidation + negative public HTTP E2E               |
| CD3-07          | Erased source cannot be replayed into new projection from old checkpoint/index/outbox                                                                   | all-copy deletion/replay with actual storage                       |

## 6. Open delivery dependencies (must not mark `ready`)

1. **DS-PC-01 canonical transport:** design review of exact singleton query/pagination/resume OpenAPI; generation into Go/TS/Zod; preservation of backward-compatible internal Run IDs where necessary.
2. **DS-PC-01 PostgreSQL:** migration with `UNIQUE(user_id)` singleton ownership after old multi-conversation merge/reconciliation, a three-cursor monotone receipt table and source/checkpoint fences; isolated PG18 up/down/replay, concurrency and rollback. Keep `000001` and migration hashes immutable.
3. **ENG-PC-01:** new Go query/commands and clear separation of one Conversation from multiple Runs/turns, preserve established 000006 Ask User behavior, Chat client removal of legacy session navigation.
4. **SF-70:** enumerate and prove cleanup of DB Messages, runtime checkpoints, retrieval indexes, caches and controlled transcript copies; no fixed arbitrary retention TTL; do not claim all data erased on cursor progress.
5. **IR-PC-01:** real cross-device/synthetic-user E2E, legacy share revoke, exact-head CI and accepted Penpot 390px behavior. Only after that may the six-gate register be updated with specific evidence.

**Track A handoff:** This file now includes a field/command/negative-path map and a 13/13-passing specification-only retention oracle. `conversation-person.view_model` and `database` remain **in_progress**. Next: Track C must implement and test the canonical OpenAPI + singleton PostgreSQL migration; green synthetic cursor tests are not a ready gate.

## 7. Ordered implementation handoff (Track C; NOT deployed)

1. **Reconcile legacy owners first.** Inventory every user with multiple or soft-deleted Conversations, overlapping active Runs, pending Interactions, per-conversation sequence collisions, source associations and public share snapshots. Produce a verifiable old-to-canonical ID/turn/sequence mapping before switching ownership; do not merely update all foreign keys or erase unreviewed histories.
2. **Then enforce singleton persistence.** Add a new numbered PostgreSQL migration (leave 000001 and historical checksums unchanged), with a database-backed per-user uniqueness guarantee, transactional get-or-create and explicit collision/rollback rules. Keep the existing `idx_runs_one_active_per_conversation`; test concurrent first-chat requests and conflicting previously active Runs.
3. **Implement durable retention proofs.** Persist owner/conversation-bound cursors and optimistic revision; derive complete interaction boundaries and dependency fences from authoritative Go/runtime records, never from client flags. Store explicit per-turn processing dispositions and per-surface cleanup receipts. Before deleting Messages, change the `interaction_resumes` cascade path so accepted-answer replay still resolves to the original Run.
4. **Freeze and generate the transport.** Review the singleton current-chat read, complete-turn pagination, original Run/pending Ask User recovery and owner-scoped send/answer commands against the existing `startConsultationRun`/`resumeConsultationInteraction` contracts. Do not invent new production URLs in Track A; Track C owns canonical OpenAPI, Go/TS/Zod generation and compatibility retirement.
5. **Prove the boundary.** Native PostgreSQL 18 full-up/down/replay; concurrent owner uniqueness, original source correction after pruning, partial cleanup retry, no orphan ToolCall or duplicated answer receipt, invalid public share token, Go race/HTTP and cross-device browser E2E. Successful synthetic oracle tests alone cannot raise any database/engineering/integration gate.
