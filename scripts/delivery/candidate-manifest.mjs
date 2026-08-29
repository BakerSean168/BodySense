#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { canonicalize, sha256Json } from './lib.mjs';

export const CANDIDATE_SCHEMA = 'bodysense.candidate-set/v1';
const COMPONENTS = ['web', 'api', 'aiService', 'runtime'];

export function candidateDigest(candidate) {
  const { digest: _digest, generatedAt: _generatedAt, ...identity } = candidate;
  return sha256Json(identity);
}

export function validateCandidate(candidate) {
  const errors = [];
  if (!candidate || typeof candidate !== 'object' || Array.isArray(candidate)) {
    return ['candidate must be an object'];
  }
  if (candidate.schema !== CANDIDATE_SCHEMA) errors.push(`schema must be ${CANDIDATE_SCHEMA}`);
  if (!/^[0-9a-f]{40}$/i.test(candidate.gitSha ?? '')) errors.push('gitSha must be a full Git SHA');
  if (!/^\d+$/.test(String(candidate.ciRunId ?? ''))) errors.push('ciRunId must be numeric');
  if (candidate.candidateTag !== `sha-${candidate.gitSha}`) {
    errors.push('candidateTag must equal sha-<full gitSha>');
  }
  if (!/^sha256:[0-9a-f]{64}$/.test(candidate.deliveryManifestDigest ?? '')) {
    errors.push('deliveryManifestDigest must be sha256:<64 hex>');
  }
  if (!Number.isInteger(candidate.migrationHead) || candidate.migrationHead < 1) {
    errors.push('migrationHead must be a positive integer');
  }

  const imageKeys = Object.keys(candidate.images ?? {}).sort();
  if (JSON.stringify(imageKeys) !== JSON.stringify([...COMPONENTS].sort())) {
    errors.push(`images must contain exactly: ${COMPONENTS.join(', ')}`);
  }
  for (const component of COMPONENTS) {
    const image = candidate.images?.[component];
    if (!image || typeof image !== 'object') {
      errors.push(`missing image contract: ${component}`);
      continue;
    }
    if (typeof image.repository !== 'string' || image.repository.length === 0) {
      errors.push(`${component}.repository is required`);
    }
    if (image.tag !== candidate.candidateTag) {
      errors.push(`${component}.tag must equal candidateTag`);
    }
    if (!/^sha256:[0-9a-f]{64}$/.test(image.digest ?? '')) {
      errors.push(`${component}.digest must be sha256:<64 hex>`);
    }
    if (image.revision !== candidate.gitSha) {
      errors.push(`${component}.revision must equal gitSha`);
    }
  }

  if (!candidate.staticAssets || typeof candidate.staticAssets !== 'object') {
    errors.push('staticAssets is required');
  } else {
    if (typeof candidate.staticAssets.enabled !== 'boolean') errors.push('staticAssets.enabled must be boolean');
    if (candidate.staticAssets.revision !== candidate.gitSha) {
      errors.push('staticAssets.revision must equal gitSha');
    }
    if (candidate.staticAssets.enabled) {
      if (!String(candidate.staticAssets.assetBase ?? '').startsWith('https://')) {
        errors.push('enabled staticAssets.assetBase must use https://');
      }
      if (!String(candidate.staticAssets.atlasCatalogUrl ?? '').startsWith('https://')) {
        errors.push('enabled staticAssets.atlasCatalogUrl must use https://');
      }
    }
  }

  const expected = candidateDigest(candidate);
  if (candidate.digest !== expected) errors.push(`candidate digest mismatch: expected ${expected}`);
  return errors;
}

export function createCandidate(input) {
  const candidate = {
    schema: CANDIDATE_SCHEMA,
    gitSha: input.gitSha,
    ciRunId: String(input.ciRunId),
    candidateTag: `sha-${input.gitSha}`,
    deliveryManifestDigest: input.deliveryManifestDigest,
    migrationHead: Number(input.migrationHead),
    images: canonicalize(input.images),
    staticAssets: canonicalize(input.staticAssets),
    generatedAt: input.generatedAt ?? new Date().toISOString(),
  };
  candidate.digest = candidateDigest(candidate);
  return candidate;
}

function parseArgs(argv) {
  const args = {};
  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (!token.startsWith('--')) throw new Error(`unexpected argument: ${token}`);
    const key = token.slice(2);
    const value = argv[i + 1];
    if (!value || value.startsWith('--')) throw new Error(`missing value for --${key}`);
    args[key] = value;
    i += 1;
  }
  return args;
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  if (args.validate) {
    const candidate = JSON.parse(fs.readFileSync(args.validate, 'utf8'));
    const errors = validateCandidate(candidate);
    if (errors.length) throw new Error(errors.join('; '));
    console.log(`CANDIDATE_MANIFEST=PASS digest=${candidate.digest}`);
    return;
  }

  const required = ['input', 'output'];
  for (const field of required) if (!args[field]) throw new Error(`--${field} is required`);
  const input = JSON.parse(fs.readFileSync(args.input, 'utf8'));
  const candidate = createCandidate(input);
  const errors = validateCandidate(candidate);
  if (errors.length) throw new Error(errors.join('; '));
  const output = path.resolve(args.output);
  fs.mkdirSync(path.dirname(output), { recursive: true });
  fs.writeFileSync(output, `${JSON.stringify(candidate, null, 2)}\n`);
  console.log(`CANDIDATE_MANIFEST=PASS digest=${candidate.digest}`);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  try {
    main();
  } catch (error) {
    console.error(`candidate manifest failed: ${error.message}`);
    process.exitCode = 1;
  }
}
