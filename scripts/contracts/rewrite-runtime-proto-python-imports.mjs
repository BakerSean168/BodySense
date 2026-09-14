#!/usr/bin/env node
import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "../..");
const files = [
  "apps/ai-service/src/generated/runtimeproto/bodysense/runtime/v1/runtime_pb2.py",
  "apps/ai-service/src/generated/runtimeproto/bodysense/runtime/v1/runtime_pb2.pyi",
];
const original = "from buf.validate import validate_pb2";
const replacement = "from src.generated.runtimeproto.buf.validate import validate_pb2";

for (const relative of files) {
  const path = resolve(root, relative);
  const source = readFileSync(path, "utf8");
  const occurrences = source.split(original).length - 1;
  if (occurrences !== 1) {
    throw new Error(`${relative}: expected exactly one Protovalidate import, got ${occurrences}`);
  }
  writeFileSync(path, source.replace(original, replacement));
}
