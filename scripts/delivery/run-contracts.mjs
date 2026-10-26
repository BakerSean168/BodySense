#!/usr/bin/env node
import fs from "node:fs";
import { spawnSync } from "node:child_process";
import { validateManifest } from "./lib.mjs";

// Contract verification runs in its own CI child (contracts-child) so its
// outcome is an independent receipt in the Quality Oracle, separate from the
// unit/integration quality lanes executed by run-quality.mjs. The checks
// themselves are the canonical, service-free contract gates: package lint /
// typecheck / tests plus the contracts:verify toolchain (OpenAPI lint,
// generated-artifact freshness, breaking-change and mutation checks,
// generated-validator conformance fixtures, and the Postman view).

const manifestPath = process.argv[2] ?? process.env.DELIVERY_MANIFEST_PATH;
if (!manifestPath) {
  console.error("usage: run-contracts.mjs <delivery-manifest-v5.json>");
  process.exit(2);
}

function run(command, args = []) {
  console.log(`\n$ ${command} ${args.join(" ")}`);
  const result = spawnSync(command, args, {
    stdio: "inherit",
    env: process.env,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(
      `${command} ${args.join(" ")} failed with exit code ${result.status}`,
    );
  }
}

function nx(project, target) {
  run("pnpm", ["nx", "run", `${project}:${target}`]);
}

try {
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  const errors = validateManifest(manifest);
  if (errors.length > 0) throw new Error(errors.join("; "));

  if (!manifest.lanes.contracts && !manifest.lanes.full) {
    console.log("DELIVERY_CONTRACTS=SKIP reason=no-contract-lanes-selected");
    process.exit(0);
  }

  nx("@bodysense/contracts", "lint");
  nx("@bodysense/contracts", "typecheck");
  nx("@bodysense/contracts", "test");
  run("pnpm", ["contracts:verify"]);
  console.log(
    `DELIVERY_CONTRACTS=PASS selected=${[
      manifest.lanes.contracts && "contracts",
      manifest.lanes.full && "full",
    ]
      .filter(Boolean)
      .join(",")}`,
  );
} catch (error) {
  console.error(`delivery contracts failed: ${error.message}`);
  process.exit(1);
}
