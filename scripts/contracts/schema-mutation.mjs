import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import Ajv2020 from "ajv/dist/2020.js";

const root = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../..",
);
const schema = JSON.parse(
  fs.readFileSync(
    path.join(root, "tools/contracts/foundation/schema.json"),
    "utf8",
  ),
);
const validate = new Ajv2020({ strict: true }).compile(schema);

const cases = [
  [{ kind: "foundation", revision: 0 }, true],
  [{ kind: "foundation", revision: -1 }, false],
  [{ kind: "wrong", revision: 0 }, false],
  [{ kind: "foundation", revision: 0, unknown: true }, false],
  [{ kind: "foundation" }, false],
];
for (const [value, expected] of cases) {
  const actual = validate(value);
  if (actual !== expected) {
    throw new Error(
      `JSON Schema mutation expectation failed: expected=${expected} value=${JSON.stringify(value)}`,
    );
  }
}
console.log(`JSON Schema semantic probes: ${cases.length} PASS`);
