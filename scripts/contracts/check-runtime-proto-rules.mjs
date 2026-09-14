#!/usr/bin/env node
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

export function assertRuntimeProtoValidationRules(source) {
  const required = [
    ['runtime version const', 'uint32 version = 1 [(buf.validate.field).uint32.const = 1];'],
    ['runtime sequence lower bound', 'uint64 seq = 2 [(buf.validate.field).uint64.gte = 1];'],
    ['typed event oneof required', 'option (buf.validate.oneof).required = true;'],
    ['thread uuid', 'string thread_id = 1 [(buf.validate.field).string.uuid = true];'],
    ['run uuid', 'string run_id = 2 [(buf.validate.field).string.uuid = true];'],
    ['conversation uuid', 'string conversation_id = 3 [(buf.validate.field).string.uuid = true];'],
    ['user uuid', 'string user_id = 4 [(buf.validate.field).string.uuid = true];'],
    ['resume interrupt uuid', 'string interrupt_id = 6 [(buf.validate.field).string.uuid = true];'],
  ];
  for (const [name, needle] of required) {
    if (!source.includes(needle)) throw new Error(`runtime Proto validation rule missing: ${name}`);
  }
  const configurationPattern = '(buf.validate.field).string.pattern = "^consult-config-[0-9a-f]{16}$"';
  const configurationMatches = source.split(configurationPattern).length - 1;
  if (configurationMatches !== 3) {
    throw new Error(`runtime Proto configuration identity rule count=${configurationMatches}, want 3`);
  }
  const eventFields = source.match(/^    [A-Z][A-Za-z0-9]+ [a-z0-9_]+ = (?:1[0-9]|2[0-6]);$/gm) ?? [];
  if (eventFields.length !== 17) {
    throw new Error(`runtime Proto typed event variants=${eventFields.length}, want 17`);
  }
}

const invoked = process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href;
if (invoked) {
  const root = resolve(import.meta.dirname, "../..");
  const path = resolve(root, "contracts/internal/agent-runtime/bodysense/runtime/v1/runtime.proto");
  assertRuntimeProtoValidationRules(readFileSync(path, "utf8"));
  console.log("runtime Proto validation rules: PASS");
}
