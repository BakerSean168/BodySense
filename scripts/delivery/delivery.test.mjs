import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';
import {
  classifyPaths,
  computeManifestDigest,
  createManifest,
  createRunSummary,
  evaluateOracle,
  validateManifest,
} from './lib.mjs';
import { createCandidate, validateCandidate } from './candidate-manifest.mjs';
import { extractReleaseNotes, validateReleaseFiles } from './release-contract.mjs';
import { createReleaseManifest, validateReleaseManifest } from './release-manifest.mjs';

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

test('candidate manifest binds all four image digests to one exact revision', () => {
  const gitSha = 'c'.repeat(40);
  const tag = `sha-${gitSha}`;
  const digest = (char) => `sha256:${char.repeat(64)}`;
  const candidate = createCandidate({
    gitSha,
    ciRunId: '12345',
    deliveryManifestDigest: digest('d'),
    migrationHead: 59,
    images: {
      web: { repository: 'registry/bodysense-web', tag, digest: digest('1'), revision: gitSha },
      api: { repository: 'registry/bodysense-api', tag, digest: digest('2'), revision: gitSha },
      aiService: { repository: 'registry/bodysense-ai-service', tag, digest: digest('3'), revision: gitSha },
      runtime: { repository: 'registry/bodysense-runtime', tag, digest: digest('4'), revision: gitSha },
    },
    staticAssets: {
      enabled: true,
      revision: gitSha,
      assetBase: `https://assets.example/web/${gitSha}/`,
      atlasCatalogUrl: 'https://assets.example/anatomy/vanatome/1.4.0/releases/1.4.0/catalog.json',
      atlasVersion: '1.4.0',
    },
    generatedAt: '2026-08-29T00:00:00.000Z',
  });
  assert.deepEqual(validateCandidate(candidate), []);
  assert.equal(candidate.candidateTag, tag);
  assert.match(candidate.digest, /^sha256:[0-9a-f]{64}$/);
});

test('candidate manifest fails closed on mixed revisions, tags, or tampered digests', () => {
  const gitSha = 'c'.repeat(40);
  const tag = `sha-${gitSha}`;
  const digest = (char) => `sha256:${char.repeat(64)}`;
  const candidate = createCandidate({
    gitSha,
    ciRunId: '12345',
    deliveryManifestDigest: digest('d'),
    migrationHead: 59,
    images: {
      web: { repository: 'registry/bodysense-web', tag, digest: digest('1'), revision: gitSha },
      api: { repository: 'registry/bodysense-api', tag, digest: digest('2'), revision: gitSha },
      aiService: { repository: 'registry/bodysense-ai-service', tag, digest: digest('3'), revision: gitSha },
      runtime: { repository: 'registry/bodysense-runtime', tag, digest: digest('4'), revision: gitSha },
    },
    staticAssets: { enabled: false, revision: gitSha, assetBase: '', atlasCatalogUrl: '', atlasVersion: '1.4.0' },
  });
  candidate.images.api.revision = 'e'.repeat(40);
  candidate.images.runtime.tag = 'staging-latest';
  candidate.images.web.digest = 'not-a-digest';
  const errors = validateCandidate(candidate);
  assert.ok(errors.some((error) => error.includes('api.revision')));
  assert.ok(errors.some((error) => error.includes('runtime.tag')));
  assert.ok(errors.some((error) => error.includes('web.digest')));
  assert.ok(errors.some((error) => error.includes('candidate digest mismatch')));
});

function runStagingWatcherCheck({ apiRevision } = {}) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'bodysense-staging-watch-'));
  const bin = path.join(root, 'bin');
  const state = path.join(root, 'state');
  const runtime = path.join(root, 'runtime');
  const config = path.join(root, 'staging-channel.env');
  const secret = path.join(root, 'staging.env');
  fs.mkdirSync(bin, { recursive: true });
  fs.writeFileSync(secret, 'DB_PASSWORD=test\nREDIS_PASSWORD=test\nJWT_SECRET_KEY=test\nLITELLM_MASTER_KEY=test\nGROQ_API_KEY=test\n');
  fs.writeFileSync(
    config,
    `STAGING_REGISTRY=registry.example\nSTAGING_NAMESPACE=bodysense\nSTAGING_CHANNEL_TAG=staging-latest\nSTAGING_SECRET_ENV=${secret}\n`,
  );
  const revision = 'f'.repeat(40);
  fs.writeFileSync(
    path.join(bin, 'docker'),
    `#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"$1\" == pull ]]; then exit 0; fi\nif [[ \"$1 $2\" == 'image inspect' ]]; then\n  ref=\"$3\"\n  if [[ \"$ref\" == *bodysense-api* ]]; then printf '%s\\n' \"${apiRevision ?? revision}\"; else printf '%s\\n' '${revision}'; fi\n  exit 0\nfi\necho \"unexpected fake docker args: $*\" >&2\nexit 99\n`,
    { mode: 0o755 },
  );
  const result = spawnSync('bash', ['scripts/staging-deploy-watch.sh', '--check-only'], {
    cwd: process.cwd(),
    encoding: 'utf8',
    env: {
      ...process.env,
      PATH: `${bin}:${process.env.PATH}`,
      BODYSENSE_STAGING_CHANNEL_CONFIG: config,
      BODYSENSE_STAGING_STATE_DIR: state,
      BODYSENSE_STAGING_RUNTIME_ROOT: runtime,
    },
  });
  fs.rmSync(root, { recursive: true, force: true });
  return { ...result, revision };
}

test('staging watcher check-only accepts one coherent four-image revision', () => {
  const result = runStagingWatcherCheck();
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, new RegExp(`STAGING_CANDIDATE=COHERENT revision=${result.revision}`));
});

test('staging watcher check-only refuses to treat split pointers as deployable', () => {
  const result = runStagingWatcherCheck({ apiRevision: 'e'.repeat(40) });
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /staging channel not coherent yet/);
  assert.doesNotMatch(result.stdout, /STAGING_CANDIDATE=COHERENT/);
});

test('staging watcher fails closed when an image revision label is missing', () => {
  const result = runStagingWatcherCheck({ apiRevision: '' });
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /api staging image has no org.opencontainers.image.revision/);
});


test('staging watcher deploys document-service as part of the coherent application revision', () => {
  const contents = fs.readFileSync('scripts/staging-deploy-watch.sh', 'utf8');
  assert.match(contents, /current_document=\$\(container_revision document-service\)/);
  assert.match(contents, /compose up -d --no-deps --no-build document-service/);
  assert.match(contents, /wait_healthy document-service 180/);
  assert.match(contents, /assert_container_revision document-service \"\$desired_revision\"/);
});

test('production watcher deploys, verifies, and conditionally rolls back document-service', () => {
  const contents = fs.readFileSync('scripts/production-deploy-watch.sh', 'utf8');
  assert.match(contents, /current_document=\$\(container_revision document-service\)/);
  assert.match(contents, /deploy_document_service\(\)/);
  assert.match(contents, /compose up -d --no-deps document-service/);
  assert.match(contents, /wait_healthy document-service 120/);
  assert.match(contents, /assert_container_revision document-service \"\$desired_revision\"/);
  assert.match(contents, /if compose_service_exists document-service; then/);
});


test('staging health-document qualification validator is fail-safe and restores Champion', () => {
  const contents = fs.readFileSync('scripts/validate-health-document-staging-qualification.sh', 'utf8');
  const runtimeDockerfile = fs.readFileSync('docker/Dockerfile.runtime', 'utf8');
  const stagingWatcher = fs.readFileSync('scripts/staging-deploy-watch.sh', 'utf8');
  assert.match(runtimeDockerfile, /validate-health-document-staging-qualification\.sh \/runtime\/staging\/scripts\/validate-health-document-staging-qualification\.sh/);
  assert.match(stagingWatcher, /scripts\/validate-health-document-staging-qualification\.sh/);
  assert.match(contents, /label=com\.docker\.compose\.service=document-service/);
  assert.match(contents, /document-service revision mismatch/);
  assert.match(contents, /trap cleanup EXIT INT TERM/);
  assert.match(contents, /HEALTH_DOCUMENT_STAGE=qualification/);
  assert.match(contents, /HEALTH_DOCUMENT_STAGE=champion/);
  assert.match(contents, /document_extraction_runs/);
  assert.match(contents, /HEALTH_DOCUMENT_STAGING_QUALIFICATION=PASS/);
  assert.match(contents, /-X DELETE/);
  assert.match(contents, /api\/v1\/uploads\/\$upload_id/);
  assert.match(contents, /DELETE FROM users WHERE email/);
});

test('release file contract recognizes release-please merge/squash identity and rejects normal commits', () => {
  const changelog = '# Changelog\n\n## [0.9.0](https://example/compare) (2026-08-29)\n\n### Features\n\n* delivery v3\n\n## [0.8.1](https://example/old)\n';
  const valid = validateReleaseFiles({
    packageVersion: '0.9.0',
    manifestVersion: '0.9.0',
    changelog,
    subjects: ['Merge pull request #140', 'chore(main): release 0.9.0'],
  });
  assert.deepEqual(valid.errors, []);
  assert.equal(valid.releaseShaped, true);

  const normal = validateReleaseFiles({
    packageVersion: '0.9.0',
    manifestVersion: '0.9.0',
    changelog,
    subjects: ['feat(ops): later normal change'],
  });
  assert.deepEqual(normal.errors, []);
  assert.equal(normal.releaseShaped, false);
});

test('release file contract fails on version drift and extracts only the requested notes section', () => {
  const changelog = '# Changelog\n\n## [0.9.0](https://example/new)\n\nnew notes\n\n## [0.8.1](https://example/old)\n\nold notes\n';
  const drift = validateReleaseFiles({
    packageVersion: '0.9.0',
    manifestVersion: '0.8.1',
    changelog,
    subjects: ['chore(main): release 0.9.0'],
  });
  assert.ok(drift.errors.some((error) => error.includes('manifest version')));
  const notes = extractReleaseNotes(changelog, '0.9.0');
  assert.match(notes, /new notes/);
  assert.doesNotMatch(notes, /old notes/);
});

test('release manifest promotes candidate digests without changing artifact identity', () => {
  const gitSha = 'c'.repeat(40);
  const tag = `sha-${gitSha}`;
  const digest = (char) => `sha256:${char.repeat(64)}`;
  const candidate = createCandidate({
    gitSha,
    ciRunId: '12345',
    deliveryManifestDigest: digest('d'),
    migrationHead: 59,
    images: {
      web: { repository: 'registry/bodysense-web', tag, digest: digest('1'), revision: gitSha },
      api: { repository: 'registry/bodysense-api', tag, digest: digest('2'), revision: gitSha },
      aiService: { repository: 'registry/bodysense-ai-service', tag, digest: digest('3'), revision: gitSha },
      runtime: { repository: 'registry/bodysense-runtime', tag, digest: digest('4'), revision: gitSha },
    },
    staticAssets: {
      enabled: true,
      revision: gitSha,
      assetBase: `https://assets.example/web/${gitSha}/`,
      atlasCatalogUrl: 'https://assets.example/anatomy/vanatome/1.4.0/releases/1.4.0/catalog.json',
      atlasVersion: '1.4.0',
    },
  });
  const release = createReleaseManifest({ candidate, version: '0.9.0', tag: 'v0.9.0', candidateRunId: '6789' });
  assert.deepEqual(validateReleaseManifest(release), []);
  assert.equal(release.images.web.digest, candidate.images.web.digest);
  assert.equal(release.images.api.digest, candidate.images.api.digest);
  assert.equal(release.candidateManifestDigest, candidate.digest);
});

test('release manifest fails closed if a promoted component digest/revision is tampered', () => {
  const gitSha = 'c'.repeat(40);
  const tag = `sha-${gitSha}`;
  const digest = (char) => `sha256:${char.repeat(64)}`;
  const candidate = createCandidate({
    gitSha,
    ciRunId: '12345',
    deliveryManifestDigest: digest('d'),
    migrationHead: 59,
    images: {
      web: { repository: 'registry/bodysense-web', tag, digest: digest('1'), revision: gitSha },
      api: { repository: 'registry/bodysense-api', tag, digest: digest('2'), revision: gitSha },
      aiService: { repository: 'registry/bodysense-ai-service', tag, digest: digest('3'), revision: gitSha },
      runtime: { repository: 'registry/bodysense-runtime', tag, digest: digest('4'), revision: gitSha },
    },
    staticAssets: { enabled: false, revision: gitSha, assetBase: '', atlasCatalogUrl: '', atlasVersion: '1.4.0' },
  });
  const release = createReleaseManifest({ candidate, version: '0.9.0', tag: 'v0.9.0', candidateRunId: '6789' });
  release.images.aiService.revision = 'a'.repeat(40);
  release.images.api.digest = 'bad';
  const errors = validateReleaseManifest(release);
  assert.ok(errors.some((error) => error.includes('aiService.revision')));
  assert.ok(errors.some((error) => error.includes('api.digest')));
  assert.ok(errors.some((error) => error.includes('release digest mismatch')));
});

test('Release Publish reconciles release-please tagged state only after publication and supports retries', () => {
  const contents = fs.readFileSync('.github/workflows/release-publish.yml', 'utf8');
  assert.match(contents, /pull-requests: write/);
  assert.match(contents, /issues: write/);
  assert.match(contents, /reconcile-release-please-state:/);
  assert.match(contents, /needs\.prepare-draft\.outputs\.already_published == 'true'/);
  assert.match(contents, /needs\.finalize\.result == 'success'/);
  assert.match(contents, /\| jq -r --arg title/);
  assert.match(contents, /gh release view \"\$RELEASE_TAG\".*--json isDraft/);
  assert.match(contents, /\[\[ \"\$tag_sha\" == \"\$RELEASE_SHA\" \]\]/);
  assert.match(contents, /autorelease%3A%20pending/);
  assert.match(contents, /labels\[\]=autorelease: tagged/);
});

test('local deploy keeps production-shaped runtime env after repository quality validation', () => {
  const contents = fs.readFileSync('scripts/local-deploy-validate.sh', 'utf8');
  const qualityGate = contents.indexOf('bash scripts/validate-repo.sh');
  const diagnosisEnv = contents.indexOf('export DIAGNOSIS_CHAMPION_CONFIGURATION_ID=');
  const dbPassword = contents.indexOf('export DB_PASSWORD=');
  const e2eStub = contents.indexOf('export BODYSENSE_E2E_STUB_AI=');
  assert.ok(qualityGate >= 0);
  assert.ok(diagnosisEnv > qualityGate);
  assert.ok(dbPassword > qualityGate);
  assert.ok(e2eStub > qualityGate);
});

test('all registry channel/release promotions carbon-copy single-platform manifests', () => {
  for (const workflow of [
    '.github/workflows/candidate-publish.yml',
    '.github/workflows/release-publish.yml',
    '.github/workflows/deploy-production.yml',
  ]) {
    const contents = fs.readFileSync(workflow, 'utf8');
    const promotionLines = contents
      .split(/\r?\n/)
      .filter((line) => line.includes('docker buildx imagetools create') && line.includes('--tag'));
    assert.ok(promotionLines.length > 0, `${workflow} must contain a registry promotion`);
    for (const line of promotionLines) {
      assert.match(line, /--prefer-index=false/, `${workflow}: ${line.trim()}`);
    }
  }
});
