#!/usr/bin/env node
/**
 * Verifies a merged Playwright report (playwright merge-reports over every
 * shard blob) against the committed shard partition in
 * apps/web/e2e/shards.json.
 *
 * This is the completeness receipt for the sharded E2E lane: the matrix job
 * aggregate alone cannot prove a shard was not silently dropped, so this
 * verifier requires the merged results to contain exactly the partitioned
 * spec set (nothing missing, nothing extra) and zero unexpected results.
 * A missing shard therefore fails the E2E report child and, through it,
 * the Experience Oracle.
 *
 * Usage:
 *   node scripts/e2e/verify-merged-report.mjs \
 *     --merged <merged-results.json> --partition apps/web/e2e/shards.json
 */
import fs from "node:fs";
import path from "node:path";

function basename(file) {
  return path.posix.basename(String(file).replaceAll("\\", "/"));
}

/** Basenames of every spec that produced results in a (merged) report. */
export function collectSpecFiles(merged) {
  const files = new Set();
  const walk = (suite) => {
    for (const spec of suite.specs ?? []) {
      if (spec.file) files.add(basename(spec.file));
    }
    for (const child of suite.suites ?? []) walk(child);
  };
  for (const suite of merged?.suites ?? []) walk(suite);
  return files;
}

export function verifyMergedReport(merged, partition) {
  const failures = [];
  if (!merged || typeof merged !== "object") {
    return { ok: false, failures: ["merged report must be an object"] };
  }
  const expected = new Set(
    (partition?.shards ?? []).flatMap((shard) =>
      (shard.files ?? []).map(basename),
    ),
  );
  if (expected.size === 0) failures.push("partition contains no spec files");
  const actual = collectSpecFiles(merged);
  for (const file of expected) {
    if (!actual.has(file)) {
      failures.push(`missing spec results (shard incomplete?): ${file}`);
    }
  }
  for (const file of actual) {
    if (!expected.has(file)) {
      failures.push(`spec results not in partition: ${file}`);
    }
  }
  const stats = merged.stats ?? {};
  const unexpected = stats.unexpected ?? 0;
  if (unexpected > 0) {
    failures.push(`${unexpected} unexpected test result(s) in merged report`);
  }
  const total =
    (stats.expected ?? 0) +
    (stats.skipped ?? 0) +
    (stats.flaky ?? 0) +
    unexpected;
  if (total === 0) failures.push("merged report contains no tests");
  return {
    ok: failures.length === 0,
    failures,
    stats: {
      files: actual.size,
      expected: stats.expected ?? 0,
      skipped: stats.skipped ?? 0,
      flaky: stats.flaky ?? 0,
      unexpected,
    },
  };
}

function argValue(name) {
  const index = process.argv.indexOf(`--${name}`);
  return index >= 0 ? process.argv[index + 1] : undefined;
}

function main() {
  const mergedPath = argValue("merged");
  const partitionPath = argValue("partition") ?? "apps/web/e2e/shards.json";
  if (!mergedPath) {
    throw new Error(
      "usage: verify-merged-report.mjs --merged <merged-results.json> [--partition <shards.json>]",
    );
  }
  if (!fs.existsSync(mergedPath)) {
    console.error(`merged report not found: ${mergedPath}`);
    process.exit(1);
  }
  const merged = JSON.parse(fs.readFileSync(mergedPath, "utf8"));
  const partition = JSON.parse(fs.readFileSync(partitionPath, "utf8"));
  const result = verifyMergedReport(merged, partition);
  if (!result.ok) {
    for (const failure of result.failures) console.error(failure);
    process.exit(1);
  }
  console.log(
    `E2E_MERGED_REPORT=PASS files=${result.stats.files} expected=${result.stats.expected} flaky=${result.stats.flaky} skipped=${result.stats.skipped}`,
  );
}

if (import.meta.url === `file://${process.argv[1]}`) {
  try {
    main();
  } catch (error) {
    console.error(`verify-merged-report failed: ${error.message}`);
    process.exit(1);
  }
}
