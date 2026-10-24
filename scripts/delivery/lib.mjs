import crypto from "node:crypto";

export const MANIFEST_SCHEMA_V1 = "bodysense.delivery-manifest/v1";
export const MANIFEST_SCHEMA_V2 = "bodysense.delivery-manifest/v2";
export const MANIFEST_SCHEMA_V3 = "bodysense.delivery-manifest/v3";
export const MANIFEST_SCHEMA = "bodysense.delivery-manifest/v4";
export const MANIFEST_SCHEMAS = Object.freeze([
  MANIFEST_SCHEMA_V1,
  MANIFEST_SCHEMA_V2,
  MANIFEST_SCHEMA_V3,
  MANIFEST_SCHEMA,
]);
export const POLICY_VERSION_V1 = "delivery-policy-v1";
export const POLICY_VERSION_V2 = "delivery-policy-v2";
export const POLICY_VERSION = "delivery-policy-v3";
export const SUMMARY_SCHEMA = "bodysense.delivery-run-summary/v1";

// Lane set for schema v1/v2 manifests. v3 adds the component lane; stored
// v1/v2 manifests keep validating against the set they were generated with.
export const LANE_NAMES_V2 = Object.freeze([
  "governance",
  "web",
  "api",
  "ai",
  "contracts",
  "database",
  "experience",
  "full",
]);

export const LANE_NAMES = Object.freeze([
  "governance",
  "web",
  "api",
  "ai",
  "contracts",
  "database",
  "experience",
  "component",
  "full",
]);

export const RISK_NAMES = Object.freeze([
  "docs",
  "web",
  "api",
  "ai",
  "contract",
  "database",
  "runtime",
  "release",
  "root",
]);

export const LANE_SKIP_REASONS = Object.freeze([
  "no-changed-paths",
  "no-matching-paths",
]);

export function laneNamesForSchema(schema) {
  if (schema === MANIFEST_SCHEMA_V1 || schema === MANIFEST_SCHEMA_V2) {
    return LANE_NAMES_V2;
  }
  return LANE_NAMES;
}

export function laneTargetsForSchema(schema) {
  if (schema === MANIFEST_SCHEMA_V1 || schema === MANIFEST_SCHEMA_V2) {
    return LANE_TARGETS_V2;
  }
  if (schema === MANIFEST_SCHEMA_V3) return LANE_TARGETS_V3;
  return LANE_TARGETS;
}

export function policyVersionForSchema(schema) {
  if (schema === MANIFEST_SCHEMA_V1 || schema === MANIFEST_SCHEMA_V2) {
    return POLICY_VERSION_V1;
  }
  if (schema === MANIFEST_SCHEMA_V3) return POLICY_VERSION_V2;
  return POLICY_VERSION;
}

// Executable units each lane gates. Recorded in the manifest so the contract
// states not only which lanes are required, but exactly what will run.
// Target policy for schema v1/v2 manifests. Kept frozen so stored laneDetails
// keep validating after the v3 policy moved contracts into its own child job.
export const LANE_TARGETS_V2 = Object.freeze({
  governance: Object.freeze(["job:governance-child"]),
  web: Object.freeze([
    "job:quality-child",
    "nx:@bodysense/web:lint",
    "nx:@bodysense/web:typecheck",
    "nx:@bodysense/web:test",
    "nx:@bodysense/web:build",
  ]),
  api: Object.freeze([
    "job:quality-child",
    "nx:api:lint",
    "nx:api:test",
    "nx:api:build",
  ]),
  ai: Object.freeze([
    "job:quality-child",
    "nx:ai-service:format:check",
    "nx:ai-service:lint",
    "nx:ai-service:typecheck",
    "nx:ai-service:test",
    "nx:ai-service:eval:diagnosis",
    "nx:ai-service:eval:diagnosis-evidence-policy",
    "nx:ai-service:eval:diagnosis-promotion",
    "nx:ai-service:eval:treatment",
    "nx:ai-service:eval:treatment-evidence-gap",
    "nx:ai-service:eval:treatment-promotion",
    "script:validate-litellm-gateway.sh",
  ]),
  contracts: Object.freeze([
    "job:quality-child",
    "nx:@bodysense/contracts:lint",
    "nx:@bodysense/contracts:typecheck",
    "nx:@bodysense/contracts:test",
    "pnpm:contracts:verify",
  ]),
  database: Object.freeze([
    "job:migrations-child",
    "job:recovery-migrations-child",
  ]),
  experience: Object.freeze(["job:e2e-child", "playwright:apps/web/e2e"]),
  full: Object.freeze([
    "job:quality-child",
    "job:migrations-child",
    "job:recovery-migrations-child",
    "job:e2e-child",
    "script:validate-repo.sh",
  ]),
});

// Target policy for schema v3 manifests: contracts execute in
// contracts-child (not quality-child), and the component lane gates the
// Storybook browser tests in component-child.
export const LANE_TARGETS_V3 = Object.freeze({
  ...LANE_TARGETS_V2,
  contracts: Object.freeze([
    "job:contracts-child",
    "nx:@bodysense/contracts:lint",
    "nx:@bodysense/contracts:typecheck",
    "nx:@bodysense/contracts:test",
    "pnpm:contracts:verify",
  ]),
  component: Object.freeze([
    "job:component-child",
    "nx:@bodysense/web:component-test",
  ]),
  full: Object.freeze([
    "job:quality-child",
    "job:contracts-child",
    "job:component-child",
    "job:migrations-child",
    "job:recovery-migrations-child",
    "job:e2e-child",
    "script:validate-repo.sh",
  ]),
});

// Target policy for schema v4 manifests: quality lanes execute in
// per-lane children (quality-web/api/ai-child) with quality:architecture
// gated by quality-arch-child, the full lane keeps the validate-repo.sh
// monolith in quality-full-child, and the experience lane adds the
// e2e-report-child merged-report receipt next to the sharded e2e-child.
export const LANE_TARGETS = Object.freeze({
  ...LANE_TARGETS_V3,
  web: Object.freeze([
    "job:quality-web-child",
    "nx:@bodysense/web:lint",
    "nx:@bodysense/web:typecheck",
    "nx:@bodysense/web:test",
    "nx:@bodysense/web:build",
    "job:quality-arch-child",
    "pnpm:quality:architecture",
  ]),
  api: Object.freeze([
    "job:quality-api-child",
    "nx:api:lint",
    "nx:api:test",
    "nx:api:build",
    "job:quality-arch-child",
    "pnpm:quality:architecture",
  ]),
  ai: Object.freeze([
    "job:quality-ai-child",
    "nx:ai-service:format:check",
    "nx:ai-service:lint",
    "nx:ai-service:typecheck",
    "nx:ai-service:test",
    "nx:ai-service:eval:diagnosis",
    "nx:ai-service:eval:diagnosis-evidence-policy",
    "nx:ai-service:eval:diagnosis-promotion",
    "nx:ai-service:eval:treatment",
    "nx:ai-service:eval:treatment-evidence-gap",
    "nx:ai-service:eval:treatment-promotion",
    "script:validate-litellm-gateway.sh",
    "job:quality-arch-child",
    "pnpm:quality:architecture",
  ]),
  experience: Object.freeze([
    "job:e2e-child",
    "job:e2e-report-child",
    "playwright:apps/web/e2e",
  ]),
  full: Object.freeze([
    "job:quality-full-child",
    "job:contracts-child",
    "job:component-child",
    "job:migrations-child",
    "job:recovery-migrations-child",
    "job:e2e-child",
    "job:e2e-report-child",
    "script:validate-repo.sh",
  ]),
});

// Which manifest lanes make each Oracle child required. Oracles derive their
// expectations from the digest-validated manifest through this table instead
// of re-deriving lane expressions in workflow YAML.
export const ORACLE_GROUPS = Object.freeze({
  governance: Object.freeze({ governance: Object.freeze(["governance"]) }),
  quality: Object.freeze({
    // The full lane keeps the validate-repo.sh monolith child; per-lane
    // children are required only when their lane is selected without the
    // full lane (their jobs are skipped in full runs to avoid executing
    // the same checks twice).
    "quality-full": Object.freeze(["full"]),
    "quality-web": Object.freeze({
      anyOf: Object.freeze(["web"]),
      noneOf: Object.freeze(["full"]),
    }),
    "quality-api": Object.freeze({
      anyOf: Object.freeze(["api"]),
      noneOf: Object.freeze(["full"]),
    }),
    "quality-ai": Object.freeze({
      anyOf: Object.freeze(["ai"]),
      noneOf: Object.freeze(["full"]),
    }),
    "quality-arch": Object.freeze({
      anyOf: Object.freeze(["web", "api", "ai"]),
      noneOf: Object.freeze(["full"]),
    }),
    contracts: Object.freeze(["contracts", "full"]),
  }),
  database: Object.freeze({
    "database-baseline": Object.freeze(["database", "full"]),
    "database-recovery": Object.freeze(["database", "full"]),
  }),
  experience: Object.freeze({
    experience: Object.freeze(["experience", "full"]),
    component: Object.freeze(["component", "full"]),
  }),
});

const FULL_LANES = Object.freeze(
  Object.fromEntries(LANE_NAMES.map((lane) => [lane, true])),
);

const RISK_PRIORITY = new Map(RISK_NAMES.map((risk, index) => [risk, index]));

function normalizePath(path) {
  return String(path).replaceAll("\\", "/").replace(/^\.\//, "");
}

export function normalizeChangedPaths(paths) {
  return [...new Set(paths.map(normalizePath).filter(Boolean))].sort();
}

export function canonicalize(value) {
  if (Array.isArray(value)) {
    return value.map(canonicalize);
  }
  if (value && typeof value === "object") {
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
  return `sha256:${crypto.createHash("sha256").update(canonical).digest("hex")}`;
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
  return Object.fromEntries(
    LANE_NAMES.map((lane) => [lane, lane === "governance"]),
  );
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
  let selected = "docs";
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
    path.startsWith("docs/") ||
    path === "README.md" ||
    path === "CONTRIBUTING.md" ||
    path.endsWith(".md")
  ) {
    return { risk: "docs", lanes: {}, reason: "documentation" };
  }

  // Delivery/release/runtime coordinates are intentionally full because a
  // detector bug in its own implementation must never reduce validation.
  if (
    path.startsWith(".github/") ||
    path.startsWith("scripts/delivery/") ||
    path.startsWith("docker/") ||
    path.startsWith("deploy/") ||
    path === ".env.production" ||
    path === "release-please-config.json" ||
    path === ".release-please-manifest.json"
  ) {
    const risk =
      path === "release-please-config.json" ||
      path === ".release-please-manifest.json"
        ? "release"
        : "runtime";
    return { risk, lanes: allTrueLanes(), reason: "delivery-runtime" };
  }

  if (
    path === "package.json" ||
    path === "pnpm-lock.yaml" ||
    path === "pnpm-workspace.yaml" ||
    path === "nx.json" ||
    path.startsWith("tsconfig") ||
    path.startsWith(".npm") ||
    path.startsWith(".node-version") ||
    path.startsWith(".python-version") ||
    path === "uv.lock"
  ) {
    return { risk: "root", lanes: allTrueLanes(), reason: "root-toolchain" };
  }

  if (path.startsWith("apps/api/migrations/")) {
    return {
      risk: "database",
      lanes: {
        api: true,
        database: true,
        experience: true,
      },
      reason: "database-migration",
    };
  }

  if (path.startsWith("apps/api/")) {
    return {
      risk: "api",
      lanes: { api: true, experience: true },
      reason: "go-api",
    };
  }

  if (path.startsWith("apps/ai-service/")) {
    return {
      risk: "ai",
      lanes: { ai: true, experience: true },
      reason: "ai-service",
    };
  }

  if (path.startsWith("apps/web/")) {
    return {
      risk: "web",
      lanes: { web: true, experience: true, component: true },
      reason: "web",
    };
  }

  if (path.startsWith("packages/contracts/")) {
    return {
      risk: "contract",
      lanes: {
        web: true,
        api: true,
        ai: true,
        contracts: true,
        experience: true,
        component: true,
      },
      reason: "shared-contract",
    };
  }

  if (path.startsWith("postman/") || path.startsWith("scripts/postman/")) {
    return {
      risk: "contract",
      lanes: { contracts: true },
      reason: "postman-contract-view",
    };
  }

  if (path.startsWith("packages/utils/")) {
    return {
      risk: "root",
      lanes: allTrueLanes(),
      reason: "shared-utility-fail-safe",
    };
  }

  if (
    path.startsWith("scripts/static-assets/") ||
    path.startsWith("scripts/anatomy/")
  ) {
    return {
      risk: "runtime",
      lanes: allTrueLanes(),
      reason: "public-asset-runtime",
    };
  }

  if (path.startsWith("scripts/")) {
    // Operational/test scripts frequently participate in deploy, migration,
    // privacy or release validation. Unknown scripts are therefore full risk.
    return {
      risk: "runtime",
      lanes: allTrueLanes(),
      reason: "repository-script",
    };
  }

  if (path.startsWith("tests/") || path.startsWith("e2e/")) {
    return {
      risk: "contract",
      lanes: { contracts: true, experience: true },
      reason: "cross-service-test",
    };
  }

  // Unknown executable or repository-root content is deliberately safest/full.
  return {
    risk: "root",
    lanes: allTrueLanes(),
    reason: "unknown-path-fail-safe",
  };
}

export function classifyPaths(paths, { forceFull = false } = {}) {
  const changedPaths = normalizeChangedPaths(paths);
  if (forceFull) {
    return {
      risk: "root",
      lanes: allTrueLanes(),
      reasons: ["full-main-policy"],
      changedPaths,
      laneFiles: Object.fromEntries(
        LANE_NAMES.map((lane) => [lane, [...changedPaths]]),
      ),
    };
  }

  if (changedPaths.length === 0) {
    // An empty diff is allowed for manual diagnostics, but it must never be
    // interpreted as docs-only. Governance remains active and the manifest
    // records the explicit no-change reason.
    return {
      risk: "docs",
      lanes: emptyLanes(),
      reasons: ["no-changed-paths"],
      changedPaths,
      laneFiles: Object.fromEntries(LANE_NAMES.map((lane) => [lane, []])),
    };
  }

  const lanes = emptyLanes();
  const laneFileSets = new Map(LANE_NAMES.map((lane) => [lane, new Set()]));
  const risks = new Set(["docs"]);
  const reasons = new Set();

  for (const path of changedPaths) {
    const classification = classifySinglePath(path);
    risks.add(classification.risk);
    reasons.add(classification.reason);
    // Governance inspects the whole diff, so every path triggers it.
    laneFileSets.get("governance").add(path);
    for (const [lane, enabled] of Object.entries(classification.lanes)) {
      if (enabled) laneFileSets.get(lane).add(path);
    }
    mergeLanes(lanes, classification.lanes);
  }

  return {
    risk: highestRisk(risks),
    lanes: lanes.full ? allTrueLanes() : lanes,
    reasons: [...reasons].sort(),
    changedPaths,
    laneFiles: Object.fromEntries(
      LANE_NAMES.map((lane) => [lane, [...laneFileSets.get(lane)].sort()]),
    ),
  };
}

function buildLaneDetails(classification) {
  const { lanes, laneFiles, changedPaths } = classification;
  return Object.fromEntries(
    LANE_NAMES.map((lane) => {
      const required = lanes[lane] === true;
      return [
        lane,
        {
          required,
          files: [...(laneFiles[lane] ?? [])],
          targets: [...LANE_TARGETS[lane]],
          skipReason: required
            ? null
            : changedPaths.length === 0
              ? "no-changed-paths"
              : "no-matching-paths",
        },
      ];
    }),
  );
}

export function createManifest({
  baseSha,
  headSha,
  requestedBaseSha = baseSha,
  event,
  ref = "",
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
    laneDetails: buildLaneDetails(classification),
    generatedAt,
  };
  manifest.digest = computeManifestDigest(manifest);
  return manifest;
}

export function validateManifest(manifest) {
  const errors = [];
  if (!manifest || typeof manifest !== "object" || Array.isArray(manifest)) {
    return ["manifest must be an object"];
  }

  if (!MANIFEST_SCHEMAS.includes(manifest.schema)) {
    errors.push(`schema must be one of: ${MANIFEST_SCHEMAS.join(", ")}`);
  }
  const expectedPolicy = policyVersionForSchema(manifest.schema);
  if (manifest.policyVersion !== expectedPolicy) {
    errors.push(`policyVersion must be ${expectedPolicy}`);
  }
  if (!["pull_request", "push", "workflow_dispatch"].includes(manifest.event)) {
    errors.push(`unsupported event: ${manifest.event}`);
  }

  for (const field of ["requestedBaseSha", "baseSha", "headSha"]) {
    if (!/^[0-9a-f]{40}$/i.test(manifest[field] ?? "")) {
      errors.push(`${field} must be a full 40-character Git SHA`);
    }
  }

  if (!RISK_NAMES.includes(manifest.risk)) {
    errors.push(`unknown risk: ${manifest.risk}`);
  }

  if (!Array.isArray(manifest.changedPaths)) {
    errors.push("changedPaths must be an array");
  } else {
    const normalized = normalizeChangedPaths(manifest.changedPaths);
    if (JSON.stringify(normalized) !== JSON.stringify(manifest.changedPaths)) {
      errors.push("changedPaths must be normalized, unique, and sorted");
    }
  }

  if (
    !Array.isArray(manifest.reasons) ||
    manifest.reasons.some((value) => typeof value !== "string")
  ) {
    errors.push("reasons must be an array of strings");
  }

  if (
    !manifest.lanes ||
    typeof manifest.lanes !== "object" ||
    Array.isArray(manifest.lanes)
  ) {
    errors.push("lanes must be an object");
  } else {
    const schemaLanes = laneNamesForSchema(manifest.schema);
    const keys = Object.keys(manifest.lanes).sort();
    const expected = [...schemaLanes].sort();
    if (JSON.stringify(keys) !== JSON.stringify(expected)) {
      errors.push(`lanes must contain exactly: ${schemaLanes.join(", ")}`);
    }
    for (const lane of schemaLanes) {
      if (typeof manifest.lanes[lane] !== "boolean") {
        errors.push(`lane ${lane} must be boolean`);
      }
    }
    if (manifest.lanes.governance !== true) {
      errors.push("governance lane must always be enabled");
    }
    if (
      manifest.lanes.full &&
      schemaLanes.some((lane) => manifest.lanes[lane] !== true)
    ) {
      errors.push("full lane requires every lane to be enabled");
    }
  }

  // laneDetails exist on every schema from v2 onward; only v1 predates
  // them. (Pinning this to "current or v2" would reject stored v3
  // manifests the moment the current schema moves to v4.)
  if (manifest.schema !== MANIFEST_SCHEMA_V1) {
    errors.push(...validateLaneDetails(manifest));
  } else if (manifest.laneDetails !== undefined) {
    errors.push(`laneDetails requires schema ${MANIFEST_SCHEMA_V2} or later`);
  }

  const expectedDigest = computeManifestDigest(manifest);
  if (manifest.digest !== expectedDigest) {
    errors.push(`manifest digest mismatch: expected ${expectedDigest}`);
  }

  return errors;
}

function validateLaneDetails(manifest) {
  const errors = [];
  const details = manifest.laneDetails;
  if (!details || typeof details !== "object" || Array.isArray(details)) {
    return ["laneDetails must be an object"];
  }
  const schemaLanes = laneNamesForSchema(manifest.schema);
  const schemaTargets = laneTargetsForSchema(manifest.schema);
  const keys = Object.keys(details).sort();
  const expectedKeys = [...schemaLanes].sort();
  if (JSON.stringify(keys) !== JSON.stringify(expectedKeys)) {
    return [`laneDetails must contain exactly: ${schemaLanes.join(", ")}`];
  }

  const changed = new Set(
    Array.isArray(manifest.changedPaths) ? manifest.changedPaths : [],
  );
  for (const lane of schemaLanes) {
    const entry = details[lane];
    if (!entry || typeof entry !== "object" || Array.isArray(entry)) {
      errors.push(`laneDetails.${lane} must be an object`);
      continue;
    }
    const required = manifest.lanes?.[lane] === true;
    if (entry.required !== required) {
      errors.push(`laneDetails.${lane}.required must match lanes.${lane}`);
    }
    if (!Array.isArray(entry.files)) {
      errors.push(`laneDetails.${lane}.files must be an array`);
    } else {
      const normalized = normalizeChangedPaths(entry.files);
      if (JSON.stringify(normalized) !== JSON.stringify(entry.files)) {
        errors.push(
          `laneDetails.${lane}.files must be normalized, unique, and sorted`,
        );
      }
      for (const file of entry.files) {
        if (!changed.has(file)) {
          errors.push(
            `laneDetails.${lane}.files contains ${file} which is not in changedPaths`,
          );
        }
      }
      if (required && changed.size > 0 && entry.files.length === 0) {
        errors.push(
          `laneDetails.${lane}.files must record the paths that require the lane`,
        );
      }
      if (!required && entry.files.length > 0) {
        errors.push(
          `laneDetails.${lane}.files must be empty for a lane that is not required`,
        );
      }
    }
    if (
      JSON.stringify(entry.targets) !== JSON.stringify([...schemaTargets[lane]])
    ) {
      errors.push(
        `laneDetails.${lane}.targets must equal the policy targets for lane ${lane}`,
      );
    }
    if (required) {
      if (entry.skipReason !== null) {
        errors.push(
          `laneDetails.${lane}.skipReason must be null for a required lane`,
        );
      }
    } else if (!LANE_SKIP_REASONS.includes(entry.skipReason)) {
      errors.push(
        `laneDetails.${lane}.skipReason must be one of: ${LANE_SKIP_REASONS.join(", ")}`,
      );
    }
  }
  return errors;
}

export function evaluateOracle({
  detector = "success",
  enabled = {},
  children = {},
}) {
  const failures = [];
  if (detector !== "success") {
    failures.push(`detector=${detector || "missing"}`);
  }

  const laneNames = [
    ...new Set([...Object.keys(enabled), ...Object.keys(children)]),
  ].sort();
  for (const lane of laneNames) {
    const shouldRun = enabled[lane] === true;
    const result = children[lane] ?? "missing";
    if (shouldRun && result !== "success") {
      failures.push(`${lane}: expected success, got ${result}`);
      continue;
    }
    if (!shouldRun && ["failure", "cancelled"].includes(result)) {
      failures.push(`${lane}: unexpected ${result} for disabled lane`);
    }
  }

  return {
    ok: failures.length === 0,
    failures,
  };
}

export function oracleEnabledFromManifest(manifest, group) {
  const mapping = ORACLE_GROUPS[group];
  if (!mapping) throw new Error(`unknown oracle group: ${group}`);
  return Object.fromEntries(
    Object.entries(mapping).map(([child, rule]) => {
      // A rule is either a lane list (child required when any listed lane
      // is selected) or { anyOf, noneOf } (required when any anyOf lane is
      // selected and no noneOf lane is).
      if (Array.isArray(rule)) {
        return [child, rule.some((lane) => manifest.lanes[lane] === true)];
      }
      const anyOf = rule.anyOf ?? [];
      const noneOf = rule.noneOf ?? [];
      return [
        child,
        anyOf.some((lane) => manifest.lanes[lane] === true) &&
          !noneOf.some((lane) => manifest.lanes[lane] === true),
      ];
    }),
  );
}

export function evaluateManifestOracle({
  manifest,
  group,
  detector = "success",
  children = {},
}) {
  const manifestErrors = validateManifest(manifest);
  if (manifestErrors.length > 0) {
    return {
      ok: false,
      failures: manifestErrors.map((error) => `manifest: ${error}`),
    };
  }
  return evaluateOracle({
    detector,
    enabled: oracleEnabledFromManifest(manifest, group),
    children,
  });
}

export function createRunSummary({
  manifest,
  laneResults = {},
  oracleResults = {},
  generatedAt,
}) {
  const manifestErrors = validateManifest(manifest);
  if (manifestErrors.length > 0) {
    throw new Error(`invalid delivery manifest: ${manifestErrors.join("; ")}`);
  }

  const enabledLanes = LANE_NAMES.filter((lane) => manifest.lanes[lane]);
  const failedLanes = Object.entries(laneResults)
    .filter(([, result]) => result !== "success" && result !== "skipped")
    .map(([lane]) => lane)
    .sort();
  const failedOracles = Object.entries(oracleResults)
    .filter(([, result]) => result !== "success")
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
