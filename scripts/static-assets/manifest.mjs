#!/usr/bin/env node
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import {
  assertGitRevision,
  describeFile,
  normalizeAssetBase,
  parseCli,
  walkFiles,
} from "./lib.mjs";

const args = parseCli(process.argv.slice(2));
const distRoot = path.resolve(args.get("dist") ?? "apps/web/dist");
const assetsRoot = path.join(distRoot, "assets");
const revision = assertGitRevision(args.get("revision"));
const baseUrl = normalizeAssetBase(args.get("base-url"));
const output = path.resolve(
  args.get("output") ?? path.join(distRoot, "static-asset-manifest.json"),
);

const assetFiles = await walkFiles(assetsRoot);
if (assetFiles.length === 0)
  throw new Error(`no Vite assets found under ${assetsRoot}`);
const files = [];
for (const relative of assetFiles) {
  files.push(await describeFile(distRoot, path.posix.join("assets", relative)));
}

const manifest = {
  schemaVersion: 1,
  revision,
  baseUrl,
  prefix: `web/${revision}`,
  files,
  totals: {
    fileCount: files.length,
    bytes: files.reduce((sum, file) => sum + file.bytes, 0),
  },
};

await mkdir(path.dirname(output), { recursive: true });
await writeFile(output, `${JSON.stringify(manifest, null, 2)}\n`);
console.log(
  `static manifest: ${files.length} files, ${manifest.totals.bytes} bytes -> ${output}`,
);
