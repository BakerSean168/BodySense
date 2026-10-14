import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

const sql = fs.readFileSync(
  "scripts/plan/sql/pc-001-singleton-readiness.sql",
  "utf8",
);
const harness = fs.readFileSync(
  "scripts/plan/test-pc-001-readonly-migration-audit.sh",
  "utf8",
);

test("PC-001 audit is a read-only repeatable snapshot without tenant identifiers", () => {
  const statements = sql.replace(/^--.*$/gm, "");
  assert.match(
    statements,
    /BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;/,
  );
  assert.match(statements, /SET LOCAL statement_timeout = '30s';/);
  assert.match(statements, /COMMIT;\s*$/);
  assert.doesNotMatch(
    statements,
    /\b(?:INSERT|UPDATE|DELETE|TRUNCATE|CREATE|ALTER|DROP|GRANT|REVOKE|EXECUTE)\b/i,
  );
  const publishedColumns = statements.slice(
    statements.lastIndexOf("\nSELECT\n"),
  );
  assert.doesNotMatch(
    publishedColumns,
    /\b(?:user_id|share_token|content_text)\b/i,
  );
  assert.match(publishedColumns, /\bowners_with_multiple_live_conversations\b/);
  assert.match(publishedColumns, /\brun_owner_reference_mismatches\b/);
  assert.match(
    publishedColumns,
    /\bpending_ask_user_interactions_to_preserve\b/,
  );
  assert.match(publishedColumns, /\blegacy_public_share_tokens_to_revoke\b/);
});

test("PC-001 preflight has explicit fixture and isolation/cleanup guarantees", () => {
  assert.match(harness, /pgvector\/pgvector:pg18@sha256:/);
  assert.match(harness, /--network none/);
  assert.match(harness, /trap cleanup EXIT/);
  assert.match(harness, /--rm -d/);
  assert.match(harness, /base_schema.*000001_vnext_baseline\.up\.sql/);
  assert.match(harness, /expect_summary empty/);
  assert.match(harness, /expect_summary conflicting/);
  assert.match(harness, /expect_summary resolved/);
  assert.match(harness, /expect_summary deleted_history/);
  assert.match(harness, /'3\|1\|3\|1\|1\|2\|1\|1\|1\|1\|f'/);
  assert.match(harness, /'1\|0\|1\|1\|0\|0\|0\|0\|0\|0\|f'/);
});

test("PC-D4/D5 migration admission cannot confuse preflight with a rollout", () => {
  assert.match(sql, /This is NOT the migration/);
  assert.match(sql, /exclusive writer lease/);
  assert.match(sql, /full retention migration/);
  assert.match(sql, /clinical acceptance/);
  assert.match(sql, /cross_owner_or_orphan_run_count/);
  assert.match(sql, /stale_or_foreign_pointer_count/);
  assert.match(sql, /deleted_count = 0/);
});
