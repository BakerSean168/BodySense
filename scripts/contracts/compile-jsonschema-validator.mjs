import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import Ajv2020 from "ajv/dist/2020.js";
import standaloneCode from "ajv/dist/standalone/index.js";
import addFormats from "ajv-formats";

const root = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../..",
);
const schemaPath = path.join(root, "tools/contracts/foundation/schema.json");
const outputPath = path.join(
  root,
  "tools/contracts/generated/jsonschema/validate-foundation.cjs",
);
const schema = JSON.parse(fs.readFileSync(schemaPath, "utf8"));
const ajv = new Ajv2020({
  allErrors: true,
  strict: true,
  strictRequired: true,
  code: { source: true, esm: false },
});
addFormats(ajv);
const validate = ajv.compile(schema);
const code = standaloneCode(ajv, validate);
fs.mkdirSync(path.dirname(outputPath), { recursive: true });
fs.writeFileSync(
  outputPath,
  `/* Code generated from tools/contracts/foundation/schema.json. DO NOT EDIT. */\n${code}`,
);
