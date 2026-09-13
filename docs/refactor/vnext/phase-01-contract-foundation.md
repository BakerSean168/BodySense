# BodySense vNext Phase 01 — Contract Foundation

- Branch: `refactor/vnext-01-contract-foundation`
- Parent integration commit: `8aecc5028652a553e58ba3e2f8564605b39f6a58`
- Scope: contract tooling/governance only; no production route, public event, or internal runtime protocol cutover

## Implemented foundation

### Canonical boundary directories

- `packages/contracts/openapi/` — public REST OpenAPI 3.1 authority starting in Phase 02.
- `packages/contracts/schemas/` — public JSON StreamEvent authority, canonicalized in Phase 03.
- `contracts/internal/agent-runtime/v1/` — private Go/Python Proto authority starting in Phase 04.

The non-serving samples under `tools/contracts/foundation/` exist only to exercise the tooling before any production boundary depends on it.

### Pinned toolchain

The exact versions are recorded in `contracts/toolchain.json` and cross-checked against `package.json`.

JavaScript tooling is installed as exact dev dependencies. Go CLI tools are installed into ignored `.tools/contracts/bin` with exact version stamps by `pnpm contracts:bootstrap`; no binary is committed.

### Deterministic generation

`pnpm contracts:generate` currently proves all three selected technology families:

- OpenAPI -> oapi-codegen Go types;
- OpenAPI -> Orval validated Fetch + Zod Mini TypeScript;
- JSON Schema -> generated TypeScript declaration + standalone Ajv validator;
- Proto/Buf/Protovalidate -> generated Go + Python boundary types.

All smoke artifacts are committed under `tools/contracts/generated/` and contain generator-owned markers. `pnpm contracts:check-generated` hashes the current outputs, regenerates them, and fails on any drift or nondeterminism.

### Governance gates

`pnpm contracts:verify` runs:

1. toolchain pin verification;
2. Redocly OpenAPI lint;
3. strict JSON parse/schema compilation support;
4. Buf STANDARD lint;
5. deterministic regenerate/no-diff;
6. OpenAPI and Proto breaking checks against approved foundation baselines;
7. deliberate OpenAPI/Proto breaking mutations that must be blocked;
8. JSON Schema semantic mutation probes;
9. generated validator behavior;
10. generated Web TypeScript typecheck;
11. generated Go syntax/type compile;
12. generated Python bytecode compile;
13. Buf build;
14. architecture guard preventing production code from depending on spike experiment paths.

### Generated-context hygiene

`.ignore`, ESLint ignore rules and Prettier ignore rules exclude generated trees from normal search/review/lint context. Generated artifacts still compile through explicit contract conformance checks.

This implements the spike finding that codegen only reduces Agent/reviewer context when generated output is excluded by default.

## Important Orval boundary retained for Phase 02

The foundation generation reconfirmed that generated transport error aliases should not become application error semantics. The generated `forceSuccessResponse` Fetch client correctly keeps non-2xx responses out of the success path, but its generated error body typing is not sufficient authority for BodySense's normalized error behavior.

Phase 02 therefore keeps the already-decided boundary:

```text
Orval validated success transport
  -> handwritten ApiRequestError/error-envelope adapter
  -> feature/query code
```

Generated error types do not leak into feature/domain code.

## Batch review repair

Phase 01 review found one P1 reproducibility defect unrelated to the new generators: Web contract imports could resolve only when an incidental pnpm workspace symlink existed because the app-level `paths` map replaced the root `@bodysense/*` map and Vite had no explicit contracts alias.

The repair adds an explicit `@bodysense/contracts` TypeScript path and Vite alias while retaining `workspace:*` as the package-graph dependency. Verification deliberately removed `apps/web/node_modules` before running Web typecheck, all 212 tests, and production build.

## Explicit non-changes

Phase 01 does not:

- add production OpenAPI routes;
- switch any Web request to generated code;
- replace the current StreamEvent parser;
- introduce the real internal Agent Proto messages;
- change HTTP/NDJSON transport;
- add GraphQL/gRPC/Connect production behavior.

Those remain dependency-ordered Phase 02–04 work.

## Verification and batch-review outcome

Phase 01 is accepted with no remaining P0/P1 findings. The batch review found `BS-VNEXT-CONTRACT-002` (P1), where Web contract resolution depended on incidental pnpm workspace-link state; it was repaired with explicit TypeScript/Vite aliases and closed by deleting `apps/web/node_modules` before verification.

Executed successfully on this branch:

```text
pnpm contracts:verify       PASS
pnpm lint                   PASS
pnpm typecheck              PASS
pnpm test                   PASS
  Web                       212/212
  Python                    475/475
  Go                        go test ./... PASS
  contracts                 12/12 + Go/Python parity PASS
pnpm build                  PASS
git diff --check            PASS
```

Additional reproducibility proof with the Web workspace symlink intentionally absent:

```text
pnpm nx run @bodysense/web:typecheck  PASS
pnpm nx run @bodysense/web:test       212/212 PASS
pnpm nx run @bodysense/web:build      PASS
```

Known non-blocking baseline findings remain in `docs/refactor/vnext/baseline/finding-ledger.json` (including test stderr noise, Nx ESLint deprecation and the existing large BodyExplorer3D bundle); they are not Phase 01 contract-foundation regressions.
