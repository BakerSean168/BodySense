import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

const sql = fs.readFileSync("scripts/plan/sql/pc-001-reference-inventory.sql", "utf8");
const harness = fs.readFileSync("scripts/plan/test-pc-001-reference-inventory.sh", "utf8");
const statements = sql.replace(/^--.*$/gm, "");
const generated = sql.match(/\$inventory\$([\s\S]*?)\$inventory\$/)?.[1];

test("inventory uses a bounded, fail-closed PG18 read-only snapshot", () => {
  assert.match(statements, /BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;/);
  assert.match(statements, /SET LOCAL statement_timeout = '30s';/);
  assert.match(statements, /SET LOCAL lock_timeout = '5s';/);
  assert.match(statements, /SET LOCAL search_path = pg_catalog;/);
  assert.match(statements, /SET LOCAL row_security = off;/);
  assert.match(statements, /BETWEEN 180000 AND 189999/);
  assert.match(statements, /THEN 1 ELSE 0 END AS pg18_target_guard/);
  assert.match(statements, /COMMIT;\s*$/);
  assert.doesNotMatch(statements, /\b(?:INSERT|UPDATE|DELETE|TRUNCATE|CREATE|ALTER|DROP|GRANT|REVOKE|CALL|DO|COPY|EXECUTE)\b/i);
  assert.deepEqual(statements.match(/^\\[^\n]*/gm), [
    "\\set ON_ERROR_STOP on", "\\set ECHO none", "\\set VERBOSITY terse", "\\gexec",
  ]);
});

test("catalog discovery pairs every FK target with its source and includes unbound columns", () => {
  assert.match(statements, /FROM pg_constraint AS k/);
  assert.match(statements, /unnest\(k\.conkey, k\.confkey\)/);
  assert.match(statements, /k\.contype = 'f'/);
  assert.match(statements, /k\.confrelid = 'public\.conversations'::regclass/);
  assert.match(statements, /target\.attname = 'id'/);
  assert.match(statements, /LEFT JOIN fk_pairs/);
  assert.match(statements, /a\.attname IN \('conversation_id', 'source_conversation_id'\)/);
  assert.match(statements, /t\.relkind IN \('r', 'p'\)/);
  assert.match(statements, /NOT a\.attisdropped/);
  assert.match(statements, /convalidated/);
  assert.match(statements, /cardinality\(k\.conkey\)/);
  assert.doesNotMatch(statements, /WHERE[^;]*convalidated\s*=\s*true/i);
  assert.match(statements, /FROM %I\.%I AS r/);
  assert.match(statements, /format\('c\.id = r\.%I', a\.attname\)/);
  assert.match(statements, /format\('c\.id::text = r\.%I::text', a\.attname\)/);
});

test("generated result projects exactly metadata and five counts, never raw values", () => {
  assert.ok(generated);
  const projection = generated.slice(0, generated.indexOf("FROM %I.%I AS r"));
  const aliases = [...projection.matchAll(/\bAS ([a-z_]+)/g)].map((match) => match[1]);
  assert.deepEqual(aliases.filter((alias) => alias !== "c"), [
    "table_schema", "table_name", "relation_kind", "reference_column",
    "column_type", "nullable", "reference_kind", "fk_name", "fk_validated",
    "fk_deferrable", "fk_initially_deferred", "fk_column_count", "on_delete",
    "on_update", "comparison_mode", "row_count", "null_reference_count",
    "nonnull_reference_count", "orphan_reference_count", "deleted_target_reference_count",
  ]);
  assert.equal((projection.match(/COUNT\(\*\)/g) ?? []).length, 5);
  assert.match(projection, /NOT EXISTS \(/);
  assert.match(projection, /c\.deleted_at IS NOT NULL/);
  assert.doesNotMatch(generated, /SELECT\s+(?:r\.|c\.|\*)|\b(?:user_id|content_text|share_token|snapshot_messages|raw_output|excerpt|payload|array_agg|json_agg|string_agg)\b/i);
  assert.doesNotMatch(statements, /\b(?:safe_for|ready_for|migration_approved)\w*/i);
});

test("native fixture is cached, isolated, synthetic and automatically removed", () => {
  assert.match(harness, /pgvector\/pgvector:pg18@sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a/);
  assert.match(harness, /docker image inspect "\$image"/);
  assert.match(harness, /--pull never --rm -d/);
  assert.match(harness, /--network none/);
  assert.match(harness, /trap cleanup EXIT/);
  assert.match(harness, /docker rm -f "\$container"/);
  assert.match(harness, /base_schema=.*000001_vnext_baseline\.up\.sql/);
  assert.doesNotMatch(harness, /--(?:publish|volume|mount|env-file)|\$\{?(?:DATABASE_URL|BODYSENSE_READONLY_DATABASE_URL)|docker (?:pull|compose|system|volume)/);
  for (const scenario of ["empty baseline", "populated/null/deleted", "non-FK/orphan/renamed/composite/unvalidated", "snapshot/read-only enforcement", "RLS incomplete-result rejection"]) {
    assert.ok(harness.includes(scenario), scenario);
  }
  assert.match(harness, /assert_private_output/);
  assert.match(harness, /no_conversation_fk\|3\|1\|2\|1\|0/);
});

test("inventory explicitly leaves full provenance, retention and migration approval gated", () => {
  assert.match(sql, /NOT a migration/);
  assert.match(sql, /JSON\/source keys, indirect Message\/Run links, checkpoints, caches and external/);
  assert.match(sql, /Counts are per table\/column\/FK, not additive/);
  assert.match(sql, /Composite FK counts inspect only the pair/);
  assert.match(sql, /Zero rows\/orphans never authorize/);
  assert.match(sql, /incomplete output is not acceptance/);
});
