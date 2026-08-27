import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";
import test from "node:test";
import {
  assertGitRevision,
  contentTypeFor,
  normalizeAssetBase,
  normalizePublicBase,
} from "./lib.mjs";

const execFileAsync = promisify(execFile);
const revision = "a".repeat(40);

test("normalizes only https public asset bases", () => {
  assert.equal(
    normalizePublicBase("https://assets.example.com///"),
    "https://assets.example.com",
  );
  assert.equal(
    normalizeAssetBase("https://assets.example.com/root"),
    "https://assets.example.com/root/",
  );
  assert.throws(
    () => normalizePublicBase("http://assets.example.com"),
    /must use https/,
  );
});

test("requires full immutable Git revisions", () => {
  assert.equal(assertGitRevision(revision), revision);
  assert.throws(() => assertGitRevision("abc123"), /40-character Git revision/);
});

test("maps release asset content types", () => {
  assert.equal(
    contentTypeFor("assets/a.js"),
    "application/javascript; charset=utf-8",
  );
  assert.equal(contentTypeFor("assets/a.css"), "text/css; charset=utf-8");
  assert.equal(contentTypeFor("models/body.glb"), "model/gltf-binary");
  assert.equal(contentTypeFor("fonts/a.woff2"), "font/woff2");
});

test("manifest + dry-run publisher keep revision-scoped immutable paths", async () => {
  const root = await mkdtemp(path.join(os.tmpdir(), "bodysense-static-test-"));
  try {
    const dist = path.join(root, "dist");
    await mkdir(path.join(dist, "assets"), { recursive: true });
    await writeFile(
      path.join(dist, "assets", "index-abc.js"),
      "console.log('ok');\n",
    );
    await writeFile(path.join(dist, "assets", "index-abc.css"), "body{}\n");
    const baseUrl = `https://assets.example.com/web/${revision}/`;
    await execFileAsync(
      process.execPath,
      [
        "scripts/static-assets/manifest.mjs",
        "--dist",
        dist,
        "--revision",
        revision,
        "--base-url",
        baseUrl,
      ],
      { cwd: process.cwd() },
    );
    const manifest = JSON.parse(
      await readFile(path.join(dist, "static-asset-manifest.json"), "utf8"),
    );
    assert.equal(manifest.prefix, `web/${revision}`);
    assert.equal(manifest.baseUrl, baseUrl);
    assert.deepEqual(
      manifest.files.map((file) => file.path),
      ["assets/index-abc.css", "assets/index-abc.js"],
    );
    const { stdout } = await execFileAsync(
      process.execPath,
      [
        "scripts/static-assets/publish-web-r2.mjs",
        "--dist",
        dist,
        "--dry-run",
        "--public-base",
        "https://assets.example.com",
      ],
      {
        cwd: process.cwd(),
        env: { ...process.env, R2_BUCKET: "bodysense-static" },
      },
    );
    assert.match(stdout, new RegExp(`web/${revision}/assets/index-abc\\.js`));
    assert.doesNotMatch(stdout, /latest/);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("R2 uploader performs verified idempotent S3 PUT/HEAD operations", async () => {
  const http = await import("node:http");
  const { once } = await import("node:events");
  const { createR2Client, describeFile, uploadImmutableFiles } =
    await import("./lib.mjs");
  const objects = new Map();
  const server = http.createServer(async (request, response) => {
    const key = new URL(request.url, "http://fake-s3.local").pathname;
    if (request.method === "HEAD") {
      const object = objects.get(key);
      if (!object) {
        response.statusCode = 404;
        response.end();
        return;
      }
      response.statusCode = 200;
      response.setHeader("content-length", String(object.body.length));
      response.setHeader("x-amz-meta-sha256", object.sha256);
      response.end();
      return;
    }
    if (request.method === "PUT") {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = Buffer.concat(chunks);
      objects.set(key, {
        body,
        sha256: request.headers["x-amz-meta-sha256"],
      });
      response.statusCode = 200;
      response.setHeader("etag", '"test-etag"');
      response.end();
      return;
    }
    response.statusCode = 501;
    response.end();
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.equal(typeof address, "object");
  const root = await mkdtemp(path.join(os.tmpdir(), "bodysense-r2-test-"));
  try {
    await mkdir(path.join(root, "assets"), { recursive: true });
    await writeFile(path.join(root, "assets", "x.js"), "export const x = 1;\n");
    const file = await describeFile(root, "assets/x.js");
    const client = createR2Client({
      endpoint: `http://127.0.0.1:${address.port}`,
      accessKeyId: "test",
      secretAccessKey: "test",
      bucket: "bodysense-static",
    });
    const first = await uploadImmutableFiles({
      client,
      bucket: "bodysense-static",
      sourceRoot: root,
      prefix: `web/${revision}`,
      files: [file],
      concurrency: 1,
    });
    assert.deepEqual(first, { uploaded: 1, skipped: 0 });
    const second = await uploadImmutableFiles({
      client,
      bucket: "bodysense-static",
      sourceRoot: root,
      prefix: `web/${revision}`,
      files: [file],
      concurrency: 1,
    });
    assert.deepEqual(second, { uploaded: 0, skipped: 1 });
    assert.ok(objects.has(`/bodysense-static/web/${revision}/assets/x.js`));
    client.destroy();
  } finally {
    server.close();
    await once(server, "close");
    await rm(root, { recursive: true, force: true });
  }
});
