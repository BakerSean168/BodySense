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
- AI-service pytest — **503/503 PASS**;
- provider-stream + gateway routing focused tests — PASS;
- writer-boundary / stream-event focused tests — PASS;
- ToolExecutor ownership tests — PASS.

## Remaining Phase 05 work

### Python

- replace the remaining in-process `StreamEvent(type + payload)` seam between Consultation runtime and the Proto adapter with typed variants so `runtime_proto_adapter.py` no longer reconstructs event payload shape from a string discriminator;
- audit broad `except Exception` fallbacks and distinguish infrastructure/protocol failures from valid domain output where the distinction affects behavior;
- continue reducing `Any` only where an upstream parser has already established a narrower type.

### TypeScript

- remove unjustified `as`, `as unknown as`, and `as never` in trust-consuming application code;
- parse external/cache/replay data before state/domain usage;
- keep casts only at proven library interop boundaries and document those boundaries.

### Go

- make transport/parser errors finite and structured where callers branch on them;
- audit `json.RawMessage` so it stops at the boundary unless the field is intentionally opaque application JSON;
- keep canonical Proto/OpenAPI/JSON-Schema parsing as the single authority rather than re-parsing downstream.

Phase 05 is not complete until the remaining language-specific trust debt is closed and final repo/local-deploy validation passes.
