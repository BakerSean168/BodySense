#!/usr/bin/env node
import fs from 'node:fs';
import process from 'node:process';
import YAML from 'yaml';

function registryRef(name, rawRef, snapshots) {
  let ref = typeof rawRef === 'object' && rawRef !== null ? rawRef.version : rawRef;
  if (typeof ref !== 'string' || !ref) return null;
  if (/^(?:link:|workspace:|file:|https?:|git\+|github:)/.test(ref)) return null;

  let packageName = name;
  if (ref.startsWith('npm:')) {
    const alias = ref.slice(4);
    const split = alias.lastIndexOf('@');
    if (split <= 0) return null;
    packageName = alias.slice(0, split);
    ref = alias.slice(split + 1);
  }

  const key = `${packageName}@${ref}`;
  if (!Object.prototype.hasOwnProperty.call(snapshots, key)) return null;
  const version = ref.split('(')[0];
  if (!/^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/.test(version)) return null;
  return { name: packageName, version, ref, key };
}

export function productionNpmPackages(lockfile) {
  const snapshots = lockfile.snapshots ?? {};
  const importers = lockfile.importers ?? {};
  const visited = new Set();
  const packages = new Map();
  const queue = [];

  const enqueue = (name, ref) => {
    const resolved = registryRef(name, ref, snapshots);
    if (!resolved || visited.has(resolved.key)) return;
    visited.add(resolved.key);
    queue.push(resolved);
  };

  // Every workspace importer is a possible production entrypoint. Only normal
  // and optional dependencies are roots; devDependencies never enter the graph.
  for (const importer of Object.values(importers)) {
    for (const section of ['dependencies', 'optionalDependencies']) {
      for (const [name, ref] of Object.entries(importer?.[section] ?? {})) enqueue(name, ref);
    }
  }

  while (queue.length > 0) {
    const current = queue.shift();
    packages.set(`${current.name}@${current.version}`, {
      name: current.name,
      version: current.version,
      ecosystem: 'npm',
    });
    const snapshot = snapshots[current.key] ?? {};
    for (const section of ['dependencies', 'optionalDependencies']) {
      for (const [name, ref] of Object.entries(snapshot[section] ?? {})) enqueue(name, ref);
    }
  }

  return [...packages.values()].sort(
    (a, b) => a.name.localeCompare(b.name) || a.version.localeCompare(b.version),
  );
}

export function osvInput(packages) {
  return {
    results: [
      {
        packages: packages.map((pkg) => ({ package: pkg })),
      },
    ],
  };
}

export function parseLockfile(text) {
  const parsed = YAML.parse(text);
  if (!parsed || parsed.lockfileVersion == null || !parsed.importers || !parsed.snapshots) {
    throw new Error('pnpm lockfile is missing lockfileVersion/importers/snapshots');
  }
  return parsed;
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const path = process.argv[2] ?? 'pnpm-lock.yaml';
  const lockfile = parseLockfile(fs.readFileSync(path, 'utf8'));
  const packages = productionNpmPackages(lockfile);
  if (packages.length === 0) {
    console.error('SUPPLY_CHAIN_OSV_INPUT=FAIL no production npm packages resolved');
    process.exit(2);
  }
  process.stdout.write(`${JSON.stringify(osvInput(packages), null, 2)}\n`);
  console.error(`SUPPLY_CHAIN_OSV_INPUT=PASS packages=${packages.length}`);
}
