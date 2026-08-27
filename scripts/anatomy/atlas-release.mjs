import { createHash } from "node:crypto";
import { createReadStream } from "node:fs";
import {
  copyFile,
  mkdir,
  open,
  readFile,
  rename,
  rm,
  stat,
} from "node:fs/promises";
import { dirname, join, posix, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { Readable } from "node:stream";

const HERE = dirname(fileURLToPath(import.meta.url));
const DEFAULT_CONCURRENCY = 4;
const USER_AGENT = "BodySense-atlas-release/1";

function fail(message) {
  throw new Error(message);
}

function assert(condition, message) {
  if (!condition) fail(message);
}

export function parseCli(argv) {
  const values = new Map();
  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (!token.startsWith("--")) fail(`unexpected argument: ${token}`);
    const key = token.slice(2);
    const value = argv[i + 1];
    if (!value || value.startsWith("--")) fail(`missing value for --${key}`);
    values.set(key, value);
    i += 1;
  }
  return values;
}

export async function loadManifest(version) {
  assert(/^\d+\.\d+\.\d+$/.test(version), `invalid atlas version: ${version}`);
  const path = join(HERE, `vanatome-${version}.manifest.json`);
  const manifest = JSON.parse(await readFile(path, "utf8"));
  validateManifest(manifest, version);
  return { manifest, path };
}

function validateManifest(manifest, expectedVersion) {
  assert(manifest.schemaVersion === 1, "unsupported anatomy manifest schema");
  assert(manifest.provider === "vanatome", "unexpected anatomy provider");
  assert(
    manifest.atlas?.version === expectedVersion,
    "manifest atlas version mismatch",
  );
  assert(manifest.atlas?.id === "vanatome-human", "unexpected atlas id");
  assert(
    manifest.integrityAuthority === "sha256",
    "SHA-256 must be the integrity authority",
  );
  assert(
    Array.isArray(manifest.files) && manifest.files.length > 0,
    "manifest has no files",
  );
  assert(
    manifest.inventory?.fileCount === manifest.files.length,
    "manifest fileCount mismatch",
  );
  assert(
    !manifest.publicRoot.includes("/latest/"),
    "publicRoot must never use /latest/",
  );
  assert(
    manifest.publicRoot.includes(`/${expectedVersion}`),
    "publicRoot must be version-scoped",
  );
  assert(
    !manifest.catalogPublicUrl.includes("?"),
    "catalog public URL must not contain a query string",
  );
  assert(
    !manifest.catalogPublicUrl.includes("/latest/"),
    "catalog public URL must never use /latest/",
  );

  const seenPaths = new Set();
  let totalBytes = 0;
  for (const file of manifest.files) {
    assert(
      typeof file.path === "string" && file.path.length > 0,
      "manifest file path missing",
    );
    assert(
      !file.path.startsWith("/") && !file.path.includes(".."),
      `unsafe manifest path: ${file.path}`,
    );
    assert(!seenPaths.has(file.path), `duplicate manifest path: ${file.path}`);
    seenPaths.add(file.path);
    assert(
      Number.isInteger(file.bytes) && file.bytes >= 0,
      `invalid byte count: ${file.path}`,
    );
    assert(/^[a-f0-9]{64}$/.test(file.sha256), `invalid sha256: ${file.path}`);
    const source = new URL(file.sourceUrl);
    assert(
      source.protocol === "https:",
      `non-HTTPS atlas source: ${file.sourceUrl}`,
    );
    assert(
      source.hostname === "atlas.vanatome.vixotic.in",
      `unexpected atlas source host: ${file.sourceUrl}`,
    );
    assert(
      source.search === "" && source.hash === "",
      `atlas source URL must be data-free: ${file.sourceUrl}`,
    );
    assert(
      !source.pathname.includes("/latest/"),
      `upstream /latest/ is forbidden: ${file.sourceUrl}`,
    );
    totalBytes += file.bytes;
  }
  assert(
    totalBytes === manifest.inventory.totalBytes,
    "manifest totalBytes mismatch",
  );
  assert(seenPaths.has(manifest.catalogPath), "catalogPath is not in manifest");
}

async function sha256File(path) {
  const hash = createHash("sha256");
  for await (const chunk of createReadStream(path)) hash.update(chunk);
  return hash.digest("hex");
}

async function fetchWithRetry(url, attempts = 3) {
  let lastError;
  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    try {
      const response = await fetch(url, {
        redirect: "follow",
        headers: { "User-Agent": USER_AGENT },
        signal: AbortSignal.timeout(120_000),
      });
      if (!response.ok)
        fail(`HTTP ${response.status} ${response.statusText} for ${url}`);
      return response;
    } catch (error) {
      lastError = error;
      if (attempt < attempts)
        await new Promise((resolveDelay) =>
          setTimeout(resolveDelay, attempt * 750),
        );
    }
  }
  throw lastError;
}

async function downloadFile(file, outputRoot) {
  const destination = resolve(outputRoot, file.path);
  const expectedPrefix = `${resolve(outputRoot)}/`;
  assert(
    destination.startsWith(expectedPrefix),
    `destination escapes release root: ${file.path}`,
  );
  await mkdir(dirname(destination), { recursive: true });
  const temp = `${destination}.partial`;
  await rm(temp, { force: true });

  const response = await fetchWithRetry(file.sourceUrl);
  const contentLength = response.headers.get("content-length");
  if (contentLength !== null) {
    assert(
      Number(contentLength) === file.bytes,
      `Content-Length mismatch for ${file.path}: expected ${file.bytes}, got ${contentLength}`,
    );
  }
  assert(response.body, `empty response body for ${file.path}`);

  const handle = await open(temp, "w");
  const hash = createHash("sha256");
  let bytes = 0;
  try {
    for await (const chunk of Readable.fromWeb(response.body)) {
      const buffer = Buffer.from(chunk);
      bytes += buffer.length;
      hash.update(buffer);
      let offset = 0;
      while (offset < buffer.length) {
        const { bytesWritten } = await handle.write(buffer, offset);
        assert(bytesWritten > 0, `short write while storing ${file.path}`);
        offset += bytesWritten;
      }
    }
  } finally {
    await handle.close();
  }

  const digest = hash.digest("hex");
  if (bytes !== file.bytes || digest !== file.sha256) {
    await rm(temp, { force: true });
    fail(
      `integrity mismatch for ${file.path}: bytes ${bytes}/${file.bytes}, sha256 ${digest}/${file.sha256}`,
    );
  }
  await rename(temp, destination);
  process.stdout.write(`FETCHED ${file.path} ${bytes} ${digest}\n`);
}

async function copyRedistributionNotices(outputRoot) {
  const notices = [
    ["notices/BODYSENSE_PROVENANCE.txt", "BODYSENSE_PROVENANCE.txt"],
    ["notices/CC-BY-SA-4.0.txt", "LICENSES/CC-BY-SA-4.0.txt"],
  ];
  for (const [sourceRelative, destinationRelative] of notices) {
    const source = join(HERE, sourceRelative);
    const destination = join(outputRoot, destinationRelative);
    await mkdir(dirname(destination), { recursive: true });
    await copyFile(source, destination);
  }
}

export async function syncRelease({
  manifest,
  outputRoot,
  concurrency = DEFAULT_CONCURRENCY,
}) {
  assert(
    Number.isInteger(concurrency) && concurrency > 0 && concurrency <= 16,
    `invalid concurrency: ${concurrency}`,
  );
  await rm(outputRoot, { recursive: true, force: true });
  await mkdir(outputRoot, { recursive: true });

  const queue = [...manifest.files];
  async function worker() {
    for (;;) {
      const file = queue.shift();
      if (!file) return;
      await downloadFile(file, outputRoot);
    }
  }
  await Promise.all(
    Array.from({ length: Math.min(concurrency, manifest.files.length) }, () =>
      worker(),
    ),
  );
  await copyRedistributionNotices(outputRoot);
  await verifyRelease({ manifest, outputRoot });
}

function resolveCatalogReference(catalogPath, reference) {
  assert(
    typeof reference === "string" && reference.length > 0,
    "empty catalog reference",
  );
  assert(
    !reference.includes("?") && !reference.includes("#"),
    `catalog reference must be data-free: ${reference}`,
  );
  assert(
    !reference.includes("://") && !reference.startsWith("/"),
    `catalog reference must stay relative: ${reference}`,
  );
  const resolved = posix.normalize(
    posix.join(posix.dirname(catalogPath), reference),
  );
  assert(
    !resolved.startsWith("../") && resolved !== "..",
    `catalog reference escapes mirrored release: ${reference}`,
  );
  return resolved;
}

export async function verifyRelease({ manifest, outputRoot }) {
  const byPath = new Map(manifest.files.map((file) => [file.path, file]));
  let verifiedBytes = 0;
  for (const file of manifest.files) {
    const path = join(outputRoot, file.path);
    const info = await stat(path).catch(() => null);
    assert(info?.isFile(), `missing atlas file: ${file.path}`);
    assert(
      info.size === file.bytes,
      `size mismatch for ${file.path}: expected ${file.bytes}, got ${info.size}`,
    );
    const digest = await sha256File(path);
    assert(
      digest === file.sha256,
      `sha256 mismatch for ${file.path}: expected ${file.sha256}, got ${digest}`,
    );
    verifiedBytes += info.size;
  }
  assert(
    verifiedBytes === manifest.inventory.totalBytes,
    "verified byte total mismatch",
  );

  const catalog = JSON.parse(
    await readFile(join(outputRoot, manifest.catalogPath), "utf8"),
  );
  assert(catalog.schemaVersion === 1, "unexpected upstream catalog schema");
  assert(catalog.atlas?.id === manifest.atlas.id, "catalog atlas id mismatch");
  assert(
    catalog.atlas?.version === manifest.atlas.version,
    "catalog atlas version mismatch",
  );
  assert(
    catalog.atlas?.buildId === manifest.atlas.buildId,
    "catalog build id mismatch",
  );
  assert(
    catalog.provenance?.licenseName === "CC BY-SA 4.0",
    "catalog atlas license mismatch",
  );

  for (const bundle of catalog.bundles ?? []) {
    const modelPath = resolveCatalogReference(
      manifest.catalogPath,
      bundle.modelUrl,
    );
    const metadataPath = resolveCatalogReference(
      manifest.catalogPath,
      bundle.metadataUrl,
    );
    const modelFile = byPath.get(modelPath);
    assert(
      modelFile,
      `catalog model is not pinned in manifest: ${bundle.modelUrl}`,
    );
    assert(
      byPath.has(metadataPath),
      `catalog metadata is not pinned in manifest: ${bundle.metadataUrl}`,
    );
    assert(
      modelFile.bytes === bundle.bytes,
      `catalog model bytes disagree with manifest: ${modelPath}`,
    );
    assert(
      modelFile.sha256 === bundle.sha256,
      `catalog model sha256 disagrees with manifest: ${modelPath}`,
    );
  }
  const noticePath = resolveCatalogReference(
    manifest.catalogPath,
    catalog.provenance.noticeUrl,
  );
  assert(
    byPath.has(noticePath),
    `catalog attribution notice is not pinned in manifest: ${catalog.provenance.noticeUrl}`,
  );

  for (const required of [
    "BODYSENSE_PROVENANCE.txt",
    "LICENSES/CC-BY-SA-4.0.txt",
  ]) {
    const info = await stat(join(outputRoot, required)).catch(() => null);
    assert(
      info?.isFile() && info.size > 0,
      `missing redistribution notice: ${required}`,
    );
  }

  process.stdout.write(
    `ATLAS_VERIFY=PASS version=${manifest.atlas.version} files=${manifest.inventory.fileCount} bytes=${verifiedBytes} catalog=${manifest.catalogPublicUrl}\n`,
  );
}
