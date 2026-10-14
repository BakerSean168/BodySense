import assert from "node:assert/strict";
import test from "node:test";
import { interpretMergeTree, checkStack } from "./v2-stack-merge-guard.mjs";

const tree = "a".repeat(40);
const stage1 = "b".repeat(40);
const stage2 = "c".repeat(40);
const stage3 = "d".repeat(40);

test("two exact SHA trees with a clean merge satisfy the integration preflight", () => {
  const result = interpretMergeTree(0, tree + "\n");
  assert.equal(result.mergeable, true);
  assert.equal(result.tree, tree);
  assert.deepEqual(result.conflictedPaths, []);
});

test("an overlapping source edit must fail closed with the precise conflicted path", () => {
  const file =
    "apps/web/src/features/body-canvas/components/BodyFactRecords.tsx";
  const result = interpretMergeTree(
    1,
    [
      tree,
      `100644 ${stage1} 1\t${file}`,
      `100644 ${stage2} 2\t${file}`,
      `100644 ${stage3} 3\t${file}`,
      "",
      `CONFLICT (content): Merge conflict in ${file}`,
    ].join("\n"),
  );
  assert.equal(result.mergeable, false);
  assert.equal(result.tree, tree);
  assert.deepEqual(result.conflictedPaths, [file]);
  assert.equal(result.exitCode, 1);
});

test("status success without a merged tree must not claim integration success", () => {
  assert.equal(interpretMergeTree(0, "").mergeable, false);
});

test("conflict metadata cannot be reported as mergeable even with a zero exit status", () => {
  const file =
    "apps/web/src/features/body-canvas/components/BodyFactRecords.tsx";
  const result = interpretMergeTree(
    0,
    `${tree}\nCONFLICT (content): Merge conflict in ${file}\n`,
  );
  assert.equal(result.mergeable, false);
  assert.deepEqual(result.conflictedPaths, [file]);
});

test("git status 1 without a merge tree is a failed operation, not a merge conflict", () => {
  const result = interpretMergeTree(1, "", "fatal: bad object");
  assert.equal(result.mergeable, false);
  assert.equal(result.tree, null);
  assert.deepEqual(result.conflictedPaths, []);
  assert.equal(result.error, "fatal: bad object");
});

test("Git execution errors must remain separate from conflicts", () => {
  const result = interpretMergeTree(128, "", "fatal: bad object");
  assert.equal(result.mergeable, false);
  assert.equal(result.error, "fatal: bad object");
  assert.deepEqual(result.conflictedPaths, []);
});

test("missing/non-exact revisions are rejected before invoking Git", () => {
  assert.throws(() => checkStack("HEAD", tree), /exact Git commit IDs/);
  assert.throws(() => checkStack(tree, "abc"), /exact Git commit IDs/);
});
