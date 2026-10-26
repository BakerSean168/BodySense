#!/usr/bin/env node
import fs from "node:fs";
import { spawnSync } from "node:child_process";
import {
  MANIFEST_SCHEMA,
  MANIFEST_SCHEMA_V7,
  MANIFEST_SCHEMA_V8,
  validateManifest,
} from "./lib.mjs";

const manifestPath = process.argv[2] ?? process.env.DELIVERY_MANIFEST_PATH;
if (!manifestPath) {
  console.error(
    "usage: run-quality.mjs <delivery-manifest-v9.json> [--lane web|api|ai|arch|full]",
  );
  process.exit(2);
}
const laneFlagIndex = process.argv.indexOf("--lane");
const onlyLane = laneFlagIndex >= 0 ? process.argv[laneFlagIndex + 1] : null;
const LANE_FLAGS = ["web", "api", "ai", "arch", "full"];
if (onlyLane !== null && !LANE_FLAGS.includes(onlyLane)) {
  console.error(
    `unknown --lane: ${onlyLane} (expected ${LANE_FLAGS.join("|")})`,
  );
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

function runWebLane() {
  nx("@bodysense/web", "lint");
  nx("@bodysense/web", "typecheck");
  nx("@bodysense/web", "test");
  nx("@bodysense/web", "build");
}

function runApiLane() {
  nx("api", "lint");
  nx("api", "test");
  nx("api", "build");
}

function runAiLane() {
  nx("ai-service", "format:check");
  nx("ai-service", "lint");
  nx("ai-service", "typecheck");
  nx("ai-service", "test");
  for (const target of [
    "eval:assessment-evidence-contract",
    "eval:diagnosis",
    "eval:diagnosis-evidence-policy",
    "eval:diagnosis-promotion",
    "eval:treatment",
    "eval:treatment-evidence-gap",
    "eval:treatment-promotion",
  ]) {
    nx("ai-service", target);
  }
  run("bash", ["scripts/validate-litellm-gateway.sh"]);
}

const LANE_RUNNERS = { web: runWebLane, api: runApiLane, ai: runAiLane };

function runArchitectureChecks(full) {
  run("pnpm", ["quality:architecture"]);
  if (full) {
    run("pnpm", ["quality:architecture:test"]);
    run("pnpm", ["quality:capacity:test"]);
  }
}

function skipLane(lane, reason) {
  console.log(`DELIVERY_QUALITY_LANE=SKIP lane=${lane} reason=${reason}`);
}

try {
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  const errors = validateManifest(manifest);
  if (errors.length > 0) throw new Error(errors.join("; "));

  // Only archived manifests used the old full-monolith job. Main v7
  // executes Web / Server / AI / Contracts in parallel, with dedicated
  // architecture and release-safety owners.
  if (
    manifest.lanes.full &&
    ![MANIFEST_SCHEMA, MANIFEST_SCHEMA_V8, MANIFEST_SCHEMA_V7].includes(
      manifest.schema,
    )
  ) {
    // Archived v1-v6 manifests ran the serial monolith. V7 Main uses the
    // parallel per-runtime owners instead of repeating full validation.
    if (onlyLane === null || onlyLane === "full") {
      run("bash", ["scripts/validate-repo.sh"]);
      process.exit(0);
    }
    skipLane(onlyLane, "archived-full-lane");
    process.exit(0);
  }
  if (onlyLane === "full") {
    throw new Error(
      "Current full quality has no serial child: use per-runtime owners",
    );
  }

  // Per-lane mode: each quality child runs exactly one lane and reports
  // its own receipt line. quality:architecture is cross-cutting (TS,
  // Python, and Go boundaries), so it gets its own child (arch) that runs
  // whenever any quality lane is selected.
  if (onlyLane === "arch") {
    if (manifest.lanes.web || manifest.lanes.api || manifest.lanes.ai) {
      runArchitectureChecks(manifest.lanes.full);
      console.log("DELIVERY_QUALITY_LANE=PASS lane=arch");
    } else {
      skipLane("arch", "no-quality-lanes-selected");
    }
    process.exit(0);
  }
  if (onlyLane !== null) {
    if (!manifest.lanes[onlyLane]) {
      skipLane(onlyLane, "lane-not-selected");
      process.exit(0);
    }
    LANE_RUNNERS[onlyLane]();
    console.log(`DELIVERY_QUALITY_LANE=PASS lane=${onlyLane}`);
    process.exit(0);
  }

  let executed = 0;

  if (manifest.lanes.web) {
    executed += 1;
    runWebLane();
  }

  if (manifest.lanes.api) {
    executed += 1;
    runApiLane();
  }

  if (manifest.lanes.ai) {
    executed += 1;
    runAiLane();
  }

  if (executed === 0) {
    console.log("DELIVERY_QUALITY=SKIP reason=no-quality-lanes-selected");
  } else {
    runArchitectureChecks(manifest.lanes.full);
    console.log(
      `DELIVERY_QUALITY=PASS selected=${[
        manifest.lanes.web && "web",
        manifest.lanes.api && "api",
        manifest.lanes.ai && "ai",
      ]
        .filter(Boolean)
        .join(",")}`,
    );
  }
} catch (error) {
  console.error(`delivery quality failed: ${error.message}`);
  process.exit(1);
}
