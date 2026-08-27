#!/usr/bin/env node
import { loadManifest, parseCli, syncRelease } from "./atlas-release.mjs";

const args = parseCli(process.argv.slice(2));
const version = args.get("version") ?? "1.4.0";
const output = args.get("output");
if (!output)
  throw new Error(
    "usage: node scripts/anatomy/sync.mjs --version 1.4.0 --output <release-root> [--concurrency 4]",
  );
const concurrency = Number(args.get("concurrency") ?? "4");
const { manifest } = await loadManifest(version);
await syncRelease({ manifest, outputRoot: output, concurrency });
