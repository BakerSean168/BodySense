#!/usr/bin/env node
// Evidence-only CI performance reporter. No workflows are dispatched or mutated.
import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { pathToFileURL } from "node:url";

export const REQUIRED_LEGACY = Object.freeze([
  "Governance Oracle",
  "Quality Oracle",
  "Database Oracle",
  "Experience Oracle",
  "Delivery Observation",
]);
export const CATEGORIES = Object.freeze([
  "root",
  "web",
  "server",
  "ai",
  "contracts",
]);
export const REQUIRED_SAMPLES = 3;

function timestamp(value) {
  if (!value) return null;
  const date = Date.parse(value);
  return Number.isFinite(date) ? date : null;
}
function duration(start, end) {
  if (start === null || end === null || end < start) return null;
  return (end - start) / 1000;
}
function round(value, digits = 1) {
  return value === null ? null : Number(value.toFixed(digits));
}
function requiredNames(run) {
  const all = run.jobs ?? [];
  if (all.some((job) => job.name === "CI Check")) return ["CI Check"];
  return [
    ...REQUIRED_LEGACY,
    ...(run.event === "pull_request" ? ["commit-lint"] : []),
  ];
}
export function analyzeRun(run, category = "root") {
  if (!CATEGORIES.includes(category)) {
    throw new Error("unrecognized change category: " + category);
  }
  const jobs = run.jobs ?? [];
  const started = jobs
    .map((j) => timestamp(j.startedAt))
    .filter((v) => v !== null);
  const finished = jobs
    .map((j) => timestamp(j.completedAt))
    .filter((v) => v !== null);
  const start = started.length ? Math.min(...started) : null;
  const end = finished.length ? Math.max(...finished) : null;
  const required = requiredNames(run);
  const missingRequired = required.filter(
    (name) => !jobs.some((j) => j.name === name && j.conclusion === "success"),
  );
  const gateEnds = required
    .map((name) => jobs.find((j) => j.name === name)?.completedAt)
    .map(timestamp);
  const endGate = gateEnds.every((v) => v !== null)
    ? Math.max(...gateEnds)
    : null;
  const jobCosts = jobs
    .filter((j) => j.conclusion !== "skipped")
    .map((j) => ({
      name: j.name,
      seconds: duration(timestamp(j.startedAt), timestamp(j.completedAt)),
      conclusion: j.conclusion,
    }));
  const observedSeconds = jobCosts
    .filter((j) => j.seconds !== null)
    .reduce((sum, j) => sum + j.seconds, 0);
  const created = timestamp(run.createdAt);
  const completed =
    run.status === "completed" &&
    run.conclusion === "success" &&
    missingRequired.length === 0 &&
    start !== null &&
    endGate !== null;
  return {
    runId: String(run.databaseId),
    url: run.url,
    sha: run.headSha,
    category,
    event: run.event,
    conclusion: run.conclusion,
    complete: completed,
    missingRequired,
    gateSeconds: completed ? round(duration(start, endGate)) : null,
    wallSeconds: completed ? round(duration(start, end)) : null,
    queueSeconds: round(duration(created, start)),
    observedRunnerMinutes: round(observedSeconds / 60, 2),
    gateName: required.includes("CI Check") ? "CI Check" : "legacy required",
    longestJobs: jobCosts
      .filter((job) => job.seconds !== null)
      .sort((a, b) => b.seconds - a.seconds)
      .slice(0, 4)
      .map((job) => ({ name: job.name, seconds: round(job.seconds) })),
  };
}

export function nearestRank(sorted, percentile) {
  if (!sorted.length) return null;
  if (percentile < 0 || percentile > 100) {
    throw new Error("invalid percentile");
  }
  const index = Math.max(0, Math.ceil((percentile / 100) * sorted.length) - 1);
  return sorted[index];
}
export function summarize(records) {
  return Object.fromEntries(
    CATEGORIES.map((category) => {
      const recordsForCategory = records.filter(
        (r) =>
          r.category === category && r.complete && r.event === "pull_request",
      );
      const sorted = recordsForCategory
        .map((r) => r.gateSeconds)
        .sort((a, b) => a - b);
      return [
        category,
        {
          count: sorted.length,
          p50: round(nearestRank(sorted, 50)),
          p95: round(nearestRank(sorted, 95)),
          max: sorted.length ? sorted.at(-1) : null,
          requiredSamples: REQUIRED_SAMPLES,
          sufficientCount: sorted.length >= REQUIRED_SAMPLES,
        },
      ];
    }),
  );
}
export function reportMarkdown(records, historicalBaseline = null) {
  const groups = summarize(records);
  const ready = CATEGORIES.every(
    (category) => groups[category].sufficientCount,
  );
  const meetsTarget =
    ready && CATEGORIES.every((category) => groups[category].p95 <= 300);
  const out = [
    "# BodySense CI Performance Evidence",
    "",
    "Source: GitHub Actions job start/completion timestamps from the linked runs.",
    "Values exclude GitHub queue time from the PR gate, but queue is recorded separately.",
    "Runner-minutes are observed summed job durations, **not billed minutes**.",
    "Nearest-rank P95 with small samples is provisional. All five change classes need at least three completed representative runs.",
    "",
    "| Change class | Successful PR samples | Gate P50 | Gate P95 | Sampling ready |",
    "| --- | ---: | ---: | ---: | --- |",
  ];
  for (const category of CATEGORIES) {
    const v = groups[category];
    const display = (seconds) =>
      seconds === null ? "—" : seconds.toFixed(1) + "s";
    out.push(
      "| " +
        category +
        " | " +
        v.count +
        " | " +
        display(v.p50) +
        " | " +
        display(v.p95) +
        " | " +
        (v.sufficientCount ? "yes" : "no") +
        " |",
    );
  }
  out.push(
    "",
    "Target: ordinary PR **P95 <= 300s**. Evidence status: **" +
      (meetsTarget
        ? "MEETS SAMPLED TARGET"
        : ready
          ? "TARGET NOT MET"
          : "NOT YET CERTIFIED") +
      "**.",
    "",
    "| Run | Event | Class | Source SHA | Gate | Wall | Queue | Sum runner-min | Longest job |",
    "| --- | --- | --- | --- | ---: | ---: | ---: | ---: | --- |",
  );
  const display = (n) => (n === null ? "—" : String(n));
  for (const r of records) {
    const link = "[" + r.runId + "](" + r.url + ")";
    const longest = r.longestJobs[0];
    out.push(
      "| " +
        link +
        " | " +
        r.event +
        " | " +
        r.category +
        " | " +
        String(r.sha).slice(0, 10) +
        " | " +
        display(r.gateSeconds) +
        "s | " +
        display(r.wallSeconds) +
        "s | " +
        display(r.queueSeconds) +
        "s | " +
        display(r.observedRunnerMinutes) +
        " | " +
        (longest ? longest.name + " (" + longest.seconds + "s)" : "—") +
        " |",
    );
  }
  if (historicalBaseline) {
    out.push(
      "",
      "Historical B8 reference **excluded from the new P95 sample**: [" +
        historicalBaseline.runId +
        "](" +
        historicalBaseline.url +
        ") — gate " +
        historicalBaseline.gateSeconds +
        "s, observed runner-minutes " +
        historicalBaseline.observedRunnerMinutes +
        "; longest job " +
        (historicalBaseline.longestJobs[0]?.name ?? "unknown") +
        ".",
      "The historic run's validation scope and job graph differ, so these figures are directional rather than a strictly controlled A/B.",
    );
  }
  out.push(
    "",
    "Incomplete/cancelled/failed runs are not included in P50/P95.",
    "These root/CI runs are stacked and may have different cache warmness, base branches and parallel runner availability; they are not a controlled T3 Code A/B comparison.",
    "Do not switch branch protection or merge the final integration PR based on this report alone.",
    "",
  );
  return out.join("\n").trimEnd();
}
function ghRun(repo, runId) {
  const result = spawnSync(
    "gh",
    [
      "run",
      "view",
      runId,
      "--repo",
      repo,
      "--json",
      "databaseId,status,conclusion,event,url,createdAt,headSha,jobs",
    ],
    { encoding: "utf8", maxBuffer: 12 * 1024 * 1024 },
  );
  if (result.error || result.status !== 0) {
    throw new Error(
      "GitHub run fetch failed for " +
        runId +
        ": " +
        (result.stderr || result.error?.message || "unknown error"),
    );
  }
  return JSON.parse(result.stdout);
}
function main() {
  const argv = process.argv.slice(2);
  const runs = [];
  let repo = "BakerSean168/BodySense";
  let output;
  let jsonFile;
  let baselineId;
  for (let i = 0; i < argv.length; i++) {
    const option = argv[i];
    const value = argv[i + 1];
    if (option === "--repo") repo = value;
    else if (option === "--output") output = value;
    else if (option === "--json-out") jsonFile = value;
    else if (option === "--baseline") baselineId = value;
    else if (option === "--run") {
      if (!value || !/^[0-9]+:(root|web|server|ai|contracts)$/.test(value)) {
        throw new Error("usage: --run runId:root|web|server|ai|contracts");
      }
      const [runId, category] = value.split(":");
      runs.push({ runId, category });
    } else {
      throw new Error("unknown argument: " + option);
    }
    i += 1;
  }
  if (!runs.length) throw new Error("provide at least one --run id:category");
  const records = runs.map(({ runId, category }) =>
    analyzeRun(ghRun(repo, runId), category),
  );
  const baseline = baselineId
    ? analyzeRun(ghRun(repo, baselineId), "root")
    : null;
  const markdown = reportMarkdown(records, baseline);
  if (output) {
    fs.mkdirSync(path.dirname(output), { recursive: true });
    fs.writeFileSync(output, markdown + "\n");
  } else {
    process.stdout.write(markdown + "\n");
  }
  if (jsonFile) {
    fs.mkdirSync(path.dirname(jsonFile), { recursive: true });
    fs.writeFileSync(
      jsonFile,
      JSON.stringify(
        {
          schema: "bodysense.ci-performance-evidence/v1",
          repo,
          records,
          categories: summarize(records),
          historicalBaseline: baseline,
        },
        null,
        2,
      ) + "\n",
    );
  }
}
if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href
) {
  try {
    main();
  } catch (err) {
    console.error("CI_PERFORMANCE_REPORT=FAIL " + err.message);
    process.exitCode = 1;
  }
}
