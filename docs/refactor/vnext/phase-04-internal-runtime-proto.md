# Phase 04 — Internal Go/Python runtime Proto authority

Status: **IN PROGRESS**

- Branch: `refactor/vnext-04-internal-runtime-proto`
- Parent canonical vNext commit: `6e84ae4ff`
- Public StreamEvent remains owned by `packages/contracts/schemas/stream-event.v1.schema.json` and is not reused as the internal runtime protocol.
- Transport remains HTTP + NDJSON for this phase; gRPC/Connect is an explicit non-goal.

## Goal

Replace the duplicated handwritten Go/Python Agent-runtime wire contract with one private Proto v1 authority while preserving the existing application/runtime ownership boundary:

- Python owns LangGraph thread/checkpoint execution and emits validated internal runtime facts.
- Go owns the durable run/event ledger and maps internal runtime facts into the public StreamEvent projection.
- generated Proto classes are boundary-only and must not become LangGraph state or Go domain/application models.

## RUNTIME-001 — Canonical IDL and validation foundation

Status: **COMPLETE**

Canonical IDL:

- `contracts/internal/agent-runtime/bodysense/runtime/v1/runtime.proto`
- Buf v2 module + pinned `buf.lock`
- Protovalidate annotations for runtime version, sequence, UUID identities, immutable Consultation configuration IDs and required oneof/message boundaries

Generated consumers:

- Go: `apps/api/internal/generated/runtimeproto/v1`
- Python: `apps/ai-service/src/generated/runtimeproto/...`

The Python generator includes the Protovalidate annotation descriptor and applies a deterministic import rewrite so generated code remains consumer-local without adding the generated root to global `PYTHONPATH`.

### Typed control commands

The IDL defines:

- `StartTurnCommand`
- `ResumeInterruptCommand`
- typed user input, image references, runtime state, spatial context and business-context envelope

`thread_id` is part of each validated command even though HTTP currently carries it in the route. The future HTTP adapter must combine path + request body before validation, so resume cannot execute against an implicit or mismatched thread identity.

### Typed internal event oneof

`RuntimeEvent` contains no public `channel`, no free-form `type`, and no generic event `payload`. It has a required oneof with 17 current serving variants:

1. Agent configuration handshake
2. Message text delta
3. Tool call
4. Tool result
5. Extracted-info upsert
6. Lifestyle-context upsert
7. Interaction required
8. Phase changed
9. Citation added
10. Answer attribution added
11. Knowledge gap
12. Red-flag detected
13. Safety output reviewed
14. Safety output rejected
15. Usage reported
16. Stream done
17. Stream error

Stable transport facts are strongly typed and validated. Complex application-owned projections such as BodyState snapshots, tool args/results, citation/attribution payloads and diagnosis/treatment context intentionally remain `google.protobuf.Struct`; their application contracts remain authoritative rather than being duplicated into the runtime IDL.

### Contract governance

The production runtime Proto participates in:

- `contracts:generate`
- deterministic `contracts:check-generated`
- `buf breaking` against `tools/contracts/baselines/runtime-proto`
- explicit validation-rule mutation checks, because custom validation option edits are not guaranteed to be classified as wire-breaking
- shared Go/Python valid + invalid fixture corpus

The fixture corpus covers start, resume, every oneof variant and invalid configuration/UUID/sequence/oneof/tool/error cases. Current evidence:

- `pnpm contracts:verify` — PASS
- Go runtime Proto corpus — PASS
- Python runtime Proto corpus — 9/9 PASS

## Serving cutover status

**Not started in RUNTIME-001.** The current Go AI client still sends handwritten JSON request structs and decodes the legacy `{channel,type,payload}` NDJSON envelope; Python still exposes Pydantic request models and its generic `StreamEvent` model.

The next checkpoint must introduce handwritten boundary adapters around generated Proto messages before deleting any legacy serving code. No generated type may leak into `internal/consultation` application state or LangGraph state.

## Next — RUNTIME-002

1. Build Go application↔Proto command adapters and validate before HTTP send.
2. Build Python Proto↔runtime adapters and validate before LangGraph execution.
3. Emit Proto JSON mapping over the existing NDJSON framing.
4. Decode + validate Proto JSON in Go before application handling.
5. Preserve exact `thread_id + configuration_id` resume semantics.
6. Characterize disconnect/cancel behavior before and after cutover.
