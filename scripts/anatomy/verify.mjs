#!/usr/bin/env node
import { loadManifest, parseCli, verifyRelease } from "./atlas-release.mjs";

const args = parseCli(process.argv.slice(2));
const version = args.get("version") ?? "1.4.0";
const root = args.get("root");
if (!root)
  throw new Error(
    "usage: node scripts/anatomy/verify.mjs --version 1.4.0 --root <release-root>",
  );
const { manifest } = await loadManifest(version);
await verifyRelease({ manifest, outputRoot: root });
