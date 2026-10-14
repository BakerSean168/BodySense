import { spawnSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const OBJECT_ID = /^[a-f0-9]{40}$/;

export function interpretMergeTree(status, stdout, stderr = "") {
  const lines = stdout.split(/\r?\n/);
  const tree = lines[0]?.trim();
  const conflicts = new Set();

  for (const line of lines) {
    const message = line.match(/^CONFLICT \([^)]*\):.*? in (.+)$/);
    if (message) conflicts.add(message[1]);
    const unmerged = line.match(/^\d{6} [a-f0-9]{40} [123]\t(.+)$/);
    if (unmerged) conflicts.add(unmerged[1]);
  }

  return {
    mergeable:
      status === 0 && OBJECT_ID.test(tree ?? "") && conflicts.size === 0,
    tree: OBJECT_ID.test(tree ?? "") ? tree : null,
    conflictedPaths: [...conflicts].sort(),
    exitCode: status,
    error:
      (status === 0 && OBJECT_ID.test(tree ?? "")) ||
      (status === 1 && OBJECT_ID.test(tree ?? "") && conflicts.size > 0)
        ? null
        : stderr.trim() || "merge-tree failed",
  };
}

export function checkStack(left, right, cwd = process.cwd()) {
  if (!OBJECT_ID.test(left) || !OBJECT_ID.test(right)) {
    throw new Error(
      "Both exact Git commit IDs must be 40 lowercase hex digits",
    );
  }
  const result = spawnSync("git", ["merge-tree", "--write-tree", left, right], {
    cwd,
    encoding: "utf8",
    maxBuffer: 4 * 1024 * 1024,
  });
  if (result.error) throw result.error;
  return {
    left,
    right,
    ...interpretMergeTree(
      result.status,
      result.stdout ?? "",
      result.stderr ?? "",
    ),
  };
}

if (
  process.argv[1] &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  try {
    const result = checkStack(process.argv[2], process.argv[3]);
    console.log(JSON.stringify(result, null, 2));
    process.exitCode = result.mergeable ? 0 : result.error ? 2 : 1;
  } catch (error) {
    console.error(String(error));
    process.exitCode = 2;
  }
}
