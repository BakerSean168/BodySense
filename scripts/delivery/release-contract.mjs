#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';

const SEMVER_RE = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/;

function git(...args) {
  return execFileSync('git', args, { encoding: 'utf8' }).trim();
}

export function validateReleaseFiles({ packageVersion, manifestVersion, changelog, subjects }) {
  const errors = [];
  if (!SEMVER_RE.test(packageVersion ?? '')) errors.push('package.json version is not a supported semantic version');
  if (manifestVersion !== packageVersion) {
    errors.push(`release-please manifest version ${manifestVersion ?? 'missing'} != package version ${packageVersion ?? 'missing'}`);
  }
  if (!(changelog ?? '').split(/\r?\n/).some((line) => line.startsWith(`## [${packageVersion}]`))) {
    errors.push(`CHANGELOG is missing release heading for ${packageVersion}`);
  }
  const expected = `chore(main): release ${packageVersion}`;
  const releaseShaped = subjects.some(
    (subject) => subject === expected || subject.startsWith(`${expected} (#`),
  );
  return { errors, releaseShaped };
}

export function extractReleaseNotes(changelog, version) {
  const lines = changelog.split(/\r?\n/);
  const start = lines.findIndex((line) => line.startsWith(`## [${version}]`));
  if (start < 0) throw new Error(`CHANGELOG release section not found for ${version}`);
  let end = lines.length;
  for (let index = start + 1; index < lines.length; index += 1) {
    if (lines[index].startsWith('## [')) {
      end = index;
      break;
    }
  }
  return `${lines.slice(start, end).join('\n').trim()}\n`;
}

export function resolveReleaseContract({ requireRelease = false } = {}) {
  const root = process.cwd();
  const packageJson = JSON.parse(fs.readFileSync(path.join(root, 'package.json'), 'utf8'));
  const releaseManifest = JSON.parse(fs.readFileSync(path.join(root, '.release-please-manifest.json'), 'utf8'));
  const changelog = fs.readFileSync(path.join(root, 'CHANGELOG.md'), 'utf8');
  const sha = git('rev-parse', 'HEAD');
  const headSubject = git('show', '-s', '--format=%s', 'HEAD');
  const parents = git('rev-list', '--parents', '-n', '1', 'HEAD').split(/\s+/).slice(1);
  const subjects = [headSubject];
  if (parents.length >= 2) {
    subjects.push(git('show', '-s', '--format=%s', parents[1]));
  }

  const version = String(packageJson.version ?? '');
  const validation = validateReleaseFiles({
    packageVersion: version,
    manifestVersion: releaseManifest['.'],
    changelog,
    subjects,
  });
  if (validation.errors.length) throw new Error(validation.errors.join('; '));
  if (requireRelease && !validation.releaseShaped) {
    throw new Error(`HEAD ${sha} is not the merge/squash of chore(main): release ${version}`);
  }
  return {
    eligible: validation.releaseShaped,
    sha,
    version,
    tag: `v${version}`,
    releaseSubjects: subjects,
  };
}

function parseArgs(argv) {
  const args = { requireRelease: false };
  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (token === '--require-release') {
      args.requireRelease = true;
      continue;
    }
    if (token === '--github-output') {
      args.githubOutput = true;
      continue;
    }
    if (token === '--write-notes') {
      args.writeNotes = argv[++i];
      if (!args.writeNotes) throw new Error('--write-notes requires a path');
      continue;
    }
    throw new Error(`unknown argument: ${token}`);
  }
  return args;
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const contract = resolveReleaseContract({ requireRelease: args.requireRelease });
  if (args.writeNotes) {
    const changelog = fs.readFileSync('CHANGELOG.md', 'utf8');
    fs.writeFileSync(args.writeNotes, extractReleaseNotes(changelog, contract.version));
  }
  if (args.githubOutput) {
    const output = process.env.GITHUB_OUTPUT;
    if (!output) throw new Error('GITHUB_OUTPUT is required with --github-output');
    fs.appendFileSync(
      output,
      `eligible=${contract.eligible}\nsha=${contract.sha}\nversion=${contract.version}\ntag=${contract.tag}\n`,
    );
  }
  console.log(
    `RELEASE_CONTRACT=${contract.eligible ? 'ELIGIBLE' : 'NOOP'} version=${contract.version} sha=${contract.sha}`,
  );
}

if (import.meta.url === `file://${process.argv[1]}`) {
  try {
    main();
  } catch (error) {
    console.error(`release contract failed: ${error.message}`);
    process.exitCode = 1;
  }
}
