import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

const sql = fs.readFileSync("scripts/plan/sql/pc-001-reconciliation-conflicts.sql", "utf8");
const shell = fs.readFileSync("scripts/plan/test-pc-001-reconciliation-conflicts.sh", "utf8");
const statements = sql.replace(/^--.*$/gm, "");
const finalSelect = statements.slice(statements.lastIndexOf("\nSELECT\n"));

test("preflight refuses mutation, partial permissions and schema drift", () => {
  assert.match(sql, /NOT a remap, canonical-ID selection, cleanup, or migration approval/);
  assert.match(statements, /BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;/);
  assert.match(statements, /SET LOCAL statement_timeout = '30s';/);
  assert.match(statements, /SET LOCAL lock_timeout = '5s';/);
  assert.match(statements, /SET LOCAL row_security = off;/);
  assert.match(statements, /SET LOCAL search_path = pg_catalog;/);
  assert.match(statements, /BETWEEN 180000 AND 189999/);
  assert.match(statements, /to_regclass\('public\.interaction_resumes'\)/);
  assert.match(statements, /attname = 'source_interaction_id'/);
  assert.match(statements, /THEN 1 ELSE 0 END AS pg18_pc001_schema_guard/);
  assert.match(statements, /COMMIT;\s*$/);
  assert.doesNotMatch(statements, /\b(?:INSERT|UPDATE|DELETE|TRUNCATE|CREATE|ALTER|DROP|GRANT|REVOKE|CALL|DO|COPY|EXECUTE)\b/i);
  assert.deepEqual(statements.match(/^\\[^\n]*/gm), [
    "\\set ON_ERROR_STOP on", "\\set ECHO none", "\\set VERBOSITY terse",
  ]);
});

test("detects same-owner sequence collisions, root clashes and deleted history", () => {
  assert.match(statements, /message_seq_collision_owners/);
  assert.match(statements, /projection_seq_collision_owners/);
  assert.match(statements, /HAVING COUNT\(DISTINCT m\.conversation_id\) > 1/g);
  assert.match(statements, /multiple_session_owners/);
  assert.match(statements, /multiple_projection_owners/);
  assert.doesNotMatch(statements, /deleted_at\s+IS\s+NULL/i);
  assert.match(sql, /including deleted Conversations/);
});

test("original Run, user ownership, source retention and controlled copies are audited", () => {
  for (const fragment of [
    "public.statement_evidence", "public.interaction_resumes", "public.agent_interactions",
    "public.runs", "public.messages", "public.thread_projection_messages",
    "public.conversation_shares", "structured_answer", "source_interaction_id",
    "source_message_id", "r.conversation_id IS DISTINCT FROM receipt.conversation_id",
    "i.run_id IS DISTINCT FROM receipt.run_id",
    "r.status <> 'waiting_user'", "i.status = 'answered'",
  ]) {
    assert.ok(statements.includes(fragment), fragment);
  }
  assert.match(statements, /e\.availability <> 'erased'/);
  assert.match(sql, /missing source Message\/Run can be legitimate/);
  assert.match(sql, /zero-count result cannot establish safe erasure/);
});

test("only eleven aggregate counters are returned, no raw health or owner values", () => {
  const aliases = [...finalSelect.matchAll(/\)::bigint AS ([a-z_]+)/g)].map((m) => m[1]);
  assert.deepEqual(aliases, [
    "owners_with_message_seq_collisions",
    "owners_with_projection_seq_collisions",
    "owners_with_multiple_consultation_roots",
    "owners_with_multiple_projection_roots",
    "evidence_identity_mismatches",
    "unresolved_provenance_to_review",
    "interaction_receipt_mismatches",
    "answered_ask_user_without_receipts",
    "pending_ask_user_wrong_original_run",
    "projection_message_mismatches",
    "legacy_controlled_snapshots_to_revoke",
  ]);
  assert.equal((finalSelect.match(/COUNT\(\*\)/g) ?? []).length, 11);
  assert.doesNotMatch(finalSelect, /\b(?:user_id|conversation_id|run_id|message_id|excerpt|share_token|payload|snapshot_messages)\b/);
  assert.doesNotMatch(finalSelect, /ready|approved|safe_for/);
});

test("real PG18 harness covers positive, conflicts and fail-closed negative cases", () => {
  assert.match(shell, /pgvector\/pgvector:pg18@sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a/);
  assert.match(shell, /docker image inspect "\$image"/);
  assert.match(shell, /--pull never --rm -d/);
  assert.match(shell, /--network none/);
  assert.match(shell, /trap cleanup EXIT/);
  assert.match(shell, /docker rm -f "\$container"/);
  assert.match(shell, /migrations\/\*\.up\.sql/);
  assert.doesNotMatch(shell, /--(?:publish|volume|mount|env-file)|docker (?:pull|compose|system|volume)|\$\{?(?:DATABASE_URL|BODYSENSE_READONLY_DATABASE_URL)/);
  for (const scenario of [
    "missing migration fail-closed", "empty_schema", "consistent_single",
    "history_merge_conflicts", "provenance_legacy", "wrong_run_and_projection",
    "immutable_snapshot", "RLS/permission fail-closed",
  ]) assert.ok(shell.includes(scenario), scenario);
  assert.match(shell, /duplicate key value violates unique constraint/);
  assert.match(shell, /cannot execute INSERT in a read-only transaction/);
  assert.match(shell, /query would be affected by row-level security policy/);
  assert.match(shell, /permission denied for table statement_evidence/);
  assert.match(shell, /SYNTHETIC_PRIVATE_SENTINEL/);
});
