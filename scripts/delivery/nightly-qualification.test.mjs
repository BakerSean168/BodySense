import assert from "node:assert/strict";
import test from "node:test";
import { sha256Json } from "./lib.mjs";
import { CI_JOB_IDS } from "./ci-check-lib.mjs";
import {
  isQualifiedNightlySource,
  validateNightlyReceipt,
} from "./nightly-qualification.mjs";

const sha = "a".repeat(40);
function receipt() {
  const item = {
    schema: "bodysense.ci-check-receipt/v2",
    runId: "777",
    runAttempt: 2,
    event: "schedule",
    ref: "refs/heads/main",
    headSha: sha,
    manifestDigest: "sha256:" + "b".repeat(64),
    results: Object.fromEntries(CI_JOB_IDS.map((id) => [id, "success"])),
    passed: true,
    failures: [],
    generatedAt: "2026-10-26T18:23:00Z",
  };
  item.digest = sha256Json(
    Object.fromEntries(
      Object.entries(item).filter(([key]) => key !== "generatedAt"),
    ),
  );
  return item;
}
const context = { sha, runId: "777", runAttempt: 2 };
test("Nightly acceptance requires the exact successful full CI receipt", () => {
  assert.deepEqual(validateNightlyReceipt(receipt(), context), []);
});
test("manual Nightly accepts a dispatched full Main CI receipt bound to its event", () => {
  const dispatched = receipt();
  dispatched.event = "workflow_dispatch";
  const { digest: _d, generatedAt: _g, ...identity } = dispatched;
  dispatched.digest = sha256Json(identity);
  assert.deepEqual(
    validateNightlyReceipt(dispatched, {
      ...context,
      event: "workflow_dispatch",
    }),
    [],
  );
  assert.ok(
    validateNightlyReceipt(dispatched, { ...context, event: "schedule" })
      .length > 0,
  );
});
test("Nightly source pairs: auto=schedule only, manual=schedule|workflow_dispatch", () => {
  assert.equal(isQualifiedNightlySource("workflow_run", "schedule"), true);
  assert.equal(
    isQualifiedNightlySource("workflow_run", "workflow_dispatch"),
    false,
  );
  assert.equal(
    isQualifiedNightlySource("workflow_dispatch", "workflow_dispatch"),
    true,
  );
  assert.equal(isQualifiedNightlySource("workflow_dispatch", "schedule"), true);
  for (const ci of ["push", "pull_request"]) {
    assert.equal(isQualifiedNightlySource("workflow_dispatch", ci), false);
    assert.equal(isQualifiedNightlySource("workflow_run", ci), false);
  }
  assert.equal(isQualifiedNightlySource("push", "schedule"), false);
});
test("nightly receipt rejects PR, push, skipped jobs, wrong attempt and tampering", () => {
  const variants = [
    { event: "push" },
    { event: "pull_request" },
    { results: { ...receipt().results, "e2e-child": "skipped" } },
    { runAttempt: 3 },
    { headSha: "b".repeat(40) },
    { digest: "sha256:" + "0".repeat(64) },
  ];
  for (const mutation of variants) {
    const result = validateNightlyReceipt(
      { ...receipt(), ...mutation },
      context,
    );
    assert.ok(result.length > 0, JSON.stringify(mutation));
  }
});
test("candidate and Stable workflow do not publish on normal main push", async () => {
  const fs = await import("node:fs");
  const nightly = fs.readFileSync(
    ".github/workflows/candidate-publish.yml",
    "utf8",
  );
  const stable = fs.readFileSync(
    ".github/workflows/release-publish.yml",
    "utf8",
  );
  const deploy = fs.readFileSync(
    ".github/workflows/deploy-production.yml",
    "utf8",
  );
  assert.match(nightly, /name: Publish Nightly Candidate/);
  assert.match(nightly, /workflow_run\.event == 'schedule'/);
  assert.match(nightly, /nightly-qualification\.mjs/);
  assert.match(nightly, /  workflow_dispatch:\n    inputs:\n      ci_run_id:/);
  assert.match(nightly, /--event "\$CI_EVENT"/);
  assert.match(nightly, /is not current main HEAD/);
  assert.match(
    nightly,
    /group: nightly-candidate-\$\{\{ github\.event\.workflow_run\.head_sha \|\| github\.sha \}\}\n  cancel-in-progress: false/,
  );
  assert.match(nightly, /ci-check-receipt-/);
  assert.match(nightly, /nightly-reuse\.mjs/);
  assert.match(nightly, /steps\.decision\.outputs\.publish/);
  assert.doesNotMatch(nightly, /\.head_sha == \$sha/);
  assert.match(stable, /name: Publish Stable/);
  assert.match(stable, /candidate_run_id:/);
  assert.match(stable, /release_tag:/);
  assert.match(stable, /ref: \$\{\{ steps\.candidate\.outputs\.revision \}\}/);
  assert.doesNotMatch(stable, /workflows: \[Publish Main Candidate\]/);
  assert.doesNotMatch(stable, /reconcile-release-please-state/);
  assert.match(stable, /gh release edit "\$RELEASE_TAG"/);
  assert.match(deploy, /if \[\[ "\$ci_event" == schedule/);
  assert.match(deploy, /elif \[\[ "\$ci_event" == workflow_dispatch/);
  assert.match(stable, /candidate_event" == workflow_dispatch/);
  assert.match(
    deploy,
    /Historical Stable releases remain deployable for rollback/,
  );
});
