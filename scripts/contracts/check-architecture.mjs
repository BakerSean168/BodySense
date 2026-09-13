import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../..",
);
const roots = ["apps", "packages"];
const violations = [];

function walk(rel) {
  for (const entry of fs.readdirSync(path.join(root, rel), {
    withFileTypes: true,
  })) {
    const child = path.posix.join(rel, entry.name);
    if (entry.isDirectory()) {
      if (["node_modules", ".venv", "dist", "generated"].includes(entry.name))
        continue;
      walk(child);
      continue;
    }
    if (!/\.(go|py|ts|tsx)$/.test(entry.name)) continue;
    const text = fs.readFileSync(path.join(root, child), "utf8");
    if (text.includes("experiments/contract-codegen")) violations.push(child);
  }
}
for (const rel of roots) walk(rel);
if (violations.length) {
  throw new Error(
    `production source imports/references spike experiment paths:\n${violations.join("\n")}`,
  );
}
console.log("contract architecture foundation guard: PASS");
