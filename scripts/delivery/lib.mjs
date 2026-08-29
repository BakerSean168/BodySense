import crypto from 'node:crypto';

export const MANIFEST_SCHEMA = 'bodysense.delivery-manifest/v1';
export const POLICY_VERSION = 'delivery-policy-v1';
export const SUMMARY_SCHEMA = 'bodysense.delivery-run-summary/v1';

export const LANE_NAMES = Object.freeze([
  'governance',
  'web',
  'api',
  'ai',
  'contracts',
  'database',
  'experience',
  'full',
]);

export const RISK_NAMES = Object.freeze([
  'docs',
  'web',
  'api',
  'ai',
  'contract',
  'database',
  'runtime',
  'release',
  'root',
]);

const FULL_LANES = Object.freeze(
  Object.fromEntries(LANE_NAMES.map((lane) => [lane, true])),
);

const RISK_PRIORITY = new Map(RISK_NAMES.map((risk, index) => [risk, index]));

function normalizePath(path) {
  return String(path).replaceAll('\\', '/').replace(/^\.\//, '');
}

export function normalizeChangedPaths(paths) {
  return [...new Set(paths.map(normalizePath).filter(Boolean))].sort();
}

export function canonicalize(value) {
  if (Array.isArray(value)) {
    return value.map(canonicalize);
  }
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.keys(value)
        .sort()
        .map((key) => [key, canonicalize(value[key])]),
    );
  }
  return value;
}

export function sha256Json(value) {
  const canonical = JSON.stringify(canonicalize(value));
  return `sha256:${crypto.createHash('sha256').update(canonical).digest('hex')}`;
}

export function computeManifestDigest(manifest) {
  const { digest: _digest, generatedAt: _generatedAt, ...identity } = manifest;
  return sha256Json(identity);
}

export function computeSummaryDigest(summary) {
  const { digest: _digest, generatedAt: _generatedAt, ...identity } = summary;
  return sha256Json(identity);
}

function allTrueLanes() {
  return { ...FULL_LANES };
}

function emptyLanes() {
  return Object.fromEntries(LANE_NAMES.map((lane) => [lane, lane === 'governance']));
}

function mergeLanes(target, patch) {
  for (const [lane, enabled] of Object.entries(patch)) {
    if (!LANE_NAMES.includes(lane)) {
      throw new Error(`unknown lane: ${lane}`);
    }
    target[lane] ||= Boolean(enabled);
  }
}

function highestRisk(risks) {
  let selected = 'docs';
  for (const risk of risks) {
    if ((RISK_PRIORITY.get(risk) ?? -1) > (RISK_PRIORITY.get(selected) ?? -1)) {
      selected = risk;
    }
  }
  return selected;
}

function classifySinglePath(path) {
  // Documentation and prose-only repository metadata are the cheapest class.
  if (
    path.startsWith('docs/') ||
    path === 'README.md' ||
    path === 'CONTRIBUTING.md' ||
    path.endsWith('.md')
  ) {
    return { risk: 'docs', lanes: {}, reason: 'documentation' };
  }

  // Delivery/release/runtime coordinates are intentionally full because a
  // detector bug in its own implementation must never reduce validation.
  if (
    path.startsWith('.github/') ||
    path.startsWith('scripts/delivery/') ||
    path.startsWith('docker/') ||
    path.startsWith('deploy/') ||
    path === '.env.production' ||
    path === 'release-please-config.json' ||
    path === '.release-please-manifest.json'
  ) {
    const risk =
      path === 'release-please-config.json' || path === '.release-please-manifest.json'
        ? 'release'
        : 'runtime';
    return { risk, lanes: allTrueLanes(), reason: 'delivery-runtime' };
  }

  if (
    path === 'package.json' ||
    path === 'pnpm-lock.yaml' ||
    path === 'pnpm-workspace.yaml' ||
    path === 'nx.json' ||
    path.startsWith('tsconfig') ||
    path.startsWith('.npm') ||
    path.startsWith('.node-version') ||
    path.startsWith('.python-version') ||
    path === 'uv.lock'
  ) {
    return { risk: 'root', lanes: allTrueLanes(), reason: 'root-toolchain' };
  }

  if (path.startsWith('apps/api/migrations/')) {
    return {
      risk: 'database',
      lanes: {
        api: true,
        database: true,
        experience: true,
      },
      reason: 'database-migration',
    };
  }

  if (path.startsWith('apps/api/')) {
    return {
      risk: 'api',
      lanes: { api: true, experience: true },
      reason: 'go-api',
    };
  }

  if (path.startsWith('apps/ai-service/')) {
    return {
      risk: 'ai',
      lanes: { ai: true, experience: true },
      reason: 'ai-service',
    };
  }

  if (path.startsWith('apps/web/')) {
    return {
      risk: 'web',
      lanes: { web: true, experience: true },
      reason: 'web',
    };
  }

  if (path.startsWith('packages/contracts/')) {
    return {
      risk: 'contract',
      lanes: {
        web: true,
        api: true,
        ai: true,
        contracts: true,
        experience: true,
      },
      reason: 'shared-contract',
    };
  }

  if (path.startsWith('packages/utils/')) {
    return {
      risk: 'root',
      lanes: allTrueLanes(),
      reason: 'shared-utility-fail-safe',
    };
  }

  if (path.startsWith('scripts/static-assets/') || path.startsWith('scripts/anatomy/')) {
    return {
      risk: 'runtime',
      lanes: allTrueLanes(),
      reason: 'public-asset-runtime',
    };
  }

  if (path.startsWith('scripts/')) {
    // Operational/test scripts frequently participate in deploy, migration,
    // privacy or release validation. Unknown scripts are therefore full risk.
    return { risk: 'runtime', lanes: allTrueLanes(), reason: 'repository-script' };
  }

  if (path.startsWith('tests/') || path.startsWith('e2e/')) {
    return {
      risk: 'contract',
      lanes: { contracts: true, experience: true },
      reason: 'cross-service-test',
    };
  }

  // Unknown executable or repository-root content is deliberately safest/full.
  return { risk: 'root', lanes: allTrueLanes(), reason: 'unknown-path-fail-safe' };
}

export function classifyPaths(paths, { forceFull = false } = {}) {
  const changedPaths = normalizeChangedPaths(paths);
  if (forceFull) {
    return {
      risk: 'root',
      lanes: allTrueLanes(),
      reasons: ['full-main-policy'],
      changedPaths,
    };
  }

  if (changedPaths.length === 0) {
    // An empty diff is allowed for manual diagnostics, but it must never be
    // interpreted as docs-only. Governance remains active and the manifest
    // records the explicit no-change reason.
    return {
      risk: 'docs',
      lanes: emptyLanes(),
      reasons: ['no-changed-paths'],
      changedPaths,
    };
  }

  const lanes = emptyLanes();
  const risks = new Set(['docs']);
  const reasons = new Set();

  for (const path of changedPaths) {
    const classification = classifySinglePath(path);
    risks.add(classification.risk);
    reasons.add(classification.reason);
    mergeLanes(lanes, classification.lanes);
  }

  if (lanes.full) {
    return {
      risk: highestRisk(risks),
      lanes: allTrueLanes(),
      reasons: [...reasons].sort(),
      changedPaths,
    };
  }

  return {
    risk: highestRisk(risks),
    lanes,
    reasons: [...reasons].sort(),
    changedPaths,
  };
}

export function createManifest({
  baseSha,
  headSha,
  requestedBaseSha = baseSha,
  event,
  ref = '',
  changedPaths,
  forceFull = false,
  generatedAt = new Date().toISOString(),
}) {
  const classification = classifyPaths(changedPaths, { forceFull });
  const manifest = {
    schema: MANIFEST_SCHEMA,
    policyVersion: POLICY_VERSION,
    event,
    ref,
    requestedBaseSha,
    baseSha,
    headSha,
    risk: classification.risk,
    reasons: classification.reasons,
    changedPaths: classification.changedPaths,
    lanes: classification.lanes,
    generatedAt,
  };
  manifest.digest = computeManifestDigest(manifest);
  return manifest;
}

export function validateManifest(manifest) {
  const errors = [];
  if (!manifest || typeof manifest !== 'object' || Array.isArray(manifest)) {
    return ['manifest must be an object'];
  }

  if (manifest.schema !== MANIFEST_SCHEMA) {
    errors.push(`schema must be ${MANIFEST_SCHEMA}`);
  }
  if (manifest.policyVersion !== POLICY_VERSION) {
    errors.push(`policyVersion must be ${POLICY_VERSION}`);
  }
  if (!['pull_request', 'push', 'workflow_dispatch'].includes(manifest.event)) {
    errors.push(`unsupported event: ${manifest.event}`);
  }

  for (const field of ['requestedBaseSha', 'baseSha', 'headSha']) {
    if (!/^[0-9a-f]{40}$/i.test(manifest[field] ?? '')) {
      errors.push(`${field} must be a full 40-character Git SHA`);
    }
  }

  if (!RISK_NAMES.includes(manifest.risk)) {
    errors.push(`unknown risk: ${manifest.risk}`);
  }

  if (!Array.isArray(manifest.changedPaths)) {
    errors.push('changedPaths must be an array');
  } else {
    const normalized = normalizeChangedPaths(manifest.changedPaths);
    if (JSON.stringify(normalized) !== JSON.stringify(manifest.changedPaths)) {
      errors.push('changedPaths must be normalized, unique, and sorted');
    }
  }

  if (!Array.isArray(manifest.reasons) || manifest.reasons.some((value) => typeof value !== 'string')) {
    errors.push('reasons must be an array of strings');
  }

  if (!manifest.lanes || typeof manifest.lanes !== 'object' || Array.isArray(manifest.lanes)) {
    errors.push('lanes must be an object');
  } else {
    const keys = Object.keys(manifest.lanes).sort();
    const expected = [...LANE_NAMES].sort();
    if (JSON.stringify(keys) !== JSON.stringify(expected)) {
      errors.push(`lanes must contain exactly: ${LANE_NAMES.join(', ')}`);
    }
    for (const lane of LANE_NAMES) {
      if (typeof manifest.lanes[lane] !== 'boolean') {
        errors.push(`lane ${lane} must be boolean`);
      }
    }
    if (manifest.lanes.governance !== true) {
      errors.push('governance lane must always be enabled');
    }
    if (manifest.lanes.full && LANE_NAMES.some((lane) => manifest.lanes[lane] !== true)) {
      errors.push('full lane requires every lane to be enabled');
    }
  }

  const expectedDigest = computeManifestDigest(manifest);
  if (manifest.digest !== expectedDigest) {
    errors.push(`manifest digest mismatch: expected ${expectedDigest}`);
  }

  return errors;
}

export function evaluateOracle({ detector = 'success', enabled = {}, children = {} }) {
  const failures = [];
  if (detector !== 'success') {
    failures.push(`detector=${detector || 'missing'}`);
  }

  const laneNames = [...new Set([...Object.keys(enabled), ...Object.keys(children)])].sort();
  for (const lane of laneNames) {
    const shouldRun = enabled[lane] === true;
    const result = children[lane] ?? 'missing';
    if (shouldRun && result !== 'success') {
      failures.push(`${lane}: expected success, got ${result}`);
      continue;
    }
    if (!shouldRun && ['failure', 'cancelled'].includes(result)) {
      failures.push(`${lane}: unexpected ${result} for disabled lane`);
    }
  }

  return {
    ok: failures.length === 0,
    failures,
  };
}

export function createRunSummary({ manifest, laneResults = {}, oracleResults = {}, generatedAt }) {
  const manifestErrors = validateManifest(manifest);
  if (manifestErrors.length > 0) {
    throw new Error(`invalid delivery manifest: ${manifestErrors.join('; ')}`);
  }

  const enabledLanes = LANE_NAMES.filter((lane) => manifest.lanes[lane]);
  const failedLanes = Object.entries(laneResults)
    .filter(([, result]) => result !== 'success' && result !== 'skipped')
    .map(([lane]) => lane)
    .sort();
  const failedOracles = Object.entries(oracleResults)
    .filter(([, result]) => result !== 'success')
    .map(([oracle]) => oracle)
    .sort();

  const summary = {
    schema: SUMMARY_SCHEMA,
    manifestDigest: manifest.digest,
    baseSha: manifest.baseSha,
    headSha: manifest.headSha,
    event: manifest.event,
    risk: manifest.risk,
    enabledLanes,
    laneResults: canonicalize(laneResults),
    oracleResults: canonicalize(oracleResults),
    failedLanes,
    failedOracles,
    generatedAt: generatedAt ?? new Date().toISOString(),
  };
  summary.digest = computeSummaryDigest(summary);
  return summary;
}
