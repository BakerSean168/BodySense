#!/usr/bin/env node
import path from "node:path";
import {
  assertGitRevision,
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
const distRoot = path.resolve(args.get("dist") ?? "apps/web/dist");
const manifestPath = path.resolve(
  args.get("manifest") ?? path.join(distRoot, "static-asset-manifest.json"),
);
const dryRun = args.get("dry-run") === "true";
const manifest = await readJson(manifestPath);
const revision = assertGitRevision(manifest.revision);
const expectedPrefix = `web/${revision}`;
if (manifest.prefix !== expectedPrefix) {
  throw new Error(
    `manifest prefix mismatch: ${manifest.prefix} != ${expectedPrefix}`,
  );
}
const publicRoot = normalizePublicBase(
  args.get("public-base") ?? process.env.STATIC_ASSET_CDN_BASE,
);
const expectedBase = `${publicRoot}/${expectedPrefix}/`;
if (manifest.baseUrl !== expectedBase) {
  throw new Error(
    `manifest base URL mismatch: ${manifest.baseUrl} != ${expectedBase}`,
  );
}

const config = dryRun
  ? {
      endpoint: "https://dry-run.invalid",
      accessKeyId: "dry-run",
      secretAccessKey: "dry-run",
      bucket: process.env.R2_BUCKET?.trim() || "bodysense-static",
    }
  : requireR2Config();
const client = dryRun ? null : createR2Client(config);
const result = await uploadImmutableFiles({
  client,
  bucket: config.bucket,
  sourceRoot: distRoot,
  prefix: expectedPrefix,
  files: manifest.files,
  dryRun,
});
await uploadJsonObject({
  client,
  bucket: config.bucket,
  key: `${expectedPrefix}/manifest.json`,
  value: manifest,
  dryRun,
});

if (!dryRun) {
  await verifyPublicUrls(publicRoot, [
    `${expectedPrefix}/manifest.json`,
    ...manifest.files.map((file) => `${expectedPrefix}/${file.path}`),
  ]);
}
console.log(
  `web static publish ${dryRun ? "plan" : "complete"}: uploaded=${result.uploaded} skipped=${result.skipped} revision=${revision}`,
);
