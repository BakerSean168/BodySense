-- PC-001 / PC-D4-D6: conflict-only, aggregate-only read-only migration planning.
-- psql -X -v ON_ERROR_STOP=1 "$BODYSENSE_READONLY_DATABASE_URL" \
--   -f scripts/plan/sql/pc-001-reconciliation-conflicts.sql
-- This is NOT a remap, canonical-ID selection, cleanup, or migration approval.
-- No owner, Conversation, Run, Message, share token, excerpt or health data
-- is projected. All history (including deleted Conversations) is inspected.
-- A missing source Message/Run can be legitimate after verified pruning:
-- unresolved_provenance_to_review is NOT synonymous with a corrupt record.
-- A zero-count result cannot establish safe erasure, external checkpointer
-- cleanup, JSON/source-key reconciliation, or an exclusive-writer lease.
\set ON_ERROR_STOP on
\set ECHO none
\set VERBOSITY terse
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL statement_timeout = '30s';
SET LOCAL lock_timeout = '5s';
SET LOCAL row_security = off;
SET LOCAL search_path = pg_catalog;

-- Fail closed if the approved schema slice has not been applied in PG18.
SELECT 1 / CASE WHEN
    current_setting('server_version_num')::integer BETWEEN 180000 AND 189999
    AND to_regclass('public.conversations') IS NOT NULL
    AND to_regclass('public.interaction_resumes') IS NOT NULL
    AND to_regclass('public.statement_evidence') IS NOT NULL
    AND EXISTS (
      SELECT 1 FROM pg_attribute
      WHERE attrelid = 'public.statement_evidence'::regclass
        AND attname = 'source_interaction_id' AND attnum > 0 AND NOT attisdropped
    )
  THEN 1 ELSE 0 END AS pg18_pc001_schema_guard;

WITH
message_seq_collision_owners AS (
  SELECT DISTINCT user_id FROM (
    SELECT c.user_id, m.seq
    FROM public.messages AS m
    JOIN public.conversations AS c ON c.id = m.conversation_id
    GROUP BY c.user_id, m.seq
    HAVING COUNT(DISTINCT m.conversation_id) > 1
  ) AS overlapping
),
projection_seq_collision_owners AS (
  SELECT DISTINCT user_id FROM (
    SELECT c.user_id, m.seq
    FROM public.thread_projection_messages AS m
    JOIN public.conversations AS c ON c.id = m.conversation_id
    GROUP BY c.user_id, m.seq
    HAVING COUNT(DISTINCT m.conversation_id) > 1
  ) AS overlapping
),
multiple_session_owners AS (
  SELECT c.user_id FROM public.consultation_sessions AS s
  JOIN public.conversations AS c ON c.id = s.conversation_id
  GROUP BY c.user_id HAVING COUNT(*) > 1
),
multiple_projection_owners AS (
  SELECT c.user_id FROM public.thread_projections AS p
  JOIN public.conversations AS c ON c.id = p.conversation_id
  GROUP BY c.user_id HAVING COUNT(*) > 1
),
evidence_links AS (
  SELECT e.source_kind, e.availability, e.user_id AS evidence_owner,
    e.source_conversation_id, e.source_run_id, e.source_message_id,
    c.id AS conversation_exists, c.user_id AS conversation_owner,
    r.id AS run_exists, r.user_id AS run_owner,
    r.conversation_id AS run_conversation_id,
    m.id AS message_exists, m.conversation_id AS message_conversation_id,
    m.run_id AS message_run_id,
    i.id AS interaction_exists, i.run_id AS interaction_run_id,
    i.conversation_id AS interaction_conversation_id
  FROM public.statement_evidence AS e
  LEFT JOIN public.conversations AS c ON c.id = e.source_conversation_id
  LEFT JOIN public.runs AS r ON r.id = e.source_run_id
  LEFT JOIN public.messages AS m ON m.id = e.source_message_id
  LEFT JOIN public.agent_interactions AS i ON i.id = e.source_interaction_id
  WHERE e.source_kind IN ('user_message', 'structured_answer')
),
evidence_identity_mismatches AS (
  SELECT 1 FROM evidence_links AS e
  WHERE
    (e.conversation_exists IS NOT NULL AND e.conversation_owner IS DISTINCT FROM e.evidence_owner)
    OR (e.run_exists IS NOT NULL AND
      (e.run_owner IS DISTINCT FROM e.evidence_owner
       OR e.run_conversation_id IS DISTINCT FROM e.source_conversation_id))
    OR (e.message_exists IS NOT NULL AND
      (e.message_conversation_id IS DISTINCT FROM e.source_conversation_id
       OR (e.message_run_id IS NOT NULL AND e.message_run_id IS DISTINCT FROM e.source_run_id)))
    OR (e.source_kind = 'structured_answer' AND e.interaction_exists IS NOT NULL AND
      (e.interaction_run_id IS DISTINCT FROM e.source_run_id
       OR e.interaction_conversation_id IS DISTINCT FROM e.source_conversation_id))
),
unresolved_provenance AS (
  SELECT 1 FROM evidence_links AS e
  WHERE e.availability <> 'erased'
    AND (e.conversation_exists IS NULL OR e.run_exists IS NULL
      OR e.message_exists IS NULL
      OR (e.source_kind = 'structured_answer' AND e.interaction_exists IS NULL))
),
interaction_receipt_mismatches AS (
  SELECT 1 FROM public.interaction_resumes AS receipt
  LEFT JOIN public.agent_interactions AS i ON i.id = receipt.interaction_id
  LEFT JOIN public.conversations AS c ON c.id = receipt.conversation_id
  LEFT JOIN public.runs AS r ON r.id = receipt.run_id
  LEFT JOIN public.messages AS um ON um.id = receipt.user_message_id
  LEFT JOIN public.messages AS am ON am.id = receipt.assistant_message_id
  WHERE i.id IS NULL OR c.id IS NULL OR r.id IS NULL
    OR i.conversation_id IS DISTINCT FROM receipt.conversation_id
    OR i.run_id IS DISTINCT FROM receipt.run_id
    OR c.user_id IS DISTINCT FROM receipt.user_id
    OR r.user_id IS DISTINCT FROM receipt.user_id
    OR r.conversation_id IS DISTINCT FROM receipt.conversation_id
    OR um.conversation_id IS DISTINCT FROM receipt.conversation_id
    OR am.conversation_id IS DISTINCT FROM receipt.conversation_id
    OR um.run_id IS DISTINCT FROM receipt.run_id
    OR am.run_id IS DISTINCT FROM receipt.run_id
    OR um.turn_id IS DISTINCT FROM r.turn_id
    OR am.turn_id IS DISTINCT FROM r.turn_id
    OR um.role IS DISTINCT FROM 'user' OR am.role IS DISTINCT FROM 'assistant'
),
unreceipted_answers AS (
  SELECT 1 FROM public.agent_interactions AS i
  LEFT JOIN public.interaction_resumes AS receipt ON receipt.interaction_id = i.id
  WHERE i.tool_name = 'ask_user' AND i.status = 'answered'
    AND receipt.interaction_id IS NULL
),
pending_wrong_original_run AS (
  SELECT 1 FROM public.agent_interactions AS i
  LEFT JOIN public.runs AS r ON r.id = i.run_id
  LEFT JOIN public.conversations AS c ON c.id = i.conversation_id
  WHERE i.tool_name = 'ask_user' AND i.status = 'pending'
    AND (r.id IS NULL OR c.id IS NULL OR r.status <> 'waiting_user'
      OR r.conversation_id IS DISTINCT FROM i.conversation_id
      OR r.user_id IS DISTINCT FROM c.user_id)
),
projection_message_mismatches AS (
  SELECT 1 FROM public.thread_projection_messages AS p
  LEFT JOIN public.messages AS m ON m.id = p.message_id
  WHERE m.id IS NULL
    OR p.conversation_id IS DISTINCT FROM m.conversation_id
    OR (p.run_id IS NOT NULL AND m.run_id IS NOT NULL
      AND p.run_id IS DISTINCT FROM m.run_id)
)
SELECT
  (SELECT COUNT(*) FROM message_seq_collision_owners)::bigint AS owners_with_message_seq_collisions,
  (SELECT COUNT(*) FROM projection_seq_collision_owners)::bigint AS owners_with_projection_seq_collisions,
  (SELECT COUNT(*) FROM multiple_session_owners)::bigint AS owners_with_multiple_consultation_roots,
  (SELECT COUNT(*) FROM multiple_projection_owners)::bigint AS owners_with_multiple_projection_roots,
  (SELECT COUNT(*) FROM evidence_identity_mismatches)::bigint AS evidence_identity_mismatches,
  (SELECT COUNT(*) FROM unresolved_provenance)::bigint AS unresolved_provenance_to_review,
  (SELECT COUNT(*) FROM interaction_receipt_mismatches)::bigint AS interaction_receipt_mismatches,
  (SELECT COUNT(*) FROM unreceipted_answers)::bigint AS answered_ask_user_without_receipts,
  (SELECT COUNT(*) FROM pending_wrong_original_run)::bigint AS pending_ask_user_wrong_original_run,
  (SELECT COUNT(*) FROM projection_message_mismatches)::bigint AS projection_message_mismatches,
  (SELECT COUNT(*) FROM public.conversation_shares)::bigint AS legacy_controlled_snapshots_to_revoke;
COMMIT;
