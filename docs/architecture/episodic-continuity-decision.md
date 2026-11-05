# ADR: Provenance-Bound Episodic Continuity for the Single Conversation

> Status: Accepted — staged and gated. This ADR approves the design and the
> Stage 0–1 work below. It changes no runtime behavior, no retention window
> and no privacy rule by itself.
> Date: 2026-11-03
> Issue: #495 (this document is the decision deliverable; implementation
> slices become separate issues)
> Owners: product semantics — the Person/Chat domain contract
> ([docs/product/domain/person-chat.md](../product/domain/person-chat.md));
> privacy/retention — the Go durable domain/runtime boundary that owns
> [privacy-erasure-retention.md](../security/privacy-erasure-retention.md).

## Context

Current state, verified against code and contracts at `main` (106824d):

- **One Conversation, rolling raw history.** Each user has exactly one
  Conversation (PC-D4). Go owns three retention cursors over its message
  seqs — `ProcessedThroughSeq (P)`, `ContextStartSeq (C)`,
  `PrunedThroughSeq (R)` with `0 <= R < C <= P + 1` — and C advances by a
  token budget (default 16K estimated tokens), cut only at complete
  interaction boundaries (`apps/api/internal/model/chat_retention.go`,
  PC-RET-001 / #307).
- **Pruning keeps only cited rows.** A prune pass deletes every raw message
  at or before `min(C - 1, P)` except rows a durable record cites, which in
  practice funnel through `statement_evidence`
  ([chat-retention-evidence-scope.md](../security/chat-retention-evidence-scope.md)).
  Statement evidence is never pruned and holds the frozen verbatim excerpt
  (PC-D6 scheme B). Everything else about a pruned episode — what the Agent
  advised, what the user tried, conversational preferences that never became
  a BodyState fact — is unrecoverable by design.
- **Historical recall today is window-bound.** The only episodic recall is
  `MessageContextRepository.SearchRelevant`: term (ILIKE) search over raw
  messages with `seq >= context_start_seq`, excluding the most recent 24,
  at most 8 messages, excerpts capped at 600 runes, explicitly labelled
  "context, never durable health truth" with BodyState taking precedence
  (`apps/api/internal/repository/message_context_repository.go`,
  `apps/api/internal/service/context_retrieval_service.go`). Once C passes a
  message, this path can never return it again.
- **Product contract already draws the line.** PC-D6: long-term information
  lives in BodyState and its domains; natural-language summaries must not be
  relied on as medical authority; context is assembled from domain facts and
  short-term WorkingContext "rather than accumulating permanently unlimited
  summaries". No WorkingContext or summary store exists in code today.
- **Erasure is inventoried, and the inventory is enforced.** AS-002
  per-asset and per-statement erasure enumerate every controlled copy; the
  schema-coverage tests (`TestUserAssetErasureInventoryCoversSchemaPostgres`,
  `TestChatRetentionEvidenceScopeCoversEveryChatReferencePostgres`) fail when
  a new table references assets, statements or chat rows without being
  classified. Any new store of user-derived text becomes a controlled copy
  or those proofs break. Full account erasure deletes every attributable
  family ([privacy-erasure-retention.md](../security/privacy-erasure-retention.md) §2).

The gap (#495): the rolling window deliberately forgets raw episodes, while
the product promise is an Agent that remembers the user's body over time.
Structured facts survive in BodyState; the *episode* — the narrative thread
connecting them — does not. The question is whether to add an episodic layer,
and under what authority, provenance and erasure rules. Retention/privacy
policy must not change silently, so this ADR is the decision point, not an
implementation of changed retention.

## Decision

**Adopt a provenance-bound Episodic Note layer, Go-owned and
non-authoritative, rolled out in stages behind hard gates. Reject a new
vector database and any always-on permanent summary store.**

1. **Clinical truth stays exclusively in Go-owned domains.** BodyState,
   Diagnosis and SafetyState remain the only clinical source of truth.
   An Episodic Note is conversational context of the same epistemic rank as
   today's `ConsultationHistoricalMessage`: quotable as "you mentioned /
   we discussed", never an input to Diagnosis decision authority, Safety
   screening, or TrainingPlan acceptance, and never a source from which a
   BodyFact, Observation or ConditionEntry may be derived without the
   normal user-statement path. Where a note and BodyState disagree,
   BodyState wins and the note is treated as stale narrative.
2. **What an Episodic Note is.** At the moment a segment is about to leave
   the raw context window (C advancing past it), Go may persist one bounded
   note for that segment: `{id, userId, conversationId, fromSeq, toSeq,
   sourceMessageIds[], generatorVersion, retentionPolicyVersion, text,
   createdAt, status}`. Content already committed as BodyState facts or
   statement evidence is referenced by id only, never copied into the note,
   so notes cannot fork clinical content. Generation is best-effort: Python
   may draft the text (typed output, as elsewhere), Go validates owner,
   bounds and provenance completeness and persists; if generation fails the
   prune proceeds without a note — memory never blocks retention.
3. **Bounded and rolling, not permanent.** Notes inherit the rolling
   philosophy of PC-D6/D9: per user, notes are capped in count and total
   estimated tokens (defaults set by the privacy owner in the Stage 1
   policy version; the cap is a policy parameter, not a product rule fixed
   here) and the oldest unpinned notes roll off. There is no unlimited
   accumulation.
4. **Provenance is structural, and notes never protect chat rows.** The
   note's message ids are provenance metadata, classified in the
   chat-retention evidence scope as a *non-protecting* reference: unlike
   `statement_evidence`, a note does not keep its source messages alive.
   After pruning, ids may point at deleted rows by design; the surviving
   provenance is the seq range, the generation policy version and the
   note's own audit trail. A note with unverifiable provenance is never
   served.
5. **Erase-on-source-deletion, no ghost memories.** Notes are controlled
   copies and enter the AS-002 inventories before any serving slice ships:
   - Per-statement erasure (inventory target `episodic_notes`): any note
     whose seq range covers an erased statement's source message is
     regenerated excluding the erased material, or deleted when the note
     materially derives from it. An erasure receipt must not reach
     `erased` while this target is unverified — the same trigger discipline
     AS-002 already uses. Withdrawal (not erasure) does not rewrite notes;
     withdrawn content simply carries no clinical weight (rule 1).
   - Per-asset erasure: notes naming an erased asset are handled by the
     same target.
   - Full account erasure deletes all notes (new row in the retention
     matrix of privacy-erasure-retention.md).
   - A user deleting a single note deletes the note only; it never deletes
     or alters BodyState facts, and note deletion is not presented as
     health-data erasure.
6. **User-visible and user-deletable.** Before notes may be served to the
   model (Stage 2), the user must be able to list them, see the date/seq
   range and provenance of each, and delete individual notes. UI copy must
   distinguish "conversation memory" from BodyState facts and from erasure,
   consistent with the existing rule that window truncation is never
   presented as privacy erasure (docs/product/interface.md).
7. **Background work joins the same Conversation.** Summaries of
   background jobs are written as ordinary assistant messages (or notes
   citing the job id) in the user's single Conversation, flowing through
   the same cursors, provenance and erasure. No new public chat surface,
   no second conversation (PC-D4), no share (PC-D8).
8. **No new vector database, no embeddings by default.** Notes live in
   PostgreSQL beside the data they summarize. Retrieval reuses the
   existing bounded pattern — term search over note text, same limits
   style as `SearchRelevant` — served through the Go context path only.
   No nanoMuse code is reused (GPL); the design above is derived from
   BodySense's own contracts. Embeddings may be revisited only if the
   Stage 2 recall benchmark (below) fails on term search alone, and would
   require a new ADR because embeddings are derived copies that complicate
   erasure verification.

## Alternatives rejected

| Alternative | Why rejected |
| --- | --- |
| Status quo only (BodyState + raw window) | The continuity hole is structural: every coaching episode that never became a fact is unrecoverable after pruning, and the hole grows with exactly the rolling retention PC-D9 mandates. The firewall in rule 1 makes the risk manageable, so a flat rejection trades a real product gap for avoidable safety. |
| Dedicated vector DB / external memory service (nanoMuse-style) | Violates the acceptance default (no new vector database), adds a second memory authority outside Go, cannot reuse GPL source, and embeddings are derived copies whose deletion cannot be verified to the AS-002 standard. |
| Enlarge the raw window / raise the token budget instead | Only delays forgetting; contradicts PC-D6/D9 rolling design and the context budget's cost basis; does nothing after pruning and worsens the privacy posture of raw retention. |
| Summaries as BodyState facts | Pollutes clinical truth with unconfirmed narrative; violates the domain authority table (Conversation must not hold a second BodyState) and the PC-D6 rule against summaries as medical authority. |
| Summaries in the Python LangGraph checkpoint / a permanent WorkingContext | Python is runtime, not business truth (ADR 0002); checkpoints are pruned controlled copies, are not user-viewable or user-deletable, and cannot carry the erasure inventory. WorkingContext stays short-term and task-scoped as PC-D6 defines it. |

## Consequences

- Positive: longitudinal coaching continuity survives pruning without
  touching clinical authority, retention cursors or the single-Conversation
  model; the feature is killable at every stage (flag off = today's exact
  behavior); provenance and erasure reuse proven AS-002 machinery instead
  of inventing a parallel one.
- Negative: one more controlled-copy family to inventory, test and keep
  honest forever; notes add a "stale narrative" hazard that the retrieval
  layer must label on every use; generation costs tokens on every window
  advance.
- Neutral/hard constraint: Stage 2 (serving) may not start until the
  product-rule follow-ups below are decided by their owners. This ADR does
  not amend docs/product.

## Migration, contract and guard plan

- **Schema (Stage 1):** one Go migration adds `episodic_notes` (no foreign
  key that protects messages from pruning; provenance ids stored as data,
  classified non-protecting in the chat-retention scope test and in
  chat-retention-evidence-scope.md *in the same change* — the coverage
  tests failing first is the expected, correct signal).
- **Generation contract:** a typed runtime request/response in the existing
  Go↔Python contract style: Go names the exact seq range and message set;
  Python returns bounded draft text only; Go rejects drafts that are
  over-length, reference ids outside the range, or restate content carried
  by cited BodyState/statement-evidence refs.
- **Policy versioning:** `retentionPolicyVersion` on each note follows the
  AS-002 inventory-version precedent (`asset-erasure-inventory-v1`,
  `statement-erasure-inventory-v3` after the #514 S3-A statement-source slice).
  Earlier revisions are historical. A policy-version change triggers
  re-evaluation (regenerate or roll off) of notes written under older
  versions; mixed-version serving is not allowed.
- **Erasure wiring (before serving):** `episodic_notes` targets added to
  the statement- and asset-erasure inventories and to the account-erasure
  family list, with PG integration tests mirroring
  `TestChatRetentionErasedStatementReleasesItsChatSourcePostgres`.
- **Serving guard:** notes reach the model only through the Go context
  path, are labelled non-authoritative in the prompt contract exactly like
  historical excerpts, and are excluded by construction — not by prompt
  wording — from Diagnosis, Safety and TrainingPlan decision inputs.
- **Kill switch:** a single server-side flag disables generation and
  serving independently; disabling serving restores today's behavior with
  no data migration.

## Benchmarks and gates

Stage 1 (shadow generation; notes are written but never served) must
report, on synthetic longitudinal conversations (no production user data):

| Gate | Threshold | Type |
| --- | --- | --- |
| Provenance completeness | 100% of generated notes carry verified, in-range, owner-checked source ids | hard |
| Traceability audit | ≥ 98% of sampled note claims (n ≥ 100 notes) traceable to their source messages; any untraceable *clinical* claim fails the gate outright | hard |
| Clinical-authority leakage | 0 cases, on the fixed diagnosis/safety regression set, where shadow notes present in context would change a Diagnosis or Safety outcome vs the BodyState-only baseline | hard |

Stage 2 (serving in Chat only) additionally requires:

| Gate | Threshold | Type |
| --- | --- | --- |
| Ghost-memory test | 0 notes or note fragments derived from a statement survive that statement's completed AS-002 erasure (PG integration suite) | hard |
| Episodic recall utility | ≥ 80% correct answers on a scripted multi-week eval set (≥ 50 conversations) for questions about prior *non-clinical* episodes, vs the raw-window-only baseline, with no regression on current-fact questions | rollout |
| Bounds | per-user note storage stays within the policy cap (count and estimated tokens) under a simulated 12-month conversation; p95 added latency to a prune pass ≤ 2 s | hard |
| Erasure timeliness | note regeneration/deletion completes inside the originating erasure job's lifecycle; no receipt reaches a terminal state with the `episodic_notes` target outstanding | hard |

Missing a hard gate stops the rollout at the current stage; missing the
recall-utility gate after two policy revisions retires the feature
(notes generation off, stored notes rolled off under the retention policy)
rather than lowering the bar. Revisit this ADR if the token budget, the
cursor model, or the AS-002 inventory discipline changes materially.

## Implementation slices (each becomes its own issue)

1. **S1 — Schema and classification.** `episodic_notes` migration,
   provenance model, retention-policy version field; classify the
   non-protecting reference in the chat-retention scope proof and docs.
   No generation, no serving.
2. **S2 — Shadow generation.** Note generation at context-start advance
   (Go-orchestrated, Python draft, Go validation/persistence) behind the
   kill switch, plus the Stage 1 benchmark harness and report.
3. **S3 — Erasure integration.** `episodic_notes` targets in the
   statement/asset erasure inventories, the account-erasure family, and
   the ghost-memory PG tests.
4. **S4 — Chat serving.** Extend the Go context-retrieval path to notes
   (term search, bounded, labelled non-authoritative), feature-flagged;
   decision-input exclusion tests for Diagnosis/Safety/TrainingPlan.
5. **S5 — Background summaries.** Route background job result summaries
   into the same Conversation as assistant messages / job-citing notes.
6. **S6 — Person UI.** List, inspect-provenance and delete episodic
   notes, with copy separating memory from clinical facts and from
   erasure; E2E coverage. Required before S4's flag is enabled for users
   (rule 6), though it may be built in parallel.

## Proposed product-rule follow-ups (not decided here)

These need their owners' decisions before Stage 2; this ADR records them
as proposals only and edits no product rule:

1. Person/Chat contract: a new PC decision defining EpisodicNote as a
   non-authoritative Conversation object — its retention cap defaults,
   user view/delete rights, and its relation to PC-D6's WorkingContext.
2. Privacy: add the `episodic_notes` family to the retention matrix and
   confirm the note roll-off cap with the privacy owner.
3. Interface: memory-view copy and empty/error states for the Person
   surface (S6), including the rule that deleting a note is not health
   data erasure.
