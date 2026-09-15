import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../..",
);
const manifest = JSON.parse(
  fs.readFileSync(path.join(root, "contracts/toolchain.json"), "utf8"),
);
const pkg = JSON.parse(
  fs.readFileSync(path.join(root, "package.json"), "utf8"),
);
for (const [name, expected] of Object.entries(manifest.javascript)) {
  const actual = pkg.devDependencies?.[name] ?? pkg.dependencies?.[name];
  if (actual !== expected) {
    throw new Error(
      `contract tool ${name} must be pinned exactly to ${expected}; package.json has ${actual ?? "missing"}`,
    );
  }
}

const inspected = [
  "scripts/contracts/bootstrap-tools.sh",
  "tools/contracts/foundation/proto/buf.gen.yaml",
];
for (const rel of inspected) {
  const text = fs.readFileSync(path.join(root, rel), "utf8");
  if (/(@|:)latest\b/.test(text))
    throw new Error(`floating latest is forbidden in ${rel}`);
}
console.log(
  `contract toolchain pins verified: ${Object.keys(manifest.javascript).length} JS packages`,
);
