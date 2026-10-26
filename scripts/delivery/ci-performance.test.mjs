import assert from "node:assert/strict";
import test from "node:test";
import {
  analyzeRun,
  nearestRank,
  reportMarkdown,
  summarize,
} from "../quality/report-ci-performance.mjs";

const z = (seconds) =>
  new Date(Date.parse("2026-10-26T00:00:00Z") + seconds * 1000).toISOString();
const REQUIRED = [
  "Governance Oracle",
  "Quality Oracle",
  "Database Oracle",
  "Experience Oracle",
  "Delivery Observation",
  "commit-lint",
];
function run(id = 101, ciCheck = false) {
  const jobs = [
    {
      name: "Test Server",
      startedAt: z(13),
      completedAt: z(75),
      conclusion: "success",
    },
    {
      name: "Delivery Scope",
      startedAt: z(10),
      completedAt: z(18),
      conclusion: "success",
    },
    ...REQUIRED.map((name, index) => ({
      name,
      startedAt: z(20),
      completedAt: z(40 + index * 4),
      conclusion: "success",
    })),
  ];
  if (ciCheck)
    jobs.push({
      name: "CI Check",
      startedAt: z(66),
      completedAt: z(87),
      conclusion: "success",
    });
  return {
    databaseId: id,
    createdAt: z(0),
    url: "https://github.com/Example/Example/actions/runs/" + id,
    headSha: "a".repeat(40),
    event: "pull_request",
    status: "completed",
    conclusion: "success",
    jobs,
  };
}

test("legacy PR gate measures first start to last required context, not queue or longest job", () => {
  const measurement = analyzeRun(run(), "root");
  assert.equal(measurement.complete, true);
  assert.equal(measurement.gateSeconds, 50);
  assert.equal(measurement.wallSeconds, 65);
  assert.equal(measurement.queueSeconds, 10);
  assert.equal(measurement.gateName, "legacy required");
  assert.equal(measurement.longestJobs[0].name, "Test Server");
  assert.ok(measurement.observedRunnerMinutes > 1);
});

test("unified CI Check's actual completion becomes the new gate end", () => {
  const measurement = analyzeRun(run(102, true), "root");
  assert.equal(measurement.complete, true);
  assert.equal(measurement.gateSeconds, 77);
  assert.equal(measurement.wallSeconds, 77);
  assert.equal(measurement.gateName, "CI Check");
});

test("incomplete/cancelled/missing CI never enters the P95 denominator", () => {
  const failed = run(103);
  failed.conclusion = "cancelled";
  assert.equal(analyzeRun(failed).complete, false);
  assert.equal(analyzeRun(failed).gateSeconds, null);
  const missing = run(104, true);
  missing.jobs.find((job) => job.name === "CI Check").conclusion = "skipped";
  assert.equal(analyzeRun(missing).complete, false);
  const badTime = run(105);
  badTime.jobs.find((job) => job.name === "Delivery Observation").completedAt =
    null;
  assert.equal(analyzeRun(badTime).complete, false);
});

test("nearest-rank P50/P95 is explicit and never silently interpolates tiny samples", () => {
  assert.equal(nearestRank([], 95), null);
  assert.equal(nearestRank([100, 150, 250, 300], 50), 150);
  assert.equal(nearestRank([100, 150, 250, 300], 95), 300);
  assert.throws(() => nearestRank([100], 150));
});

test("a root-only CI data set cannot certify Web/Go/AI/Contract P95", () => {
  const runs = [
    analyzeRun(run(101)),
    analyzeRun(run(102)),
    analyzeRun(run(103)),
  ];
  const stats = summarize(runs);
  assert.equal(stats.root.count, 3);
  assert.equal(stats.root.sufficientCount, true);
  assert.equal(stats.web.count, 0);
  assert.equal(stats.server.count, 0);
  assert.equal(stats.ai.count, 0);
  assert.equal(stats.contracts.count, 0);
  const markdown = reportMarkdown(runs);
  assert.match(markdown, /NOT YET CERTIFIED/);
  assert.match(markdown, /Runner-minutes are observed/);
  assert.match(markdown, /actions\/runs\/101/);
});

test("scenario names are validated and no missing data is turned into zeros", () => {
  assert.throws(() => analyzeRun(run(), "untrusted"));
  const empty = { ...run(), jobs: [] };
  assert.equal(analyzeRun(empty).gateSeconds, null);
  assert.equal(analyzeRun(empty).complete, false);
});

test("historical B8 reference is separate from new-version P95 samples", () => {
  const current = [analyzeRun(run(202))];
  const legacy = analyzeRun(run(99));
  const markdown = reportMarkdown(current, legacy);
  assert.match(markdown, /Historical B8 reference/);
  assert.match(markdown, /excluded from the new P95 sample/);
  assert.equal(summarize(current).root.count, 1);
});

test("new flattened Check is the only required gate for the performance interval", () => {
  const current = run(300, true);
  const check = current.jobs.find((job) => job.name === "CI Check");
  check.name = "Check";
  const result = analyzeRun(current);
  assert.equal(result.complete, true);
  assert.equal(result.gateName, "Check");
  assert.equal(result.gateSeconds, 77);
});
