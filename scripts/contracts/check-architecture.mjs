import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../..",
);
const roots = ["apps", "packages"];
const violations = [];

const allowedGoRuntimeProtoConsumers = new Set([
  "apps/api/internal/service/runtime_proto_adapter.go",
  "apps/api/internal/service/consultation_runtime_event.go",
  "apps/api/internal/runtimeproto/contract_test.go",
]);
const allowedPythonRuntimeProtoConsumers = new Set([
  "apps/ai-service/src/api/runtime_proto_adapter.py",
  "apps/ai-service/tests/unit/test_runtime_proto_contract.py",
]);

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
    if (text.includes("experiments/contract-codegen")) {
      violations.push(`${child}: references spike experiment path`);
    }
    if (
      child.endsWith(".go") &&
      text.includes("internal/generated/runtimeproto") &&
      !allowedGoRuntimeProtoConsumers.has(child)
    ) {
      violations.push(`${child}: generated runtime Proto leaked past Go boundary adapter`);
    }
    if (
      child.endsWith(".py") &&
      text.includes("generated.runtimeproto") &&
      !allowedPythonRuntimeProtoConsumers.has(child)
    ) {
      violations.push(`${child}: generated runtime Proto leaked past Python boundary adapter`);
    }
    if (
      text.includes("consultationInternalEventChannels") ||
      text.includes("validateConsultationInternalEvent")
    ) {
      violations.push(`${child}: retired generic internal runtime authority resurfaced`);
    }
  }
}
for (const rel of roots) walk(rel);
if (violations.length) {
  throw new Error(
    `contract architecture violations:\n${violations.join("\n")}`,
  );
}
console.log("contract architecture foundation guard: PASS");
