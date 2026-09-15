import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { compile } from "json-schema-to-typescript";
import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";
import standaloneCode from "ajv/dist/standalone/index.js";
import { build } from "esbuild";

const root = process.cwd();
const schemaPath = path.join(
  root,
  "packages/contracts/schemas/stream-event.v1.schema.json",
);
const outputDir = path.join(root, "packages/contracts/generated");
const typesPath = path.join(outputDir, "stream-event.v1.d.ts");
const validatorPath = path.join(outputDir, "stream-event-validator.v1.js");
const validatorTypesPath = path.join(
  outputDir,
  "stream-event-validator.v1.d.ts",
);
const schema = JSON.parse(fs.readFileSync(schemaPath, "utf8"));

fs.mkdirSync(outputDir, { recursive: true });

const types = await compile(schema, schema.title ?? "BodySenseStreamEventV1", {
  bannerComment: [
    "/* eslint-disable */",
    "/**",
    " * Code generated from packages/contracts/schemas/stream-event.v1.schema.json.",
    " * DO NOT EDIT. Run `pnpm contracts:generate` instead.",
    " */",
  ].join("\n"),
  unknownAny: true,
});
if (/\bany\b/.test(types)) {
  throw new Error(
    "generated public StreamEvent types contain `any`; model the schema explicitly or preserve extensibility as `unknown`",
  );
}
fs.writeFileSync(typesPath, types);

const ajv = new Ajv2020({
  allErrors: true,
  strict: true,
  strictRequired: true,
  code: { source: true, esm: true },
});
addFormats(ajv);
ajv.addSchema(schema, "stream-event-v1");
const standalone = standaloneCode(ajv, {
  validateStreamEvent: "stream-event-v1",
});

const tempDir = fs.mkdtempSync(path.join(root, ".tmp-stream-validator-"));
try {
  const standalonePath = path.join(tempDir, "validator.mjs");
  fs.writeFileSync(standalonePath, standalone);
  await build({
    entryPoints: [standalonePath],
    outfile: validatorPath,
    bundle: true,
    platform: "browser",
    format: "esm",
    target: ["es2022"],
    minify: true,
    legalComments: "none",
    logLevel: "silent",
  });
} finally {
  fs.rmSync(tempDir, { recursive: true, force: true });
}

fs.writeFileSync(
  validatorTypesPath,
  `import type { BodySenseStreamEventV1 } from "./stream-event.v1";\n\n` +
    `export interface StreamEventValidationError {\n` +
    `  instancePath: string;\n` +
    `  schemaPath: string;\n` +
    `  keyword: string;\n` +
    `  params: Record<string, unknown>;\n` +
    `  message?: string;\n` +
    `}\n\n` +
    `export interface StreamEventValidator {\n` +
    `  (data: unknown): data is BodySenseStreamEventV1;\n` +
    `  errors: StreamEventValidationError[] | null;\n` +
    `}\n\n` +
    `export const validateStreamEvent: StreamEventValidator;\n`,
);

console.log("public StreamEvent generated artifacts: PASS");
