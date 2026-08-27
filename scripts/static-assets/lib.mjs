import { createHash } from "node:crypto";
import { createReadStream } from "node:fs";
import { readdir, readFile, stat } from "node:fs/promises";
import path from "node:path";
import {
  HeadObjectCommand,
  PutObjectCommand,
  S3Client,
} from "@aws-sdk/client-s3";

export const IMMUTABLE_CACHE_CONTROL = "public, max-age=31536000, immutable";

export function parseCli(argv) {
  const args = new Map();
  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (!token.startsWith("--")) continue;
    const key = token.slice(2);
    const next = argv[index + 1];
    if (!next || next.startsWith("--")) {
      args.set(key, "true");
      continue;
    }
    args.set(key, next);
    index += 1;
  }
  return args;
}

export function normalizePublicBase(value) {
  const trimmed = value?.trim();
  if (!trimmed) throw new Error("public base URL is required");
  const parsed = new URL(trimmed);
  if (parsed.protocol !== "https:") {
    throw new Error(`public base URL must use https: ${trimmed}`);
  }
  return parsed.href.replace(/\/+$/, "");
}

export function normalizeAssetBase(value) {
  return `${normalizePublicBase(value)}/`;
}

export function assertGitRevision(revision) {
  if (!/^[0-9a-f]{40}$/i.test(revision ?? "")) {
    throw new Error(
      `expected full 40-character Git revision, got: ${revision}`,
    );
  }
  return revision.toLowerCase();
}

export async function walkFiles(root) {
  const result = [];
  async function visit(relative) {
    const absolute = path.join(root, relative);
    const entries = await readdir(absolute, { withFileTypes: true });
    for (const entry of entries) {
      const next = path.posix.join(
        relative.split(path.sep).join("/"),
        entry.name,
      );
      if (entry.isDirectory()) await visit(next);
      else if (entry.isFile()) result.push(next);
    }
  }
  await visit("");
  return result.sort();
}

export async function sha256File(filePath) {
  const hash = createHash("sha256");
  await new Promise((resolve, reject) => {
    const stream = createReadStream(filePath);
    stream.on("data", (chunk) => hash.update(chunk));
    stream.on("error", reject);
    stream.on("end", resolve);
  });
  return hash.digest("hex");
}

export async function describeFile(root, relativePath) {
  const normalized = relativePath.split(path.sep).join("/");
  if (normalized.startsWith("../") || normalized.includes("/../")) {
    throw new Error(`file escapes publication root: ${relativePath}`);
  }
  const absolute = path.join(root, normalized);
  const info = await stat(absolute);
  if (!info.isFile()) throw new Error(`not a file: ${absolute}`);
  return {
    path: normalized,
    bytes: info.size,
    sha256: await sha256File(absolute),
  };
}

export function contentTypeFor(filePath) {
  const ext = path.extname(filePath).toLowerCase();
  switch (ext) {
    case ".js":
    case ".mjs":
      return "application/javascript; charset=utf-8";
    case ".css":
      return "text/css; charset=utf-8";
    case ".json":
      return "application/json; charset=utf-8";
    case ".glb":
      return "model/gltf-binary";
    case ".svg":
      return "image/svg+xml";
    case ".png":
      return "image/png";
    case ".jpg":
    case ".jpeg":
      return "image/jpeg";
    case ".webp":
      return "image/webp";
    case ".woff":
      return "font/woff";
    case ".woff2":
      return "font/woff2";
    case ".ttf":
      return "font/ttf";
    case ".txt":
      return "text/plain; charset=utf-8";
    default:
      return "application/octet-stream";
  }
}

export function requireR2Config(env = process.env) {
  const endpoint = env.R2_ENDPOINT?.trim();
  const accessKeyId = env.R2_ACCESS_KEY_ID?.trim();
  const secretAccessKey = env.R2_SECRET_ACCESS_KEY?.trim();
  const bucket = env.R2_BUCKET?.trim();
  const missing = [
    ["R2_ENDPOINT", endpoint],
    ["R2_ACCESS_KEY_ID", accessKeyId],
    ["R2_SECRET_ACCESS_KEY", secretAccessKey],
    ["R2_BUCKET", bucket],
  ]
    .filter(([, value]) => !value)
    .map(([name]) => name);
  if (missing.length > 0) {
    throw new Error(`missing R2 configuration: ${missing.join(", ")}`);
  }
  return { endpoint, accessKeyId, secretAccessKey, bucket };
}

export function createR2Client(config) {
  return new S3Client({
    region: "auto",
    endpoint: config.endpoint,
    // R2's account-scoped S3 endpoint supports path-style bucket addressing;
    // forcing it keeps behavior deterministic across AWS SDK endpoint heuristics.
    forcePathStyle: true,
    // We already carry our own SHA-256 metadata and verify it with HeadObject.
    // Avoid optional SDK checksum trailers that some S3-compatible providers do
    // not implement identically to AWS S3.
    requestChecksumCalculation: "WHEN_REQUIRED",
    responseChecksumValidation: "WHEN_REQUIRED",
    credentials: {
      accessKeyId: config.accessKeyId,
      secretAccessKey: config.secretAccessKey,
    },
  });
}

export async function uploadImmutableFiles({
  client,
  bucket,
  sourceRoot,
  prefix,
  files,
  concurrency = 6,
  dryRun = false,
}) {
  const queue = [...files];
  let uploaded = 0;
  let skipped = 0;

  async function processFile(file) {
    const key = path.posix.join(prefix, file.path);
    if (dryRun) {
      console.log(`[dry-run] ${file.path} -> s3://${bucket}/${key}`);
      return;
    }

    const existing = await headIfExists(client, bucket, key);
    if (
      existing &&
      Number(existing.ContentLength) === Number(file.bytes) &&
      existing.Metadata?.sha256 === file.sha256
    ) {
      skipped += 1;
      return;
    }

    const absolute = path.join(sourceRoot, file.path);
    await client.send(
      new PutObjectCommand({
        Bucket: bucket,
        Key: key,
        Body: createReadStream(absolute),
        ContentLength: file.bytes,
        ContentType: contentTypeFor(file.path),
        CacheControl: IMMUTABLE_CACHE_CONTROL,
        Metadata: {
          sha256: file.sha256,
        },
      }),
    );

    const head = await client.send(
      new HeadObjectCommand({ Bucket: bucket, Key: key }),
    );
    if (
      Number(head.ContentLength) !== Number(file.bytes) ||
      head.Metadata?.sha256 !== file.sha256
    ) {
      throw new Error(`R2 verification failed for ${key}`);
    }
    uploaded += 1;
  }

  const workers = Array.from(
    { length: Math.max(1, Math.min(concurrency, files.length || 1)) },
    async () => {
      while (queue.length > 0) {
        const file = queue.shift();
        if (file) await processFile(file);
      }
    },
  );
  await Promise.all(workers);
  return { uploaded, skipped };
}

export async function uploadJsonObject({
  client,
  bucket,
  key,
  value,
  dryRun = false,
}) {
  const body = `${JSON.stringify(value, null, 2)}\n`;
  const sha256 = createHash("sha256").update(body).digest("hex");
  if (dryRun) {
    console.log(`[dry-run] manifest -> s3://${bucket}/${key}`);
    return { bytes: Buffer.byteLength(body), sha256 };
  }
  await client.send(
    new PutObjectCommand({
      Bucket: bucket,
      Key: key,
      Body: body,
      ContentLength: Buffer.byteLength(body),
      ContentType: "application/json; charset=utf-8",
      CacheControl: IMMUTABLE_CACHE_CONTROL,
      Metadata: { sha256 },
    }),
  );
  const head = await client.send(
    new HeadObjectCommand({ Bucket: bucket, Key: key }),
  );
  if (head.Metadata?.sha256 !== sha256) {
    throw new Error(`R2 verification failed for ${key}`);
  }
  return { bytes: Buffer.byteLength(body), sha256 };
}

export async function verifyPublicUrls(
  baseUrl,
  paths,
  { concurrency = 8 } = {},
) {
  const base = normalizePublicBase(baseUrl);
  const queue = [...paths];
  const failures = [];
  const workers = Array.from(
    { length: Math.max(1, Math.min(concurrency, paths.length || 1)) },
    async () => {
      while (queue.length > 0) {
        const relative = queue.shift();
        if (!relative) continue;
        const url = `${base}/${relative.replace(/^\/+/, "")}`;
        try {
          const response = await fetch(url, {
            method: "HEAD",
            redirect: "follow",
          });
          if (!response.ok) failures.push(`${response.status} ${url}`);
        } catch (error) {
          failures.push(
            `${url}: ${error instanceof Error ? error.message : String(error)}`,
          );
        }
      }
    },
  );
  await Promise.all(workers);
  if (failures.length > 0) {
    throw new Error(`public CDN verification failed:\n${failures.join("\n")}`);
  }
}

export async function readJson(filePath) {
  return JSON.parse(await readFile(filePath, "utf8"));
}

async function headIfExists(client, bucket, key) {
  try {
    return await client.send(
      new HeadObjectCommand({ Bucket: bucket, Key: key }),
    );
  } catch (error) {
    if (error?.$metadata?.httpStatusCode === 404 || error?.name === "NotFound")
      return null;
    throw error;
  }
}
