import assert from 'node:assert/strict';
import test from 'node:test';
import { osvInput, parseLockfile, productionNpmPackages } from './production-npm-osv-input.mjs';

const fixture = `
lockfileVersion: '9.0'
importers:
  .:
    devDependencies:
      dev-only:
        version: 9.0.0
  apps/web:
    dependencies:
      app-runtime:
        version: 1.0.0(peer-runtime@3.0.0)
      internal:
        version: link:../../packages/internal
    optionalDependencies:
      optional-runtime:
        version: 2.0.0
snapshots:
  dev-only@9.0.0:
    dependencies:
      dev-transitive: 9.1.0
  dev-transitive@9.1.0: {}
  app-runtime@1.0.0(peer-runtime@3.0.0):
    dependencies:
      runtime-transitive: 1.1.0
    optionalDependencies:
      optional-transitive: 1.2.0
  runtime-transitive@1.1.0: {}
  optional-runtime@2.0.0: {}
  optional-transitive@1.2.0: {}
`;

test('extracts only production-reachable pnpm packages', () => {
  const packages = productionNpmPackages(parseLockfile(fixture));
  assert.deepEqual(
    packages.map((pkg) => `${pkg.name}@${pkg.version}`),
    ['app-runtime@1.0.0', 'optional-runtime@2.0.0', 'optional-transitive@1.2.0', 'runtime-transitive@1.1.0'],
  );
  assert.equal(JSON.stringify(osvInput(packages)).includes('dev-only'), false);
});

test('rejects an incomplete lockfile instead of silently producing an empty scan', () => {
  assert.throws(() => parseLockfile("lockfileVersion: '9.0'\nimporters: {}\n"), /missing/);
});
