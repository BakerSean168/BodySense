import assert from 'node:assert/strict';
import test from 'node:test';
import {
  classifyPaths,
  computeManifestDigest,
  createManifest,
  createRunSummary,
  evaluateOracle,
  validateManifest,
} from './lib.mjs';

const A = 'a'.repeat(40);
const B = 'b'.repeat(40);

function manifestFor(paths, options = {}) {
  return createManifest({
    requestedBaseSha: A,
    baseSha: A,
    headSha: B,
    event: options.event ?? 'pull_request',
    ref: options.ref ?? 'refs/pull/1/merge',
    changedPaths: paths,
    forceFull: options.forceFull ?? false,
    generatedAt: options.generatedAt ?? '2026-08-29T00:00:00.000Z',
  });
}

function assertLanes(paths, expected) {
  const actual = classifyPaths(paths).lanes;
  for (const [lane, enabled] of Object.entries(expected)) {
    assert.equal(actual[lane], enabled, `${paths.join(', ')} lane ${lane}`);
  }
}

test('docs-only changes select governance only', () => {
  const result = classifyPaths(['docs/architecture/foo.md']);
  assert.equal(result.risk, 'docs');
  assert.deepEqual(result.lanes, {
    governance: true,
    web: false,
    api: false,
    ai: false,
    contracts: false,
    database: false,
    experience: false,
    full: false,
  });
});

test('Web changes select Web and experience lanes without forcing full', () => {
  const result = classifyPaths(['apps/web/src/features/foo.tsx']);
  assert.equal(result.risk, 'web');
  assertLanes(['apps/web/src/features/foo.tsx'], {
    governance: true,
    web: true,
    experience: true,
    contracts: false,
    api: false,
    ai: false,
    database: false,
    full: false,
  });
});

test('Web E2E changes remain in the Web + experience policy', () => {
  assertLanes(['apps/web/e2e/consultation.spec.ts'], {
    web: true,
    contracts: false,
    experience: true,
  });
});

test('Go API changes select API and experience', () => {
  const result = classifyPaths(['apps/api/internal/service/foo.go']);
  assert.equal(result.risk, 'api');
  assertLanes(['apps/api/internal/service/foo.go'], {
    api: true,
    contracts: false,
    database: false,
    experience: true,
  });
});

test('Python runtime changes select AI and experience', () => {
  const result = classifyPaths(['apps/ai-service/src/runtime/foo.py']);
  assert.equal(result.risk, 'ai');
  assertLanes(['apps/ai-service/src/runtime/foo.py'], {
    ai: true,
    contracts: false,
    experience: true,
    database: false,
  });
});

test('migration changes select database safety lanes', () => {
  const result = classifyPaths(['apps/api/migrations/000060_example.up.sql']);
  assert.equal(result.risk, 'database');
  assertLanes(['apps/api/migrations/000060_example.up.sql'], {
    api: true,
    contracts: false,
    database: true,
    experience: true,
    full: false,
  });
});

test('shared contract changes select every application surface and experience', () => {
  const result = classifyPaths(['packages/contracts/src/foo.ts']);
  assert.equal(result.risk, 'contract');
  assertLanes(['packages/contracts/src/foo.ts'], {
    web: true,
    api: true,
    ai: true,
    contracts: true,
    experience: true,
    database: false,
    full: false,
  });
});

test('CI, Docker, release and unknown paths fail safe to full', () => {
  for (const path of [
    '.github/workflows/ci.yml',
    'docker/docker-compose.prod.yml',
    'release-please-config.json',
    'some-new-root-file.toml',
  ]) {
    const result = classifyPaths([path]);
    assert.equal(result.lanes.full, true, path);
    assert.ok(Object.values(result.lanes).every(Boolean), path);
  }
});

test('mixed docs and executable paths use the highest risk', () => {
  const result = classifyPaths(['docs/foo.md', 'apps/api/internal/foo.go']);
  assert.equal(result.risk, 'api');
  assert.equal(result.lanes.api, true);
});

test('full-main policy enables every lane even for docs-only diff', () => {
  const result = classifyPaths(['docs/foo.md'], { forceFull: true });
  assert.equal(result.lanes.full, true);
  assert.ok(Object.values(result.lanes).every(Boolean));
  assert.deepEqual(result.reasons, ['full-main-policy']);
});

test('manifest digest is deterministic across generatedAt timestamps', () => {
  const first = manifestFor(['apps/web/src/foo.ts'], { generatedAt: '2026-08-29T00:00:00.000Z' });
  const second = manifestFor(['apps/web/src/foo.ts'], { generatedAt: '2026-08-29T00:00:01.000Z' });
  assert.equal(first.digest, second.digest);
  assert.equal(first.digest, computeManifestDigest(first));
});

test('manifest validation rejects digest tampering and malformed lane contracts', () => {
  const manifest = manifestFor(['apps/api/internal/foo.go']);
  assert.deepEqual(validateManifest(manifest), []);

  const tampered = structuredClone(manifest);
  tampered.risk = 'docs';
  assert.ok(validateManifest(tampered).some((error) => error.includes('digest mismatch')));

  const malformed = structuredClone(manifest);
  delete malformed.lanes.api;
  malformed.digest = computeManifestDigest(malformed);
  assert.ok(validateManifest(malformed).some((error) => error.includes('lanes must contain exactly')));
});

test('manifest validation rejects unsupported events and short SHAs', () => {
  const manifest = manifestFor(['docs/foo.md']);
  manifest.event = 'schedule';
  manifest.headSha = 'abc';
  manifest.digest = computeManifestDigest(manifest);
  const errors = validateManifest(manifest);
  assert.ok(errors.some((error) => error.includes('unsupported event')));
  assert.ok(errors.some((error) => error.includes('headSha')));
});

test('Oracle fail-closed matrix', () => {
  const cases = [
    {
      name: 'disabled plus skipped passes',
      input: { detector: 'success', enabled: { web: false }, children: { web: 'skipped' } },
      ok: true,
    },
    {
      name: 'enabled plus success passes',
      input: { detector: 'success', enabled: { web: true }, children: { web: 'success' } },
      ok: true,
    },
    {
      name: 'enabled plus failure fails',
      input: { detector: 'success', enabled: { web: true }, children: { web: 'failure' } },
      ok: false,
    },
    {
      name: 'enabled plus skipped fails',
      input: { detector: 'success', enabled: { web: true }, children: { web: 'skipped' } },
      ok: false,
    },
    {
      name: 'enabled plus cancelled fails',
      input: { detector: 'success', enabled: { web: true }, children: { web: 'cancelled' } },
      ok: false,
    },
    {
      name: 'enabled plus missing fails',
      input: { detector: 'success', enabled: { web: true }, children: {} },
      ok: false,
    },
    {
      name: 'detector failure fails',
      input: { detector: 'failure', enabled: { web: false }, children: { web: 'skipped' } },
      ok: false,
    },
    {
      name: 'disabled unexpected failure fails',
      input: { detector: 'success', enabled: { web: false }, children: { web: 'failure' } },
      ok: false,
    },
  ];

  for (const fixture of cases) {
    assert.equal(evaluateOracle(fixture.input).ok, fixture.ok, fixture.name);
  }
});

test('run observation binds manifest digest and reports failing evidence', () => {
  const manifest = manifestFor(['apps/web/src/foo.ts']);
  const summary = createRunSummary({
    manifest,
    laneResults: { governance: 'success', web: 'failure', contracts: 'success' },
    oracleResults: { quality: 'failure' },
    generatedAt: '2026-08-29T00:00:00.000Z',
  });
  assert.equal(summary.manifestDigest, manifest.digest);
  assert.deepEqual(summary.failedLanes, ['web']);
  assert.deepEqual(summary.failedOracles, ['quality']);
  assert.match(summary.digest, /^sha256:[0-9a-f]{64}$/);
});
