#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import {
  computeManifestDigest,
  createManifest,
  evaluateOracle,
  validateManifest,
} from './lib.mjs';
import { createCandidate, validateCandidate } from './candidate-manifest.mjs';
import { validateReleaseFiles } from './release-contract.mjs';
import { createReleaseManifest, validateReleaseManifest } from './release-manifest.mjs';

const SHA_A = 'a'.repeat(40);
const SHA_B = 'b'.repeat(40);
const shaDigest = (char) => `sha256:${char.repeat(64)}`;

function check(name, fn) {
  try {
    const detail = fn();
    return { name, status: 'PASS', detail: detail ?? '' };
  } catch (error) {
    return { name, status: 'FAIL', detail: error.message };
  }
}

function requireCondition(condition, message) {
  if (!condition) throw new Error(message);
}

const results = [];

results.push(
  check('manifest digest tamper is rejected', () => {
    const manifest = createManifest({
      requestedBaseSha: SHA_A,
      baseSha: SHA_A,
      headSha: SHA_B,
      event: 'pull_request',
      ref: 'refs/pull/1/head',
      changedPaths: ['apps/web/src/foo.tsx'],
      generatedAt: '2026-08-29T00:00:00Z',
    });
    manifest.risk = 'docs';
    const errors = validateManifest(manifest);
    requireCondition(errors.some((error) => error.includes('digest mismatch')), 'tampered manifest was accepted');
    return errors.find((error) => error.includes('digest mismatch'));
  }),
);

results.push(
  check('enabled missing Oracle child fails closed', () => {
    const oracle = evaluateOracle({ detector: 'success', enabled: { database: true }, children: {} });
    requireCondition(!oracle.ok, 'Oracle accepted a missing enabled child');
    requireCondition(oracle.failures.some((failure) => failure.includes('missing')), 'Oracle failure did not identify missing child');
    return oracle.failures.join('; ');
  }),
);

results.push(
  check('candidate split revision is rejected', () => {
    const tag = `sha-${SHA_A}`;
    const candidate = createCandidate({
      gitSha: SHA_A,
      ciRunId: '1',
      deliveryManifestDigest: shaDigest('d'),
      migrationHead: 59,
      images: {
        web: { repository: 'r/web', tag, digest: shaDigest('1'), revision: SHA_A },
        api: { repository: 'r/api', tag, digest: shaDigest('2'), revision: SHA_A },
        aiService: { repository: 'r/ai', tag, digest: shaDigest('3'), revision: SHA_A },
        runtime: { repository: 'r/runtime', tag, digest: shaDigest('4'), revision: SHA_A },
      },
      staticAssets: { enabled: false, revision: SHA_A, assetBase: '', atlasCatalogUrl: '', atlasVersion: '1.4.0' },
    });
    candidate.images.api.revision = SHA_B;
    candidate.digest = computeManifestDigest(candidate);
    const errors = validateCandidate(candidate);
    requireCondition(errors.some((error) => error.includes('api.revision')), 'candidate accepted split revision');
    return errors.join('; ');
  }),
);

results.push(
  check('normal commit cannot masquerade as Release PR merge', () => {
    const contract = validateReleaseFiles({
      packageVersion: '0.9.0',
      manifestVersion: '0.9.0',
      changelog: '# Changelog\n\n## [0.9.0](https://example)\n',
      subjects: ['feat(ops): unrelated main change'],
    });
    requireCondition(contract.errors.length === 0, `release file fixture invalid: ${contract.errors.join('; ')}`);
    requireCondition(!contract.releaseShaped, 'normal commit was classified as release merge');
    return 'releaseShaped=false';
  }),
);

results.push(
  check('release manifest component tamper is rejected', () => {
    const candidateTag = `sha-${SHA_A}`;
    const candidate = createCandidate({
      gitSha: SHA_A,
      ciRunId: '1',
      deliveryManifestDigest: shaDigest('d'),
      migrationHead: 59,
      images: {
        web: { repository: 'r/web', tag: candidateTag, digest: shaDigest('1'), revision: SHA_A },
        api: { repository: 'r/api', tag: candidateTag, digest: shaDigest('2'), revision: SHA_A },
        aiService: { repository: 'r/ai', tag: candidateTag, digest: shaDigest('3'), revision: SHA_A },
        runtime: { repository: 'r/runtime', tag: candidateTag, digest: shaDigest('4'), revision: SHA_A },
      },
      staticAssets: { enabled: false, revision: SHA_A, assetBase: '', atlasCatalogUrl: '', atlasVersion: '1.4.0' },
    });
    const release = createReleaseManifest({ candidate, version: '0.9.0', tag: 'v0.9.0', candidateRunId: '2' });
    release.images.runtime.digest = shaDigest('9');
    const errors = validateReleaseManifest(release);
    requireCondition(errors.some((error) => error.includes('release digest mismatch')), 'tampered release manifest was accepted');
    return errors.join('; ');
  }),
);

const report = {
  schema: 'bodysense.delivery-platform-audit/v1',
  generatedAt: new Date().toISOString(),
  status: results.every((result) => result.status === 'PASS') ? 'PASS' : 'FAIL',
  results,
};
const output = path.resolve(process.argv[2] ?? 'reports/delivery/platform-audit-v1.json');
fs.mkdirSync(path.dirname(output), { recursive: true });
fs.writeFileSync(output, `${JSON.stringify(report, null, 2)}\n`);
console.log(`DELIVERY_PLATFORM_AUDIT=${report.status}`);
for (const result of results) console.log(`${result.status} ${result.name}`);
if (report.status !== 'PASS') process.exitCode = 1;
