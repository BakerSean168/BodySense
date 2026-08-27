#!/usr/bin/env node
import path from "node:path";
import {
  createR2Client,
  normalizePublicBase,
  parseCli,
  readJson,
  requireR2Config,
  uploadImmutableFiles,
  uploadJsonObject,
  verifyPublicUrls,
} from "./lib.mjs";

const args = parseCli(process.argv.slice(2));
const version = args.get("version") ?? "1.4.0";
const releaseRoot = path.resolve(
  args.get("root") ?? `.tmp/anatomy/vanatome/${version}`,
);
const manifestPath = path.resolve(
  args.get("manifest") ?? `scripts/anatomy/vanatome-${version}.manifest.json`,
);
const dryRun = args.get("dry-run") === "true";
const manifest = await readJson(manifestPath);
if (manifest.atlas?.version !== version) {
  throw new Error(
    `atlas manifest version mismatch: ${manifest.atlas?.version} != ${version}`,
  );
}
const prefix = `anatomy/vanatome/${version}`;
const publicRoot = normalizePublicBase(
  args.get("public-base") ?? process.env.STATIC_ASSET_CDN_BASE,
);
const config = dryRun
  ? {
      endpoint: "https://dry-run.invalid",
      accessKeyId: "dry-run",
      secretAccessKey: "dry-run",
      bucket: process.env.R2_BUCKET?.trim() || "bodysense-static",
    }
  : requireR2Config();
const client = dryRun ? null : createR2Client(config);
const files = manifest.files.map((file) => ({
  path: file.path,
  bytes: file.bytes,
  sha256: file.sha256,
}));
const result = await uploadImmutableFiles({
  client,
  bucket: config.bucket,
  sourceRoot: releaseRoot,
  prefix,
  files,
  concurrency: 4,
  dryRun,
});
await uploadJsonObject({
  client,
  bucket: config.bucket,
  key: `${prefix}/release-manifest.json`,
  value: manifest,
  dryRun,
});
if (!dryRun) {
  await verifyPublicUrls(publicRoot, [
    `${prefix}/release-manifest.json`,
    `${prefix}/${manifest.catalogPath}`,
    `${prefix}/models/z-anatomy-${version}-regional-anatomy.glb`,
  ]);
}
console.log(
  `atlas publish ${dryRun ? "plan" : "complete"}: uploaded=${result.uploaded} skipped=${result.skipped} version=${version}`,
);
