# BodySense vNext Engineering Reset — Master Refactor Plan

- Status: ACTIVE MASTER PLAN — Phases 00–02 complete; Phase 03 in progress
- Date: 2026-09-13
- Integration branch: `refactor/bodysense-vnext`
- Integration worktree: `/home/dev/projects/bodysense-vnext-refactor`
- Base: `3eaab969ca4cce15ca0ad1e945d11f3581e673cf`
- Owner constraint: no concurrent product/feature work during this refactor; no business data requires migration
- Governing ADRs: 0002, 0003, 0004, 0005, 0008, 0009, 0014, 0015

## 0. Executive decision

BodySense is entering a deliberate **pre-user engineering reset**, not an incremental compatibility migration.

The repository has already proven the main product architecture: Go owns durable business truth, Python owns Agent runtime execution, and Web consumes public projections. The remaining problem is that several implementation generations still coexist inside that architecture: handwritten cross-language contracts, runtime compatibility aliases, legacy schema columns, stringly state machines, shallow runtime validation, historical serving configuration branches, and tests that sometimes bypass the same trust boundaries production relies on.

This refactor therefore optimizes for the final architecture, not for preservation of migration-era implementation details.

The target is:

```text
one semantic owner per boundary
+ one authoritative contract per wire boundary
+ explicit runtime trust boundaries
+ typed/exhaustive state machines
+ atomic durable mutations
+ generated transport types kept outside domain logic
+ no legacy serving aliases / dual read / dual write / compatibility projections
+ one clean database baseline
+ one coherent CI quality contract
```

The final system must remain behaviorally complete and safe, but it does not need to remain wire/schema compatible with the pre-vNext development system.

## 1. Evidence base

This master plan consolidates, rather than replaces, the strongest prior work:

- `docs/architecture/current-longitudinal-system.md`
- `docs/adr/0002-agent-runtime-ownership.md`
- `docs/adr/0003-stream-event-versioning.md`
- `docs/adr/0004-adopt-longitudinal-body-state-model.md`
- `docs/adr/0005-adopt-standalone-litellm-model-gateway.md`
- `docs/adr/0008-adopt-delivery-platform-v3.md`
- `docs/adr/0009-adopt-evidence-grounded-assessment-contract.md`
- `docs/adr/0014-adopt-boundary-specific-contract-codegen-strategy.md`
- `docs/architecture/contract-codegen-spike-results-2026-09-09.md`
- `docs/architecture/contract-codegen-production-north-star-2026-09-09.md`
- `docs/plan/contract-codegen-architecture-spike-plan-2026-09-09.md`
- `docs/plan/stream-event-exhaustiveness-hardening-plan-2026-09-09.md`
- `docs/plan/postman-repo-native-api-workspace-plan-2026-09-10.md`
- `docs/plan/active/2026-09-01-documentation-code-alignment-audit.md`
- `docs/plan/archive/consultation-workbench-code-quality-fixes.md`

The 2026-09-09 contract spike already produced a measured decision, so this refactor does not reopen a generic "OpenAPI vs Protobuf" debate:

| Boundary                    | Canonical authority         | Final direction                                                                                                            |
| --------------------------- | --------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| Browser-facing REST         | OpenAPI 3.1 spec-first      | Go strict generated boundary + request validation; Web Orval validated Fetch + Zod Mini + handwritten query/error adapters |
| Public stream               | JSON Schema 2020-12         | generated TS discriminated union + generated runtime validator; SSE/JSON stays public wire                                 |
| Go <-> Python Agent runtime | Proto + Buf + Protovalidate | generated Go/Python internal boundary types; transport stays HTTP/NDJSON initially                                         |
| Domain models               | handwritten                 | never generated from transport schema                                                                                      |
| Postman/API workspace       | derived                     | generated from canonical OpenAPI, never a second hand-maintained contract                                                  |

GraphQL is not adopted in this refactor. Its schema/discovery/selection ideas remain useful learning material, but adding a second public API paradigm does not solve BodySense's current contract-drift problem and would increase the number of authorities.

gRPC/Connect transport is also not adopted globally. The spike proved Proto is useful as IDL while the measured fine-grained Python gRPC server stream was much slower than current NDJSON. Transport remains an independent future decision.

## 2. Non-negotiable architecture invariants

### 2.1 Ownership

```text
Web
  owns UI/presentation/local interaction state only

Go
  owns authentication, authorization, durable business state, public API authority,
  run/job/event persistence, domain transitions and public projections

Python
  owns LangGraph/PydanticAI runtime execution, model/tool orchestration,
  RAG/perception computation and internal runtime state
```

No refactor branch may move durable health/business authority into Python or Web for convenience.

### 2.2 Runtime truth

- LangGraph checkpoint state is runtime truth for an Agent thread, not the durable health-domain database.
- `BodyState` is durable longitudinal health truth.
- Diagnosis/Treatment/Assessment artifacts remain explicitly provenance-bound.
- HTTP/SSE disconnect remains different from business cancellation.
- Web never invents resume semantics.

### 2.3 Generated code

Generated code owns serialization and trust-boundary representation only.

Required dependency direction:

```text
canonical schema/IDL
  -> generated transport model/client/server shape
  -> handwritten adapter/presenter
  -> handwritten application/domain logic
```

Forbidden:

```text
generated DTO -> domain aggregate -> domain semantics depend on generator layout
```

### 2.4 Failure semantics

Illegal/unknown/malformed input must remain illegal/unknown/malformed.

Forbidden normalization examples:

```text
malformed tool JSON -> {}
unknown state -> default happy path
DB/event append failure -> pretend durable transition fully succeeded
unknown public event -> silently ignored by generic default
```

### 2.5 No compatibility-by-default

Because this reset has no business-data migration requirement and no external-user compatibility requirement:

- no dual read;
- no dual write;
- no old/new request aliases;
- no deprecated field projection merely for old clients;
- no old state-name aliases;
- no legacy token fallback;
- no migration-era DB columns kept for old rows;
- no serving-path historical Agent configs;
- no temporary parser kept after the implementation branch proves the replacement locally/staging before integration.

Git history and archived fixtures are the historical record. Runtime code is not the archive.

Operational safety is **not** compatibility debt: backup, rollback of an unreleased integration build, immutable release artifacts, fail-closed deployment and recovery remain mandatory.

## 3. Current root causes to eliminate

### 3.1 Contract duplication

Current facts are spread across handwritten Go DTOs, TS interfaces, Python models, JSON Schema and runtime validators.

Target: one authority per semantic wire boundary, generated representations behind stable facades.

### 3.2 Runtime trust erosion after validation

Representative pattern:

```text
parse/validate JSON
  -> static union already knows variant
  -> payload becomes unknown again
  -> consumer casts it back with `as`
```

Target: validate once at the boundary; downstream application code consumes trusted types without re-casting.

### 3.3 Partial state machines

Representative examples include raw string Job statuses, `completed` vs `succeeded`, consultation phase aliases, optional fields encoding impossible states, and non-exhaustive reducers.

Target: typed finite state, explicit transitions, exhaustive consumers, no duplicate terminal semantics.

### 3.4 Non-atomic durable mutations

Representative Job flow:

```text
read status
-> validate transition
-> update status
-> best-effort append lifecycle event
```

Target: invariant-carrying repository command with compare-and-set and authoritative lifecycle event in the same database transaction.

### 3.5 Invalid output normalized into valid-looking data

Representative provider/tool path can turn malformed tool-call JSON into `{}`.

Target: explicit invalid variant/error; never execute the tool as though `{}` were a legitimate model decision.

### 3.6 Helper mutation / aliasing

Factories and validators may mutate caller-owned values while appearing to validate/build.

Target: value semantics by default; normalization returns a new value explicitly.

### 3.7 Migration-era runtime compatibility

Examples currently visible in the codebase include:

- legacy Assessment report union/types;
- `analysis_ready` vs `ready_for_analysis`;
- Job `succeeded` compatibility;
- legacy authentication tokens without session identity;
- upload `file_path` compatibility projection / local compatibility backend;
- nullable/migration-era BodyRegion compatibility handling;
- `consultation_sessions.health_features` and `thread_projections.health_features` lineage;
- historical Agent configuration serving/rollback accessors mixed with current deployment policy;
- health-document legacy mechanism/current-projection compatibility;
- historical evidence-policy aliases used by serving code.

Each must be classified as either:

1. **runtime compatibility to delete**;
2. **offline historical/eval evidence to move out of serving paths**;
3. **current real capability to keep and rename canonically**.

No fourth category of "keep just in case" is allowed.

### 3.8 Tests that bypass production contracts

Examples include helper builders returning `as StreamEvent` or `as never`, and source-string assertions used where behavior/type/AST checks are more appropriate.

Target test preference:

```text
behavior test
-> runtime contract test
-> type-level compile test
-> architecture/dependency test
-> AST/static rule
-> source-string assertion only as last-resort migration guard
```

## 4. North-star system shape

```text
                         PUBLIC BOUNDARIES

OpenAPI 3.1                                  JSON Schema 2020-12
(public REST authority)                     (public StreamEvent authority)
      |                                              |
      v                                              v
Go generated HTTP boundary                  generated TS event union/validator
+ OpenAPI request validator                          |
      |                                              v
handwritten Go adapter/presenter               Web exhaustive reducer
      |
      v
Go application/domain services
      |
      | internal Agent command/runtime boundary
      v
Proto + Buf + Protovalidate
      |
      +-------------------+
      |                   |
      v                   v
generated Go types    generated Python types
      |                   |
handwritten adapter   handwritten adapter
      |                   |
      +---- HTTP/NDJSON ---+
                          |
                          v
                 LangGraph / PydanticAI
```

The internal NDJSON stream should carry a dedicated typed **InternalRuntimeEvent**, not reuse the public `StreamEvent` concept. The two protocols are deliberately different:

```text
Python InternalRuntimeEvent
  -> Go validates internal protocol
  -> Go applies/persists business/event effects
  -> Go maps selected facts to PublicStreamEvent
  -> Web validates PublicStreamEvent
```

## 5. Repository target layout

```text
packages/contracts/
  openapi/
    bodysense.v1.openapi.yaml
  schemas/
    stream-event.v1.schema.json
  fixtures/
    rest/
    stream/
  src/
    generated/
      stream-events.ts
      stream-event-validator.ts
    index.ts

contracts/
  internal/
    agent-runtime/
      v1/
        runtime.proto
        buf.yaml
        buf.gen.yaml

apps/api/internal/generated/
  openapi/
  runtimeproto/

apps/api/internal/transport/
  httpapi/          # generated HTTP boundary adapters/presenters only
  agentruntime/     # generated proto <-> app adapters

apps/web/src/generated/api/
apps/web/src/features/*/api/   # handwritten query/error/view adapters

apps/ai-service/src/generated/runtimeproto/
apps/ai-service/src/runtime/adapters/
```

Generated paths must be excluded from default Agent/search context and code-review focus.

## 6. Branch and worktree governance

### 6.1 Integration branch

`refactor/bodysense-vnext` is the only integration target during the reset.

Rules:

- based on `3eaab969c` because it already contains the completed contract spike + north-star design;
- no product feature work is merged into this branch;
- direct code commits are forbidden except master-plan/ledger/merge-resolution updates;
- every implementation branch starts from the latest green integration commit;
- every implementation branch returns to this branch only after focused + repository validation;
- `main` remains untouched until the vNext branch reaches final acceptance;
- the dirty `docs/bodysense-open-curriculum` worktree remains isolated and is not used as a refactor base.

### 6.2 Child branch rule

Do **not** pre-create all child branches. A child branch is created only when its predecessor has merged, unless the two lanes have proven-disjoint file ownership.

Default naming:

```text
refactor/vnext-00-baseline
refactor/vnext-01-contract-foundation
refactor/vnext-02-rest-openapi
refactor/vnext-03-public-stream
refactor/vnext-04-internal-runtime-proto
refactor/vnext-05-runtime-trust
refactor/vnext-06-durable-state-machines
refactor/vnext-07-legacy-retirement
refactor/vnext-08-schema-rebaseline
refactor/vnext-09-api-tooling
refactor/vnext-10-quality-gates
refactor/vnext-11-final-simplification
```

Optional focused repair branches after a batch review:

```text
repair/vnext-<phase>-<root-cause>
```

These branch from the integration commit that contains the reviewed batch and merge back before the next phase begins.

### 6.3 Merge rule

For each child branch:

```text
latest refactor/bodysense-vnext
  -> child worktree
  -> characterization/failing tests
  -> implementation
  -> focused validation
  -> full affected validation
  -> batch review
  -> repair if needed
  -> no-ff merge into refactor/bodysense-vnext
  -> delete child branch/worktree
```

This gives each phase an auditable diff and prevents a single giant unreviewable branch.

## 7. Phase plan

### Execution status

| Phase                    | Status          | Integration evidence                                                                                                               |
| ------------------------ | --------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| 00 — Baseline            | **COMPLETE**    | `a2c68a38d` merged baseline evidence, fresh PG18+pgvector schema snapshot, retirement/finding ledgers and self-contained typecheck |
| 01 — Contract foundation | **COMPLETE**    | `100018dd3` merged contract authority foundation; see `docs/refactor/vnext/phase-01-contract-foundation.md`                        |
| 02 — Public REST         | **COMPLETE**    | `ea343d7c5` merged 95/95 eligible browser routes OpenAPI-authoritative; see `docs/refactor/vnext/phase-02-rest-openapi.md`         |
| 03 — Public StreamEvent  | **IN PROGRESS** | STREAM-001 schema/parser parity complete on `refactor/vnext-03-public-stream`                                                      |
| 04–11                    | QUEUED          | blocked by preceding dependency phases                                                                                             |

## Phase 00 — Freeze baseline and create the refactor ledger

Branch: `refactor/vnext-00-baseline`

### Goal

Create an immutable factual baseline before deleting compatibility code.

### Work

- inventory public routes, internal Python routes, public stream variants, internal runtime events, DB tables/columns, Agent configurations and environment variables;
- create a machine-readable retirement ledger with `KEEP / REPLACE / DELETE / ARCHIVE-OFFLINE` classification;
- capture current behavior/E2E results;
- record current generated/handwritten contract definition count;
- record `any` / `Any` / unsafe cast / raw-status hotspots by category, not as a vanity total;
- freeze exact root verification commands.

### Acceptance

- no production behavior changed;
- every compatibility marker targeted later has an owner/category;
- baseline tests are green or existing debt is explicitly recorded.

## Phase 01 — Contract authority foundation

**Status: COMPLETE — merged at `100018dd3`; evidence: `docs/refactor/vnext/phase-01-contract-foundation.md`.**

Branch: `refactor/vnext-01-contract-foundation`

### Goal

Turn ADR 0014 from design evidence into repository infrastructure.

### Work

- pin Redocly, oasdiff, Orval, Zod Mini, oapi-codegen, Buf, protoc plugins and Protovalidate;
- establish canonical contract directories;
- establish deterministic generated directories;
- add `contracts:lint`, `contracts:generate`, `contracts:check-generated`, `contracts:breaking`, `contracts:mutation`, `contracts:conformance`, `contracts:verify`;
- add generated-file headers and Agent/search exclusions;
- make generated code read-only by policy/check;
- remove spike-only scaffolding from production reasoning paths while retaining experiment evidence under docs/architecture.

### Acceptance

- deterministic regenerate produces zero diff;
- generator versions are pinned;
- no production code imports spike experiment modules;
- generated artifacts are excluded from default Agent context.

## Phase 02 — Public REST reset to OpenAPI 3.1 spec-first

**Status: COMPLETE — 95/95 eligible browser-facing routes; Phase 03 not started.**

Branch: `refactor/vnext-02-rest-openapi`

### Goal

Replace handwritten browser/API transport duplication with one public HTTP authority.

### Work

1. model all currently supported public `/api/v1` routes in canonical OpenAPI 3.1;
2. define shared error envelope, authentication requirements and revision-conflict semantics;
3. generate Go strict server/request/response boundary;
4. enforce OpenAPI request validation before application handlers;
5. introduce handwritten transport-to-domain adapters and domain-to-response presenters;
6. generate Web Fetch/Zod client through Orval;
7. retain handwritten TanStack Query adapters and normalized `ApiRequestError` behavior;
8. migrate routes feature-by-feature, starting with HealthWorkspace + BodyState command and then completing the public surface;
9. delete equivalent handwritten Web DTO mirrors and generic `request<T>` trust assumptions after migration;
10. remove route-level compatibility aliases instead of dual-serving them.

### Required behavior

- malformed input never reaches services;
- non-2xx is never parsed as success;
- known response fields are runtime validated;
- because this is a coordinated pre-user cutover, no old-client compatibility shim is carried in production code;
- domain services never import generated transport types.

### Acceptance

- OpenAPI describes 100% of supported public API routes;
- Web public HTTP calls use generated transport boundary or an explicitly documented non-REST special case;
- no byte-for-byte handwritten public transport DTO mirror remains;
- auth, 400/401/403/404/409/422/500 contracts have executable tests;
- browser E2E behavior remains functionally equivalent.

## Phase 03 — Public StreamEvent reset to JSON Schema-first

Branch: `refactor/vnext-03-public-stream`

### Goal

Make the public event schema the only static/runtime authority and make every consumer exhaustive.

### Work

- port the five spike-proven schema/parser semantic fixes;
- strengthen typed payload schemas for extracted info, citations, red flags, interactions and other currently shallow `unknown` payloads where public semantics are known;
- generate TS discriminated union from schema;
- generate/compile runtime validator;
- expose through stable `@bodysense/contracts` facade;
- rewrite `activeTurnReducer` to rely on narrowing rather than payload casts;
- explicitly list intentional no-op variants;
- add `assertNever`/`satisfies never` exhaustiveness gate;
- replace cast-based test event builders with type-directed builders + parser-backed fixtures;
- delete handwritten parser/static duplicate after the branch's full parity suite passes;
- keep public SSE/JSON wire unchanged unless an explicit event-version decision is made.

### Acceptance

- adding a known event variant without reducer treatment fails typecheck;
- malformed payloads fail at parser/validator boundary;
- reducer contains no `as unknown as` payload recovery;
- live SSE and durable replay use the same validator;
- public schema/types/parser facts have one authority.

## Phase 04 — Internal Go/Python runtime reset to Proto IDL

Branch: `refactor/vnext-04-internal-runtime-proto`

### Goal

Separate internal Agent protocol from public StreamEvent and remove handwritten Go/Python duplication.

### Work

- define `runtime.proto` with typed start-turn, resume-interrupt, configuration handshake and internal runtime event messages;
- model internal events as a typed `oneof`, not `type: string + dict payload`;
- apply Protovalidate to required IDs, versions, sequence and field-level constraints;
- generate Go/Python boundary representations;
- create handwritten adapters into Go application structures and Python LangGraph/runtime structures;
- carry proto JSON mapping over the existing HTTP/NDJSON transport first;
- delete `runtime` from the public event vocabulary entirely;
- delete hand-maintained internal event-type/channel maps once Proto validation/mapping owns them;
- preserve Go as the mapper from internal runtime facts to public StreamEvent.

### Explicit non-goal

Do not replace fine-grained NDJSON streaming with gRPC/Connect during this phase.

### Acceptance

- no shared generic `StreamEvent` type crosses both internal and public boundaries;
- invalid internal messages fail before business/runtime execution;
- HITL resume preserves exact `thread_id` + immutable Agent configuration semantics;
- generated proto classes do not leak into LangGraph state or Go domain services;
- current disconnect/cancel semantics remain unchanged.

## Phase 05 — Runtime trust and explicit error semantics

Branch: `refactor/vnext-05-runtime-trust`

### Goal

Eliminate data that looks valid only because code silently repaired invalid external input.

### TypeScript

- remove unjustified `as`, `as unknown as`, `as never` at trust-consuming application layers;
- preserve `unknown` only at actual boundaries;
- parse before cache/state/domain usage;
- replace impossible-state optional-field bags with discriminated unions where appropriate.

### Python

- replace `AiStreamEvent(type: str + optional fields)` with typed event variants;
- malformed tool JSON becomes explicit invalid-tool-call/protocol failure, never `{}`;
- ToolExecutor validation returns normalized values rather than mutating caller dictionaries;
- factories use value semantics and do not mutate caller-owned Pydantic models;
- reduce broad `Any` propagation after Pydantic/Proto validation;
- replace `except Exception` fallbacks that conflate infrastructure failure with valid domain output where discovered.

### Go

- public/internal transport parsing returns typed structured errors;
- error codes become finite constants/types where callers depend on them;
- no `json.RawMessage` survives deeper than the boundary unless the field is intentionally opaque JSON domain data.

### Acceptance

For every external boundary:

```text
untrusted input -> one explicit parser/validator -> trusted typed value
```

No downstream code must cast the same wire shape back into existence.

## Phase 06 — Typed and atomic durable state machines

Branch: `refactor/vnext-06-durable-state-machines`

### Goal

Move invariants into the mutation boundary and collapse duplicate state vocabularies.

### Work

- introduce typed `JobStatus` and typed transition map;
- remove Job `succeeded`; keep only canonical `completed`;
- collapse consultation phase to one canonical vocabulary (`collecting`, `ready_for_analysis` unless a new ADR changes it);
- audit Run, ToolCall, Interaction, Upload/OCR, Treatment and Assessment lifecycle strings for duplicate aliases;
- replace read-check-write state changes with conditional/compare-and-set repository commands;
- write authoritative lifecycle event and state mutation in the same transaction;
- distinguish authoritative domain/audit events from best-effort telemetry;
- use optimistic concurrency/version checks consistently where state can race;
- make illegal transitions typed/testable rather than convention-based strings.

### Acceptance

- two concurrent terminal transitions cannot both win;
- a committed authoritative state transition cannot lose its corresponding durable lifecycle event;
- canonical lifecycle vocabularies have exactly one spelling/semantic meaning;
- database constraints reinforce important finite-state invariants where practical.

## Phase 07 — Remove all runtime compatibility and migration-era branches

Branch: `refactor/vnext-07-legacy-retirement`

### Goal

Delete migration scaffolding now that every target boundary is stable.

### Required deletion audit

At minimum inspect and remove/relocate:

- `LegacyAssessment*` public/runtime types and output-v1 serving branches;
- old Assessment/Diagnosis/Treatment configuration serving aliases;
- rollback-as-serving config pointers that exist only for pre-user migration history;
- legacy evidence-policy/tool-policy conditionals in serving code;
- `analysis_ready` phase alias;
- legacy auth token path without `session_id`;
- upload `file_path` compatibility response and local compatibility storage backend if no longer canonical;
- BodyRegion legacy null/free-text compatibility behavior where current canonical ID is now required;
- old `health_features` authority/projection semantics;
- health-document legacy Tesseract compatibility wrapper/current JSONB projection once the accepted current pipeline can be the single path;
- deprecated environment variables and config aliases;
- old route aliases and old request fields;
- migration-only validators whose only purpose was proving an already-retired format upgrade.

### Historical evidence policy

A historical artifact may stay only when it is useful for offline evaluation/research and cannot be selected by production serving code.

Preferred location/shape:

```text
docs/archive / test fixtures / eval corpus
```

not:

```text
production switch/case / deployment policy / public union / DB compatibility column
```

### Safety exception

Do not force an unqualified health-document model/mechanism into Champion solely to delete legacy code. If mechanism selection evidence is incomplete, keep one current canonical mechanism and simplify around it; algorithm promotion remains evidence-gated.

### Acceptance

- repository runtime grep for `legacy`, `compatibility`, `deprecated` is explainable line-by-line;
- no production code branch exists only to support pre-vNext clients/data;
- current behavior has one code path per capability.

## Phase 08 — Database schema rebaseline

Branch: `refactor/vnext-08-schema-rebaseline`

### Goal

Replace migration history as runtime baggage with one clean vNext schema baseline.

### Preconditions

- Phase 07 has removed old fields/paths from application code;
- no business data needs preservation;
- final domain schema is known.

### Work

- generate a clean canonical schema from the final domain model;
- create new `000001_vnext_baseline` migration (and only subsequent genuinely new vNext migrations);
- remove old migration files/checksum chain from the active migration directory; Git history preserves them;
- remove production-baseline upgrade tests that exist only to preserve pre-vNext schema evolution;
- retain migration framework tests: empty DB up, down/up where meaningful, checksum/no-edit policy for new vNext migrations;
- explicitly reset Dev/Staging/Production databases during final cutover rather than simulate a fake migration of unneeded data;
- update backup/restore scripts to the new baseline.

### Acceptance

- empty PostgreSQL 18 database reaches the complete current schema from one baseline chain;
- no migration-era dead columns/tables exist;
- all application queries/models compile and integration tests pass;
- final schema is documented from current semantics rather than historical accidents.

## Phase 09 — Repo-native API tooling derived from canonical contracts

Branch: `refactor/vnext-09-api-tooling`

### Goal

Make API exploration/testing a generated view of contract authority rather than a parallel specification.

### Work

- generate/sync Postman workspace/collection from canonical OpenAPI;
- retain handwritten scenario suites only for authentication, concurrency, negative behavior and multi-step workflows;
- mark internal Agent runtime API separately from public API;
- ensure no secrets are committed;
- update README/developer docs to point to canonical OpenAPI and generated workspace;
- remove any source-scraping API inventory that became unnecessary after OpenAPI completeness.

### Acceptance

- route parity is machine checked against OpenAPI;
- Postman is reproducible from repository state;
- no request/response contract is manually specified twice.

## Phase 10 — Quality gates and architecture enforcement

Branch: `refactor/vnext-10-quality-gates`

### Goal

Make the new architecture hard to accidentally undo.

### Work

- contract lint/generate/breaking/mutation/conformance gates;
- architecture dependency tests preventing generated DTO -> domain leakage;
- lint/static rules for trust-boundary casts and unsafe `any`/`Any` propagation where enforceable;
- exhaustive switch/state tests;
- AST/static checks where behavior/type checks cannot enforce architecture;
- remove migration-era source-string tests where a stronger layer now exists;
- standardize Go vet/staticcheck, Python Ruff/Pyright and TS ESLint/typecheck quality baselines;
- ensure generated code is excluded from lint rules that should apply only to handwritten source, while generated code still compiles;
- keep full repository validation hermetic.

### Acceptance

A representative architecture violation intentionally introduced in a test fixture must fail CI for each protected boundary.

## Phase 11 — Final simplification and whole-system review

Branch: `refactor/vnext-11-final-simplification`

### Goal

Delete the scaffolding introduced only to perform the refactor and prove the final architecture as a whole.

### Review layers

1. contract correctness;
2. end-to-end vertical completeness;
3. create/reload/retry/cancel/interrupt/resume/error behavior;
4. concurrency and durable state correctness;
5. type/runtime trust;
6. domain/generated ownership boundaries;
7. security/privacy/auth boundaries;
8. observability and operator failure visibility;
9. documentation/current-code alignment;
10. diff/dependency/generated-artifact hygiene.

### Final deletions

- temporary comparison adapters;
- temporary migration scripts;
- refactor-only feature flags;
- obsolete docs that still describe the old runtime as current;
- unused packages/dependencies/generators;
- compatibility fixtures not serving future contract evolution tests.

### Acceptance

- `git grep` and dependency graph show one intended implementation path per boundary;
- full local deploy validation passes on a fresh DB;
- current architecture docs describe only the implemented vNext design;
- no active plan falsely presents completed migration work as current debt.

## 8. Verification contract

Every child branch runs increasing-scope validation.

### Focused

- directly affected package/module tests;
- contract mutation/parity test for any boundary change;
- concurrency/error-path tests for state-machine changes.

### Repository

Canonical target remains:

```text
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm contracts:verify       # introduced by Phase 01
pnpm validate:local-deploy
```

Language-specific claims must also be proven by the actual relevant commands, for example:

```text
Go:     go test ./... && go vet ./...
Python: pytest + ruff + pyright
Web:    vitest + typecheck + production build
DB:     fresh PG18 migration/schema smoke
E2E:    longitudinal primary flow + cancellation/HITL/error paths
```

No branch may claim repository-wide green from a focused test only.

## 9. Refactor finding/closure ledger

Every finding uses:

```text
OPEN
IN_PROGRESS
FIXED
DELETED_WITH_LEGACY
ARCHIVED_OFFLINE
NOT_REPRODUCED
DEFERRED_WITH_REASON
NEW_REGRESSION
```

A finding is closed only by evidence, not by a green build alone.

Each batch review records:

- finding ID;
- evidence/path;
- root cause;
- affected invariant;
- repair commit;
- focused validation;
- repository validation;
- final disposition.

## 10. Scope boundaries

### In scope

- transport/API contract authority;
- internal Agent runtime contracts;
- Web/runtime type trust;
- Python provider/tool/runtime typing;
- Go state machines/transactions/repositories;
- compatibility-code retirement;
- DB schema rebaseline;
- API docs/tooling generated from canonical contracts;
- CI architecture enforcement;
- documentation alignment required by the new architecture.

### Out of scope unless required to preserve behavior

- new user-facing product features;
- GraphQL adoption;
- global gRPC/Connect migration;
- changing the core Go/Python/Web ownership model;
- changing Diagnosis/Treatment clinical decision semantics merely for code style;
- promoting an unqualified OCR/Posture/Agent mechanism to eliminate old naming;
- visual redesign unrelated to changed contract plumbing.

## 11. Final Definition of Done

The reset is complete only when all statements are true:

1. Public REST has one OpenAPI 3.1 authority and no duplicate handwritten transport model mirrors.
2. Public StreamEvent has one JSON Schema authority; generated static/runtime contract is exhaustive downstream.
3. Internal Go/Python Agent protocol has one Proto/Buf authority and is distinct from public StreamEvent.
4. Generated types are boundary-only and cannot leak into domain modules without CI failure.
5. Network/provider data is `unknown/untrusted` until runtime validation and trusted afterward without repeated casting.
6. Malformed model tool arguments can never become a legitimate empty tool call.
7. Caller-owned Python values are not silently mutated by factories/validators.
8. Durable lifecycle states are typed, canonical and concurrency-safe.
9. Authoritative state transition + audit event are atomic where both represent one business fact.
10. No production runtime compatibility path exists solely for pre-vNext clients, schemas, tokens, state names or Agent serving configs.
11. Active database migration history starts from a clean vNext baseline; old migration lineage survives only in Git history/archive evidence.
12. Postman/API documentation derives from canonical contract sources.
13. CI proves contract generation, breaking rules, semantic mutation rules, typecheck, tests, build and production-shaped local deployment.
14. A fresh checkout + fresh PostgreSQL 18 database can build, test and run the entire system without historical migration knowledge.
15. The final architecture is materially simpler to explain: every state owner, wire authority and validation boundary has one answer.
