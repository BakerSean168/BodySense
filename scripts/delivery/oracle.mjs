#!/usr/bin/env node
import fs from "node:fs";
import { evaluateManifestOracle, evaluateOracle } from "./lib.mjs";

function readInput() {
  const fileIndex = process.argv.indexOf("--input");
  if (fileIndex >= 0) {
    const file = process.argv[fileIndex + 1];
    if (!file) throw new Error("--input requires a file path");
    return JSON.parse(fs.readFileSync(file, "utf8"));
  }
  if (process.env.ORACLE_INPUT) return JSON.parse(process.env.ORACLE_INPUT);
  throw new Error("provide --input <json-file> or ORACLE_INPUT");
}

function argValue(name) {
  const index = process.argv.indexOf(`--${name}`);
  return index >= 0 ? process.argv[index + 1] : undefined;
}

function evaluateManifestMode() {
  const manifestFile = argValue("manifest");
  const group = argValue("group");
  if (!group) throw new Error("--group is required with --manifest");
  const manifest = JSON.parse(fs.readFileSync(manifestFile, "utf8"));
  const expectedDigest = argValue("expect-digest");
  if (expectedDigest && manifest.digest !== expectedDigest) {
    console.error(
      `manifest digest does not match expected digest: ${manifest.digest} !== ${expectedDigest}`,
    );
    process.exit(1);
  }
  const childrenFile = argValue("children");
  const children = childrenFile
    ? JSON.parse(fs.readFileSync(childrenFile, "utf8"))
    : JSON.parse(argValue("children-json") ?? "{}");
  return evaluateManifestOracle({
    manifest,
    group,
    detector: argValue("detector") ?? "success",
    children,
  });
}

try {
  if (argValue("manifest")) {
    const result = evaluateManifestMode();
    if (!result.ok) {
      for (const failure of result.failures) console.error(failure);
      process.exit(1);
    }
    console.log("DELIVERY_ORACLE=PASS");
    process.exit(0);
  }
  const input = readInput();
  const result = evaluateOracle(input);
  if (!result.ok) {
    for (const failure of result.failures) console.error(failure);
    process.exit(1);
  }
  console.log("DELIVERY_ORACLE=PASS");
} catch (error) {
  console.error(`delivery oracle failed: ${error.message}`);
  process.exit(1);
}
