-- PC-001 / PC-D4-D5: aggregate-only reference inventory, NOT a migration.
-- psql -X -v ON_ERROR_STOP=1 "$BODYSENSE_READONLY_DATABASE_URL" \
--   -f scripts/plan/sql/pc-001-reference-inventory.sql
-- No IDs, identities, messages, tokens or health payloads are projected.
-- Catalog metadata discovers every FK column mapped to public.conversations.id
-- (including renamed/composite/unvalidated FKs), plus conversation_id and
-- source_conversation_id without that FK, in all non-system stored tables.
-- provider_conversation_id is an external provider identity, not a local ID.
-- JSON/source keys, indirect Message/Run links, checkpoints, caches and external
-- stores require separate application/source review; this is not full coverage.
-- Counts are per table/column/FK, not additive: partition parents include their
-- children, and multiple FK constraints can report the same column again.
-- Composite FK counts inspect only the pair targeting id, not the full tuple.
-- Non-uuid columns use text comparison for review only (no UUID cast of data).
-- Zero rows/orphans never authorize reconciliation, erasure or singleton rollout.
-- Permission/RLS/timeout errors abort: incomplete output is not acceptance.
\set ON_ERROR_STOP on
\set ECHO none
\set VERBOSITY terse

BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL statement_timeout = '30s';
SET LOCAL lock_timeout = '5s';
SET LOCAL search_path = pg_catalog;
SET LOCAL row_security = off;

-- Fail closed on a wrong major version or absent canonical target columns.
SELECT 1 / CASE WHEN
  current_setting('server_version_num')::integer BETWEEN 180000 AND 189999
  AND EXISTS (
    SELECT 1 FROM pg_attribute
    WHERE attrelid = to_regclass('public.conversations')
      AND attname = 'id' AND atttypid = 'uuid'::regtype
      AND attnum > 0 AND NOT attisdropped
  )
  AND EXISTS (
    SELECT 1 FROM pg_attribute
    WHERE attrelid = to_regclass('public.conversations')
      AND attname = 'deleted_at' AND attnum > 0 AND NOT attisdropped
  )
  THEN 1 ELSE 0 END AS pg18_target_guard;

WITH fk_pairs AS (
  SELECT k.conrelid, pair.source_attnum, k.conname,
    k.convalidated, k.condeferrable, k.condeferred,
    cardinality(k.conkey) AS fk_column_count,
    CASE k.confdeltype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT'
      WHEN 'c' THEN 'CASCADE' WHEN 'n' THEN 'SET NULL'
      WHEN 'd' THEN 'SET DEFAULT' END AS on_delete,
    CASE k.confupdtype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT'
      WHEN 'c' THEN 'CASCADE' WHEN 'n' THEN 'SET NULL'
      WHEN 'd' THEN 'SET DEFAULT' END AS on_update
  FROM pg_constraint AS k
  CROSS JOIN LATERAL unnest(k.conkey, k.confkey)
    AS pair(source_attnum, target_attnum)
  JOIN pg_attribute AS target ON target.attrelid = k.confrelid
    AND target.attnum = pair.target_attnum
  WHERE k.contype = 'f'
    AND k.confrelid = 'public.conversations'::regclass
    AND target.attname = 'id' AND NOT target.attisdropped
), reference_columns AS (
  SELECT n.nspname, t.relname, t.relkind, a.attname,
    format_type(a.atttypid, a.atttypmod) AS column_type,
    NOT a.attnotnull AS nullable,
    f.conname, f.convalidated, f.condeferrable, f.condeferred,
    f.fk_column_count, f.on_delete, f.on_update,
    CASE WHEN a.atttypid = 'uuid'::regtype THEN
      format('c.id = r.%I', a.attname)
    ELSE format('c.id::text = r.%I::text', a.attname) END AS match_expression,
    CASE WHEN a.atttypid = 'uuid'::regtype THEN 'uuid'
      ELSE 'text_review_only' END AS comparison_mode
  FROM pg_class AS t
  JOIN pg_namespace AS n ON n.oid = t.relnamespace
  JOIN pg_attribute AS a ON a.attrelid = t.oid
  LEFT JOIN fk_pairs AS f ON f.conrelid = t.oid
    AND f.source_attnum = a.attnum
  WHERE t.relkind IN ('r', 'p')
    AND n.nspname <> 'information_schema' AND n.nspname !~ '^pg_'
    AND a.attnum > 0 AND NOT a.attisdropped
    AND (f.conname IS NOT NULL OR a.attname IN ('conversation_id', 'source_conversation_id'))
)
-- gexec executes only these catalog-generated aggregate SELECTs; the generated
-- SQL itself is not printed. %I quotes identifiers; %L quotes metadata literals.
SELECT format($inventory$
SELECT %L::text AS table_schema, %L::text AS table_name,
  %L::text AS relation_kind, %L::text AS reference_column,
  %L::text AS column_type, %L::boolean AS nullable,
  %L::text AS reference_kind, %L::text AS fk_name,
  %L::boolean AS fk_validated, %L::boolean AS fk_deferrable,
  %L::boolean AS fk_initially_deferred, %L::integer AS fk_column_count,
  %L::text AS on_delete, %L::text AS on_update,
  %L::text AS comparison_mode,
  COUNT(*)::bigint AS row_count,
  COUNT(*) FILTER (WHERE r.%I IS NULL)::bigint AS null_reference_count,
  COUNT(*) FILTER (WHERE r.%I IS NOT NULL)::bigint AS nonnull_reference_count,
  COUNT(*) FILTER (WHERE r.%I IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM public.conversations AS c WHERE %s
  ))::bigint AS orphan_reference_count,
  COUNT(*) FILTER (WHERE r.%I IS NOT NULL AND EXISTS (
    SELECT 1 FROM public.conversations AS c WHERE %s AND c.deleted_at IS NOT NULL
  ))::bigint AS deleted_target_reference_count
FROM %I.%I AS r;
$inventory$,
  nspname, relname, relkind, attname, column_type, nullable,
  CASE WHEN conname IS NULL THEN 'no_conversation_fk' ELSE 'foreign_key' END,
  conname, convalidated, condeferrable, condeferred, fk_column_count,
  on_delete, on_update, comparison_mode,
  attname, attname, attname, match_expression, attname, match_expression,
  nspname, relname)
FROM reference_columns
ORDER BY nspname, relname, attname, conname
\gexec

COMMIT;
