import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const REPO_ROOT = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../..",
);
const STATES = new Set(["queued", "in_progress", "blocked", "done"]);
const SHA = /^[0-9a-f]{40}$/;
export const MAX_PARALLEL_WRITERS = 4;
// Shared generated hotspots (baseline migration + checksums, public OpenAPI and
// its generated Go/TS/Zod/Postman artifacts, the domain contract). Parallel
// writers may hold them at the same time: whoever merges second rebases onto
// main, takes main's generated files, regenerates and re-runs the checks.
// Every other exclusive resource names domain files and still collides.
export const SHARED_HOTSPOTS = new Set([
  "database-migrations",
  "public-openapi",
  "domain-contract",
]);

export function validateQueue(queue, root = REPO_ROOT) {
  const errors = [];
  if (!queue || queue.version !== 1 || !Array.isArray(queue.tickets))
    return ["queue: invalid version or tickets"];
  if (
    !Number.isInteger(queue.maxParallelWriters) ||
    queue.maxParallelWriters < 1 ||
    queue.maxParallelWriters > MAX_PARALLEL_WRITERS
  )
    errors.push(
      "maxParallelWriters: parallel lanes must be 1.." + MAX_PARALLEL_WRITERS,
    );
  if (!SHA.test(queue.seedSha || ""))
    errors.push("seedSha: expected exact 40-hex SHA");
  if (!SHA.test(queue.mainBaselineSha || ""))
    errors.push("mainBaselineSha: expected exact frozen main SHA");
  // The integration checkout is retired; a recorded one must still be absolute.
  if (
    queue.integrationWorktree != null &&
    (typeof queue.integrationWorktree !== "string" ||
      !path.isAbsolute(queue.integrationWorktree))
  )
    errors.push("integration checkout mapping must be an absolute path");
  const ids = new Set();
  for (const t of queue.tickets) {
    if (!/^[A-Z][A-Z0-9-]+$/.test(t.id || ""))
      errors.push("ticket: invalid id");
    if (ids.has(t.id)) errors.push("ticket: duplicate " + t.id);
    ids.add(t.id);
    if (!STATES.has(t.status)) errors.push(t.id + ": invalid status");
    if (!Array.isArray(t.dependsOn))
      errors.push(t.id + ": dependencies missing");
    if (!Array.isArray(t.exclusiveResources) || !t.exclusiveResources.length)
      errors.push(t.id + ": exclusive resources missing");
    if (
      !t.document ||
      !t.document.startsWith("docs/plan/tickets/v2/") ||
      t.document.includes("..") ||
      !fs.existsSync(path.join(root, t.document))
    )
      errors.push(t.id + ": missing/outside ticket document");
    if (t.status === "done" && !SHA.test(t.acceptedSha || ""))
      errors.push(t.id + ": done without exact accepted SHA");
    if (
      t.status === "in_progress" &&
      t.id !== "INT-001" &&
      t.baseSha !== queue.mainBaselineSha
    )
      errors.push(t.id + ": active writer did not start at frozen main SHA");
    if (
      t.status === "blocked" &&
      (typeof t.blockedReason !== "string" || !t.blockedReason.trim())
    )
      errors.push(t.id + ": blocked without reason");
    if (t.status === "queued" && (t.threadId || t.projectId || t.worktree))
      errors.push(t.id + ": queued but already has an active binding");
    if (t.threadId && (!t.projectId || !t.worktree))
      errors.push(t.id + ": thread without verified project and checkout");
  }
  for (const t of queue.tickets) {
    for (const dep of t.dependsOn || []) {
      if (!ids.has(dep)) errors.push(t.id + ": missing dependency " + dep);
      if (dep === t.id) errors.push(t.id + ": self dependency");
    }
  }
  const byId = new Map(queue.tickets.map((t) => [t.id, t]));
  const visit = (id, stack, done) => {
    if (stack.has(id)) {
      errors.push("dependency cycle: " + [...stack, id].join(" -> "));
      return;
    }
    if (done.has(id)) return;
    stack.add(id);
    for (const dep of byId.get(id)?.dependsOn || [])
      if (byId.has(dep)) visit(dep, stack, done);
    stack.delete(id);
    done.add(id);
  };
  const done = new Set();
  for (const id of ids) visit(id, new Set(), done);
  for (const t of queue.tickets) {
    if (
      t.status === "in_progress" &&
      (!t.worktree || !path.isAbsolute(t.worktree))
    )
      errors.push(t.id + ": active writer missing absolute checkout");
    if (t.status === "in_progress" || t.status === "done") {
      for (const dep of t.dependsOn || []) {
        if (byId.has(dep) && byId.get(dep).status !== "done")
          errors.push(t.id + ": prerequisite not accepted " + dep);
      }
    }
  }
  const active = queue.tickets.filter((t) => t.status === "in_progress");
  if (active.length > queue.maxParallelWriters)
    errors.push("too many simultaneous active writers");
  for (let i = 0; i < active.length; i++)
    for (let j = i + 1; j < active.length; j++) {
      const a = active[i],
        b = active[j];
      const shared = (a.exclusiveResources || []).filter(
        (x) =>
          !SHARED_HOTSPOTS.has(x) && (b.exclusiveResources || []).includes(x),
      );
      if (shared.length)
        errors.push(
          "concurrent resource collision: " +
            a.id +
            "/" +
            b.id +
            " " +
            shared.join(","),
        );
      if (a.worktree && b.worktree && a.worktree === b.worktree)
        errors.push("concurrent checkout collision: " + a.id + "/" + b.id);
    }
  return errors;
}

if (
  process.argv[1] &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const f = path.join(REPO_ROOT, "docs/plan/tickets/v2/queue.json");
  const q = JSON.parse(fs.readFileSync(f, "utf8"));
  const errors = validateQueue(q);
  if (errors.length) {
    console.error(errors.map((x) => "FAIL: " + x).join("\n"));
    process.exitCode = 1;
  } else
    console.log(
      "V2 execution queue: " +
        q.tickets.length +
        " tickets, DAG/doc/resource guards PASS",
    );
}
