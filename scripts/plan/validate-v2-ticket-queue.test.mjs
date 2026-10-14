import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { REPO_ROOT, validateQueue } from "./validate-v2-ticket-queue.mjs";
const original = JSON.parse(
  fs.readFileSync(
    path.join(REPO_ROOT, "docs/plan/tickets/v2/queue.json"),
    "utf8",
  ),
);
const mutate = (fn) => {
  const q = structuredClone(original);
  fn(q);
  return validateQueue(q);
};
test("real execution queue is a valid dependency DAG with all documents", () =>
  assert.deepEqual(validateQueue(original), []));
test("unknown dependencies are rejected", () =>
  assert.ok(
    mutate((q) => q.tickets[1].dependsOn.push("DOES-NOT-EXIST")).some((s) =>
      s.includes("missing dependency"),
    ),
  ));
test("cyclic ticket graph is rejected", () =>
  assert.ok(
    mutate((q) => q.tickets[0].dependsOn.push("OPS-001")).some((s) =>
      s.includes("cycle"),
    ),
  ));
test("premature done without immutable SHA is rejected", () =>
  assert.ok(
    mutate((q) => (q.tickets[1].status = "done")).some((s) =>
      s.includes("accepted SHA"),
    ),
  ));
test("active competing shared-resource writers are rejected", () =>
  assert.ok(
    mutate((q) => {
      // INT-001 is now an accepted baseline; synthesize two active writers.
      q.tickets[0].status = "in_progress";
      q.tickets[1].status = "in_progress";
      q.tickets[1].exclusiveResources = ["integration-worktree"];
    }).some((s) => s.includes("resource collision")),
  ));
test("active competing worktree writers are rejected", () =>
  assert.ok(
    mutate((q) => {
      q.tickets[0].status = "in_progress";
      q.tickets[0].worktree = "/tmp/a";
      q.tickets[1].status = "in_progress";
      q.tickets[1].worktree = "/tmp/a";
    }).some((s) => s.includes("checkout collision")),
  ));
test("T3 thread without project mapping is rejected", () =>
  assert.ok(
    mutate((q) => {
      q.tickets[0].threadId = "some-thread";
    }).some((s) => s.includes("thread without verified project")),
  ));
test("ticket count cannot silently exceed pilot capacity", () =>
  assert.ok(
    mutate((q) => (q.maxParallelWriters = 8)).some((s) => s.includes("1..3")),
  ));

test("cannot admit an active ticket whose prerequisite is not accepted", () =>
  assert.ok(
    mutate((q) => {
      // Deliberately revoke the prerequisite's accepted state.
      q.tickets[0].status = "in_progress";
      q.tickets[1].status = "in_progress";
      q.tickets[1].worktree = "/tmp/independent";
    }).some((x) => x.includes("prerequisite not accepted")),
  ));
test("active writer must have an absolute checkout", () =>
  assert.ok(
    mutate((q) => {
      q.tickets[0].status = "in_progress";
      q.tickets[0].worktree = null;
    }).some((x) => x.includes("missing absolute checkout")),
  ));

test("blocked admission requires a real reason", () =>
  assert.ok(
    mutate((q) => {
      q.tickets[1].blockedReason = "";
    }).some((x) => x.includes("blocked without reason")),
  ));

test("main baseline must be a real exact immutable SHA", () =>
  assert.ok(
    mutate((q) => {
      q.mainBaselineSha = "1234";
    }).some((x) => x.includes("mainBaselineSha")),
  ));

test("new writer must start from the same frozen main baseline", () => {
  const wrong = mutate((q) => {
    q.tickets[2].status = "in_progress";
    q.tickets[2].worktree = "/tmp/isolated-int002";
    q.tickets[2].baseSha = "a".repeat(40);
  });
  assert.ok(wrong.some((x) => x.includes("frozen main SHA")));
  const correct = mutate((q) => {
    q.tickets[2].status = "in_progress";
    q.tickets[2].worktree = "/tmp/isolated-int002";
    q.tickets[2].baseSha = q.mainBaselineSha;
  });
  assert.deepEqual(correct, []);
});
