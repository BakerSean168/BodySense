import assert from "node:assert/strict";
import test from "node:test";
import {
  balanceShards,
  durationsFromPlaywrightJson,
  validatePartition,
} from "../e2e/plan-shards.mjs";
import {
  collectSpecFiles,
  verifyMergedReport,
} from "../e2e/verify-merged-report.mjs";
import { createManifest, evaluateManifestOracle } from "./lib.mjs";

const A = "a".repeat(40);
const B = "b".repeat(40);

const FILES = [
  "apps/web/e2e/a.spec.ts",
  "apps/web/e2e/b.spec.ts",
  "apps/web/e2e/c.spec.ts",
  "apps/web/e2e/d.spec.ts",
  "apps/web/e2e/e.spec.ts",
  "apps/web/e2e/f.spec.ts",
];

function partitionFor(durations, shardCount = 3) {
  return {
    schema: "bodysense.e2e-shards/v1",
    shardCount,
    shards: balanceShards(durations, shardCount, FILES),
  };
}

test("durations are summed per spec file across tests and retries", () => {
  const report = {
    suites: [
      {
        title: "a.spec.ts",
        file: "apps/web/e2e/a.spec.ts",
        specs: [
          {
            file: "apps/web/e2e/a.spec.ts",
            tests: [
              { results: [{ duration: 1500 }, { duration: 2500 }] },
              { results: [{ duration: 1000 }] },
            ],
          },
        ],
        suites: [
          {
            title: "nested",
            specs: [
              {
                file: "apps/web/e2e/b.spec.ts",
                tests: [{ results: [{ duration: 3000 }] }],
              },
            ],
          },
        ],
      },
    ],
  };
  assert.deepEqual(durationsFromPlaywrightJson(report), {
    "a.spec.ts": 5,
    "b.spec.ts": 3,
  });
});

test("LPT balance covers every spec exactly once and evens the load", () => {
  const partition = partitionFor({
    "a.spec.ts": 100,
    "b.spec.ts": 90,
    "c.spec.ts": 80,
    "d.spec.ts": 10,
    "e.spec.ts": 10,
    "f.spec.ts": 10,
  });
  assert.deepEqual(validatePartition(partition, FILES), []);
  const totals = partition.shards.map((shard) => shard.totalSeconds);
  // Greedy LPT: 100 | 90 | 80, then the three 10s fill the lightest -> max is 100, far under the
  // 300s a single unsharded run or a naive 2-file split (190) would cost.
  assert.equal(Math.max(...totals), 100);
  assert.equal(
    partition.shards.flatMap((shard) => shard.files).length,
    FILES.length,
  );
});

test("unmeasured specs fall back to the median cost", () => {
  const partition = partitionFor({ "a.spec.ts": 60, "b.spec.ts": 30 }, 2);
  assert.deepEqual(validatePartition(partition, FILES), []);
  // Median of the measured [30, 60] is 60, so the four unmeasured specs
  // are estimated at 60 each: 60 + 30 + 4*60 = 330 across both shards.
  const totals = partition.shards.map((shard) => shard.totalSeconds);
  assert.equal(
    totals.reduce((sum, total) => sum + total, 0),
    330,
  );
});

test("partition validation rejects missing, duplicate, and unknown specs", () => {
  const good = partitionFor({}, 2);
  const missing = structuredClone(good);
  missing.shards[0].files.pop();
  assert.ok(
    validatePartition(missing, FILES).some((error) =>
      error.includes("missing from partition"),
    ),
  );
  const duplicate = structuredClone(good);
  duplicate.shards[1].files.push(duplicate.shards[0].files[0]);
  assert.ok(
    validatePartition(duplicate, FILES).some((error) =>
      error.includes("assigned to shards"),
    ),
  );
  const unknown = structuredClone(good);
  unknown.shards[0].files.push("apps/web/e2e/ghost.spec.ts");
  assert.ok(
    validatePartition(unknown, FILES).some((error) =>
      error.includes("unknown spec"),
    ),
  );
});

function mergedReportFor(files, stats = {}) {
  return {
    suites: files.map((file) => ({
      title: file,
      file: `apps/web/e2e/${file}`,
      specs: [{ file: `apps/web/e2e/${file}`, tests: [] }],
    })),
    stats: {
      expected: files.length,
      skipped: 0,
      flaky: 0,
      unexpected: 0,
      ...stats,
    },
  };
}

const ALL_BASENAMES = FILES.map((file) => file.split("/").pop());
const PARTITION = partitionFor({}, 3);

test("merged report verification passes a complete clean run", () => {
  const result = verifyMergedReport(mergedReportFor(ALL_BASENAMES), PARTITION);
  assert.equal(result.ok, true);
  assert.equal(result.stats.files, ALL_BASENAMES.length);
  assert.deepEqual(
    [...collectSpecFiles(mergedReportFor(ALL_BASENAMES))].sort(),
    [...ALL_BASENAMES].sort(),
  );
});

test("fault injection: a dropped shard fails merged report verification", () => {
  // Remove every spec of shard 2, as if that matrix leg never uploaded
  // its blob: the merged report is green for what ran, but incomplete.
  const dropped = new Set(
    PARTITION.shards[1].files.map((file) => file.split("/").pop()),
  );
  const partial = ALL_BASENAMES.filter((file) => !dropped.has(file));
  const result = verifyMergedReport(mergedReportFor(partial), PARTITION);
  assert.equal(result.ok, false);
  assert.ok(
    result.failures.some((failure) => failure.includes("missing spec")),
  );
});

test("merged report verification rejects unexpected results and extras", () => {
  const failed = verifyMergedReport(
    mergedReportFor(ALL_BASENAMES, { unexpected: 2 }),
    PARTITION,
  );
  assert.equal(failed.ok, false);
  assert.ok(failed.failures.some((failure) => failure.includes("unexpected")));
  const extra = verifyMergedReport(
    mergedReportFor([...ALL_BASENAMES, "ghost.spec.ts"]),
    PARTITION,
  );
  assert.equal(extra.ok, false);
  assert.ok(
    extra.failures.some((failure) => failure.includes("not in partition")),
  );
});

test("experience oracle goes red when the merged-report receipt is missing or failed", () => {
  const manifest = createManifest({
    requestedBaseSha: A,
    baseSha: A,
    headSha: B,
    event: "push",
    ref: "refs/heads/main",
    changedPaths: ["apps/web/e2e/a.spec.ts"],
    generatedAt: "2026-08-29T00:00:00.000Z",
  });
  assert.equal(manifest.lanes.experience, true);
  const missing = evaluateManifestOracle({
    manifest,
    group: "experience",
    children: { component: "success" },
  });
  assert.equal(missing.ok, false);
  assert.ok(
    missing.failures.some((failure) =>
      failure.includes("experience: expected success, got missing"),
    ),
  );
  const failed = evaluateManifestOracle({
    manifest,
    group: "experience",
    children: { experience: "failure", component: "success" },
  });
  assert.equal(failed.ok, false);
  const green = evaluateManifestOracle({
    manifest,
    group: "experience",
    children: { experience: "success", component: "success" },
  });
  assert.equal(green.ok, true);
});
