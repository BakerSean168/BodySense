-- PC-001 / PC-D4-D5. Non-mutating, aggregated preflight for a future
-- one-user-one-Conversation data migration. Never returns user IDs,
-- message contents, share tokens, or other health data.
--
-- Run with: psql -X -v ON_ERROR_STOP=1 "$BODYSENSE_READONLY_DATABASE_URL" \
--   -f scripts/plan/sql/pc-001-singleton-readiness.sql
--
-- This is NOT the migration. A zero-blocker result does not prove an
-- exclusive writer lease, full retention migration or clinical acceptance.
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL statement_timeout = '30s';

WITH conversation_stats AS (
  SELECT
    user_id,
    COUNT(*) FILTER (WHERE deleted_at IS NULL) AS live_count,
    COUNT(*) FILTER (WHERE deleted_at IS NOT NULL) AS deleted_count
  FROM public.conversations
  GROUP BY user_id
),
owners AS (
  -- Include owners of malformed orphan/mismatched Runs even if they do
  -- not have a matching Conversation; otherwise a broken run may disappear
  -- from the aggregate and incorrectly signal readiness.
  SELECT user_id FROM conversation_stats
  UNION
  SELECT user_id FROM public.runs
),
run_stats AS (
  SELECT
    r.user_id,
    COUNT(*) FILTER (WHERE r.status IN ('running', 'waiting_user')) AS live_run_count,
    COUNT(*) FILTER (
      WHERE c.id IS NULL OR c.user_id IS DISTINCT FROM r.user_id
    ) AS cross_owner_or_orphan_run_count
  FROM public.runs AS r
  LEFT JOIN public.conversations AS c ON c.id = r.conversation_id
  GROUP BY r.user_id
),
active_pointers AS (
  SELECT
    c.user_id,
    COUNT(*) FILTER (WHERE c.active_run_id IS NOT NULL) AS active_pointer_count,
    COUNT(*) FILTER (
      WHERE c.active_run_id IS NOT NULL
        AND (
          r.id IS NULL
          OR r.conversation_id IS DISTINCT FROM c.id
          OR r.user_id IS DISTINCT FROM c.user_id
          OR r.status NOT IN ('running', 'waiting_user')
        )
    ) AS stale_or_foreign_pointer_count
  FROM public.conversations AS c
  LEFT JOIN public.runs AS r ON r.id = c.active_run_id
  WHERE c.deleted_at IS NULL
  GROUP BY c.user_id
),
pending_interactions AS (
  SELECT
    c.user_id,
    COUNT(*) FILTER (WHERE i.status = 'pending') AS pending_count
  FROM public.agent_interactions AS i
  JOIN public.conversations AS c ON c.id = i.conversation_id
  GROUP BY c.user_id
),
legacy_shares AS (
  SELECT
    c.user_id,
    COUNT(*) AS share_count
  FROM public.conversation_shares AS s
  JOIN public.conversations AS c ON c.id = s.conversation_id
  GROUP BY c.user_id
),
owner_summary AS (
  SELECT
    o.user_id,
    COALESCE(c.live_count, 0) AS live_count,
    COALESCE(c.deleted_count, 0) AS deleted_count,
    COALESCE(r.live_run_count, 0) AS live_run_count,
    COALESCE(r.cross_owner_or_orphan_run_count, 0) AS cross_owner_or_orphan_run_count,
    COALESCE(p.active_pointer_count, 0) AS active_pointer_count,
    COALESCE(p.stale_or_foreign_pointer_count, 0) AS stale_or_foreign_pointer_count,
    COALESCE(i.pending_count, 0) AS pending_count,
    COALESCE(s.share_count, 0) AS share_count
  FROM owners AS o
  LEFT JOIN conversation_stats AS c ON c.user_id = o.user_id
  LEFT JOIN run_stats AS r ON r.user_id = o.user_id
  LEFT JOIN active_pointers AS p ON p.user_id = o.user_id
  LEFT JOIN pending_interactions AS i ON i.user_id = o.user_id
  LEFT JOIN legacy_shares AS s ON s.user_id = o.user_id
)
SELECT
  COUNT(*)::bigint AS owners_with_history,
  COUNT(*) FILTER (WHERE live_count > 1)::bigint AS owners_with_multiple_live_conversations,
  COALESCE(SUM(live_count), 0)::bigint AS live_conversations,
  COALESCE(SUM(deleted_count), 0)::bigint AS deleted_conversations_requiring_history_review,
  COUNT(*) FILTER (WHERE live_run_count > 1)::bigint AS owners_with_parallel_live_runs,
  COALESCE(SUM(live_run_count), 0)::bigint AS live_or_waiting_runs_to_drain_or_resume,
  COALESCE(SUM(cross_owner_or_orphan_run_count), 0)::bigint AS run_owner_reference_mismatches,
  COALESCE(SUM(stale_or_foreign_pointer_count), 0)::bigint AS stale_or_foreign_active_run_pointers,
  COALESCE(SUM(pending_count), 0)::bigint AS pending_ask_user_interactions_to_preserve,
  COALESCE(SUM(share_count), 0)::bigint AS legacy_public_share_tokens_to_revoke,
  COALESCE(BOOL_AND(
    live_count <= 1
    AND deleted_count = 0
    AND live_run_count = 0
    AND cross_owner_or_orphan_run_count = 0
    AND stale_or_foreign_pointer_count = 0
    AND pending_count = 0
    AND share_count = 0
  ), TRUE) AS safe_for_offline_review_only
FROM owner_summary;

COMMIT;
