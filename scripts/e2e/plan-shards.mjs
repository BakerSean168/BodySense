#!/usr/bin/env node
/**
 * E2E shard planner.
 *
 * The browser E2E suite runs as parallel CI shards. The partition is NOT a
 * naive file-count split: it is generated from measured per-file durations
 * (Playwright JSON reporter output of a full serial run) with an LPT
 * (longest-processing-time) greedy balance, and committed as
 * apps/web/e2e/shards.json so every CI run uses the same reviewable
 * partition and the merged-report verifier can prove the shard set is
 * complete.
 *
 * Usage:
 *   node scripts/e2e/plan-shards.mjs --durations <playwright-json> \
 *     --shards 3 --out apps/web/e2e/shards.json [--note "..."]
 *   node scripts/e2e/plan-shards.mjs --check \
 *     --partition apps/web/e2e/shards.json [--e2e-dir apps/web/e2e]
 *   node scripts/e2e/plan-shards.mjs --files-for 1 \
 *     --partition apps/web/e2e/shards.json
 */
import fs from "node:fs";
import path from "node:path";

export const SHARDS_SCHEMA = "bodysense.e2e-shards/v1";
export const DEFAULT_E2E_DIR = "apps/web/e2e";
export const DEFAULT_PARTITION = "apps/web/e2e/shards.json";

function round2(value) {
  return Math.round(value * 100) / 100;
}

/** Repo-relative paths of every spec file directly under the E2E dir. */
export function listSpecFiles(e2eDir = DEFAULT_E2E_DIR) {
  return fs
    .readdirSync(e2eDir)
    .filter((name) => name.endsWith(".spec.ts"))
    .sort()
    .map((name) => path.posix.join(e2eDir, name));
}

/**
 * Per-file serial cost in seconds from a Playwright JSON report: the sum of
 * every result duration in the file (a retried test really costs all of its
 * attempts under workers=1). Keys are basenames; spec files are flat.
 */
export function durationsFromPlaywrightJson(report) {
  const totals = new Map();
  const walk = (suite) => {
    for (const spec of suite.specs ?? []) {
      const file = path.posix.basename(spec.file ?? "");
      if (!file) continue;
      let seconds = totals.get(file) ?? 0;
      for (const result of spec.tests?.flatMap((t) => t.results ?? []) ?? []) {
        seconds += (result.duration ?? 0) / 1000;
      }
      totals.set(file, seconds);
    }
    for (const child of suite.suites ?? []) walk(child);
  };
  for (const suite of report.suites ?? []) walk(suite);
  return Object.fromEntries(
    [...totals.entries()].map(([file, seconds]) => [file, round2(seconds)]),
  );
}

/**
 * LPT greedy balance: sort files by estimated cost descending, assign each
 * to the currently lightest shard. Files without a measurement get the
 * median measured cost so a new spec never lands at zero cost.
 */
export function balanceShards(durationsByBase, shardCount, allFiles) {
  const measured = allFiles
    .map((file) => durationsByBase[path.posix.basename(file)])
    .filter((value) => typeof value === "number")
    .sort((a, b) => a - b);
  const median =
    measured.length > 0 ? measured[Math.floor(measured.length / 2)] : 30;
  const estimate = (file) =>
    durationsByBase[path.posix.basename(file)] ?? median;
  const ordered = [...allFiles].sort(
    (a, b) => estimate(b) - estimate(a) || a.localeCompare(b),
  );
  const shards = Array.from({ length: shardCount }, (_, i) => ({
    index: i + 1,
    files: [],
    totalSeconds: 0,
  }));
  for (const file of ordered) {
    const target = shards.reduce((lightest, shard) =>
      shard.totalSeconds < lightest.totalSeconds ? shard : lightest,
    );
    target.files.push(file);
    target.totalSeconds = round2(target.totalSeconds + estimate(file));
  }
  for (const shard of shards) shard.files.sort();
  return shards;
}

/** The partition must cover every spec exactly once, with no empty shard. */
export function validatePartition(partition, allFiles) {
  const errors = [];
  if (!partition || typeof partition !== "object") {
    return ["partition must be an object"];
  }
  if (partition.schema !== SHARDS_SCHEMA) {
    errors.push(`partition schema must be ${SHARDS_SCHEMA}`);
  }
  const shards = partition.shards ?? [];
  if (shards.length === 0) errors.push("partition has no shards");
  const seen = new Map();
  for (const shard of shards) {
    if (!Array.isArray(shard.files) || shard.files.length === 0) {
      errors.push(`shard ${shard.index} is empty`);
      continue;
    }
    for (const file of shard.files) {
      if (seen.has(file)) {
        errors.push(
          `spec assigned to shards ${seen.get(file)} and ${shard.index}: ${file}`,
        );
      }
      seen.set(file, shard.index);
    }
  }
  const known = new Set(allFiles);
  for (const file of allFiles) {
    if (!seen.has(file)) errors.push(`spec missing from partition: ${file}`);
  }
  for (const file of seen.keys()) {
    if (!known.has(file)) errors.push(`partition lists unknown spec: ${file}`);
  }
  return errors;
}

function argValue(name) {
  const index = process.argv.indexOf(`--${name}`);
  return index >= 0 ? process.argv[index + 1] : undefined;
}

function main() {
  const partitionPath = argValue("partition") ?? DEFAULT_PARTITION;
  const e2eDir = argValue("e2e-dir") ?? DEFAULT_E2E_DIR;

  if (process.argv.includes("--files-for")) {
    const index = Number(argValue("files-for"));
    const partition = JSON.parse(fs.readFileSync(partitionPath, "utf8"));
    const shard = partition.shards.find((entry) => entry.index === index);
    if (!shard) throw new Error(`no shard ${index} in ${partitionPath}`);
    console.log(shard.files.join(" "));
    return;
  }

  if (process.argv.includes("--check")) {
    const partition = JSON.parse(fs.readFileSync(partitionPath, "utf8"));
    const errors = validatePartition(partition, listSpecFiles(e2eDir));
    if (errors.length > 0) {
      for (const error of errors) console.error(error);
      process.exit(1);
    }
    console.log(
      `E2E_SHARDS=PASS shards=${partition.shards.length} specs=${
        partition.shards.flatMap((shard) => shard.files).length
      }`,
    );
    return;
  }

  const durationsPath = argValue("durations");
  const durationsMapPath = argValue("durations-map");
  const outPath = argValue("out") ?? DEFAULT_PARTITION;
  const shardCount = Number(argValue("shards") ?? "3");
  if (!durationsPath && !durationsMapPath) {
    throw new Error(
      "usage: plan-shards.mjs --durations <playwright-json> | --durations-map <{file: seconds}> [--shards N] [--out <partition>] | --check | --files-for N",
    );
  }
  if (!Number.isInteger(shardCount) || shardCount < 1) {
    throw new Error(`--shards must be a positive integer, got ${shardCount}`);
  }
  // --durations-map accepts a plain { "file.spec.ts": seconds } object,
  // e.g. extracted from a CI HTML report when no JSON report is at hand.
  const durations = durationsMapPath
    ? JSON.parse(fs.readFileSync(durationsMapPath, "utf8"))
    : durationsFromPlaywrightJson(
        JSON.parse(fs.readFileSync(durationsPath, "utf8")),
      );
  const allFiles = listSpecFiles(e2eDir);
  const shards = balanceShards(durations, shardCount, allFiles);
  const partition = {
    schema: SHARDS_SCHEMA,
    generatedFrom:
      argValue("note") ??
      "playwright JSON reporter durations of a full serial run",
    shardCount,
    totalSeconds: round2(
      shards.reduce((sum, shard) => sum + shard.totalSeconds, 0),
    ),
    shards,
  };
  const errors = validatePartition(partition, allFiles);
  if (errors.length > 0) throw new Error(errors.join("; "));
  fs.writeFileSync(outPath, `${JSON.stringify(partition, null, 2)}\n`);
  console.log(
    `E2E_SHARDS=PLANNED out=${outPath} totals=${shards
      .map((shard) => `${shard.index}:${shard.totalSeconds}s`)
      .join(" ")}`,
  );
}

if (import.meta.url === `file://${process.argv[1]}`) {
  try {
    main();
  } catch (error) {
    console.error(`plan-shards failed: ${error.message}`);
    process.exit(1);
  }
}
