# Phase 05 — Runtime trust and explicit error semantics

Status: **IN PROGRESS**

- Branch: `refactor/vnext-05-runtime-trust`
- Parent canonical vNext commit: `ab8256260`
- Phase 04 private Proto boundary remains authoritative; this phase tightens trust-consuming application code on both sides of canonical transport parsers.

## Goal

Enforce one rule at every trust boundary:

```text
untrusted input -> one explicit parser/validator -> trusted typed value
```

Downstream code must not reconstruct the same wire shape with casts, optional-field bags, silent defaults, or caller mutation.

## TRUST-001 — Python provider/runtime trust foundations

Status: **COMPLETE**

### AI provider stream variants

The former `AiStreamEvent(type: str + optional fields)` bag has been replaced by four finite immutable variants:

- `AiTextDeltaEvent`
- `AiToolCallDoneEvent`
- `AiUsageEvent`
- `AiDoneEvent`

`AIService.generate_stream()` produces only those variants. The Consultation consumer narrows with `isinstance` and ends in `assert_never`, so adding a new provider-stream variant requires every consumer to make an explicit handling decision.

### Tool-call JSON fails closed

Provider tool-call arguments now pass through one `_decode_tool_arguments()` boundary in both streaming and non-streaming flows.

- malformed JSON raises `GatewayProtocolError`;
- valid JSON that is not an object raises `GatewayProtocolError`;
- no path converts malformed provider JSON into an apparently valid `{}` tool call;
- `AiToolCallDoneEvent.tool_arguments` is always a parsed object.

### LangGraph custom-writer boundary

LangGraph `stream_mode="custom"` values now pass through a Pydantic discriminated union with `extra="forbid"` before runtime mapping. The current writer vocabulary is finite and explicit:

- text delta;
- tool call/result;
- extracted info;
- lifestyle context;
- phase change;
- citation;
- answer attribution;
- knowledge gap;
- red flag;
- usage;
- stream error;
- `__done__` completion sentinel.

Unknown variants and malformed payloads raise `ConsultationWriterProtocolError`. `__done__` is the only explicitly non-protocol sentinel. The previously dropped `stream_error` writer event now maps to the private runtime `stream.error` path instead of disappearing silently.

### Value semantics

- `StreamEventFactory` copies caller-provided `StreamEventIds` before filling the conversation identity; caller-owned Pydantic models are not mutated.
- `ToolExecutor._validate()` returns a normalized argument copy plus an optional error. Integer-compatible floats are normalized for the handler without changing the provider/caller dictionary retained by runtime/audit state.

### Evidence

- AI-service Ruff — PASS;
- AI-service Pyright — **0 errors**;
- AI-service pytest — **503/503 PASS** at the TRUST-001 checkpoint;
- provider-stream + gateway routing focused tests — PASS;
- writer-boundary focused tests — PASS;
- ToolExecutor ownership tests — PASS.

## TRUST-002 — Typed Python private runtime events

Status: **COMPLETE**

The remaining Python application seam no longer reconstitutes the private wire from a generic `StreamEvent(channel + type + payload)` object. That model and its duplicate public-schema parity tests have been retired.

The application seam is now:

```text
LangGraph writer value
  -> strict ConsultationWriterEvent parser
  -> handwritten ConsultationRuntimeEvent variant
  -> exhaustive Proto adapter
  -> generated RuntimeEvent oneof + Protovalidate
```

`ConsultationRuntimeEvent.event` is a finite union covering all 17 canonical private Proto variants. The application variants carry direct fields instead of a generic payload dictionary, and the envelope owns only sequence plus runtime identities. There is no public StreamEvent channel in the Python private runtime model.

`runtime_proto_adapter.py` now maps each handwritten class directly into its corresponding generated Proto message and ends in `assert_never`. The `_RUNTIME_EVENT_FIELD_BY_TYPE` string lookup table has been deleted. Complex application-owned JSON still enters only the specific Proto `Struct` field that owns it.

The 17-variant fixture corpus now checks both oneof payload parity and exact runtime IDs. This caught and fixed a protobuf-copy regression where the LangGraph 32-hex `interaction_id` was present in the payload but missing from `RuntimeEvent.ids`.

Architecture governance now fails if either the retired Python `models/stream_event.py` or `_RUNTIME_EVENT_FIELD_BY_TYPE` authority reappears.

Evidence after TRUST-002:

- AI-service Ruff — PASS;
- AI-service Pyright — **0 errors**;
- AI-service pytest — **497/497 PASS**;
- typed runtime + Proto adapter focused tests — PASS;
- 17/17 private Proto variant corpus parity — PASS;
- generic Python `StreamEvent` references under AI-service — **0**.

## TRUST-003 — Explicit Python degradation and failure semantics

Status: **COMPLETE**

Broad fallbacks were classified by behavior rather than mechanically removed. Paths that already wrap an exception into an explicit failure (`stream.error`, domain infrastructure error, retry-then-rethrow, or opt-in ephemeral CI fallback) remain broad where the boundary must contain unknown failures. Paths that returned apparently valid business data were narrowed.

- Consultation reply fallback now catches only `GatewayUnavailableError`; unexpected runtime/programming errors propagate to the streaming boundary and become explicit `stream.error`.
- Consultation intake degrades only for `AgentRunError` or `GatewayUnavailableError`; unexpected runtime errors propagate.
- Knowledge splitter/curator degrade only for `AIError` or their explicit malformed-model-output `ValueError`; implementation defects no longer silently become heuristic/original output.
- Pose extraction preserves legitimate no-person/no-decode empty results but converts unexpected detector/runtime exceptions into `PoseMechanismError` instead of an empty metric set.
- MiMo Omni ASR fails the entire transcription if any chunk errors. It never publishes an unmarked partial transcript; every temporary chunk is still cleaned in `finally`.

Evidence after TRUST-003:

- AI-service Ruff — PASS;
- AI-service Pyright — **0 errors**;
- AI-service pytest — **505/505 PASS**;
- consultation model/intake error-semantic tests — PASS;
- Knowledge splitter/curator expected-degradation tests — PASS;
- posture mechanism tests — PASS;
- MiMo partial-transcript fail-closed test — PASS.

## Remaining Phase 05 work

### Python

- continue reducing `Any` only where an upstream parser has already established a narrower type; no broad-fallback cleanup remains unless later review finds another path that returns valid-looking data without explicit degraded/failed semantics.

### TypeScript

- remove unjustified `as`, `as unknown as`, and `as never` in trust-consuming application code;
- parse external/cache/replay data before state/domain usage;
- keep casts only at proven library interop boundaries and document those boundaries.

### Go

- make transport/parser errors finite and structured where callers branch on them;
- audit `json.RawMessage` so it stops at the boundary unless the field is intentionally opaque application JSON;
- keep canonical Proto/OpenAPI/JSON-Schema parsing as the single authority rather than re-parsing downstream.

Phase 05 is not complete until the remaining language-specific trust debt is closed and final repo/local-deploy validation passes.

## TRUST-004 — TypeScript public payload trust

Status: **COMPLETE**

The Consultation Web feature no longer reconstructs validated public payloads with trust-consuming casts.

### Shared StreamEvent sub-structures

The canonical StreamEvent validator now exposes narrow helpers for shared structures that also appear in REST/durable projections:

- `InteractionQuestion`;
- `ExtractedInfo`;
- `Citation`;
- red-flag payloads.

`consultationService` parses pending-interaction questions and extracted-info projections before feature state usage. Historical assistant-message rendering parses persisted citation/red-flag data before constructing its view model. Malformed persisted data fails with the same `StreamEventParseError` used by live SSE and durable replay instead of being cast into validity.

The UI-specific AskUser vocabulary remains an explicit domain projection rather than a second wire schema. `normalizeAskUserQuestion()` is shared by the live reducer and REST session/thread mapper and owns only the deliberate UI normalization (`select -> single_choice`, `scale -> number`, missing answer type -> `text`).

### Persisted message parts

`ConversationMessage.parts` is no longer `JsonObject[]` in the canonical OpenAPI contract. It is a five-variant discriminated `MessagePart` union:

- text;
- source;
- data;
- tool-call;
- tool-result.

Generated Go/OpenAPI and Web/Zod artifacts therefore validate the discriminant and required fields at the public REST boundary. `consultationService` can pass the already-validated generated union directly into the feature-domain message model; the former `as Message["parts"]` cast is gone.

Diagnosis projection cleanup follows the same rule: generated candidates/freshness flow directly where structurally compatible, while citations are parsed through the canonical Citation validator rather than double-asserted.

### Evidence

- Web typecheck — PASS;
- Web lint — PASS;
- Consultation focused tests — **61/61 PASS**;
- contract parser/parity tests — **18/18 PASS**;
- `pnpm contracts:verify` — PASS, public REST remains **95/95**;
- Go HTTP transport + consultation focused tests — PASS;
- `pnpm contracts:check-generated` — PASS;
- production Consultation feature trust casts (`as unknown as`, `as Message["parts"]`, `as StreamEvent`, `as never`) — **0**.

## TRUST-005 — Structured Go runtime errors

Status: **COMPLETE**

The private Go runtime parser now reports finite machine-readable failure classes instead of collapsing every trust-boundary problem into formatted strings.

`RuntimeProtocolErrorCode` distinguishes:

- Proto JSON decode failure;
- Protovalidate failure;
- invalid sequence representation;
- unsupported/missing oneof variants;
- Proto payload serialization failure;
- application-owned opaque payload validation failure.

`RuntimeProtocolError` preserves the underlying cause through `Unwrap()`, so callers/tests can use `errors.As` without branching on message text. Characterization covers both malformed legacy/generic wire and a Proto-valid but application-invalid Thought Forest citation.

Consultation application HTTP errors now use the finite `ConsultationErrorCode` vocabulary rather than an unconstrained `string`. All runtime constructors use named constants; the generated/public HTTP adapter converts the code back to its wire string only at the final response boundary.

### Evidence

- Go `go test ./...` — PASS;
- runtime protocol focused tests — PASS;
- consultation/httpapi focused tests — PASS;
- Consultation runtime raw error-code literals outside the constant declarations — **0**.

## TRUST-006 — Typed Go private runtime payloads

Status: **COMPLETE**

The Phase 04 Proto boundary no longer collapses a validated `oneof` back into `Kind + json.RawMessage` for application code to reconstruct later.

`ConsultationRuntimeEvent` now carries a closed handwritten payload interface whose concrete variants mirror the private runtime facts while remaining independent of generated Proto classes. The event kind is derived from the concrete payload variant rather than stored as a second string tag, so a `tool_call` kind cannot coexist with a `stream_done` payload.

Proto scalar/list/optional fields become normal Go fields at the AI boundary. `google.protobuf.Struct` / `ListValue` data remains `json.RawMessage` only on the specific fields whose application contract is intentionally opaque JSON (tool args/results, extracted/lifestyle state, citation/attribution, interaction question, usage/governance, etc.). Thought Forest citation and answer-attribution semantics are still validated once before the event becomes trusted application input.

The Consultation application consumes the typed payload directly:

- no private `PayloadAs()` reparse remains;
- Agent configuration uses its typed control-plane payload;
- phase/text/tool/state/safety/done/error facts use concrete structs;
- interaction handling receives the typed private question and only creates the public payload after replacing the LangGraph interrupt id with the durable Go interaction UUID;
- public StreamEvent JSON is marshalled exactly once at the Go→Web projection boundary.

A package-private marker prevents arbitrary external types — including the event envelope itself — from satisfying the private payload interface accidentally.

### Evidence

- Go `go test ./... -count=1` — PASS;
- private runtime `PayloadAs()` usages — **0**;
- generic `ConsultationRuntimeEvent.Payload json.RawMessage` — **0**;
- Proto default/presence characterization remains covered: `has_red_flags=false` and empty flags survive, absent `stream.done` message fields remain absent;
- Agent-configuration handshake remains private and HITL public identity replacement remains unchanged.

## TRUST-007 — Remaining Web transport and asset trust boundaries

Status: **COMPLETE**

The remaining obvious Web trust shortcuts outside Consultation were removed without turning legitimate compile-time assertions (`as const`, branded IDs, CSS/library interop) into a meaningless zero-cast target.

### Generic HTTP JSON helper

`safeJson()` now returns `unknown`; it never manufactures a caller-selected generic type. `expectJson()` requires an explicit parser function before returning `T`, and structured error parsing uses an `isRecord` type guard rather than `as Record<string, unknown>`. Production business requests remain on generated OpenAPI clients; the generic helper no longer provides a bypass around runtime validation.

### Pinned anatomy registry

The generated Vanatome 1.4.0 registry is parsed by a strict Zod schema before use. The parser also made an existing hidden field explicit: upstream source provenance (catalog/repository/commit/fixture URLs and SHA-256 digests) is now part of `AtlasRegistryInventory` instead of being silently ignored by a double assertion. Existing business consistency checks then verify the pinned release, hierarchy, geometry counts and laterality evidence. Invalid registry structure fails module initialization.

### Training plan projection

The canonical OpenAPI contract now defines `TrainingPhase` and `TrainingExercise` rather than exposing `TrainingPlan.phases` as `JsonObject[]`. Generated Zod therefore validates week/focus/exercises and their stable fields at the REST boundary. Training phases, Outcome and TreatmentRevision projections are structurally compatible with the feature domain and no longer use assertion-based conversion.

### Evidence

- `pnpm contracts:verify` — PASS; public REST remains **95/95**;
- Web lint — PASS;
- Web typecheck — PASS;
- API helper + Atlas + Training focused tests — **7/7 PASS**;
- production `as unknown as` / `as never` — **0**.

## TRUST-008 — Web runtime projection and static-asset trust

Status: **COMPLETE**

The next Web trust pass removes assertion-based trust from runtime projections that sit outside the generated REST envelope while preserving intentionally narrow compatibility seams for later deletion.

### Body-region ontology and anatomy mapping

The generated BodyRegion ontology and Vanatome region map now follow the same two-stage rule as the pinned Atlas registry: strict Zod parsing establishes structural trust first, then the existing domain validators enforce canonical IDs, laterality, parent/group vocabulary, atlas release identity, reverse ownership and registry membership. JSON imports no longer become trusted values through `as Type` or array assertions.

### Consultation compatibility projection

`ConsultationThread.body_state` now consumes the generated `BodyStateSnapshot` directly instead of asserting the feature type. The still-live pre-envelope Diagnosis rejection branch remains isolated until Phase 07, but its `JsonObject` response is now parsed field-by-field through a finite compatibility schema: status and confidence/severity enums are closed, candidate and citation arrays are validated explicitly, malformed values fail closed, and the branch no longer casts arbitrary records into `DiagnosisAnalysis`.

### Upload UI projection

`UserUpload` still intentionally exposes OCR and posture internals as public `JsonObject` values because the browser must not own the full OCR mechanism/provenance schema. A feature-local Zod projection now validates only the fields the UI consumes before they enter Zustand state. Unsupported provenance fields are dropped rather than silently trusted. Malformed nested posture/OCR data fails the load instead of being asserted into `OCRResult` / `PostureAnalysis`.

### UI unknown-value guards

User-selected body-region values use the canonical BodyRegion parser; ask-user answers, tool arguments and workspace JSON observations use record guards; shared message text relies on the `MessagePart` discriminant; treatment mutation results are narrowed through a structural guard; the SSE network failure holder has an explicit mutable `Error | null` type rather than an assertion inserted to defeat closure control-flow analysis.

### Evidence

- Web lint — PASS;
- Web typecheck — PASS;
- BodyRegion ontology + anatomy mapping + consultation service + upload store + assistant message projection focused tests — **44/44 PASS**;
- malformed legacy Diagnosis and malformed nested upload projections are covered by fail-closed tests;
- production `as unknown as` / `as never` remain **0**.
