# BodySense Contract Codegen Architecture Spike Results

> Date: 2026-09-09
> Base commit: `19957109cc9cf6692f2beec8c6afd249011e0bdb`
> Scope: isolated experiment only; no production transport or application behavior changed.

## Executive decision

The spike does **not** support one codegen technology for every BodySense protocol boundary.

Recommended architecture by boundary:

| Boundary | Recommendation | Why |
|---|---|---|
| Browser-facing REST | **OpenAPI 3.1 spec-first + O1 Orval validated Fetch/Zod Mini** | Best fail-closed runtime trust in the measured frontend candidates; small Mini validator bundle; shared Go/Python codegen works with explicit request-validation boundaries. |
| Browser REST runner-up | **Hey API** | Better generated SDK/TanStack DX and faster validator, but generated SDK does not parse responses and current Zod output strips unknown keys instead of honoring closed schemas as rejection. |
| Public `StreamEvent` | **JSON Schema 2020-12 first (J1), after semantic cleanup** | Existing JSON wire is natural, one-source TS/runtime generation works, M9 schema-only evolution passed, and no transport rewrite is needed. |
| Go ↔ Python internal runtime | **Proto/Buf + Protovalidate as IDL (P1-A)** | Largest reduction in handwritten cross-language surface and strongest generated multi-language model, presence/evolution and breaking governance. |
| Internal transport | **Keep transport decision separate; do not 1:1 replace NDJSON with Python gRPC stream** | Protobuf framing was smaller, but the measured fine-grained synchronous Python gRPC server stream was much slower. |
| Selective browser/internal unary RPC | **Connect is viable, not mandatory** | Real Connect-Web -> generated Connect-Go unary round-trip passed, but browser bundle is substantially heavier than REST/Zod. |

The architecture should therefore be **hybrid by protocol boundary**, not technology-uniform.

## Isolation / reproducibility

All candidate worktrees forked from the same common baseline commit:

- `bodysense-contract-codegen-common` — shared OpenAPI, fixtures, mutation matrix and baselines;
- `bodysense-contract-codegen-orval` — O1;
- `bodysense-contract-codegen-heyapi` — O2;
- `bodysense-contract-codegen-openapi-shared` — shared OpenAPI Go/Python and Go-first control;
- `bodysense-contract-codegen-jsonschema` — J1;
- `bodysense-contract-codegen-proto` — P1.

Candidate worktrees changed only `experiments/contract-codegen/...`; the canonical BodySense production worktree was not modified. Phase 11 later removed the checked-in benchmark harness from the active tree after Phases 01–04 implemented the selected OpenAPI / JSON Schema / Proto architecture. This result document, the governing ADRs, and Git history retain the decision evidence; the obsolete worktree-specific harness is no longer a supported current tool.

## Baseline

The existing system is already disciplined, but the same wire facts are spread across multiple handwritten surfaces.

Representative `StreamEvent` contract surface:

- TypeScript static contract: 334 LOC;
- handwritten TypeScript runtime parser: 194 LOC;
- JSON Schema: 1,037 LOC;
- Go DTO: 50 LOC;
- Python Pydantic model: 93 LOC;
- fixtures/parity tests on top.

Representative REST surface also duplicates `HealthWorkspace` across Go DTO, TS view types and a generic client where `request<T>` is compile-time only.

Baseline checks in the common worktree:

- Web typecheck: PASS (~13.07 s);
- Web production build: PASS (~6.71 s);
- Go tests: PASS;
- Python AI tests: **475 passed** after installing the repository's OCR/document extras correctly.

The earlier Python collection failure was an experiment-environment setup mistake (missing optional extras), not a BodySense regression.

## REST: shared OpenAPI architecture

### Source of truth

OpenAPI 3.1 spec-first was stronger than the Go-first control for this boundary.

`oapi-codegen` v2.8.0 generated Go models, client, Gin server and strict-server shape. `datamodel-code-generator` generated Pydantic v2 models with UUID/date/enum/constraint handling.

### Go runtime trust boundary

A critical result is that generated Go models / plain Gin wrapper are **not** sufficient runtime validation by themselves.

Without OpenAPI request validation, invalid bodies such as missing required fields, `minLength` violations, negative revisions and closed-schema extra fields reached the handler. With strict-server plus the OpenAPI Gin request validator, those inputs were blocked before application logic.

Required production architecture if adopted:

`OpenAPI -> generated strict server/models -> OpenAPI request validator -> handwritten application service`

### Go-first control

Swaggo + current BodySense-style Gin tags inferred some useful constraints (`required`, `gte=0`) and documented 200/400/409 when handler comments declared them. But it generated Swagger 2.0, did not infer the same `minLength` semantics from ordinary `binding:"required"`, and still required response annotations. OpenAPI 3.1 closed/nullable/union semantics are not naturally all encoded by plain Go structs/tags.

Therefore Go-first was not selected as the REST canonical source.

## REST frontend: O1 Orval vs O2 Hey API

### O1 Orval

Strengths:

- real Zod response parsing in Fetch runtime path;
- strict generated object behavior;
- Zod Mini validator is very small;
- `forceSuccessResponse` path aligns with BodySense's existing non-2xx fail-fast semantics.

Measured Mini validator bundle:

- 27,352 B raw;
- 8,582 B gzip;
- 7,740 B Brotli.

Important limitations found with Orval 8.30.0:

- ordinary Zod/error-response alias output could generate missing type exports;
- legitimate 409 could be parsed with the success response schema unless `forceSuccessResponse` is used;
- a combined `react-query + Zod runtimeValidation` target generated invalid TypeScript.

The production-shaped recommendation is therefore **validated Fetch + Zod Mini + existing BodySense error normalization + thin handwritten TanStack Query adapter**, not fully generated validated hooks.

### O2 Hey API

Strengths:

- SDK/types/Zod/TanStack generated coherently and typechecked;
- clean 409 error channel;
- better direct TanStack Query ergonomics;
- faster validator in the same fixture benchmark.

Limitations:

- SDK responses are not automatically passed through generated Zod schemas;
- current generated Zod objects strip unknown keys rather than reject them for `additionalProperties: false`;
- the public 0.99.0 Zod plugin config does not expose a direct strict/unknown-key policy switch.

Validator bundle:

- 98,060 B raw;
- 27,206 B gzip;
- 24,119 B Brotli.

### Browser validator benchmark

Headless Chromium 1228, same scaled `HealthWorkspace` fixture:

| Payload | Orval Mini p50 / p95 | Hey Zod p50 / p95 |
|---:|---:|---:|
| ~2.1 KB | 0.0 / 0.1 ms | 0.0 / 0.1 ms |
| ~10 KB | 0.1 / 0.3 ms | 0.0 / 0.1 ms |
| ~100 KB | 1.0 / 1.3 ms | 0.4 / 0.7 ms |

The performance delta is measurable but not large enough to dominate the correctness/maintenance decision.

### REST weighted score

| Candidate | Score /100 |
|---|---:|
| B0 handwritten | 64.5 |
| **O1 OpenAPI + Orval** | **83.5** |
| O2 OpenAPI + Hey API | 82.5 |

This is a **narrow** O1 win, not a general claim that Orval is better software. Contract correctness and runtime trust were intentionally weighted above hook-generation convenience.

## OpenAPI governance mutation matrix

Redocly lint passes on the common contract. With `oasdiff breaking --fail-on ERR`:

- M1 required field rename: blocked;
- M2 field type change: blocked;
- M3 enum removal: blocked;
- M4 optional -> required: blocked;
- M5 nullable -> non-null: **not detected**;
- M6 add optional field: allowed.

Thus `oasdiff` is useful but not a complete semantic oracle. Nullable/presence mutation tests remain necessary.

## Public stream: J1 JSON Schema-first

The existing checked-in schema could not replace the parser immediately.

Initial findings:

- 34 existing real event fixtures matched the parser;
- 10 basic malformed cases matched;
- five targeted semantic probes exposed real schema/parser drift:
  - `run.failed` reason-only;
  - safety verdict enum;
  - safety reasons string-array rule;
  - interaction fields maximum of three;
  - interaction options string-only rule.

An isolated candidate schema fixed only the schema, not the parser. After cleanup:

- Ajv compiles with strict required checking;
- 34/34 real fixtures agree;
- 10/10 malformed cases agree;
- 5/5 semantic probes agree.

M9 then added `state.contract_probe` only to the schema. Regeneration automatically updated the TS discriminated union and runtime validator; valid new event passed and malformed new event failed.

### Stream performance / bundle

Node, 10,000 events:

- handwritten parser: ~5.814 ms (~1.72M events/s);
- fixed Ajv standalone: ~23.509 ms (~425k events/s).

Chromium:

- 10,000 events: ~27.1 ms (~369k events/s).

Browser bundle:

- current hand parser: 5,854 B raw / 1,760 B gzip;
- generated Ajv browser validator: 103,993 B raw / 12,501 B gzip / 9,552 B Brotli.

J1 loses raw performance and bundle size, but wins source-of-truth reduction and remains vastly faster than the real event arrival rate.

### Stream weighted score

| Candidate | Score /100 |
|---|---:|
| B0 handwritten/parity | 72.0 |
| **J1 JSON Schema-first** | **85.5** |

Adoption must start with schema cleanup + parity gates; do not delete the existing parser first.

## Internal runtime: P1 Proto/Buf

### P1-A IDL/codegen

One `.proto` generated Go, Python and TypeScript representations plus gRPC/Connect glue. Protovalidate was verified in TS, Go and Python for non-empty configuration and valid StreamEvent version/sequence.

Buf breaking results:

| Mutation | FILE | WIRE_JSON | WIRE |
|---|---|---|---|
| rename field | block | block | pass |
| change field number | block | block | block |
| string -> bytes | block | block | pass |
| remove field | block | block | block |
| add field | pass | pass | pass |
| stream -> unary | block | block | block |
| tighten Protovalidate rule | pass | pass | pass |

BodySense should use FILE/WIRE_JSON gates; validation-rule mutation tests are still required because Buf breaking does not reason about every Protovalidate tightening.

Forward compatibility probes also verified binary unknown-field preservation and proto optional presence.

### P1-B transport

The same semantic event sequence was served from one Python process over HTTP/NDJSON and synchronous Python gRPC server streaming.

At 10,000 events:

- HTTP/NDJSON: ~102.8 ms, ~97k events/s;
- gRPC receive-only: ~1,647.9 ms, ~6.1k events/s;
- gRPC + Protovalidate: ~1,652.3 ms, ~6.1k events/s;
- framed protobuf bytes: ~1.27 MB vs NDJSON ~2.09 MB.

So protobuf framing was ~39% smaller, but this 1:1 fine-grained gRPC stream was drastically slower. Validation was not the source of the slowdown.

**Result:** select Proto as an internal IDL candidate, not as an automatic transport rewrite.

### Connect unary

A real `connect-web` client called a generated Connect-Go unary handler successfully. Browser bundle for the client path was ~194,920 B raw / 46,989 B gzip / 40,143 B Brotli.

Connect is viable for deliberate unary browser/internal surfaces, but not a reason to replace REST everywhere.

### Internal weighted score

| Candidate | Score /100 |
|---|---:|
| B0 handwritten HTTP/NDJSON | 71.5 |
| **P1-A Proto/Buf IDL** | **84.5** |

The score is for **IDL/codegen/validation with transport kept as a separate decision**. It must not be read as a score for the measured gRPC streaming transport.

## AI / Agent context result

Pixel Control Plane was unavailable during the final measurement (`127.0.0.1:8320` not listening), so the spike did **not** fabricate real Agent token telemetry. It used the plan's approved fallback: tokenizer-based context-surface proxy with `o200k_base`.

Generated directories excluded from normal Agent context:

| Scope | Baseline required context | Candidate required context | Change |
|---|---:|---:|---:|
| REST O1 | 4,403 tokens | 3,958 | -10.1% |
| REST O2 | 4,403 | 3,850 | -12.6% |
| Stream J1 | 9,965 | 5,283 | -47.0% |
| Internal P1 | 7,776 | 961 | -87.6% |

But if generated output is made visible by default:

- O1 context becomes ~22.6k tokens;
- O2 ~34.5k;
- J1 ~49.8k;
- P1 Go/Python ~77.1k;
- P1 three-language ~173.9k.

The hypothesis is therefore confirmed in a qualified form:

> **Codegen reduces Agent context only when generated-context hygiene is part of the architecture. Codegen alone can increase token cost by an order of magnitude.**

## Required generated-code policy

Any later migration should include all of the following from day one:

- one obvious generated directory per contract family;
- generated file headers / no manual edits;
- deterministic regenerate + Git no-diff CI gate;
- review defaults to canonical schema + semantic diff, generated diff folded;
- ForgeFlow/Agent search defaults exclude generated output;
- generated code read only for generator debugging;
- pinned generator/runtime versions;
- mutation/conformance fixtures remain handwritten evidence.

## Final rollout sequence if migration is approved later

This spike does **not** perform production migration. A safe follow-up order is:

1. **REST foundation:** create canonical OpenAPI 3.1 package, Go strict server generation + request middleware, deterministic CI gates.
2. **One REST vertical slice:** migrate `GET /api/v1/health-workspace` and one BodyState mutation to Orval validated Fetch/Zod Mini with a thin TanStack adapter.
3. **Stream canonicalization:** repair the five discovered JSON Schema semantic drifts, add strict compile/parity gate, then shadow generated validation before removing the handwritten parser.
4. **Internal IDL:** introduce Proto/Buf/Protovalidate for `StartConsultationTurnRequest` / `ResumeInterruptRequest` behind existing transport adapters.
5. **Transport separately:** only migrate selected unary calls to Connect/gRPC after workload-specific benchmarks; keep current event stream transport until a better experiment proves otherwise.

## Non-decisions

The spike does not authorize:

- a big-bang rewrite;
- replacing all REST with Connect;
- replacing public StreamEvent JSON with protobuf;
- deleting parity/fixture tests;
- accepting generated code in normal Agent context;
- merging any experiment worktree into production automatically.
