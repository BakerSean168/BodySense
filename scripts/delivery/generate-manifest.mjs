#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { createManifest, validateManifest } from './lib.mjs';

function parseArgs(argv) {
  const args = {};
  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (!token.startsWith('--')) throw new Error(`unexpected argument: ${token}`);
    const key = token.slice(2);
    if (key === 'force-full') {
      args.forceFull = true;
      continue;
    }
    const value = argv[index + 1];
    if (!value || value.startsWith('--')) throw new Error(`missing value for --${key}`);
    args[key] = value;
    index += 1;
  }
  return args;
}

function git(...args) {
  return execFileSync('git', args, { encoding: 'utf8' }).trim();
}

function resolveSha(value, field) {
  if (!value) throw new Error(`${field} is required`);
  const resolved = git('rev-parse', `${value}^{commit}`);
  if (!/^[0-9a-f]{40}$/i.test(resolved)) throw new Error(`${field} did not resolve to a full SHA`);
  return resolved;
}

function changedPaths(baseSha, headSha) {
  if (baseSha === headSha) return [];
  const output = execFileSync(
    'git',
    ['diff', '--name-only', '--diff-filter=ACDMRTUXB', '-z', baseSha, headSha],
    { encoding: 'utf8' },
  );
  return output.split('\0').filter(Boolean);
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const event = args.event ?? process.env.GITHUB_EVENT_NAME;
  const ref = args.ref ?? process.env.GITHUB_REF ?? '';
  if (!['pull_request', 'push', 'workflow_dispatch'].includes(event)) {
    throw new Error(`unsupported event: ${event || 'missing'}`);
  }

  const requestedBaseSha = resolveSha(args.base ?? process.env.DELIVERY_BASE_SHA, 'base');
  const headSha = resolveSha(args.head ?? process.env.DELIVERY_HEAD_SHA ?? 'HEAD', 'head');
  const baseSha =
    event === 'pull_request'
      ? resolveSha(git('merge-base', requestedBaseSha, headSha), 'merge-base')
      : requestedBaseSha;

  const forceFull =
    args.forceFull === true ||
    process.env.DELIVERY_FORCE_FULL === '1' ||
    (event === 'push' && ref === 'refs/heads/main') ||
    event === 'workflow_dispatch';

  const manifest = createManifest({
    requestedBaseSha,
    baseSha,
    headSha,
    event,
    ref,
    changedPaths: changedPaths(baseSha, headSha),
    forceFull,
  });
  const errors = validateManifest(manifest);
  if (errors.length > 0) throw new Error(errors.join('; '));

  const output = path.resolve(args.output ?? process.env.DELIVERY_MANIFEST_OUTPUT ?? 'reports/delivery/delivery-manifest-v1.json');
  fs.mkdirSync(path.dirname(output), { recursive: true });
  fs.writeFileSync(output, `${JSON.stringify(manifest, null, 2)}\n`);

  if (process.env.GITHUB_OUTPUT) {
    fs.appendFileSync(
      process.env.GITHUB_OUTPUT,
      [
        `manifest_path=${output}`,
        `manifest_digest=${manifest.digest}`,
        `risk=${manifest.risk}`,
        ...Object.entries(manifest.lanes).map(([lane, enabled]) => `lane_${lane}=${enabled}`),
      ].join('\n') + '\n',
    );
  }

  process.stdout.write(`${JSON.stringify(manifest, null, 2)}\n`);
}

try {
  main();
} catch (error) {
  console.error(`delivery manifest generation failed: ${error.message}`);
  process.exitCode = 1;
}
