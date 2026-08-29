#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { canonicalize, sha256Json } from './lib.mjs';
import { validateCandidate } from './candidate-manifest.mjs';

export const RELEASE_SCHEMA = 'bodysense.release-set/v1';
const COMPONENTS = ['web', 'api', 'aiService', 'runtime'];

export function releaseDigest(release) {
  const { digest: _digest, generatedAt: _generatedAt, ...identity } = release;
  return sha256Json(identity);
}

export function createReleaseManifest({ candidate, version, tag, candidateRunId, generatedAt }) {
  const candidateErrors = validateCandidate(candidate);
  if (candidateErrors.length) throw new Error(`invalid candidate: ${candidateErrors.join('; ')}`);
  if (tag !== `v${version}`) throw new Error('tag must equal v<version>');
  const images = Object.fromEntries(
    COMPONENTS.map((component) => {
      const image = candidate.images[component];
      return [
        component,
        {
          repository: image.repository,
          digest: image.digest,
          revision: image.revision,
          candidateTag: image.tag,
          releaseTag: tag,
        },
      ];
    }),
  );
  const release = {
    schema: RELEASE_SCHEMA,
    version,
    tag,
    gitSha: candidate.gitSha,
    ciRunId: candidate.ciRunId,
    candidateRunId: String(candidateRunId),
    candidateManifestDigest: candidate.digest,
    deliveryManifestDigest: candidate.deliveryManifestDigest,
    migrationHead: candidate.migrationHead,
    images: canonicalize(images),
    staticAssets: canonicalize(candidate.staticAssets),
    generatedAt: generatedAt ?? new Date().toISOString(),
  };
  release.digest = releaseDigest(release);
  return release;
}

export function validateReleaseManifest(release) {
  const errors = [];
  if (!release || typeof release !== 'object' || Array.isArray(release)) return ['release must be an object'];
  if (release.schema !== RELEASE_SCHEMA) errors.push(`schema must be ${RELEASE_SCHEMA}`);
  if (!/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(release.version ?? '')) errors.push('version must be semantic version');
  if (release.tag !== `v${release.version}`) errors.push('tag must equal v<version>');
  if (!/^[0-9a-f]{40}$/i.test(release.gitSha ?? '')) errors.push('gitSha must be a full Git SHA');
  if (!/^\d+$/.test(String(release.ciRunId ?? ''))) errors.push('ciRunId must be numeric');
  if (!/^\d+$/.test(String(release.candidateRunId ?? ''))) errors.push('candidateRunId must be numeric');
  for (const field of ['candidateManifestDigest', 'deliveryManifestDigest']) {
    if (!/^sha256:[0-9a-f]{64}$/.test(release[field] ?? '')) errors.push(`${field} must be sha256:<64 hex>`);
  }
  if (!Number.isInteger(release.migrationHead) || release.migrationHead < 1) errors.push('migrationHead must be positive integer');

  const keys = Object.keys(release.images ?? {}).sort();
  if (JSON.stringify(keys) !== JSON.stringify([...COMPONENTS].sort())) {
    errors.push(`images must contain exactly: ${COMPONENTS.join(', ')}`);
  }
  for (const component of COMPONENTS) {
    const image = release.images?.[component];
    if (!image) {
      errors.push(`missing image: ${component}`);
      continue;
    }
    if (!image.repository) errors.push(`${component}.repository is required`);
    if (!/^sha256:[0-9a-f]{64}$/.test(image.digest ?? '')) errors.push(`${component}.digest must be sha256:<64 hex>`);
    if (image.revision !== release.gitSha) errors.push(`${component}.revision must equal gitSha`);
    if (image.candidateTag !== `sha-${release.gitSha}`) errors.push(`${component}.candidateTag must bind gitSha`);
    if (image.releaseTag !== release.tag) errors.push(`${component}.releaseTag must equal release tag`);
  }
  if (release.staticAssets?.revision !== release.gitSha) errors.push('staticAssets.revision must equal gitSha');
  const expected = releaseDigest(release);
  if (release.digest !== expected) errors.push(`release digest mismatch: expected ${expected}`);
  return errors;
}

function parseArgs(argv) {
  const args = {};
  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (!token.startsWith('--')) throw new Error(`unexpected argument: ${token}`);
    const key = token.slice(2);
    const value = argv[++i];
    if (!value || value.startsWith('--')) throw new Error(`missing value for --${key}`);
    args[key] = value;
  }
  return args;
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  if (args.validate) {
    const release = JSON.parse(fs.readFileSync(args.validate, 'utf8'));
    const errors = validateReleaseManifest(release);
    if (errors.length) throw new Error(errors.join('; '));
    console.log(`RELEASE_MANIFEST=PASS digest=${release.digest}`);
    return;
  }
  for (const field of ['candidate', 'version', 'tag', 'candidate-run-id', 'output']) {
    if (!args[field]) throw new Error(`--${field} is required`);
  }
  const candidate = JSON.parse(fs.readFileSync(args.candidate, 'utf8'));
  const release = createReleaseManifest({
    candidate,
    version: args.version,
    tag: args.tag,
    candidateRunId: args['candidate-run-id'],
  });
  const errors = validateReleaseManifest(release);
  if (errors.length) throw new Error(errors.join('; '));
  const output = path.resolve(args.output);
  fs.mkdirSync(path.dirname(output), { recursive: true });
  fs.writeFileSync(output, `${JSON.stringify(release, null, 2)}\n`);
  console.log(`RELEASE_MANIFEST=PASS digest=${release.digest}`);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  try {
    main();
  } catch (error) {
    console.error(`release manifest failed: ${error.message}`);
    process.exitCode = 1;
  }
}
