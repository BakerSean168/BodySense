#!/usr/bin/env node
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { assertRuntimeProtoValidationRules } from "./check-runtime-proto-rules.mjs";

const root = resolve(import.meta.dirname, "../..");
const path = resolve(root, "contracts/internal/agent-runtime/bodysense/runtime/v1/runtime.proto");
const source = readFileSync(path, "utf8");
const rule = '(buf.validate.field).string.pattern = "^consult-config-[0-9a-f]{16}$"';
if (!source.includes(rule)) throw new Error("runtime Proto mutation anchor missing");
const mutated = source.replaceAll(rule, '(buf.validate.field).string.min_len = 1');
let rejected = false;
try {
  assertRuntimeProtoValidationRules(mutated);
} catch {
  rejected = true;
}
if (!rejected) throw new Error("runtime Proto validation-rule mutation was not detected");
console.log("runtime Proto validation mutation: PASS");
