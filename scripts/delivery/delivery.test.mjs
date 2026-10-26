import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";
import {
  classifyPaths,
  computeManifestDigest,
  createManifest,
  createRunSummary,
  evaluateManifestOracle,
  evaluateOracle,
  LANE_TARGETS_V2,
  LANE_TARGETS_V3,
  MANIFEST_SCHEMA_V1,
  MANIFEST_SCHEMA_V2,
  MANIFEST_SCHEMA_V3,
  MANIFEST_SCHEMA_V5,
  POLICY_VERSION_V4,
  LANE_TARGETS_V5,
  oracleEnabledFromManifest,
  POLICY_VERSION_V1,
  POLICY_VERSION_V2,
  validateManifest,
} from "./lib.mjs";
import { createCandidate, validateCandidate } from "./candidate-manifest.mjs";
import {
  AI_RUNTIME_BASE_INPUTS,
  createAiRuntimeBaseIdentity,
  extractAiRuntimeBaseRecipe,
} from "./ai-runtime-base.mjs";
import {
  extractReleaseNotes,
  validateReleaseFiles,
} from "./release-contract.mjs";
import {
  createReleaseManifest,
  validateReleaseManifest,
} from "./release-manifest.mjs";

const A = "a".repeat(40);
const B = "b".repeat(40);

test("workflow actions use immutable SHAs with version comments", () => {
  const workflows = fs
    .readdirSync(".github/workflows")
    .filter((file) => /\.ya?ml$/.test(file));
  let references = 0;
  for (const file of workflows) {
    const contents = fs.readFileSync(
      path.join(".github/workflows", file),
      "utf8",
    );
    for (const line of contents.split("\n")) {
      const match = line.match(/^\s*(?:- )?uses:\s*(\S+)(.*)$/);
      if (!match) continue;
      references++;
      assert.match(
        match[1],
        /^[\w.-]+\/[\w.-]+@[0-9a-f]{40}$/,
        `${file}: ${line.trim()}`,
      );
      assert.match(
        match[2],
        /^\s+# v\d+(?:\.\d+){0,2}\s*$/,
        `${file}: ${line.trim()}`,
      );
    }
  }
  assert.ok(references > 0, "expected workflow action references");
});

function manifestFor(paths, options = {}) {
  return createManifest({
    requestedBaseSha: A,
    baseSha: A,
    headSha: B,
    event: options.event ?? "pull_request",
    ref: options.ref ?? "refs/pull/1/merge",
    changedPaths: paths,
    forceFull: options.forceFull ?? false,
    generatedAt: options.generatedAt ?? "2026-08-29T00:00:00.000Z",
  });
}

function assertLanes(paths, expected) {
  const actual = classifyPaths(paths).lanes;
  for (const [lane, enabled] of Object.entries(expected)) {
    assert.equal(actual[lane], enabled, `${paths.join(", ")} lane ${lane}`);
  }
}

test("docs-only changes select governance only", () => {
  const result = classifyPaths(["docs/architecture/foo.md"]);
  assert.equal(result.risk, "docs");
  assert.deepEqual(result.lanes, {
    governance: true,
    web: false,
    api: false,
    ai: false,
    contracts: false,
    database: false,
    experience: false,
    component: false,
    full: false,
  });
});

test("Web changes select Web and experience lanes without forcing full", () => {
  const result = classifyPaths(["apps/web/src/features/foo.tsx"]);
  assert.equal(result.risk, "web");
  assertLanes(["apps/web/src/features/foo.tsx"], {
    governance: true,
    web: true,
    experience: true,
    component: true,
    contracts: false,
    api: false,
    ai: false,
    database: false,
    full: false,
  });
});

test("Web E2E changes remain in the Web + experience policy", () => {
  assertLanes(["apps/web/e2e/consultation.spec.ts"], {
    web: true,
    contracts: false,
    experience: true,
    component: true,
  });
});

test("Go API changes select API and experience", () => {
  const result = classifyPaths(["apps/api/internal/service/foo.go"]);
  assert.equal(result.risk, "api");
  assertLanes(["apps/api/internal/service/foo.go"], {
    api: true,
    contracts: false,
    database: false,
    experience: true,
  });
});

test("Python runtime changes select AI and experience", () => {
  const result = classifyPaths(["apps/ai-service/src/runtime/foo.py"]);
  assert.equal(result.risk, "ai");
  assertLanes(["apps/ai-service/src/runtime/foo.py"], {
    ai: true,
    contracts: false,
    experience: true,
    database: false,
  });
});

test("migration changes select database safety lanes", () => {
  const result = classifyPaths(["apps/api/migrations/000060_example.up.sql"]);
  assert.equal(result.risk, "database");
  assertLanes(["apps/api/migrations/000060_example.up.sql"], {
    api: true,
    contracts: false,
    database: true,
    experience: true,
    full: false,
  });
});

test("shared contract changes select every application surface and experience", () => {
  const result = classifyPaths(["packages/contracts/src/foo.ts"]);
  assert.equal(result.risk, "contract");
  assertLanes(["packages/contracts/src/foo.ts"], {
    web: true,
    api: true,
    ai: true,
    contracts: true,
    experience: true,
    component: true,
    database: false,
    full: false,
  });
});

test("generated Postman assets select contract validation without forcing full", () => {
  const paths = [
    "postman/collections/BodySense Public API.postman_collection.json",
    "scripts/postman/generate.mjs",
  ];
  const result = classifyPaths(paths);
  assert.equal(result.risk, "contract");
  assertLanes(paths, {
    web: false,
    api: false,
    ai: false,
    contracts: true,
    experience: false,
    database: false,
    full: false,
  });
});

test("contract verification runs in its own child, not inside run-quality", () => {
  const quality = fs.readFileSync("scripts/delivery/run-quality.mjs", "utf8");
  assert.ok(
    !quality.includes("manifest.lanes.contracts"),
    "run-quality must not execute the contracts lane after the split",
  );
  const contracts = fs.readFileSync(
    "scripts/delivery/run-contracts.mjs",
    "utf8",
  );
  assert.ok(contracts.includes('"contracts:verify"'));
  for (const target of ["lint", "typecheck", "test"]) {
    assert.ok(
      contracts.includes(`nx("@bodysense/contracts", "${target}")`),
      `run-contracts must run the contracts ${target} target`,
    );
  }
  const ci = fs.readFileSync(".github/workflows/ci.yml", "utf8");
  assert.ok(ci.includes("contracts-child:"));
  assert.ok(ci.includes("run-contracts.mjs"));
  assert.ok(ci.includes("component-child:"));
  assert.ok(ci.includes("@bodysense/web:component-test"));
});

test("every selected delivery quality lane runs architecture boundaries", () => {
  const contents = fs.readFileSync("scripts/delivery/run-quality.mjs", "utf8");
  assert.match(
    contents,
    /run\(["']pnpm["'], \[["']quality:architecture["']\]\)/,
  );
});

test("CI governance validates the committed manifest base-to-head diff", () => {
  const contents = fs.readFileSync(".github/workflows/ci.yml", "utf8");
  assert.match(contents, /governance-child:[\s\S]*fetch-depth: 0/);
  assert.match(
    contents,
    /base_sha="\$\(jq -r \.baseSha "\$DELIVERY_MANIFEST_PATH"\)"/,
  );
  assert.match(
    contents,
    /head_sha="\$\(jq -r \.headSha "\$DELIVERY_MANIFEST_PATH"\)"/,
  );
  assert.match(
    contents,
    /git diff --check "\$\{base_sha\}\.\.\.\$\{head_sha\}"/,
  );
});

test("PR commit lint keeps merge type tied to real Git merge topology", () => {
  const workflow = fs.readFileSync(".github/workflows/ci.yml", "utf8");
  const guard = fs.readFileSync("scripts/quality/lint-commit-range.sh", "utf8");
  assert.match(workflow, /bash scripts\/quality\/lint-commit-range\.sh/);
  assert.match(guard, /subject.*merge:\*/s);
  assert.match(guard, /parent_count.*-le 1/s);
  assert.match(
    guard,
    /pnpm exec commitlint --from "\$base_sha" --to "\$head_sha"/,
  );
});

test("CI, Docker, release and unknown paths fail safe to full", () => {
  for (const path of [
    ".github/workflows/ci.yml",
    "docker/docker-compose.prod.yml",
    "release-please-config.json",
    "some-new-root-file.toml",
  ]) {
    const result = classifyPaths([path]);
    assert.equal(result.lanes.full, true, path);
    assert.ok(Object.values(result.lanes).every(Boolean), path);
  }
});

test("mixed docs and executable paths use the highest risk", () => {
  const result = classifyPaths(["docs/foo.md", "apps/api/internal/foo.go"]);
  assert.equal(result.risk, "api");
  assert.equal(result.lanes.api, true);
});

test("full-main policy enables every lane even for docs-only diff", () => {
  const result = classifyPaths(["docs/foo.md"], { forceFull: true });
  assert.equal(result.lanes.full, true);
  assert.ok(Object.values(result.lanes).every(Boolean));
  assert.deepEqual(result.reasons, ["full-main-policy"]);
});

test("manifest digest is deterministic across generatedAt timestamps", () => {
  const first = manifestFor(["apps/web/src/foo.ts"], {
    generatedAt: "2026-08-29T00:00:00.000Z",
  });
  const second = manifestFor(["apps/web/src/foo.ts"], {
    generatedAt: "2026-08-29T00:00:01.000Z",
  });
  assert.equal(first.digest, second.digest);
  assert.equal(first.digest, computeManifestDigest(first));
});

test("manifest validation rejects digest tampering and malformed lane contracts", () => {
  const manifest = manifestFor(["apps/api/internal/foo.go"]);
  assert.deepEqual(validateManifest(manifest), []);

  const tampered = structuredClone(manifest);
  tampered.risk = "docs";
  assert.ok(
    validateManifest(tampered).some((error) =>
      error.includes("digest mismatch"),
    ),
  );

  const malformed = structuredClone(manifest);
  delete malformed.lanes.api;
  malformed.digest = computeManifestDigest(malformed);
  assert.ok(
    validateManifest(malformed).some((error) =>
      error.includes("lanes must contain exactly"),
    ),
  );
});

test("v8 manifest records per-lane required, files, targets, and skip reasons", () => {
  const manifest = manifestFor(["apps/web/src/features/foo.tsx", "README.md"]);
  assert.equal(manifest.schema, "bodysense.delivery-manifest/v8");
  assert.deepEqual(validateManifest(manifest), []);

  const web = manifest.laneDetails.web;
  assert.equal(web.required, true);
  assert.deepEqual(web.files, ["apps/web/src/features/foo.tsx"]);
  assert.equal(web.skipReason, null);
  assert.ok(web.targets.includes("nx:@bodysense/web:test"));

  const governance = manifest.laneDetails.governance;
  assert.equal(governance.required, true);
  assert.deepEqual(governance.files, [
    "README.md",
    "apps/web/src/features/foo.tsx",
  ]);

  const api = manifest.laneDetails.api;
  assert.equal(api.required, false);
  assert.deepEqual(api.files, []);
  assert.equal(api.skipReason, "no-matching-paths");

  const component = manifest.laneDetails.component;
  assert.equal(component.required, true);
  assert.deepEqual(component.files, ["apps/web/src/features/foo.tsx"]);
  assert.equal(component.skipReason, null);
  assert.ok(component.targets.includes("nx:@bodysense/web:component-test"));
});

test("v2 manifests remain valid under the v2 lane set and target policy", () => {
  const current = manifestFor(["apps/web/src/features/foo.tsx"]);
  const { component: _componentLane, ...lanesV2 } = current.lanes;
  const { component: _componentDetail, ...detailsV2 } = current.laneDetails;
  for (const lane of [
    "governance",
    "web",
    "api",
    "ai",
    "contracts",
    "database",
    "experience",
    "full",
  ]) {
    detailsV2[lane] = {
      ...detailsV2[lane],
      targets: [...LANE_TARGETS_V2[lane]],
    };
  }
  const v2 = {
    ...current,
    schema: MANIFEST_SCHEMA_V2,
    policyVersion: POLICY_VERSION_V1,
    lanes: lanesV2,
    laneDetails: detailsV2,
  };
  v2.digest = computeManifestDigest(v2);
  assert.deepEqual(validateManifest(v2), []);

  // A v2 manifest carrying the v3-only component lane is rejected: lane
  // sets are pinned to the schema version that generated the manifest.
  const widened = structuredClone(v2);
  widened.lanes.component = false;
  widened.digest = computeManifestDigest(widened);
  assert.ok(
    validateManifest(widened).some((error) =>
      error.includes("lanes must contain exactly"),
    ),
  );
});

test("v3 manifests remain valid under the v3 target policy and policy version", () => {
  const current = manifestFor(["apps/web/src/features/foo.tsx"]);
  const detailsV3 = { ...current.laneDetails };
  for (const lane of [
    "governance",
    "web",
    "api",
    "ai",
    "database",
    "experience",
    "full",
  ]) {
    detailsV3[lane] = {
      ...detailsV3[lane],
      targets: [...LANE_TARGETS_V3[lane]],
    };
  }
  const v3 = {
    ...current,
    schema: MANIFEST_SCHEMA_V3,
    policyVersion: POLICY_VERSION_V2,
    laneDetails: detailsV3,
  };
  v3.digest = computeManifestDigest(v3);
  assert.deepEqual(validateManifest(v3), []);

  // A v3 manifest carrying v4 targets is rejected: target policy is
  // pinned to the schema version that generated the manifest.
  const widened = structuredClone(v3);
  widened.laneDetails.web.targets = [...current.laneDetails.web.targets];
  widened.digest = computeManifestDigest(widened);
  assert.ok(
    validateManifest(widened).some((error) =>
      error.includes("targets must equal"),
    ),
  );
});

test("empty diff records no-changed-paths skip reasons", () => {
  const manifest = manifestFor([]);
  assert.deepEqual(validateManifest(manifest), []);
  assert.equal(manifest.laneDetails.governance.required, true);
  assert.deepEqual(manifest.laneDetails.governance.files, []);
  assert.equal(manifest.laneDetails.web.required, false);
  assert.equal(manifest.laneDetails.web.skipReason, "no-changed-paths");
});

test("Main full manifests require every lane with the full change set", () => {
  const manifest = manifestFor(["docs/guide.md"], {
    forceFull: true,
    event: "push",
    ref: "refs/heads/main",
  });
  assert.deepEqual(validateManifest(manifest), []);
  for (const detail of Object.values(manifest.laneDetails)) {
    assert.equal(detail.required, true);
    assert.deepEqual(detail.files, ["docs/guide.md"]);
    assert.equal(detail.skipReason, null);
  }
});

test("v1 manifests remain valid without laneDetails", () => {
  const v3 = manifestFor(["apps/web/src/features/foo.tsx"]);
  const { laneDetails: _dropped, ...rest } = v3;
  const { component: _componentLane, ...lanesV1 } = rest.lanes;
  const v1 = {
    ...rest,
    schema: MANIFEST_SCHEMA_V1,
    policyVersion: POLICY_VERSION_V1,
    lanes: lanesV1,
  };
  v1.digest = computeManifestDigest(v1);
  assert.deepEqual(validateManifest(v1), []);

  const v1WithDetails = { ...v1, laneDetails: v3.laneDetails };
  v1WithDetails.digest = computeManifestDigest(v1WithDetails);
  assert.ok(
    validateManifest(v1WithDetails).some((error) =>
      error.includes("laneDetails requires schema"),
    ),
  );
});

test("tampered laneDetails are rejected even with a recomputed digest", () => {
  const manifest = manifestFor(["apps/web/src/features/foo.tsx"]);
  manifest.laneDetails.web.required = false;
  manifest.laneDetails.web.skipReason = "no-matching-paths";
  manifest.laneDetails.web.files = [];
  // Recomputing the digest after tampering must not launder the contract:
  // structural validation cross-checks laneDetails against the lanes map.
  manifest.digest = computeManifestDigest(manifest);
  const errors = validateManifest(manifest);
  assert.ok(errors.some((error) => error.includes("laneDetails.web.required")));
});

test("manifest oracle fails when a required lane is skipped or missing", () => {
  const manifest = manifestFor(["apps/web/src/features/foo.tsx"]);

  const skipped = evaluateManifestOracle({
    manifest,
    group: "quality",
    detector: "success",
    children: { quality: "skipped" },
  });
  assert.equal(skipped.ok, false);
  assert.ok(skipped.failures.some((failure) => failure.includes("quality")));

  const missing = evaluateManifestOracle({
    manifest,
    group: "quality",
    detector: "success",
    children: {},
  });
  assert.equal(missing.ok, false);
  assert.ok(missing.failures.some((failure) => failure.includes("missing")));

  const notRequired = evaluateManifestOracle({
    manifest: manifestFor(["docs/guide.md"]),
    group: "quality",
    detector: "success",
    children: { quality: "skipped" },
  });
  assert.equal(notRequired.ok, true);
});

test("manifest oracle rejects a tampered manifest instead of trusting it", () => {
  const manifest = manifestFor(["apps/web/src/features/foo.tsx"]);
  manifest.lanes.api = true;
  const result = evaluateManifestOracle({
    manifest,
    group: "quality",
    detector: "success",
    children: { quality: "success" },
  });
  assert.equal(result.ok, false);
  assert.ok(
    result.failures.some((failure) => failure.includes("digest mismatch")),
  );
});

test("oracle group enablement follows the manifest lanes", () => {
  const dbManifest = manifestFor(["apps/api/migrations/0001_init.sql"]);
  assert.deepEqual(oracleEnabledFromManifest(dbManifest, "database"), {
    "database-baseline": true,
    "database-recovery": false,
    "database-integration": true,
  });
  assert.deepEqual(oracleEnabledFromManifest(dbManifest, "experience"), {
    experience: false,
    component: false,
  });
  const contractManifest = manifestFor([
    "postman/collections/BodySense Public API.postman_collection.json",
  ]);
  assert.deepEqual(oracleEnabledFromManifest(contractManifest, "quality"), {
    "quality-web": false,
    "quality-api": false,
    "quality-ai": false,
    "quality-arch": false,
    "quality-release": false,
    contracts: true,
  });
  const webManifest = manifestFor(["apps/web/src/features/foo.tsx"]);
  assert.deepEqual(oracleEnabledFromManifest(webManifest, "quality"), {
    "quality-web": true,
    "quality-api": false,
    "quality-ai": false,
    "quality-arch": true,
    "quality-release": false,
    contracts: false,
  });
  // In Main Full v7, each service lane is still required in parallel with
  // the dedicated, narrower release-safety child.
  const fullManifest = manifestFor(["docs/guide.md"], {
    forceFull: true,
    event: "push",
    ref: "refs/heads/main",
  });
  assert.deepEqual(oracleEnabledFromManifest(fullManifest, "quality"), {
    "quality-web": true,
    "quality-api": true,
    "quality-ai": true,
    "quality-arch": true,
    "quality-release": true,
    contracts: true,
  });
  const docsManifest = manifestFor(["docs/guide.md"]);
  assert.deepEqual(oracleEnabledFromManifest(docsManifest, "database"), {
    "database-baseline": false,
    "database-recovery": false,
    "database-integration": false,
  });
});

test("manifest validation rejects unsupported events and short SHAs", () => {
  const manifest = manifestFor(["docs/foo.md"]);
  manifest.event = "merge_group";
  manifest.headSha = "abc";
  manifest.digest = computeManifestDigest(manifest);
  const errors = validateManifest(manifest);
  assert.ok(errors.some((error) => error.includes("unsupported event")));
  assert.ok(errors.some((error) => error.includes("headSha")));
});

test("schedule manifests validate and force the full lane set", () => {
  // Nightly runs carry no PR diff (base == head); the schedule event plus
  // forceFull must still select every lane and pass validation, so the
  // nightly run is a true full-set safety net rather than an empty pass.
  const manifest = manifestFor([], {
    event: "schedule",
    ref: "refs/heads/main",
    forceFull: true,
  });
  assert.deepEqual(validateManifest(manifest), []);
  for (const [lane, enabled] of Object.entries(manifest.lanes)) {
    assert.equal(
      enabled,
      true,
      "lane " + lane + " must be enabled for nightly",
    );
  }
});

test("Oracle fail-closed matrix", () => {
  const cases = [
    {
      name: "disabled plus skipped passes",
      input: {
        detector: "success",
        enabled: { web: false },
        children: { web: "skipped" },
      },
      ok: true,
    },
    {
      name: "enabled plus success passes",
      input: {
        detector: "success",
        enabled: { web: true },
        children: { web: "success" },
      },
      ok: true,
    },
    {
      name: "enabled plus failure fails",
      input: {
        detector: "success",
        enabled: { web: true },
        children: { web: "failure" },
      },
      ok: false,
    },
    {
      name: "enabled plus skipped fails",
      input: {
        detector: "success",
        enabled: { web: true },
        children: { web: "skipped" },
      },
      ok: false,
    },
    {
      name: "enabled plus cancelled fails",
      input: {
        detector: "success",
        enabled: { web: true },
        children: { web: "cancelled" },
      },
      ok: false,
    },
    {
      name: "enabled plus missing fails",
      input: { detector: "success", enabled: { web: true }, children: {} },
      ok: false,
    },
    {
      name: "detector failure fails",
      input: {
        detector: "failure",
        enabled: { web: false },
        children: { web: "skipped" },
      },
      ok: false,
    },
    {
      name: "disabled unexpected failure fails",
      input: {
        detector: "success",
        enabled: { web: false },
        children: { web: "failure" },
      },
      ok: false,
    },
  ];

  for (const fixture of cases) {
    assert.equal(evaluateOracle(fixture.input).ok, fixture.ok, fixture.name);
  }
});

test("run observation binds manifest digest and reports failing evidence", () => {
  const manifest = manifestFor(["apps/web/src/foo.ts"]);
  const summary = createRunSummary({
    manifest,
    laneResults: {
      governance: "success",
      web: "failure",
      contracts: "success",
    },
    oracleResults: { quality: "failure" },
    generatedAt: "2026-08-29T00:00:00.000Z",
  });
  assert.equal(summary.manifestDigest, manifest.digest);
  assert.deepEqual(summary.failedLanes, ["web"]);
  assert.deepEqual(summary.failedOracles, ["quality"]);
  assert.match(summary.digest, /^sha256:[0-9a-f]{64}$/);
});

test("candidate manifest binds all four image digests to one exact revision", () => {
  const gitSha = "c".repeat(40);
  const tag = `sha-${gitSha}`;
  const digest = (char) => `sha256:${char.repeat(64)}`;
  const candidate = createCandidate({
    gitSha,
    ciRunId: "12345",
    deliveryManifestDigest: digest("d"),
    migrationHead: 59,
    images: {
      web: {
        repository: "registry/bodysense-web",
        tag,
        digest: digest("1"),
        revision: gitSha,
      },
      api: {
        repository: "registry/bodysense-api",
        tag,
        digest: digest("2"),
        revision: gitSha,
      },
      aiService: {
        repository: "registry/bodysense-ai-service",
        tag,
        digest: digest("3"),
        revision: gitSha,
      },
      runtime: {
        repository: "registry/bodysense-runtime",
        tag,
        digest: digest("4"),
        revision: gitSha,
      },
    },
    staticAssets: {
      enabled: true,
      revision: gitSha,
      assetBase: `https://assets.example/web/${gitSha}/`,
      atlasCatalogUrl:
        "https://assets.example/anatomy/vanatome/1.4.0/releases/1.4.0/catalog.json",
      atlasVersion: "1.4.0",
    },
    generatedAt: "2026-08-29T00:00:00.000Z",
  });
  assert.deepEqual(validateCandidate(candidate), []);
  assert.equal(candidate.candidateTag, tag);
  assert.match(candidate.digest, /^sha256:[0-9a-f]{64}$/);
});

test("candidate manifest fails closed on mixed revisions, tags, or tampered digests", () => {
  const gitSha = "c".repeat(40);
  const tag = `sha-${gitSha}`;
  const digest = (char) => `sha256:${char.repeat(64)}`;
  const candidate = createCandidate({
    gitSha,
    ciRunId: "12345",
    deliveryManifestDigest: digest("d"),
    migrationHead: 59,
    images: {
      web: {
        repository: "registry/bodysense-web",
        tag,
        digest: digest("1"),
        revision: gitSha,
      },
      api: {
        repository: "registry/bodysense-api",
        tag,
        digest: digest("2"),
        revision: gitSha,
      },
      aiService: {
        repository: "registry/bodysense-ai-service",
        tag,
        digest: digest("3"),
        revision: gitSha,
      },
      runtime: {
        repository: "registry/bodysense-runtime",
        tag,
        digest: digest("4"),
        revision: gitSha,
      },
    },
    staticAssets: {
      enabled: false,
      revision: gitSha,
      assetBase: "",
      atlasCatalogUrl: "",
      atlasVersion: "1.4.0",
    },
  });
  candidate.images.api.revision = "e".repeat(40);
  candidate.images.runtime.tag = "staging-latest";
  candidate.images.web.digest = "not-a-digest";
  const errors = validateCandidate(candidate);
  assert.ok(errors.some((error) => error.includes("api.revision")));
  assert.ok(errors.some((error) => error.includes("runtime.tag")));
  assert.ok(errors.some((error) => error.includes("web.digest")));
  assert.ok(
    errors.some((error) => error.includes("candidate digest mismatch")),
  );
});

function runStagingWatcherCheck({ apiRevision, pullFailures = 0 } = {}) {
  const root = fs.mkdtempSync(
    path.join(os.tmpdir(), "bodysense-staging-watch-"),
  );
  const bin = path.join(root, "bin");
  const state = path.join(root, "state");
  const runtime = path.join(root, "runtime");
  const config = path.join(root, "staging-channel.env");
  const secret = path.join(root, "staging.env");
  fs.mkdirSync(bin, { recursive: true });
  fs.writeFileSync(
    secret,
    "DB_PASSWORD=test\nREDIS_PASSWORD=test\nJWT_SECRET_KEY=test\nLITELLM_MASTER_KEY=test\nGROQ_API_KEY=test\n",
  );
  fs.writeFileSync(
    config,
    `STAGING_REGISTRY=registry.example\nSTAGING_NAMESPACE=bodysense\nSTAGING_CHANNEL_TAG=staging-latest\nSTAGING_SECRET_ENV=${secret}\n`,
  );
  const revision = "f".repeat(40);
  const pullCounter = path.join(root, "pull-count");
  fs.writeFileSync(
    path.join(bin, "docker"),
    `#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"$1\" == pull ]]; then\n  count=0\n  [[ -f \"$STAGING_TEST_PULL_COUNTER\" ]] && count=\"$(cat \"$STAGING_TEST_PULL_COUNTER\")\"\n  count=$((count + 1))\n  printf '%s\\n' \"$count\" > \"$STAGING_TEST_PULL_COUNTER\"\n  if (( count <= STAGING_TEST_PULL_FAILURES )); then\n    echo 'simulated registry reset' >&2\n    exit 75\n  fi\n  exit 0\nfi\nif [[ \"$1 $2\" == 'image inspect' ]]; then\n  ref=\"$3\"\n  if [[ \"$ref\" == *bodysense-api* ]]; then printf '%s\\n' \"${apiRevision ?? revision}\"; else printf '%s\\n' '${revision}'; fi\n  exit 0\nfi\necho \"unexpected fake docker args: $*\" >&2\nexit 99\n`,
    { mode: 0o755 },
  );
  const result = spawnSync(
    "bash",
    ["scripts/staging-deploy-watch.sh", "--check-only"],
    {
      cwd: process.cwd(),
      encoding: "utf8",
      env: {
        ...process.env,
        PATH: `${bin}:${process.env.PATH}`,
        BODYSENSE_STAGING_CHANNEL_CONFIG: config,
        BODYSENSE_STAGING_STATE_DIR: state,
        BODYSENSE_STAGING_RUNTIME_ROOT: runtime,
        STAGING_REGISTRY_RETRY_ATTEMPTS: "4",
        STAGING_REGISTRY_RETRY_DELAY_SECONDS: "0",
        STAGING_TEST_PULL_COUNTER: pullCounter,
        STAGING_TEST_PULL_FAILURES: String(pullFailures),
      },
    },
  );
  const pullAttempts = Number(fs.readFileSync(pullCounter, "utf8").trim());
  fs.rmSync(root, { recursive: true, force: true });
  return { ...result, revision, pullAttempts };
}

test("staging watcher check-only accepts one coherent four-image revision", () => {
  const result = runStagingWatcherCheck();
  assert.equal(result.status, 0, result.stderr);
  assert.match(
    result.stdout,
    new RegExp(`STAGING_CANDIDATE=COHERENT revision=${result.revision}`),
  );
});

test("staging watcher retries transient registry pulls before evaluating coherence", () => {
  const result = runStagingWatcherCheck({ pullFailures: 2 });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.pullAttempts, 6);
  assert.match(
    result.stderr,
    /registry operation failed; retrying operation=pull /,
  );
  assert.match(
    result.stdout,
    new RegExp(`STAGING_CANDIDATE=COHERENT revision=${result.revision}`),
  );
});

test("staging watcher check-only refuses to treat split pointers as deployable", () => {
  const result = runStagingWatcherCheck({ apiRevision: "e".repeat(40) });
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /staging channel not coherent yet/);
  assert.doesNotMatch(result.stdout, /STAGING_CANDIDATE=COHERENT/);
});

test("staging watcher fails closed when an image revision label is missing", () => {
  const result = runStagingWatcherCheck({ apiRevision: "" });
  assert.notEqual(result.status, 0);
  assert.match(
    result.stderr,
    /api staging image has no org.opencontainers.image.revision/,
  );
});

test("staging watcher deploys document-service as part of the coherent application revision", () => {
  const contents = fs.readFileSync("scripts/staging-deploy-watch.sh", "utf8");
  assert.match(
    contents,
    /current_document=\$\(container_revision document-service\)/,
  );
  assert.match(contents, /compose up -d --no-deps --no-build document-service/);
  assert.match(contents, /wait_healthy document-service 180/);
  assert.match(
    contents,
    /assert_container_revision document-service \"\$desired_revision\"/,
  );
});

test("staging watcher force-recreates LiteLLM after swapping the runtime directory", () => {
  const contents = fs.readFileSync("scripts/staging-deploy-watch.sh", "utf8");
  assert.match(contents, /compose up -d --no-build postgres redis/);
  assert.match(
    contents,
    /compose up -d --no-deps --no-build --force-recreate litellm-gateway/,
  );
  assert.doesNotMatch(
    contents,
    /compose up -d --no-build postgres redis litellm-gateway/,
  );
});

test("production watcher deploys, verifies, and conditionally rolls back document-service", () => {
  const contents = fs.readFileSync(
    "scripts/production-deploy-watch.sh",
    "utf8",
  );
  assert.match(
    contents,
    /current_document=\$\(container_revision document-service\)/,
  );
  assert.match(contents, /deploy_document_service\(\)/);
  assert.match(contents, /compose up -d --no-deps document-service/);
  assert.match(contents, /wait_healthy document-service 120/);
  assert.match(
    contents,
    /assert_container_revision document-service \"\$desired_revision\"/,
  );
  assert.match(contents, /if compose_service_exists document-service; then/);
});

test("production deploy and rollback force-recreate LiteLLM for runtime config changes", () => {
  const contents = fs.readFileSync(
    "scripts/production-deploy-watch.sh",
    "utf8",
  );
  const matches =
    contents.match(
      /compose up -d --no-deps --force-recreate litellm-gateway/g,
    ) ?? [];
  assert.ok(matches.length >= 2);
});

test("production watcher hands a changed runtime contract to the target deployer before backup or service mutation", () => {
  const contents = fs.readFileSync(
    "scripts/production-deploy-watch.sh",
    "utf8",
  );
  assert.match(contents, /^DEPLOY_WATCH_HANDOFF_PROTOCOL=1$/m);
  assert.match(
    contents,
    /runtime watcher handoff is missing the inherited deployment lock/,
  );
  assert.match(contents, /changed without compatible handoff protocol 1/);
  const execEnv = contents.indexOf("exec env");
  const handoffProtocolEnv = contents.indexOf(
    'BODYSENSE_DEPLOY_HANDOFF_PROTOCOL="$DEPLOY_WATCH_HANDOFF_PROTOCOL"',
    execEnv,
  );
  assert.ok(
    execEnv >= 0 && handoffProtocolEnv > execEnv,
    "handoff must exec target watcher with protocol token",
  );
  const handoffCall = contents.indexOf(
    'handoff_to_runtime_watcher_if_needed "$desired_revision"',
  );
  const backup = contents.indexOf('backup="$BACKUP_DIR/bodysense-pre-');
  const sync = contents.indexOf('sync_runtime "$desired_revision"');
  assert.ok(handoffCall >= 0, "runtime watcher handoff call must exist");
  assert.ok(
    backup > handoffCall,
    "handoff must happen before the database backup",
  );
  assert.ok(
    sync > backup,
    "managed runtime synchronization must remain inside the target-owned transaction",
  );
});

test("staging health-document qualification validator is fail-safe and restores Champion", () => {
  const contents = fs.readFileSync(
    "scripts/validate-health-document-staging-qualification.sh",
    "utf8",
  );
  const runtimeDockerfile = fs.readFileSync(
    "docker/Dockerfile.runtime",
    "utf8",
  );
  const stagingWatcher = fs.readFileSync(
    "scripts/staging-deploy-watch.sh",
    "utf8",
  );
  assert.match(
    runtimeDockerfile,
    /validate-health-document-staging-qualification\.sh \/runtime\/staging\/scripts\/validate-health-document-staging-qualification\.sh/,
  );
  assert.match(
    stagingWatcher,
    /scripts\/validate-health-document-staging-qualification\.sh/,
  );
  assert.match(
    contents,
    /label=com\.docker\.compose\.service=document-service/,
  );
  assert.match(contents, /document-service revision mismatch/);
  assert.match(contents, /trap cleanup EXIT INT TERM/);
  assert.match(contents, /HEALTH_DOCUMENT_STAGE=qualification/);
  assert.match(contents, /HEALTH_DOCUMENT_STAGE=champion/);
  assert.match(contents, /document_extraction_runs/);
  assert.match(contents, /HEALTH_DOCUMENT_STAGING_QUALIFICATION=PASS/);
  assert.match(contents, /-X DELETE/);
  assert.match(contents, /api\/v1\/uploads\/\$upload_id/);
  assert.match(contents, /DELETE FROM users WHERE email/);
});

test("release file contract recognizes release-please merge/squash identity and rejects normal commits", () => {
  const changelog =
    "# Changelog\n\n## [0.9.0](https://example/compare) (2026-08-29)\n\n### Features\n\n* delivery v3\n\n## [0.8.1](https://example/old)\n";
  const valid = validateReleaseFiles({
    packageVersion: "0.9.0",
    manifestVersion: "0.9.0",
    changelog,
    subjects: ["Merge pull request #140", "chore(main): release 0.9.0"],
  });
  assert.deepEqual(valid.errors, []);
  assert.equal(valid.releaseShaped, true);

  const normal = validateReleaseFiles({
    packageVersion: "0.9.0",
    manifestVersion: "0.9.0",
    changelog,
    subjects: ["feat(ops): later normal change"],
  });
  assert.deepEqual(normal.errors, []);
  assert.equal(normal.releaseShaped, false);
});

test("release file contract fails on version drift and extracts only the requested notes section", () => {
  const changelog =
    "# Changelog\n\n## [0.9.0](https://example/new)\n\nnew notes\n\n## [0.8.1](https://example/old)\n\nold notes\n";
  const drift = validateReleaseFiles({
    packageVersion: "0.9.0",
    manifestVersion: "0.8.1",
    changelog,
    subjects: ["chore(main): release 0.9.0"],
  });
  assert.ok(drift.errors.some((error) => error.includes("manifest version")));
  const notes = extractReleaseNotes(changelog, "0.9.0");
  assert.match(notes, /new notes/);
  assert.doesNotMatch(notes, /old notes/);
});

test("release manifest promotes candidate digests without changing artifact identity", () => {
  const gitSha = "c".repeat(40);
  const tag = `sha-${gitSha}`;
  const digest = (char) => `sha256:${char.repeat(64)}`;
  const candidate = createCandidate({
    gitSha,
    ciRunId: "12345",
    deliveryManifestDigest: digest("d"),
    migrationHead: 59,
    images: {
      web: {
        repository: "registry/bodysense-web",
        tag,
        digest: digest("1"),
        revision: gitSha,
      },
      api: {
        repository: "registry/bodysense-api",
        tag,
        digest: digest("2"),
        revision: gitSha,
      },
      aiService: {
        repository: "registry/bodysense-ai-service",
        tag,
        digest: digest("3"),
        revision: gitSha,
      },
      runtime: {
        repository: "registry/bodysense-runtime",
        tag,
        digest: digest("4"),
        revision: gitSha,
      },
    },
    staticAssets: {
      enabled: true,
      revision: gitSha,
      assetBase: `https://assets.example/web/${gitSha}/`,
      atlasCatalogUrl:
        "https://assets.example/anatomy/vanatome/1.4.0/releases/1.4.0/catalog.json",
      atlasVersion: "1.4.0",
    },
  });
  const release = createReleaseManifest({
    candidate,
    version: "0.9.0",
    tag: "v0.9.0",
    candidateRunId: "6789",
  });
  assert.deepEqual(validateReleaseManifest(release), []);
  assert.equal(release.images.web.digest, candidate.images.web.digest);
  assert.equal(release.images.api.digest, candidate.images.api.digest);
  assert.equal(release.candidateManifestDigest, candidate.digest);
});

test("release manifest fails closed if a promoted component digest/revision is tampered", () => {
  const gitSha = "c".repeat(40);
  const tag = `sha-${gitSha}`;
  const digest = (char) => `sha256:${char.repeat(64)}`;
  const candidate = createCandidate({
    gitSha,
    ciRunId: "12345",
    deliveryManifestDigest: digest("d"),
    migrationHead: 59,
    images: {
      web: {
        repository: "registry/bodysense-web",
        tag,
        digest: digest("1"),
        revision: gitSha,
      },
      api: {
        repository: "registry/bodysense-api",
        tag,
        digest: digest("2"),
        revision: gitSha,
      },
      aiService: {
        repository: "registry/bodysense-ai-service",
        tag,
        digest: digest("3"),
        revision: gitSha,
      },
      runtime: {
        repository: "registry/bodysense-runtime",
        tag,
        digest: digest("4"),
        revision: gitSha,
      },
    },
    staticAssets: {
      enabled: false,
      revision: gitSha,
      assetBase: "",
      atlasCatalogUrl: "",
      atlasVersion: "1.4.0",
    },
  });
  const release = createReleaseManifest({
    candidate,
    version: "0.9.0",
    tag: "v0.9.0",
    candidateRunId: "6789",
  });
  release.images.aiService.revision = "a".repeat(40);
  release.images.api.digest = "bad";
  const errors = validateReleaseManifest(release);
  assert.ok(errors.some((error) => error.includes("aiService.revision")));
  assert.ok(errors.some((error) => error.includes("api.digest")));
  assert.ok(errors.some((error) => error.includes("release digest mismatch")));
});

test("Release Publish reconciles release-please tagged state only after publication and supports retries", () => {
  const contents = fs.readFileSync(
    ".github/workflows/release-publish.yml",
    "utf8",
  );
  assert.match(contents, /pull-requests: write/);
  assert.match(contents, /issues: write/);
  assert.match(contents, /reconcile-release-please-state:/);
  assert.match(
    contents,
    /needs\.prepare-draft\.outputs\.already_published == 'true'/,
  );
  assert.match(contents, /needs\.finalize\.result == 'success'/);
  assert.match(contents, /\| jq -r --arg title/);
  assert.match(contents, /gh release view \"\$RELEASE_TAG\".*--json isDraft/);
  assert.match(contents, /\[\[ \"\$tag_sha\" == \"\$RELEASE_SHA\" \]\]/);
  assert.match(contents, /autorelease%3A%20pending/);
  assert.match(contents, /labels\[\]=autorelease: tagged/);
});

test("local deploy keeps production-shaped runtime env after repository quality validation", () => {
  const contents = fs.readFileSync("scripts/local-deploy-validate.sh", "utf8");
  const qualityGate = contents.indexOf("bash scripts/validate-repo.sh");
  const diagnosisEnv = contents.indexOf(
    "export DIAGNOSIS_CHAMPION_CONFIGURATION_ID=",
  );
  const dbPassword = contents.indexOf("export DB_PASSWORD=");
  const e2eStub = contents.indexOf("export BODYSENSE_E2E_STUB_AI=");
  assert.ok(qualityGate >= 0);
  assert.ok(diagnosisEnv > qualityGate);
  assert.ok(dbPassword > qualityGate);
  assert.ok(e2eStub > qualityGate);
});

test("non-AI candidate Dockerfiles apply revision metadata after filesystem layers", () => {
  for (const dockerfile of [
    "apps/api/Dockerfile",
    "docker/Dockerfile.web",
    "docker/Dockerfile.runtime",
  ]) {
    const contents = fs.readFileSync(dockerfile, "utf8");
    const label = contents.lastIndexOf("LABEL org.opencontainers.image.title=");
    const lastCopy = contents.lastIndexOf("\nCOPY ");
    const lastRun = contents.lastIndexOf("\nRUN ");
    assert.ok(
      label > Math.max(lastCopy, lastRun),
      `${dockerfile} revision label must follow filesystem mutations`,
    );
  }
});

test("API release image strips Go debug and symbol metadata from every shipped binary", () => {
  const contents = fs.readFileSync("apps/api/Dockerfile", "utf8");
  const outputs = [
    "/server",
    "/production-dr-manager",
    "/domain-validator",
    "/upload-storage-migrator",
    "/knowledge-publication-manager",
    "/knowledge-source-manager",
  ];
  for (const output of outputs) {
    const pattern = new RegExp(
      `go build -trimpath -ldflags="-s -w" -o ${output.replaceAll("/", "\\/")} `,
    );
    assert.match(
      contents,
      pattern,
      `${output} must use stripped release linker flags`,
    );
  }
});

test("AI Dockerfile keeps heavy runtime identity independent from Git release metadata", () => {
  const contents = fs.readFileSync("apps/ai-service/Dockerfile", "utf8");
  const recipe = extractAiRuntimeBaseRecipe(contents);
  const recipeEnd = contents.indexOf("# BODYSENSE_AI_RUNTIME_BASE_END");
  const applicationCopy = contents.lastIndexOf("\nCOPY --link . .");

  assert.match(contents, /^ARG AI_RUNTIME_BASE=runtime-base/m);
  assert.match(contents, /FROM \${AI_RUNTIME_BASE} AS application/);
  assert.doesNotMatch(contents, /^ARG (BUILD_DATE|VCS_REF)$/m);
  assert.doesNotMatch(
    contents,
    /org\.opencontainers\.image\.(created|revision)=/,
  );
  assert.match(recipe, /python:3\.13\.15-slim@sha256:[0-9a-f]{64}/);
  assert.match(recipe, /ghcr\.io\/astral-sh\/uv:latest@sha256:[0-9a-f]{64}/);
  assert.match(recipe, /uv sync --frozen --no-dev/);
  assert.doesNotMatch(recipe, /--no-cache/);
  assert.match(recipe, /tesseract-ocr=5\.5\.0-1\+b1/);
  assert.ok(
    applicationCopy > recipeEnd,
    "application source must be copied only after runtime-base boundary",
  );
  assert.doesNotMatch(recipe, /COPY(?: --link)? \. \./);
  assert.match(contents.slice(recipeEnd), /COPY --link \. \./);
  assert.match(
    fs.readFileSync("apps/ai-service/.dockerignore", "utf8"),
    /^models\/$/m,
  );
});

test("AI runtime-base identity is content-derived from runtime inputs only", () => {
  const root = fs.mkdtempSync(
    path.join(os.tmpdir(), "bodysense-ai-runtime-base-"),
  );
  try {
    const inputs = ["apps/ai-service/Dockerfile", ...AI_RUNTIME_BASE_INPUTS];
    for (const relativePath of inputs) {
      const destination = path.join(root, relativePath);
      fs.mkdirSync(path.dirname(destination), { recursive: true });
      fs.copyFileSync(relativePath, destination);
    }

    const first = createAiRuntimeBaseIdentity(root);
    fs.writeFileSync(
      path.join(root, "CHANGELOG.md"),
      "release-only metadata\n",
    );
    const releaseOnly = createAiRuntimeBaseIdentity(root);
    assert.equal(releaseOnly.tag, first.tag);
    assert.match(first.tag, /^runtime-base-[0-9a-f]{64}$/);

    fs.appendFileSync(
      path.join(root, "apps/ai-service/uv.lock"),
      "\n# dependency identity mutation\n",
    );
    const dependencyChange = createAiRuntimeBaseIdentity(root);
    assert.notEqual(dependencyChange.tag, first.tag);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

test("GitHub candidate API build uses the public Go module proxy explicitly", () => {
  const workflow = fs.readFileSync(
    ".github/workflows/candidate-publish.yml",
    "utf8",
  );
  const apiStart = workflow.indexOf("- component: api");
  const runtimeStart = workflow.indexOf("- component: runtime", apiStart);
  const apiMatrix = workflow.slice(apiStart, runtimeStart);
  assert.ok(apiStart >= 0 && runtimeStart > apiStart);
  assert.match(
    apiMatrix,
    /extra_build_args: GOPROXY=https:\/\/proxy\.golang\.org,direct/,
  );
  assert.match(workflow, /\$\{\{ matrix\.extra_build_args \}\}/);
});

test("candidate publishing verifies remote OCI identity without pulling heavy image layers", () => {
  const workflow = fs.readFileSync(
    ".github/workflows/candidate-publish.yml",
    "utf8",
  );
  const identityStart = workflow.indexOf(
    "name: Verify immutable candidate digest and OCI revision label",
  );
  const identityEnd = workflow.indexOf(
    "name: Record component evidence",
    identityStart,
  );
  const identity = workflow.slice(identityStart, identityEnd);
  assert.ok(identityStart >= 0 && identityEnd > identityStart);
  assert.match(
    identity,
    /docker buildx imagetools inspect --format '\{\{ index \.Image\.Config\.Labels "org\.opencontainers\.image\.revision" \}\}'/,
  );
  assert.doesNotMatch(identity, /docker pull "\$ref"/);
  assert.match(
    workflow,
    /timeout-minutes: \$\{\{ matrix\.timeout_minutes \}\}/,
  );
});

test("AI candidate uses a content-addressed runtime base and workflow-injected release metadata", () => {
  const workflow = fs.readFileSync(
    ".github/workflows/candidate-publish.yml",
    "utf8",
  );
  const baseStart = workflow.indexOf("  ai-runtime-base:");
  const genericBuildStart = workflow.indexOf("  build:", baseStart);
  const aiStart = workflow.indexOf("  build-ai:", genericBuildStart);
  const verifyStart = workflow.indexOf("  verify-static-coherence:", aiStart);
  assert.ok(
    baseStart >= 0 &&
      genericBuildStart > baseStart &&
      aiStart > genericBuildStart,
  );

  const baseJob = workflow.slice(baseStart, genericBuildStart);
  const genericBuild = workflow.slice(genericBuildStart, aiStart);
  const aiJob = workflow.slice(aiStart, verifyStart);

  assert.match(baseJob, /target: runtime-base/);
  assert.match(
    baseJob,
    /group: ai-runtime-base-\$\{\{ needs\.resolve\.outputs\.ai_runtime_base_tag \}\}/,
  );
  assert.match(baseJob, /cancel-in-progress: false/);
  assert.match(baseJob, /UV_INDEX_URL=https:\/\/pypi\.org\/simple\//);
  assert.match(
    baseJob,
    /Reuse immutable AI runtime base when dependency identity already exists/,
  );
  assert.match(
    baseJob,
    /io\.bodysense\.runtime-base\.identity=\$\{\{ env\.TAG \}\}/,
  );
  assert.doesNotMatch(genericBuild, /component: aiService/);

  assert.match(aiJob, /needs: \[resolve, ai-runtime-base\]/);
  assert.doesNotMatch(aiJob, /static-assets/);
  assert.match(
    aiJob,
    /AI_RUNTIME_BASE=\$\{\{ needs\.ai-runtime-base\.outputs\.ref \}\}/,
  );
  assert.match(
    aiJob,
    /org\.opencontainers\.image\.created=\$\{\{ needs\.resolve\.outputs\.build_date \}\}/,
  );
  assert.match(
    aiJob,
    /org\.opencontainers\.image\.revision=\$\{\{ env\.REVISION \}\}/,
  );
  assert.match(
    aiJob,
    /io\.bodysense\.runtime-base\.digest=\$\{\{ needs\.ai-runtime-base\.outputs\.digest \}\}/,
  );
  assert.match(aiJob, /docker buildx imagetools inspect --format/);
  assert.doesNotMatch(aiJob, /docker pull "\$ref"/);
  assert.match(aiJob, /timeout-minutes: 20/);
  assert.match(
    workflow,
    /needs: \[resolve, static-assets, build, build-ai, verify-static-coherence\]/,
  );
});

test("candidate staging coherence verifies remote digest and revision without heavy image pulls", () => {
  const workflow = fs.readFileSync(
    ".github/workflows/candidate-publish.yml",
    "utf8",
  );
  const promoteStart = workflow.indexOf(
    "name: Promote all exact-SHA artifacts to staging-latest",
  );
  const start = workflow.indexOf(
    "name: Verify staging channel converged to exact candidate manifests",
  );
  const promoteBlock = workflow.slice(promoteStart, start);
  const block = workflow.slice(start);
  assert.ok(promoteStart >= 0 && start > promoteStart);
  assert.match(
    promoteBlock,
    /retry_registry docker buildx imagetools create --prefer-index=false/,
  );
  assert.match(block, /src_digest=/);
  assert.match(block, /dst_digest=/);
  assert.match(block, /retry_registry docker buildx imagetools inspect/);
  assert.match(
    block,
    /retry_registry docker buildx imagetools inspect --format/,
  );
  assert.match(block, /max_attempts=5/);
  assert.doesNotMatch(block, /docker pull "\$ref"/);
  assert.doesNotMatch(block, /docker image inspect/);
});

test("all registry channel/release promotions carbon-copy single-platform manifests", () => {
  for (const workflow of [
    ".github/workflows/candidate-publish.yml",
    ".github/workflows/release-publish.yml",
    ".github/workflows/deploy-production.yml",
  ]) {
    const contents = fs.readFileSync(workflow, "utf8");
    const promotionLines = contents
      .split(/\r?\n/)
      .filter(
        (line) =>
          line.includes("docker buildx imagetools create") &&
          line.includes("--tag"),
      );
    assert.ok(
      promotionLines.length > 0,
      `${workflow} must contain a registry promotion`,
    );
    for (const line of promotionLines) {
      assert.match(line, /--prefer-index=false/, `${workflow}: ${line.trim()}`);
    }
  }
});

test("stacked pull requests receive CI without broadening main candidate promotion", () => {
  const ci = fs.readFileSync(".github/workflows/ci.yml", "utf8");
  const candidate = fs.readFileSync(
    ".github/workflows/candidate-publish.yml",
    "utf8",
  );
  assert.match(ci, /on:\n  push:\n    branches: \[main\]/);
  assert.match(ci, /  pull_request:\n/);
  assert.doesNotMatch(ci, /  pull_request:\n    branches:/);
  assert.match(
    ci,
    /DELIVERY_BASE_SHA: \$\{\{ github\.event\.pull_request\.base\.sha \|\| github\.event\.before \|\| github\.sha \}\}/,
  );
  assert.match(
    ci,
    /DELIVERY_HEAD_SHA: \$\{\{ github\.event\.pull_request\.head\.sha \|\| github\.sha \}\}/,
  );
  assert.match(ci, /permissions:\n  contents: read\n  actions: read/);
  assert.match(ci, /if: github\.event_name == 'pull_request'/);
  assert.match(candidate, /    workflows: \[CI\]/);
  assert.match(candidate, /    branches: \[main\]/);
  assert.match(candidate, /github\.event\.workflow_run\.event == 'push'/);
  assert.match(candidate, /github\.event\.workflow_run\.head_branch == 'main'/);
  assert.match(
    candidate,
    /needs\.staging-readiness\.outputs\.eligible == 'true'/,
  );
});

test("stacked PR manifest resolves moving non-main base without misclassifying parent changes", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "bodysense-int002-"));
  const env = {
    ...process.env,
    GIT_AUTHOR_NAME: "CI Test",
    GIT_COMMITTER_NAME: "CI Test",
    GIT_AUTHOR_EMAIL: "ci-test@example.invalid",
    GIT_COMMITTER_EMAIL: "ci-test@example.invalid",
    GITHUB_OUTPUT: "",
  };
  const runGit = (...args) => {
    const result = spawnSync("git", args, { cwd: root, encoding: "utf8", env });
    assert.equal(result.status, 0, [args.join(" "), result.stderr].join("\n"));
    return result.stdout.trim();
  };
  const commitFile = (file, value) => {
    const full = path.join(root, file);
    fs.mkdirSync(path.dirname(full), { recursive: true });
    fs.writeFileSync(full, value);
    runGit("add", file);
    runGit("commit", "-qm", "test: update staged history");
    return runGit("rev-parse", "HEAD");
  };
  try {
    runGit("init", "-q", "-b", "main");
    commitFile("README.md", "root\n");
    runGit("checkout", "-qb", "feat/parent");
    commitFile("apps/web/src/parent.ts", "export const parent = true;\n");
    runGit("checkout", "-qb", "feat/child");
    const childHead = commitFile("docs/new-feature.md", "# child\n");
    runGit("checkout", "-q", "feat/parent");
    const advancedParent = commitFile(
      "apps/api/internal/model/new.go",
      "package model\n",
    );

    const output = path.join(root, "manifest.json");
    const generated = spawnSync(
      "node",
      [
        path.resolve("scripts/delivery/generate-manifest.mjs"),
        "--event",
        "pull_request",
        "--ref",
        "refs/pull/2468/merge",
        "--base",
        advancedParent,
        "--head",
        childHead,
        "--output",
        output,
      ],
      { cwd: root, encoding: "utf8", env },
    );
    assert.equal(generated.status, 0, generated.stderr);
    const manifest = JSON.parse(fs.readFileSync(output, "utf8"));
    assert.equal(manifest.event, "pull_request");
    assert.equal(manifest.requestedBaseSha, advancedParent);
    assert.notEqual(manifest.baseSha, advancedParent);
    assert.equal(manifest.headSha, childHead);
    assert.deepEqual(manifest.changedPaths, ["docs/new-feature.md"]);
    assert.equal(manifest.risk, "docs");
    assert.equal(manifest.lanes.web, false);
    assert.equal(manifest.lanes.api, false);
    assert.equal(manifest.lanes.full, false);
    assert.deepEqual(validateManifest(manifest), []);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

test("archived v4 manifests stay valid under the old policy version", () => {
  const current = manifestFor(["apps/web/src/features/foo.tsx"]);
  const previous = {
    ...current,
    schema: "bodysense.delivery-manifest/v4",
    policyVersion: "delivery-policy-v3",
  };
  delete previous.stage;
  for (const lane of Object.keys(previous.laneDetails)) {
    previous.laneDetails[lane].targets = [...LANE_TARGETS_V5[lane]];
  }
  previous.digest = computeManifestDigest(previous);
  assert.deepEqual(validateManifest(previous), []);
});

test("archived v5 stages remain valid with the old v5 task targets", () => {
  const current = manifestFor(["apps/api/internal/service/foo.go"]);
  const legacy = structuredClone(current);
  legacy.schema = MANIFEST_SCHEMA_V5;
  legacy.policyVersion = POLICY_VERSION_V4;
  for (const lane of Object.keys(legacy.laneDetails)) {
    legacy.laneDetails[lane].targets = [...LANE_TARGETS_V5[lane]];
  }
  legacy.digest = computeManifestDigest(legacy);
  assert.deepEqual(validateManifest(legacy), []);
});
