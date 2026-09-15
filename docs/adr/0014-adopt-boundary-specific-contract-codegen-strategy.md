# ADR 0014: Adopt a Boundary-Specific Contract Codegen Strategy

- Status: Accepted and implemented by the completed vNext engineering reset
- Date: 2026-09-09
- Decision evidence: `docs/architecture/contract-codegen-spike-results-2026-09-09.md`

## Context

BodySense currently maintains several important protocol facts in more than one language and representation. Examples include:

- browser REST projections represented independently in Go DTOs, TypeScript types and clients;
- `StreamEvent` represented in TypeScript static types, a handwritten TypeScript parser, JSON Schema, Go DTO and Python Pydantic model;
- the Go ↔ Python AI runtime request/stream boundary represented independently in Go and Python.

The existing parity/fixture discipline is valuable, but it makes contract changes fan out across several handwritten files. The 2026-09-09 spike tested whether canonical schemas plus generated representations can reduce drift and maintenance without forcing one transport/IDL onto every boundary.

## Decision

BodySense should use **different canonical contract technologies for different protocol boundaries**.

### 1. Browser-facing REST: OpenAPI 3.1 spec-first

Canonical source:

`OpenAPI 3.1`

Generation / trust boundaries:

- Go: `oapi-codegen` strict server/models;
- Go ingress: OpenAPI request-validation middleware is mandatory;
- Python: generated Pydantic v2 models where Python consumes the contract;
- Web default candidate: Orval validated Fetch + Zod Mini;
- Web query integration: thin handwritten TanStack Query adapter;
- Web errors: preserve BodySense's normalized `ApiRequestError` boundary rather than trusting generated error aliases blindly.

Orval is selected only narrowly over Hey API for the measured BodySense contract. The selection should be revisited on generator upgrades.

### 2. Public StreamEvent: JSON Schema 2020-12 first

Keep JSON/SSE/NDJSON as the public wire representation.

After fixing the semantic gaps discovered by the spike:

- JSON Schema becomes the canonical event envelope/variant source;
- TypeScript static types are generated;
- runtime validator is compiled/generated;
- existing fixture/parity tests remain as behavioral evidence;
- handwritten parser is removed only after a shadow/parity migration proves equivalence.

Do not convert the public stream to Protobuf only for uniformity.

### 3. Go ↔ Python internal runtime: Proto/Buf + Protovalidate as IDL

Canonical source:

`.proto` + Protovalidate annotations

Governance:

- Buf deterministic generation;
- `buf breaking` using FILE and/or WIRE_JSON compatibility appropriate to consumers;
- explicit tests for validation-rule changes that Buf breaking does not detect;
- generated Go/Python code excluded from normal Agent/review context.

This decision is initially about **IDL/codegen/validation**, not an automatic transport migration.

### 4. Transport decisions remain independent

The spike rejects a 1:1 migration of the current fine-grained NDJSON event stream to the measured synchronous Python gRPC server-streaming design. Protobuf framing was smaller, but throughput was much worse.

Connect/gRPC may be introduced selectively for unary or coarser-grained internal calls after workload-specific benchmarks.

### 5. Generated-context hygiene is architectural, not optional

Generated code must be excluded from normal Agent context and folded in review by default. The canonical schema, handwritten adapter and handwritten tests are the primary review/reasoning surface.

Without this rule, codegen increased measured context tokens dramatically; with it, required context fell by ~10% for REST, ~47% for StreamEvent and ~88% for the internal Proto candidate in the measured proxy.

## Required CI controls

### OpenAPI

- Redocly lint;
- `oasdiff breaking --fail-on ERR`;
- explicit nullable/presence/closed-schema mutation cases;
- deterministic regenerate + no-diff;
- generated Go request-validation middleware test;
- generated Web malformed-response tests.

### JSON Schema / StreamEvent

- strict schema compile;
- generated static/runtime output no-diff;
- real fixture parity;
- semantic mutation probes;
- new-variant evolution test.

### Proto/Buf

- Buf lint/generate;
- Buf FILE/WIRE_JSON breaking gate;
- Protovalidate invalid fixture tests in each runtime used by the boundary;
- validation-rule mutation test;
- deterministic regenerate + no-diff.

## Consequences

### Positive

- protocol ownership becomes explicit;
- fewer handwritten cross-language definitions;
- drift is moved earlier into schema/breaking/codegen/compile gates;
- runtime trust boundaries become reviewable architecture instead of generic type assertions;
- migration can be incremental by boundary;
- generated-context exclusion can materially reduce Agent reasoning surface.

### Negative

- BodySense will intentionally have more than one contract technology;
- generators and validation runtimes add pinned toolchain dependencies;
- generated source is large, especially Proto validation descriptors;
- OpenAPI/Buf breaking tools do not detect every semantic tightening;
- Orval 8.30.0 requires a specific production shape and has generator edge cases documented by the spike;
- JSON Schema generated runtime validation is larger/slower than the current handwritten stream parser;
- Proto transport cannot be assumed faster from smaller payload size alone.

## Alternatives rejected

### One technology for all boundaries

Rejected because browser REST, public JSON streaming and internal cross-language runtime have materially different requirements and measured costs.

### Go-first as the REST canonical source

Rejected for this boundary because the tested Go annotation path did not preserve the same OpenAPI 3.1 fidelity without additional duplicated annotations/transformation rules.

### Hey API as the default Web generator

Not rejected generally; retained as a close runner-up. The measured version had better SDK/TanStack ergonomics but weaker default runtime trust semantics for this contract.

### Proto-ize public StreamEvent

Rejected because it adds browser/runtime bundle and debug cost without solving a transport problem that JSON Schema-first can solve more locally.

### Replace NDJSON with gRPC server streaming now

Rejected by the measured transport experiment. A future optimized/async/batched transport experiment can revisit this independently.

## Migration policy

ADR 0015 accepted this boundary-specific strategy for the coordinated pre-user vNext reset. The completed implementation and acceptance evidence are recorded in `docs/plan/archive/2026-09-13-bodysense-vnext-engineering-reset.md`; migration-era compatibility/shadow requirements were replaced by the reset policy where no user/data compatibility obligation existed.
