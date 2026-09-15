# Phase 09 — Repo-native API tooling

Status: **COMPLETE / READY TO MERGE**

Canonical parent before Phase 09: `refactor/bodysense-vnext` at `70aeeb88337c40fb59d1da54f1177a6ae085beb5`.

## Goal

Make API exploration and Postman tooling a deterministic projection of the canonical BodySense public REST contract instead of a second manually maintained API specification.

## Contract authority

The only browser-facing REST schema authority remains:

```text
packages/contracts/openapi/bodysense.v1.openapi.yaml
```

The generated Postman collection must never be edited as an independent request/response contract. Python AI-service endpoints (`/runtime/*`, `/api/diagnosis/*`, `/api/treatment/*`, `/api/assessment/*`, `/api/knowledge/*`, `/api/ocr/*`, `/api/posture/*`, `/api/title/*`, and related service-local routes) remain internal service boundaries and are intentionally excluded from the public collection.

## Deterministic generation

`pnpm postman:generate` converts the canonical OpenAPI with a pinned `openapi-to-postmanv2@6.3.3` dependency and then normalizes all generator-owned nondeterminism:

1. a fixed pseudorandom seed makes schema-faked values reproducible;
2. a fixed generator clock makes generated date/time examples reproducible;
3. collection, folder, request, response, and environment IDs are derived from stable semantic identities using SHA-256-backed UUID-shaped values;
4. route parity compares the generated collection directly with the canonical OpenAPI after normalizing path parameters;
5. `pnpm postman:verify` generates the expected bytes in memory and fails if any committed Postman asset is stale.

This was necessary because the upstream OpenAPI converter otherwise creates random UUIDs and random schema examples even when the input specification has not changed.

## Repository assets

Generated / repo-owned:

```text
postman/collections/BodySense Public API.postman_collection.json
postman/environments/BodySense Dev.environment.json
postman/environments/BodySense Staging.environment.json
postman/environments/BodySense Production.environment.json
postman/README.md
```

Environment policy:

- Dev: `http://127.0.0.1:20101`;
- Production: `https://body.bakersean.top`;
- Staging: intentionally empty because the repository does not own a stable public staging origin;
- `bearerToken`: always committed empty and typed as a secret environment variable.

No Postman API key, access token, application bearer token, password, or provider credential is stored in these files.

## Native Git / current Postman CLI validation

The GCP development host was missing Postman CLI, so Phase 09 installed and verified `postman-cli@1.56.2` before relying on Native Git behavior.

The current CLI confirms collection/environment linting and v2.1 -> Native Git v3 migration. The repository therefore keeps the deterministic JSON projection as the canonical generated artifact and validates that it migrates deterministically to current v3 layout.

A fabricated `.postman/resources.yaml` is deliberately not committed. Current Native Git workspace metadata requires a real, non-empty cloud workspace ID; `postman workspace prepare/push` also requires Postman credentials. The development host has no Postman login/API key, so cloud binding remains a one-time credentialed activation step. This does not change the local contract authority or reproducibility guarantee.

## Retired source-scraping gate

The former active `scripts/contracts/public-route-coverage.mjs` gate depended on the frozen Phase 00 source-scraped route inventory. That inventory was useful while the OpenAPI migration was incomplete, but after the public contract reached 95/95 it became a migration-era second inventory.

Phase 09 removes that active gate. The generated Phase 02 coverage reports remain historical evidence, while current validation is now:

```text
canonical OpenAPI -> deterministic Postman projection -> 95/95 parity
```

The source-scraped Phase 00 inventory remains only as refactor evidence and can still be explicitly recaptured by the refactor tooling; it is no longer part of normal API contract verification.

## CI / delivery enforcement

The delivery contracts lane now runs `pnpm contracts:verify`, not only the contracts package's Nx lint/typecheck/test targets. The full repository release gate also runs `pnpm contracts:verify`.

Generated `postman/**` and `scripts/postman/**` changes are explicitly classified as contract-view changes, so they run the contract lane without unnecessarily forcing the full delivery matrix. A change to the canonical OpenAPI under `packages/contracts/**` still selects Web, API, AI, contracts, and experience lanes.

## Scenario suites

No pre-existing handwritten Postman scenario collection existed in the repository, so Phase 09 does not invent one merely to satisfy a folder structure. Authentication, concurrency, negative behavior, recovery, and multi-step workflows already have stronger repository-level unit/integration/E2E coverage.

Future Postman scenario suites may be added when they provide interactive debugging value, but their request definitions must be derived from or reference canonical operations rather than copying request/response schemas into a second specification.

## Acceptance evidence

Focused evidence already green before the final whole-repository gate:

```text
Postman generation: PASS routes=95
Postman verification: PASS routes=95
Dev environment lint: 0 errors / 0 warnings
Staging environment lint: 0 errors / 0 warnings
Production environment lint: 0 errors / 0 warnings
Native Git v3 migration repeatability: PASS
Native Git v3 collection lint: 95 scanned / 0 errors / 0 warnings
POSTMAN_NATIVE_VALIDATION=PASS
pnpm contracts:verify=PASS
pnpm test:delivery=33/33 PASS
git diff --check=PASS
```

The two existing Redocly ambiguous-path diagnostics around conversation share/id routes remain warnings and are not introduced by Phase 09.

Final whole-repository quality evidence:

```text
pnpm verify:release=PASS
Web tests=266 PASS
AI-service tests=507 PASS
Go tests=PASS
contracts:verify=PASS
Postman verification=95/95 PASS
MIGRATION_SEQUENCE=PASS latest=1
MIGRATION_IMMUTABILITY=PASS
offhost DR unit tests=88/88 PASS
OFFHOST_DR_INTEGRATION=PASS
assessment/diagnosis/treatment eval lanes=PASS
LITELLM_GATEWAY_SMOKE=PASS
PYDANTICAI_LITELLM_ADAPTER_SMOKE=PASS
AI_SERVICE_LITELLM_GATEWAY_SMOKE=PASS
build=PASS
REPO_QUALITY=PASS
```

The first whole-repository attempt was infrastructure-blocked when the GCP development host reached `98%` root-disk usage and the disposable DR validator build hit `no space left on device`. No application or contract failure was involved. Phase 09 reclaimed only unused Docker build cache and zero-consumer validation/debug images, leaving active containers and data volumes untouched; the host then had about `25 GB` available and the complete gate passed.
