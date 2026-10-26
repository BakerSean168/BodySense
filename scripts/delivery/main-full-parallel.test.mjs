import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import YAML from "yaml";
import {
  MANIFEST_SCHEMA,
  MANIFEST_SCHEMA_V6,
  POLICY_VERSION_V5,
  LANE_TARGETS_V6,
  LANE_TARGETS,
  computeManifestDigest,
  createManifest,
  evaluateManifestOracle,
  oracleEnabledFromManifest,
  validateManifest,
} from "./lib.mjs";

const A = "a".repeat(40);
const B = "b".repeat(40);
function selected(event, files = ["docs/overview.md"], forceFull = false) {
  return createManifest({
    event,
    ref: event === "pull_request" ? "refs/pull/473/merge" : "refs/heads/main",
    requestedBaseSha: A,
    baseSha: A,
    headSha: B,
    changedPaths: files,
    forceFull,
    generatedAt: "2026-10-26T00:00:00.000Z",
  });
}
function workflows() {
  const source = fs.readFileSync(".github/workflows/ci.yml", "utf8");
  const doc = YAML.parseDocument(source, { uniqueKeys: true });
  assert.deepEqual(doc.errors, []);
  return doc.toJS().jobs;
}

test("Main v7 requires all independent quality owners, never the old serial full child", () => {
  assert.equal(MANIFEST_SCHEMA, "bodysense.delivery-manifest/v7");
  const main = selected("push", ["docs/a.md"], true);
  assert.deepEqual(validateManifest(main), []);
  assert.equal(main.stage, "main-full");
  const quality = oracleEnabledFromManifest(main, "quality");
  assert.deepEqual(quality, {
    "quality-web": true,
    "quality-api": true,
    "quality-ai": true,
    "quality-arch": true,
    "quality-release": true,
    contracts: true,
  });
  const successful = Object.fromEntries(
    Object.keys(quality).map((key) => [key, "success"]),
  );
  assert.equal(
    evaluateManifestOracle({
      manifest: main,
      group: "quality",
      children: successful,
    }).ok,
    true,
  );
  for (const name of Object.keys(quality)) {
    const denied = evaluateManifestOracle({
      manifest: main,
      group: "quality",
      children: { ...successful, [name]: "skipped" },
    });
    assert.equal(denied.ok, false, name);
    assert.ok(
      denied.failures.some((f) => f.includes(name)),
      name,
    );
  }
  assert.ok(!main.laneDetails.full.targets.includes("job:quality-full-child"));
  assert.ok(!main.laneDetails.full.targets.includes("script:validate-repo.sh"));
  assert.ok(LANE_TARGETS.full.includes("job:quality-release-child"));
});

test("PR fast retains existing selected low-level gates, no release or Playwright E2E", () => {
  const pr = selected("pull_request", ["apps/web/src/x.ts"]);
  assert.equal(pr.stage, "pr-fast");
  assert.deepEqual(validateManifest(pr), []);
  assert.deepEqual(oracleEnabledFromManifest(pr, "quality"), {
    "quality-web": true,
    "quality-api": false,
    "quality-ai": false,
    "quality-arch": true,
    "quality-release": false,
    contracts: false,
  });
  assert.equal(pr.lanes.full, false);
  assert.equal(pr.lanes.experience, false);
});

test("Main and Nightly quality runners are flat and the exact Digest is required", () => {
  const jobs = workflows();
  assert.ok(!("quality-full-child" in jobs));
  const owners = [
    "quality-web-child",
    "quality-api-child",
    "quality-ai-child",
    "quality-arch-child",
    "quality-release-child",
    "contracts-child",
    "db-integration-child",
  ];
  for (const owner of owners) {
    assert.deepEqual(jobs[owner].needs, "scope", owner);
  }
  assert.match(
    jobs["quality-release-child"].if,
    /github.event_name != 'pull_request'/,
  );
  assert.ok(jobs["quality-oracle"].needs.includes("quality-release-child"));
  assert.ok(
    jobs["delivery-observation"].needs.includes("quality-release-child"),
  );
  const oracleCommand = jobs["quality-oracle"].steps.at(-1).run;
  assert.match(oracleCommand, /quality-release/);
  assert.doesNotMatch(oracleCommand, /quality-full/);
  const releaseCommands = jobs["quality-release-child"].steps
    .filter((step) => step.run)
    .map((step) => step.run)
    .join("\n");
  assert.match(releaseCommands, /validate-manifest\.mjs/);
  assert.match(releaseCommands, /expect-digest/);
  assert.match(releaseCommands, /\.stage != "pr-fast"/);
  assert.match(releaseCommands, /run-release-safety\.sh/);
  const child = jobs["quality-release-child"];
  assert.ok(
    child.steps.some((step) => step.with?.name === "delivery-manifest-v7"),
  );
});

test("v6 stored manifests preserve the old full serial target and archived Oracle policy", () => {
  const m = selected("push", ["docs/a.md"], true);
  const legacy = structuredClone(m);
  legacy.schema = MANIFEST_SCHEMA_V6;
  legacy.policyVersion = POLICY_VERSION_V5;
  for (const lane of Object.keys(legacy.laneDetails)) {
    legacy.laneDetails[lane].targets = [...LANE_TARGETS_V6[lane]];
  }
  legacy.digest = computeManifestDigest(legacy);
  assert.deepEqual(validateManifest(legacy), []);
  const map = oracleEnabledFromManifest(legacy, "quality");
  assert.equal(map["quality-full"], true);
  assert.equal(map["quality-web"], false);
  assert.equal(map["quality-release"], undefined);
});

test("old release, supply-chain, migration and disaster-recovery checks have a new owner", () => {
  const old = fs.readFileSync("scripts/validate-repo.sh", "utf8");
  const release = fs.readFileSync(
    "scripts/quality/run-release-safety.sh",
    "utf8",
  );
  const quality = fs.readFileSync("scripts/delivery/run-quality.mjs", "utf8");
  const contracts = fs.readFileSync(
    "scripts/delivery/run-contracts.mjs",
    "utf8",
  );
  const jobs = workflows();
  for (const command of [
    "pnpm quality:capacity",
    "bash scripts/validate-production-capacity.sh",
    "bash scripts/validate-production-postgres18.sh",
    "bash scripts/validate-production-dr-installer.sh",
    "bash scripts/validate-production-proxy.sh",
    "bash scripts/test-validate-supply-chain.sh",
    "bash scripts/validate-supply-chain.sh",
    "bash scripts/test-v2-release-dry-run.sh",
    "pnpm test:static-assets",
    "pnpm test:validation-lifecycle",
    "bash scripts/validate-migration-history.sh",
    "python3 scripts/test_offhost_s3.py",
    "bash scripts/validate-offhost-dr-unit.sh",
    "bash scripts/validate-offhost-dr.sh",
    "git diff --check",
    "git diff --cached --check",
  ]) {
    assert.ok(old.includes(command), "source missing " + command);
    assert.ok(release.includes(command), "new owner missing " + command);
  }
  const syntaxCommands = old
    .split("bash -n ")[1]
    .split("bash scripts/validate-production-capacity.sh")[0];
  for (const script of syntaxCommands.match(/scripts\/[\w/.-]+\.sh/g) ?? []) {
    assert.ok(release.includes(script), "release syntax missing " + script);
  }
  for (const target of [
    "eval:assessment-evidence-contract",
    "eval:diagnosis",
    "eval:treatment-promotion",
  ]) {
    assert.ok(old.includes(target), "original target " + target);
    assert.ok(quality.includes(target), "AI owner missing " + target);
  }
  assert.match(quality, /run\("pnpm", \["quality:architecture"\]\)/);
  assert.match(quality, /run\("pnpm", \["quality:architecture:test"\]\)/);
  assert.match(quality, /run\("pnpm", \["quality:capacity:test"\]\)/);
  assert.match(quality, /if \(full\)/);
  assert.ok(contracts.includes('"contracts:verify"'));
  assert.ok(
    jobs["governance-child"].steps.some((step) =>
      (step.run ?? "").includes("node --test scripts/delivery/*.test.mjs"),
    ),
  );
  const lint = jobs.lint.steps.map((step) => step.run ?? "").join("\n");
  for (const command of [
    "pnpm format:check",
    "pnpm format:check:go",
    "pnpm lint:oxlint",
    "pnpm lint:oxlint:types",
  ]) {
    assert.ok(lint.includes(command), "Lint missing " + command);
  }
});

test("Main/Nightly release-safety fails closed on candidate identity mismatch", () => {
  const manifest = selected("schedule", [], true);
  assert.equal(manifest.stage, "nightly-audit");
  assert.deepEqual(validateManifest(manifest), []);
  const altered = structuredClone(manifest);
  altered.stage = "pr-fast";
  altered.digest = computeManifestDigest(altered);
  assert.ok(validateManifest(altered).some((error) => error.includes("stage")));
});
