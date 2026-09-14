# Phase 04 — Internal Go/Python runtime Proto authority

Status: **COMPLETE ON BRANCH — awaiting canonical integration**

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

## RUNTIME-002 — Control-command serving cutover

Status: **COMPLETE**

The start-turn and resume-control paths now use generated Proto as the serving authority while preserving HTTP transport and handwritten application inputs:

- Go maps the existing `StartConsultationTurnRequest` / `ResumeConsultationInterruptRequest` into generated commands, validates them with Protovalidate, and only then emits Proto JSON over HTTP.
- `thread_id` and `interrupt_id` are present in the validated command as well as the route. Python rejects any path/body identity mismatch before LangGraph execution.
- Python parses the incoming object into generated Proto, runs Protovalidate, then immediately projects it back into the existing Pydantic/runtime input. Generated messages never enter LangGraph state.
- Invalid UUID/configuration/input commands fail before the AI HTTP boundary; route mismatch tests fail closed with 422.

Focused evidence:

- Go AI-client/runtime Proto tests — PASS
- Python Proto/adapter/route tests — 15/15 PASS
- focused Ruff — PASS

## RUNTIME-003 — Internal event serving cutover

Status: **COMPLETE**

The Python→Go runtime stream is now Proto-authoritative while retaining HTTP + NDJSON framing:

- Python converts the in-process consultation event into generated `RuntimeEvent` oneof, validates it with Protovalidate, and emits Proto JSON NDJSON.
- Go decodes each NDJSON record with `protojson.Unmarshal(DiscardUnknown=false)`, validates it with Protovalidate, and immediately projects it into handwritten `ConsultationRuntimeEvent`.
- `ConsultationRuntimeEvent` is private application-facing state (`Kind + Seq + IDs + validated payload`) and is deliberately distinct from public `dto.StreamEvent`.
- Go consultation runtime explicitly maps each private event kind into the public StreamEvent vocabulary. The Agent-configuration handshake remains private control-plane state and is never projected to Web.
- the legacy internal `channel/type` allowlist and generic payload-shape validator have been deleted.
- Thought Forest citation identity and answer-attribution validation remain explicit application-level validation for the intentionally opaque `Struct` payloads, after Proto trust has been established.

### HITL identity semantics

The cutover keeps two different identities explicit rather than conflating them:

- the LangGraph interrupt identifier emitted by Python is a 32-character lower-case hex value (`xxh3_128_hexdigest`); the private event Proto validates that exact shape;
- Go persists a separate durable `AgentInteraction` UUID and replaces the private LangGraph identifier before emitting the public interaction event;
- the Resume command carries the Go durable interaction UUID because it is the external durable interaction identity. Python validates path/body equality before executing the resume command, while LangGraph receives the resume value through `Command(resume=...)` rather than treating that UUID as its checkpoint interrupt id.

This preserves the existing public HITL contract while making the private checkpoint identity honest.

### Configuration provenance parity

The Agent-configuration handshake now preserves the complete `ConsultationAgentManifest.provenance()` shape. Optional structured-intake provenance remains nested and carries its own immutable model/prompt/output-schema/policy revisions plus generation configuration. The default `consultation-v2` FastAPI stub route is covered end-to-end so these fields cannot silently disappear at the Proto boundary.

### Proto JSON default-value semantics

Boundary serialization uses default-value emission only for non-presence scalar/repeated/map fields. Presence-aware fields remain omitted when absent. This preserves required semantic defaults such as `has_red_flags=false` and empty safety arrays without fabricating `null` message fields on `stream.done`.

### Contract/codegen hardening

Normal code generation no longer depends on Buf remote plugins:

- `protoc-gen-go v1.36.12` is pinned and bootstrapped locally;
- official `protoc 32.1` is downloaded with a release SHA256 check and provides builtin Python + `.pyi` generation;
- `buf.lock` continues to pin the Protovalidate schema module;
- codegen consumes the lock and local plugins rather than updating dependencies during generation.

This removes BSR remote-plugin rate limiting from normal `contracts:generate/check-generated` and makes the generated artifact check reproducible.

### Evidence

Current Phase 04 branch evidence after RUNTIME-003:

- `pnpm contracts:verify` — PASS, including deterministic regenerate, Buf breaking, validation mutation, conformance and architecture guards;
- Go `go test ./...` — PASS;
- Python `uv sync --frozen --extra dev --extra ocr` then full pytest — **497/497 PASS**;
- Python Ruff — PASS;
- focused private-runtime Go service/consultation tests — PASS;
- focused Python Proto adapter/route tests — PASS;
- FastAPI E2E stub emits Proto oneof NDJSON, preserves nested v2 intake provenance and exposes no legacy `channel/type/payload` wire;
- architecture guard requires generated runtime Proto imports to remain adapter/contract-test only and rejects resurrection of the generic internal channel/type authority.

## Phase 04 final validation

Status: **COMPLETE**

Final branch evidence:

- `bash scripts/validate-repo.sh` — `REPO_QUALITY=PASS`;
- `SKIP_QUALITY=1 bash scripts/local-deploy-validate.sh` — `LOCAL_DEPLOY_VALIDATION=PASS`;
- browser E2E — **10/10 PASS** on one worker;
- explicit Consultation cancel — PASS (41.7s);
- API process restart / `execution_lost` recovery — PASS (26.9s);
- BodyState → Diagnosis → Treatment → Training → Outcome longitudinal loop — PASS;
- migration replay / PG18 domain semantics / Knowledge verticals — PASS;
- independent architecture review — generated Proto remains boundary-only, the public StreamEvent authority remains JSON Schema, and the private Agent-configuration handshake never reaches Web.

The recovery E2E file now carries an explicit 60s budget. The prior 30s default was proven too small by a run where every cancel/reload assertion succeeded at roughly 35s before Playwright timed out during teardown; no product timeout or recovery behavior was relaxed.

### Acceptance mapping

- **No shared generic StreamEvent crosses internal and public boundaries** — PASS. Python's generic event remains in-process only; the HTTP/NDJSON seam is generated Proto, and Go immediately projects it into handwritten private state before public projection.
- **Invalid internal messages fail before business/runtime execution** — PASS. Start/resume commands and every streamed event are strict Proto JSON + Protovalidate boundaries; unknown fields/variants fail closed.
- **HITL resume preserves exact thread + immutable Agent configuration semantics** — PASS. Path/body identity is checked before LangGraph; Go durable interaction UUID and LangGraph private hex interrupt identity remain distinct.
- **Generated Proto classes do not leak into LangGraph state or Go domain services** — PASS via automated architecture guard and direct import audit.
- **Disconnect/cancel semantics remain unchanged** — PASS via full local browser characterization, including explicit cancel and API restart recovery.

Canonical integration SHA is intentionally recorded only after the Phase 04 PR is merged into `refactor/bodysense-vnext`.
