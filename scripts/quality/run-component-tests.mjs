#!/usr/bin/env node
import fs from "node:fs";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

// Component browser tests: build the Storybook statically, serve it from a
// throwaway local server, and run @storybook/test-runner (Playwright) against
// it. No API, AI service, or database is started — this is the component
// lane's independence from the full-stack E2E child.

const root = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../..",
);
const port = Number(process.env.COMPONENT_TEST_PORT ?? 6016);
const url = `http://127.0.0.1:${port}`;

const MIME = {
  ".html": "text/html",
  ".js": "text/javascript",
  ".mjs": "text/javascript",
  ".css": "text/css",
  ".json": "application/json",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".svg": "image/svg+xml",
  ".woff": "font/woff",
  ".woff2": "font/woff2",
  ".ttf": "font/ttf",
  ".map": "application/json",
  ".ico": "image/x-icon",
  ".webp": "image/webp",
  ".glb": "model/gltf-binary",
  ".wasm": "application/wasm",
};

function run(command, args, options = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: "inherit", ...options });
    child.on("error", reject);
    child.on("exit", (code) =>
      code === 0
        ? resolve()
        : reject(new Error(`${command} ${args.join(" ")} exited ${code}`)),
    );
  });
}

function serve(directory) {
  const server = http.createServer((request, response) => {
    let pathname = decodeURIComponent(request.url.split("?")[0]);
    if (pathname.endsWith("/")) pathname += "index.html";
    const file = path.join(directory, pathname);
    fs.readFile(file, (error, data) => {
      if (error) {
        response.writeHead(404);
        response.end();
        return;
      }
      response.writeHead(200, {
        "content-type": MIME[path.extname(file)] ?? "application/octet-stream",
      });
      response.end(data);
    });
  });
  return new Promise((resolve) => {
    server.listen(port, "127.0.0.1", () => resolve(server));
  });
}

async function waitForServer(deadlineMs = 30_000) {
  const deadline = Date.now() + deadlineMs;
  for (;;) {
    try {
      const response = await fetch(`${url}/index.json`);
      if (response.ok) return;
    } catch {
      // Server not up yet.
    }
    if (Date.now() > deadline)
      throw new Error(`storybook server did not start at ${url}`);
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
}

let server;
let staticDir;
try {
  staticDir = fs.mkdtempSync(path.join(os.tmpdir(), "bodysense-storybook-"));
  console.log(`building storybook into ${staticDir}`);
  await run("pnpm", ["exec", "storybook", "build", "--output-dir", staticDir], {
    cwd: path.join(root, "apps/web"),
  });
  server = await serve(staticDir);
  await waitForServer();
  await run("pnpm", [
    "exec",
    "test-storybook",
    "--config-dir",
    "apps/web/.storybook",
    "--url",
    url,
  ]);
  console.log("COMPONENT_TESTS=PASS");
} catch (error) {
  console.error(`component tests failed: ${error.message}`);
  process.exitCode = 1;
} finally {
  server?.close();
  if (staticDir) fs.rmSync(staticDir, { recursive: true, force: true });
}
