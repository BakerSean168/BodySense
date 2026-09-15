import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');

function fixtureTree() {
  const fixture = fs.mkdtempSync(path.join(os.tmpdir(), 'bodysense-architecture-fixture-'));
  fs.mkdirSync(path.join(fixture, 'apps/web/src/features/profile/components'), { recursive: true });
  fs.mkdirSync(path.join(fixture, 'apps/ai-service/src/runtime'), { recursive: true });
  fs.mkdirSync(path.join(fixture, 'apps/api/internal/service'), { recursive: true });
  return fixture;
}

function run(command, args) {
  return spawnSync(command, args, {
    cwd: root,
    encoding: 'utf8',
    env: process.env,
  });
}

function combinedOutput(result) {
  return `${result.stdout ?? ''}\n${result.stderr ?? ''}`;
}

test('TypeScript architecture gate exits non-zero for generated API leakage', () => {
  const fixture = fixtureTree();
  try {
    fs.writeFileSync(
      path.join(fixture, 'apps/web/src/features/profile/components/Leaky.tsx'),
      'import { getUserProfile } from "@/generated/api/bodysense";\nexport const Leaky = () => null;\n',
    );
    const result = run('node', ['scripts/quality/check-ts-boundaries.mjs', '--root', fixture]);
    assert.notEqual(result.status, 0);
    assert.match(combinedOutput(result), /generated browser API leaked outside an approved trust-boundary adapter/);
  } finally {
    fs.rmSync(fixture, { recursive: true, force: true });
  }
});

test('Python architecture gate exits non-zero for generated runtime Proto leakage', () => {
  const fixture = fixtureTree();
  try {
    fs.writeFileSync(
      path.join(fixture, 'apps/ai-service/src/runtime/leaky.py'),
      'from src.generated.runtimeproto.bodysense.runtime.v1 import runtime_pb2\n',
    );
    const result = run('python3', [
      'scripts/quality/check_python_boundaries.py',
      '--root',
      fixture,
    ]);
    assert.notEqual(result.status, 0);
    assert.match(combinedOutput(result), /generated runtime Proto leaked outside the Python boundary adapter/);
  } finally {
    fs.rmSync(fixture, { recursive: true, force: true });
  }
});

test('Go architecture gate exits non-zero for generated OpenAPI leakage', () => {
  const fixture = fixtureTree();
  try {
    fs.writeFileSync(
      path.join(fixture, 'apps/api/internal/service/leaky.go'),
      'package service\nimport openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"\nvar _ = openapiv1.GetSpec\n',
    );
    const result = run('go', [
      'run',
      'scripts/quality/check_go_boundaries.go',
      '-root',
      fixture,
    ]);
    assert.notEqual(result.status, 0);
    assert.match(combinedOutput(result), /generated OpenAPI types leaked outside HTTP transport boundary/);
  } finally {
    fs.rmSync(fixture, { recursive: true, force: true });
  }
});
