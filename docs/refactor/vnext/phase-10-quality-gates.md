# Phase 10 — Quality gates and architecture enforcement

Status: **COMPLETE / READY TO MERGE**

Canonical parent before Phase 10: `refactor/bodysense-vnext` at `b37409b8d28a46113825ae2d661679e9345e5bcb`.

## Goal

Turn the architecture established by Phases 01–09 into executable policy. The repository should reject accidental transport/generated-type leakage, unsafe trust-boundary shortcuts, and selected static-analysis regressions before they become review-only findings.

This phase deliberately does **not** introduce a vanity rule that makes every historical style warning fatal. It promotes rules only when the current repository is already clean or when a small, semantically safe cleanup can make the rule clean.

## Structured architecture policy

The old `scripts/contracts/check-architecture.mjs` implementation relied on source-string matching. Phase 10 replaces it with language-aware checks under `scripts/quality/`.

### TypeScript

`check-ts-boundaries.mjs` parses TypeScript/TSX with the TypeScript compiler API.

Generated browser OpenAPI modules under `@/generated/api` may be consumed only by explicit trust-boundary adapters:

- feature `services/**`;
- `features/workspace/api/**`;
- stores that own API/session persistence adaptation;
- `lib/clientDiagnostics.ts`;
- the explicit consultation pending-interaction projection adapter.

UI/domain components cannot import generated API types directly. Retired generic internal-runtime authority identifiers are also rejected structurally.

### Python

`check_python_boundaries.py` uses Python `ast`.

Generated runtime Proto may be imported only by:

- `src/api/runtime_proto_adapter.py`;
- the dedicated runtime Proto contract test.

The retired generic `src/models/stream_event.py` authority and `_RUNTIME_EVENT_FIELD_BY_TYPE` registry are prohibited from resurfacing.

### Go

`check_go_boundaries.go` uses the Go parser and import AST.

- generated public OpenAPI types are restricted to `internal/transport/httpapi/**` and server bootstrap;
- generated runtime Proto is restricted to the two runtime boundary adapters plus the contract test;
- model/repository/service domain packages cannot import the HTTP transport package.

Generated transport types therefore remain at serialization/trust boundaries rather than becoming domain models.

## Mutation / violation fixtures

`quality:architecture:test` contains positive and negative fixtures for all three languages. In addition to unit-testing the analyzers, it launches each real checker against an isolated temporary repository tree and verifies a representative architecture violation returns a non-zero process status.

Current real-gate negative fixtures prove:

```text
TypeScript UI -> generated OpenAPI import        => FAIL
Python runtime domain -> generated Proto import => FAIL
Go service/domain -> generated OpenAPI import   => FAIL
```

The current repository itself must simultaneously report:

```text
TS_GENERATED_BOUNDARY=PASS
PYTHON_GENERATED_BOUNDARY=PASS
GO_GENERATED_BOUNDARY=PASS
ARCHITECTURE_BOUNDARY_POLICY=PASS
ARCHITECTURE_POLICY_MUTATION_TESTS=PASS
```

This is stronger than checking source snippets for specific strings: imports are parsed according to the language grammar and the same CLI gate used by CI is exercised by the negative fixtures.

## Static analysis baselines

### TypeScript / ESLint

Generated directories remain excluded from handwritten ESLint policy and are still compiled by the TypeScript/codegen checks.

Phase 10 enables:

- `@typescript-eslint/no-explicit-any = error` for handwritten TypeScript;
- `@typescript-eslint/no-unsafe-type-assertion = error` on browser/API trust-boundary modules, while excluding test fixtures.

The narrower trust-boundary rule is intentional. Enabling `no-unsafe-type-assertion` over all UI and tests currently identifies unrelated Canvas/DOM test doubles and branded-type casts; those are not wire-trust regressions and are not made Phase 10 blockers.

### Python / Ruff + Pyright

Ruff now explicitly excludes `src/generated`, while generated Proto continues to be imported/compiled by runtime contract checks.

Pyright remains `basic` for the overall dynamic AI runtime, but the private Go↔Python Proto adapter is promoted to `strict`. A redundant `MessageToDict` result check was removed because the protobuf stub already proves the return type, while the final runtime-event variant was rewritten as an exhaustive `match` + `assert_never` so strict checking does not weaken exhaustiveness. Its focused runtime Proto tests remain green.

### Go / vet + staticcheck

Go lint now runs both:

```text
go vet ./...
staticcheck v0.8.1: SA*, S1*, QF*
```

`staticcheck` is version-pinned. Style-only `ST*` and unused-code `U1000` are not promoted wholesale because the existing repository contains historical style/dead-code findings unrelated to runtime correctness. Semantic/simplification checks are clean and therefore become a hard baseline.

`SA1019` is included rather than suppressed. Handwritten code that still used the generated compatibility wrapper `GetSwagger()` was migrated to current `GetSpec()` first.

## CI / release integration

- `pnpm quality:architecture` runs the real three-language boundary policy.
- `pnpm quality:architecture:test` runs the violation fixtures.
- `pnpm quality:verify` runs both.
- every selected delivery quality lane runs `quality:architecture` before it can report PASS;
- the full repository release gate runs `quality:verify`;
- language-specific lint/typecheck lanes continue to enforce static-analysis rules appropriate to the changed surface.

Quality-policy implementation files remain fail-safe under the existing delivery classifier because `scripts/**` changes select the full delivery policy.

## Validation-host capacity preflight

The full release gate now performs an early host-capacity check before expensive Docker/Go validation. The default policy requires at least `8 GiB` free on the validation filesystem and can be overridden explicitly for a different runner policy.

A dedicated fixture proves both sufficient-capacity PASS and low-capacity FAIL behavior. This converts the Phase 09 `no space left on device` incident from a late compiler failure into an immediate, actionable `VALIDATION_HOST_CAPACITY=FAIL` result without deleting or mutating any runtime data.

## Existing exhaustive/state-machine protection retained

Phase 10 does not duplicate stronger gates already established earlier:

- public StreamEvent schema mutation/conformance plus exhaustive Web consumers remain under `contracts:mutation` / contracts tests;
- private Proto typed `oneof` and Protovalidate rules remain under `check-runtime-proto-rules.mjs` and runtime contract tests;
- durable finite lifecycle/CAS transition behavior remains covered by Phase 06 Go tests and production-shaped validation.

The migration-era source-string architecture checker is removed only after the meaningful generated-boundary and retired-runtime-authority protections are represented structurally.

## Focused acceptance evidence

Already green during implementation:

```text
pnpm quality:verify=PASS
architecture real-gate negative fixtures: 3/3 PASS
TypeScript analyzer tests: PASS
Python analyzer tests: PASS
Go analyzer tests: PASS
Go full package tests after GetSpec migration=PASS
Go vet + staticcheck SA*/S1*/QF*=PASS
Web ESLint with trust-boundary rules=PASS
AI Ruff=PASS
Pyright including strict runtime Proto adapter=PASS
runtime Proto focused tests=22 PASS
pnpm test:delivery=34/34 PASS
contracts:verify=PASS
VALIDATION_HOST_CAPACITY=PASS (25 GiB free, 8 GiB required)
VALIDATION_HOST_CAPACITY_TEST=PASS
git diff --check=PASS
```

The two existing Redocly ambiguous conversation paths remain warnings from the canonical OpenAPI and are not introduced by Phase 10.

## Final whole-repository acceptance

The closing `pnpm verify:release` completed with exit code `0`:

```text
VALIDATION_HOST_CAPACITY=PASS free_gib=25 required_gib=8
lint=PASS
staticcheck SA*/S1*/QF*=PASS
typecheck=PASS
all repository tests=PASS
ARCHITECTURE_BOUNDARY_POLICY=PASS
ARCHITECTURE_POLICY_MUTATION_TESTS=PASS
VALIDATION_HOST_CAPACITY_TEST=PASS
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

Phase 10 therefore closes with executable architecture boundaries rather than review-only conventions, static-analysis baselines that are already clean, representative fail-closed mutation fixtures, and an early capacity guard for the heavyweight validation path.
