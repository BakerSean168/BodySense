#!/usr/bin/env node
import fs from 'node:fs';
import { spawnSync } from 'node:child_process';
import { validateManifest } from './lib.mjs';

const manifestPath = process.argv[2] ?? process.env.DELIVERY_MANIFEST_PATH;
if (!manifestPath) {
  console.error('usage: run-quality.mjs <delivery-manifest-v1.json>');
  process.exit(2);
}

function run(command, args = []) {
  console.log(`\n$ ${command} ${args.join(' ')}`);
  const result = spawnSync(command, args, {
    stdio: 'inherit',
    env: process.env,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`${command} ${args.join(' ')} failed with exit code ${result.status}`);
  }
}

function nx(project, target) {
  run('pnpm', ['nx', 'run', `${project}:${target}`]);
}

try {
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
  const errors = validateManifest(manifest);
  if (errors.length > 0) throw new Error(errors.join('; '));

  if (manifest.lanes.full) {
    run('bash', ['scripts/validate-repo.sh']);
    process.exit(0);
  }

  let executed = 0;

  if (manifest.lanes.web) {
    executed += 1;
    nx('@bodysense/web', 'lint');
    nx('@bodysense/web', 'typecheck');
    nx('@bodysense/web', 'test');
    nx('@bodysense/web', 'build');
  }

  if (manifest.lanes.api) {
    executed += 1;
    nx('api', 'lint');
    nx('api', 'test');
    nx('api', 'build');
  }

  if (manifest.lanes.ai) {
    executed += 1;
    nx('ai-service', 'lint');
    nx('ai-service', 'typecheck');
    nx('ai-service', 'test');
    for (const target of [
      'eval:diagnosis',
      'eval:diagnosis-evidence-policy',
      'eval:diagnosis-promotion',
      'eval:treatment',
      'eval:treatment-evidence-gap',
      'eval:treatment-promotion',
    ]) {
      nx('ai-service', target);
    }
    run('bash', ['scripts/validate-litellm-gateway.sh']);
  }

  if (manifest.lanes.contracts) {
    executed += 1;
    nx('@bodysense/contracts', 'lint');
    nx('@bodysense/contracts', 'typecheck');
    nx('@bodysense/contracts', 'test');
  }

  if (executed === 0) {
    console.log('DELIVERY_QUALITY=SKIP reason=no-quality-lanes-selected');
  } else {
    console.log(`DELIVERY_QUALITY=PASS selected=${[
      manifest.lanes.web && 'web',
      manifest.lanes.api && 'api',
      manifest.lanes.ai && 'ai',
      manifest.lanes.contracts && 'contracts',
    ]
      .filter(Boolean)
      .join(',')}`);
  }
} catch (error) {
  console.error(`delivery quality failed: ${error.message}`);
  process.exit(1);
}
