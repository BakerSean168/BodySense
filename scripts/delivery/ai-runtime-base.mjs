#!/usr/bin/env node

import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const AI_RUNTIME_BASE_SCHEMA = 'bodysense-ai-runtime-base-v1';
export const AI_RUNTIME_BASE_BEGIN = '# BODYSENSE_AI_RUNTIME_BASE_BEGIN';
export const AI_RUNTIME_BASE_END = '# BODYSENSE_AI_RUNTIME_BASE_END';

export const AI_RUNTIME_BASE_INPUTS = Object.freeze([
  'apps/ai-service/pyproject.toml',
  'apps/ai-service/uv.lock',
  'apps/ai-service/scripts/ensure_pose_model.py',
  'apps/ai-service/scripts/ensure_health_document_models.py',
  'apps/ai-service/src/__init__.py',
  'apps/ai-service/src/configuration/__init__.py',
  'apps/ai-service/src/configuration/posture_agent_config.py',
  'apps/ai-service/src/configuration/health_document_config.py',
  'apps/ai-service/config/agents/posture-v2.yaml',
  'apps/ai-service/config/document-extraction/health-document-v20.yaml',
]);

function sha256(buffer) {
  return crypto.createHash('sha256').update(buffer).digest('hex');
}

export function extractAiRuntimeBaseRecipe(dockerfile) {
  const begin = dockerfile.indexOf(AI_RUNTIME_BASE_BEGIN);
  const end = dockerfile.indexOf(AI_RUNTIME_BASE_END);
  if (begin < 0 || end < 0 || end <= begin) {
    throw new Error('AI runtime-base Dockerfile markers are missing or out of order');
  }
  return dockerfile.slice(begin, end + AI_RUNTIME_BASE_END.length).replace(/\r\n/g, '\n');
}

export function createAiRuntimeBaseIdentity(repositoryRoot = process.cwd()) {
  const dockerfilePath = 'apps/ai-service/Dockerfile';
  const dockerfile = fs.readFileSync(path.join(repositoryRoot, dockerfilePath), 'utf8');
  const syntaxLine = dockerfile.split(/\r?\n/, 1)[0];
  if (!/^# syntax=docker\/dockerfile:[^@]+@sha256:[0-9a-f]{64}$/.test(syntaxLine)) {
    throw new Error('AI Dockerfile syntax frontend must be pinned by digest');
  }
  const recipe = Buffer.from(`${syntaxLine}\n${extractAiRuntimeBaseRecipe(dockerfile)}`, 'utf8');

  const records = [
    {
      path: `${dockerfilePath}#runtime-base`,
      sha256: sha256(recipe),
    },
    ...AI_RUNTIME_BASE_INPUTS.map((relativePath) => {
      const absolutePath = path.join(repositoryRoot, relativePath);
      if (!fs.statSync(absolutePath).isFile()) {
        throw new Error(`AI runtime-base input must be a file: ${relativePath}`);
      }
      return {
        path: relativePath,
        sha256: sha256(fs.readFileSync(absolutePath)),
      };
    }),
  ].sort((a, b) => a.path.localeCompare(b.path));

  const aggregate = crypto.createHash('sha256');
  aggregate.update(`${AI_RUNTIME_BASE_SCHEMA}\0`, 'utf8');
  for (const record of records) {
    aggregate.update(`${record.path}\0${record.sha256}\0`, 'utf8');
  }
  const hash = aggregate.digest('hex');

  return {
    schema: AI_RUNTIME_BASE_SCHEMA,
    inputDigest: `sha256:${hash}`,
    tag: `runtime-base-${hash}`,
    inputs: records,
  };
}

function parseFormat(argv) {
  const arg = argv.find((item) => item.startsWith('--format='));
  if (!arg) return 'json';
  const format = arg.slice('--format='.length);
  if (!['json', 'tag', 'digest'].includes(format)) {
    throw new Error(`unsupported --format=${format}`);
  }
  return format;
}

const isCli = process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (isCli) {
  const identity = createAiRuntimeBaseIdentity();
  const format = parseFormat(process.argv.slice(2));
  if (format === 'tag') {
    process.stdout.write(`${identity.tag}\n`);
  } else if (format === 'digest') {
    process.stdout.write(`${identity.inputDigest}\n`);
  } else {
    process.stdout.write(`${JSON.stringify(identity, null, 2)}\n`);
  }
}
