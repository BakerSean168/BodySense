import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";

const repoRoot = process.cwd();
const lifecycle = path.join(
  repoRoot,
  "scripts/validation/validator-lifecycle.sh",
);

async function withFakeDocker(fn) {
  const root = await mkdtemp(
    path.join(tmpdir(), "bodysense-validator-lifecycle-"),
  );
  const bin = path.join(root, "bin");
  const log = path.join(root, "docker.log");
  await mkdir(bin);
  await writeFile(
    path.join(bin, "docker"),
    `#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' "$*" >> "$DOCKER_LOG"\nif [[ "$1 $2" == "image ls" ]]; then\n  printf '%s\\n' image-a image-b image-a\nfi\n`,
    { mode: 0o755 },
  );
  try {
    await fn({ root, bin, log });
  } finally {
    await rm(root, { recursive: true, force: true });
  }
}

function runLifecycle({ bin, log, script }) {
  return spawnSync("bash", ["-c", script], {
    cwd: repoRoot,
    env: {
      ...process.env,
      PATH: `${bin}:${process.env.PATH}`,
      DOCKER_LOG: log,
      LIFECYCLE: lifecycle,
      REPO_ROOT: repoRoot,
    },
    encoding: "utf8",
  });
}

test("cleanup removes only the validator project stack, project images, and its builder", async () => {
  await withFakeDocker(async ({ bin, log }) => {
    const result = runLifecycle({
      bin,
      log,
      script: `source "$LIFECYCLE"; validator_cleanup "$REPO_ROOT" bodysense-validator-test bodysense-validator-test-builder 1 0`,
    });
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /VALIDATOR_TEARDOWN=PASS/);

    const calls = await readFile(log, "utf8");
    assert.match(
      calls,
      /compose .* -p bodysense-validator-test down -v --remove-orphans/,
    );
    assert.match(
      calls,
      /image ls --filter label=com\.docker\.compose\.project=bodysense-validator-test/,
    );
    assert.match(calls, /image rm -f image-a image-b/);
    assert.match(calls, /buildx rm -f bodysense-validator-test-builder/);
    assert.doesNotMatch(calls, /volume prune|system prune|container prune/);
  });
});

test("KEEP_VALIDATOR_STACK leaves runtime resources untouched for diagnosis", async () => {
  await withFakeDocker(async ({ bin, log }) => {
    const result = runLifecycle({
      bin,
      log,
      script: `source "$LIFECYCLE"; validator_cleanup "$REPO_ROOT" bodysense-validator-test bodysense-validator-test-builder 1 1`,
    });
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /VALIDATOR_TEARDOWN=SKIP/);
    await assert.rejects(readFile(log, "utf8"), /ENOENT/);
  });
});

test("local deploy uses a unique project and an isolated builder before runtime build", async () => {
  const script = await readFile(
    path.join(repoRoot, "scripts/local-deploy-validate.sh"),
    "utf8",
  );
  assert.match(script, /bodysense-validator-\$\{revision\}-\$\{run_token\}/);
  assert.match(script, /validator_create_builder "\$builder"/);
  assert.match(
    script,
    /build --builder "\$builder" api ai-service document-service web/,
  );
  assert.match(script, /trap finish_validation EXIT/);
  assert.doesNotMatch(script, /docker (system|volume|container) prune/);
});
