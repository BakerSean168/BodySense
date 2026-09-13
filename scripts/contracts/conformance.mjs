import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

const root = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../..",
);
const require = createRequire(import.meta.url);
const validatorPath = path.join(
  root,
  "tools/contracts/generated/jsonschema/validate-foundation.cjs",
);
const validate = require(validatorPath);

for (const value of [
  { kind: "foundation", revision: 0 },
  { kind: "foundation", revision: 2, note: null },
]) {
  if (!validate(value))
    throw new Error(
      `generated validator rejected valid fixture: ${JSON.stringify(value)}`,
    );
}
for (const value of [
  { kind: "foundation", revision: -1 },
  { kind: "foundation", revision: 0, extra: true },
]) {
  if (validate(value))
    throw new Error(
      `generated validator accepted invalid fixture: ${JSON.stringify(value)}`,
    );
}

const generated = [
  "tools/contracts/generated/openapi-go/foundation.gen.go",
  "tools/contracts/generated/openapi-web/client.ts",
  "tools/contracts/generated/jsonschema/foundation.d.ts",
  "tools/contracts/generated/jsonschema/validate-foundation.cjs",
  "tools/contracts/generated/proto-go/bodysense/foundation/v1/foundation.pb.go",
  "tools/contracts/generated/proto-python/bodysense/foundation/v1/foundation_pb2.py",
];
for (const rel of generated) {
  if (!fs.existsSync(path.join(root, rel)))
    throw new Error(`missing generated artifact: ${rel}`);
}
console.log(`generated contract artifacts present: ${generated.length}`);
