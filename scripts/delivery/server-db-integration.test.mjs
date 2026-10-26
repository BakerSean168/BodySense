import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import YAML from "yaml";
import {
  MANIFEST_SCHEMA,
  MANIFEST_SCHEMA_V5,
  POLICY_VERSION_V4,
  LANE_TARGETS_V5,
  computeManifestDigest,
  createManifest,
  evaluateManifestOracle,
  oracleEnabledFromManifest,
  validateManifest,
} from "./lib.mjs";

const a = "a".repeat(40);
const b = "b".repeat(40);
function m(files, event = "pull_request", forceFull = false) {
  return createManifest({
    event,
    ref: event === "pull_request" ? "refs/pull/471/merge" : "refs/heads/main",
    requestedBaseSha: a,
    baseSha: a,
    headSha: b,
    changedPaths: files,
    forceFull,
  });
}

test("new v6 manifest requires Go DB integration on API, database and all Main runs", () => {
  assert.equal(MANIFEST_SCHEMA, "bodysense.delivery-manifest/v6");
  for (const files of [
    ["apps/api/internal/service/episode.go"],
    ["apps/api/migrations/000061_body.up.sql"],
    ["packages/contracts/src/episode.ts"],
  ]) {
    const manifest = m(files);
    assert.deepEqual(validateManifest(manifest), []);
    assert.equal(manifest.lanes.experience, false);
    assert.equal(manifest.lanes.full, false);
    const owners = oracleEnabledFromManifest(manifest, "database");
    assert.equal(owners["database-integration"], true, files.join(","));
    const rejected = evaluateManifestOracle({
      manifest,
      group: "database",
      children: {
        "database-baseline": owners["database-baseline"]
          ? "success"
          : "skipped",
        "database-recovery": "skipped",
        "database-integration": "missing",
      },
    });
    assert.equal(rejected.ok, false);
    assert.ok(
      rejected.failures.some((x) => x.includes("database-integration")),
    );
  }
  const noGo = m(["apps/web/src/foo.ts"]);
  assert.equal(
    oracleEnabledFromManifest(noGo, "database")["database-integration"],
    false,
  );
  for (const event of ["push", "schedule"]) {
    const full = m(["docs/ops.md"], event, true);
    assert.deepEqual(validateManifest(full), []);
    assert.deepEqual(oracleEnabledFromManifest(full, "database"), {
      "database-baseline": true,
      "database-recovery": true,
      "database-integration": true,
    });
  }
});

test("v5 archived manifest keeps old immutable task policy and Oracle behavior", () => {
  const original = m(["apps/api/internal/service/episode.go"]);
  const legacy = structuredClone(original);
  legacy.schema = MANIFEST_SCHEMA_V5;
  legacy.policyVersion = POLICY_VERSION_V4;
  for (const lane of Object.keys(legacy.laneDetails)) {
    legacy.laneDetails[lane].targets = [...LANE_TARGETS_V5[lane]];
  }
  legacy.digest = computeManifestDigest(legacy);
  assert.deepEqual(validateManifest(legacy), []);
  const expected = oracleEnabledFromManifest(legacy, "database");
  assert.deepEqual(expected, {
    "database-baseline": false,
    "database-recovery": false,
  });
});

test("workflow's PostgreSQL integration Job is a real database job with a strict Oracle", () => {
  const c = YAML.parseDocument(
    fs.readFileSync(".github/workflows/ci.yml", "utf8"),
    {
      uniqueKeys: true,
    },
  );
  assert.deepEqual(c.errors, []);
  const jobs = c.toJS().jobs;
  const job = jobs["db-integration-child"];
  assert.ok(job);
  assert.equal(job.name, "Test Server (PostgreSQL Integration)");
  assert.equal(job.services.postgres.ports[0], "5432:5432");
  assert.match(job.services.postgres.image, /pg18@sha256/);
  const steps = job.steps.flatMap((s) => (s.run ? [s.run] : [])).join("\n");
  assert.match(steps, /go run .\/cmd\/migration-validator/);
  assert.match(steps, /run-go-db-tests\.py --profile/);
  assert.match(job.if, /lane_api/);
  assert.match(job.if, /lane_database/);
  const oracle = jobs["database-oracle"];
  assert.ok(oracle.needs.includes("db-integration-child"));
  assert.match(oracle.steps.at(-1).run, /database-integration/);
  assert.ok(
    jobs["delivery-observation"].needs.includes("db-integration-child"),
  );
});

test("PR never runs a full-stack Browser E2E and full Main still requires all results", () => {
  const pr = m(["apps/api/internal/service/episode.go"]);
  const main = m(["docs/guide.md"], "push", true);
  assert.equal(pr.lanes.experience, false);
  assert.equal(main.lanes.experience, true);
  assert.equal(
    oracleEnabledFromManifest(pr, "database")["database-recovery"],
    false,
  );
});
