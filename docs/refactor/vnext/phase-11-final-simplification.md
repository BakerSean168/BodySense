# Phase 11 — Final simplification and whole-system review

Status: **COMPLETE / READY TO MERGE**

Canonical parent before Phase 11: `refactor/bodysense-vnext` at `b53404134b11936b374620b6a51ab762f01fbd6b`.

## Goal

Remove refactor-only scaffolding, collapse the remaining duplicate implementation path found during the final audit, align current documentation with the implemented vNext system, and prove the complete architecture against a fresh production-shaped local deployment.

Phase 11 does not reopen completed migration history or erase historical evidence. The rule is:

```text
historical evidence stays
current executable scaffolding goes when it has no current owner
permanent capabilities move out of the refactor namespace
real unresolved product/operations gates remain explicit
```

## Refactor scaffolding cleanup

### Phase 00 baseline capture

`scripts/refactor/capture-vnext-baseline.mjs` and the `refactor:vnext:baseline` package command were one-time Phase 00 capture machinery. The frozen evidence under `docs/refactor/vnext/baseline/` remains, while the live capture script is removed.

The historical `verification-results.md` and generated baseline file still name the commands that actually ran at the Phase 00 commit. They are evidence, not current instructions.

### Current schema snapshot

The PostgreSQL 18 schema snapshot remains useful after the reset, so it is not deleted. The generator is promoted from:

```text
scripts/refactor/capture-vnext-schema.sh
pnpm refactor:vnext:schema
```

into the permanent repository capability:

```text
scripts/schema/capture-current.sh
pnpm schema:snapshot
```

The default evidence path remains `docs/refactor/vnext/schema/database-schema.json` to preserve the provenance of the reset-established baseline. A fresh disposable PG18 capture produced the same SHA-256 before and after the move and again reported migration `1:false`, 48 tables and 672 columns.

### Contract-codegen spike harness

`experiments/contract-codegen/common` was a 2026-09-09 worktree-specific experiment harness with hard-coded candidate-worktree assumptions. The chosen architecture has already been implemented by Phases 01–04:

- public REST: canonical OpenAPI;
- public StreamEvent: canonical JSON Schema;
- private Go↔Python runtime: Proto/Protovalidate.

The checked-in experiment harness is deleted. The result report, ADRs and Git history retain the decision evidence; the harness is no longer a supported current tool.

## Dependency / generator hygiene

A repository dependency audit distinguished true direct consumers from configuration tools that generic `depcheck` cannot understand.

Removed direct development dependencies:

```text
@bufbuild/protobuf
@bufbuild/protovalidate
@nx/js
@nx/vitest
@nx/web
@nx/workspace
jiti
jsdom
nx-mcp
```

Reasons:

- the two Buf JavaScript packages had no generated/runtime consumer and were not used by the current Go/Python Proto generation path;
- Nx JS/workspace/vitest are already supplied transitively by the actually configured Nx plugins; `@nx/web` has no configured executor;
- Web tests use `happy-dom`, not `jsdom`;
- commitlint/Vite/ESLint/Orval carry the Jiti versions they require;
- `.mcp.json` launches exact `nx-mcp@0.25.0` through its own pinned `npx` command, so the root package copy was duplicate installation state.

Retained even when a generic dependency scanner reports them as unused:

- `@commitlint/cli` / `@commitlint/config-conventional` — Husky commit-msg + TS config;
- `@nx/eslint` / `@nx/vite` — configured Nx executor/plugin;
- `@redocly/cli` — contract lint command;
- root `vite` / `vitest` — Web config/tests execute them directly.

Validation after dependency removal:

```text
pnpm install --frozen-lockfile=PASS
pnpm nx show projects=PASS
commitlint TS config=PASS
Web typecheck=PASS
Web build=PASS
Web tests=52 files / 266 tests PASS
contract toolchain pin check=PASS (6 JS packages)
```

## Final implementation-path simplification

The documentation alignment audit still contained one real P3 hardening item: Diagnosis had a second PydanticAI OpenAI-compatible transport constructor even though it had no distinct transport semantics.

Phase 11 removes that duplicate. `apps/ai-service/src/ai/diagnosis_gateway_model.py` now owns only:

- Diagnosis logical model-group revision validation;
- Diagnosis generation settings;
- delegation to the shared `ai/gateway.py::get_gateway_model` constructor.

The AI service therefore has one PydanticAI OpenAI transport owner:

```text
apps/ai-service/src/ai/gateway.py
  -> OpenAIProvider
  -> OpenAIChatModel
```

The Python architecture checker now rejects any future business module that imports `pydantic_ai.models.openai` or `pydantic_ai.providers.openai` outside that owner. A real CLI negative fixture proves the gate exits non-zero for a role-specific transport reintroduction.

Focused Diagnosis validation:

```text
Diagnosis gateway/service/agent tests=17 PASS
Ruff=PASS
Pyright=PASS
```

## Documentation alignment

Current contract READMEs are rewritten to describe implemented authority rather than future phases:

- `packages/contracts/openapi/README.md` now identifies `bodysense.v1.openapi.yaml` as the canonical browser REST authority;
- `packages/contracts/schemas/README.md` now identifies `stream-event.v1.schema.json` as the canonical public stream authority and generated runtime trust boundary;
- the stale consultation comment that said MessagePart/StreamEvent unification was waiting for Phase 03 is removed;
- the contract-codegen result document records that the experiment harness was intentionally retired after implementation;
- the documentation/code alignment audit marks the duplicate Diagnosis gateway helper resolved.

The active-plan review intentionally leaves real unfinished work active. In particular:

- Health Document Challenger → Champion selection is still blocked on the private double-reviewed real-layout corpus;
- Assessment serving request/dependency hardening (B1) is still real: raw `images` / `rag_context` remain representable and fail closed at service validation;
- Assessment source-key semantics (B2) remains a domain decision;
- ASR mechanism identity (B3) remains incomplete;
- observability, final 3D visual acceptance, static-asset production acceptance and the explicitly parked durability plan keep their own owners/status.

No plan is archived merely to make the vNext reset look complete.

## Whole-system architecture audit

Current source inspection now shows:

```text
PydanticAI OpenAI provider constructors: one application owner (`ai/gateway.py`)
refactor-only package commands: none
scripts/refactor/: removed
active contract-codegen experiment harness: removed
canonical public REST authority: OpenAPI
canonical public stream authority: JSON Schema
canonical private runtime authority: Proto/Protovalidate
current schema generator: permanent `schema:snapshot`
```

Compatibility/legacy hits that remain in application tests or code were reviewed rather than mechanically removed. They include fail-closed regression fixtures, historical replay/configuration identities, the still-current health-document Tesseract Champion/rollback path, and backward-compatible Knowledge snapshot/source semantics. These have current behavior/evolution value and are not refactor scaffolding.

## Focused evidence so far

```text
schema snapshot deterministic SHA=PASS
pnpm quality:verify=PASS
architecture real-gate negative fixtures=4/4 PASS
Diagnosis focused tests=17 PASS
Diagnosis Ruff/Pyright=PASS
Web typecheck=PASS
Web build=PASS
Web tests=52 files / 266 tests PASS
pnpm install --frozen-lockfile=PASS
Nx project discovery=PASS
git diff --check=PASS
```

## Final whole-system acceptance

The first full `validate:local-deploy` attempt intentionally exposed one remaining validation-chain reference to the deleted Diagnosis-specific gateway constructor: `scripts/validate-litellm-gateway.sh` still imported `get_diagnosis_gateway_model`. Product tests, DR and Agent evals had already passed; the failure occurred in the PydanticAI gateway smoke itself. The smoke was migrated to the real current path — `get_diagnosis_runtime_model(default config)` backed by shared `get_gateway_model()` — and its three focused LiteLLM/PydanticAI smoke checks then passed.

The complete second fresh-stack run finished with exit code `0`:

```text
VALIDATION_HOST_CAPACITY=PASS
lint / staticcheck / typecheck / repository tests=PASS
AI service tests=507 PASS
Web tests=52 files / 266 tests PASS
ARCHITECTURE_BOUNDARY_POLICY=PASS
ARCHITECTURE_POLICY_MUTATION_TESTS=PASS
contracts:verify=PASS
Postman verification=95/95 PASS
MIGRATION_SEQUENCE=PASS latest=1
MIGRATION_IMMUTABILITY=PASS
offhost DR unit tests=88/88 PASS
OFFHOST_DR_INTEGRATION=PASS
assessment / diagnosis / treatment eval lanes=PASS
LITELLM_GATEWAY_SMOKE=PASS
PYDANTICAI_LITELLM_ADAPTER_SMOKE=PASS
AI_SERVICE_LITELLM_GATEWAY_SMOKE=PASS
build=PASS
REPO_QUALITY=PASS
API_HEALTH=PASS
AI_HEALTH=PASS
WEB_HEALTH=PASS
POSTURE_GEOMETRY_MECHANISM=PASS
fresh PG18 FULL_UP / LATEST_DOWN / LATEST_REPLAY_UP=PASS
BODY_STATE_SEMANTICS=PASS
BODY_REGION_ID_ROUNDTRIP=PASS
TREATMENT_ACTIVATION_ATOMICITY=PASS
OUTCOME_FEEDBACK_ATOMICITY=PASS
DOMAIN_SEMANTICS=PASS
KNOWLEDGE_PUBLICATION_VERTICAL=PASS
KNOWLEDGE_ROLLBACK_VERTICAL=PASS
Playwright=10/10 PASS (8.8m)
DIAGNOSIS_BASELINE_VALIDATION=PASS current=3 non_current=0 rollout_observations=0
TREATMENT_BASELINE_VALIDATION=PASS current=3 non_current=0 rollout_observations=0
TREATMENT_DECISION_TRACE_VALIDATION=PASS accepted_traces=2
TREATMENT_REPLAY_INPUT_VALIDATION=PASS replay_inputs=3
LOCAL_DEPLOY_VALIDATION=PASS
```

This closes the vNext engineering reset implementation: the current repository has one intended contract/transport owner per boundary, the temporary reset machinery has been removed or promoted to permanent tooling, current documentation no longer describes completed contract migration phases as future work, and the full longitudinal product loop has been re-proven from a fresh PostgreSQL 18 database and production-shaped runtime.
