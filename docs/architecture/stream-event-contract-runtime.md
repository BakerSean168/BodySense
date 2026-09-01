# StreamEvent Contract Runtime

- Status: Current public stream/recovery contract
- Updated: 2026-09-01
- Historical design snapshot: [`docs/plan/archive/architecture-snapshots/2026-09-01/stream-event-contract-runtime.md`](../plan/archive/architecture-snapshots/2026-09-01/stream-event-contract-runtime.md)
- ADR: [0003 — StreamEvent Contract Versioning](../adr/0003-stream-event-versioning.md)

## 1. Authority

`packages/contracts` is the public Web stream contract authority:

```text
packages/contracts/src/stream-events.ts
packages/contracts/src/stream-event-parser.ts
packages/contracts/schemas/stream-event.v1.schema.json
packages/contracts/fixtures/stream-events.v1.json
```

Public events are not raw provider/Python events.

```text
Python internal runtime protocol
  -> Go internal protocol validation
  -> Go Runtime Event Log / projection mapping
  -> Public StreamEvent v1
  -> Web parseStreamEvent()
  -> ActiveTurn reducer/projection
```

## 2. Implemented invariants

- live SSE and durable recovery both pass runtime validation before reducer consumption;
- internal `runtime.*` control events do not leak to the browser;
- malformed/unknown internal protocol data fails closed rather than being silently forwarded;
- `runtime.agent_configuration` is an internal Consultation identity handshake, not a public event;
- public `seq` is monotonic and unique within a `run_id`;
- durable replay uses the same public semantic contract as live delivery;
- Web deduplicates by run/sequence semantics and treats transport disconnect separately from business cancellation;
- `run.cancelled`, `stream.done` and `stream.error` have explicit terminal semantics;
- channels in v1 include the currently modeled message/run/tool/state/source/safety/usage/stream/job/conversation/title surfaces encoded by the shared schema/fixtures.

## 3. Runtime Event Log vs Public StreamEvent

These are related but different contracts:

```text
Runtime Event Log
  -> durable internal/audit facts and public-event ledger

Public StreamEvent
  -> versioned client projection contract
```

Not every internal event is public. Public replay must reconstruct the same `StreamEvent` semantics rather than expose internal Python/Go payloads directly.

## 4. Recovery

```text
live run
  -> StreamEvent seq 1..N
  -> network disconnect
  -> GET durable events after_seq=N_seen
  -> parse/dispatch same event contract
  -> projection catches up
```

SSE transport loss does not cancel the durable Run. Explicit cancellation is a separate command and terminal event.

## 5. Evolution rule

A new client-visible event requires aligned changes to the shared TypeScript contract/schema/fixtures and all affected producer/consumer tests. Do not add a one-off JSON payload in Go/Python/Web and call it “compatible”.

Breaking semantics require an explicit versioning decision under ADR 0003 rather than silent mutation of v1.

## 6. Current implementation evidence

- Go: `apps/api/internal/stream`, Consultation runtime, RuntimeEvent service/repository.
- Web: `useSSEProcessor`, `parseStreamEvent`, ActiveTurn reducer, durable recovery helpers.
- Cross-language: contract fixture tests under `packages/contracts` and Go DTO validation.

Remaining improvements should be tied to a concrete new event/use case. The migration-era percentage/phase checklist is retired because the public v1 schema now encodes and tests the active channel/event set.
