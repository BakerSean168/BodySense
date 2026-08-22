# Consultation Runtime & RAG Hardening Plan — 2026-08-22

> Status: **COMPLETED — release-gate validated 2026-08-23**
>
> Scope: **L3 Consultation Runtime + L4 Python Async/RAG engineering debt discovered after the Agent Platform North-Star closeout**
>
> Non-goal: this is **not** a new Agent-platform migration and **not** a redesign of the successful Diagnosis/Treatment North-Star architecture.
>
> Audit baseline: local worktree `688be4c`; `origin/main` at audit time `89e7d5c`. The five remote-only commits touch CI / repository-owner references / setup scripts, not the L3/L4 architecture files reviewed here, so the findings apply to current `origin/main` as well.

---

## Closeout — 2026-08-23

All tickets in this bounded hardening program are complete. The implementation preserved the closed Agent-platform ownership model while repairing the audited L3/L4 boundaries.

| Ticket | Final state | Closure evidence |
| --- | --- | --- |
| BS-HARD-001/002 | **DONE** | Consultation execution identity is run-local; shared `pendingAgentConfiguration*` state removed; deterministic concurrency and race tests pass. |
| BS-HARD-003 | **DONE** | Python emits `runtime.agent_configuration` first; Go validates repository-known ID/role/policy/model and persists identity before semantic output; missing/malformed/mismatch paths fail closed. |
| BS-HARD-004 | **DONE** | HITL resume reloads the interrupted source Run configuration and reconciles it with the LangGraph checkpoint before resume. |
| BS-HARD-010/011 | **DONE** | `@bodysense/contracts` exports the public runtime parser; live SSE and durable recovery both use `unknown -> parseStreamEvent`; negative fixtures reject internal/unknown/malformed events. |
| BS-HARD-012 | **DONE** | Go no longer skips malformed Python NDJSON; protocol/type/channel/authority-payload failures become deterministic `stream.error` and failed runs. |
| BS-HARD-020 | **DONE** | Transport disconnect is detached from business cancellation; explicit authorized/idempotent Run cancel exists; terminal DB transitions are race-safe and `run.cancelled` is durable/replayable. |
| BS-HARD-030 | **DONE** | Out-of-band public events allocate `MAX(seq)+1` transactionally under the owning Run row lock; concurrent allocation test is monotonic/unique. |
| BS-HARD-031 | **DONE** | React projection deduplicates by `(run_id, seq)`, uses server timestamps/derived IDs, and identical durable histories reduce to deep-equal state. |
| BS-HARD-040 | **DONE** | KnowledgeLibrary uses one lifecycle-owned bounded `AsyncConnectionPool`; search/list/stats/ingest are async; ingestion rollback and bounded-startup tests pass. |
| BS-HARD-041 | **DONE** | Local SentenceTransformer initialization/encode are offloaded with `asyncio.to_thread` behind a bounded semaphore; event-loop heartbeat and concurrency tests pass. |
| BS-HARD-050 | **DONE — eval-only** | Grounding Eval v2 covers the required 10-case dataset and records 3 meaningful v1/v2 disagreements. Production Treatment governance remains v1 pending a separate qualification/promotion decision. |
| BS-HARD-060 | **DONE** | Authoritative architecture docs synchronized and the full local release gate passed. |

### Final validation evidence

```text
Focused Go consultation/service/repository: PASS
Focused Go race: PASS
Contracts parser/schema/typecheck: PASS
Focused Web Consultation runtime: 36 passed
Focused AI/RAG/Grounding: 46 passed

pnpm lint: PASS
pnpm typecheck: PASS (Pyright 0 errors)
pnpm test: PASS
  Web: 144 passed
  AI: 303 passed
  Go: go test ./... PASS
pnpm build: PASS

validate:local-deploy: PASS
  API_HEALTH=PASS
  AI_HEALTH=PASS
  WEB_HEALTH=PASS
  FULL_UP=PASS version=49
  LATEST_DOWN=PASS version=48
  LATEST_REPLAY_UP=PASS version=49
  BODY_STATE_SEMANTICS=PASS
  TREATMENT_ACTIVATION_ATOMICITY=PASS
  OUTCOME_FEEDBACK_ATOMICITY=PASS
  DOMAIN_SEMANTICS=PASS
  Playwright longitudinal E2E: 3 passed
  DIAGNOSIS_SHADOW_VALIDATION=PASS observations=3 blockers=0
  TREATMENT_SHADOW_VALIDATION=PASS observations=3 blockers=0
  TREATMENT_DECISION_TRACE_VALIDATION=PASS accepted_traces=2
  TREATMENT_REPLAY_INPUT_VALIDATION=PASS replay_inputs=3
  LOCAL_DEPLOY_VALIDATION=PASS

git diff --check: PASS
```

Grounding Eval v2 remains deliberately **qualification-only**. The three captured v1 false-pass disagreements are unsupported dosage, contraindicated evidence, and misleading high lexical overlap. This closeout does not promote v2 into production governance.

---

## 0. Executive decision summary

The 2026-08 Agent Platform convergence succeeded. BodySense now has a strong production-shaped ownership model:

```text
Go
= durable domain truth
= business/clinical authority
= immutable AgentConfiguration selection
= durable event / analysis / treatment history

Python
= typed semantic Agent runtime
= PydanticAI / LangGraph execution
= tool orchestration / checkpoint runtime state

LiteLLM
= physical model/provider routing, retry, fallback

React
= server projection consumer + active streaming projection
```

Diagnosis and Treatment are already close to the intended North-Star. Consultation also has the correct large-scale shape: Go run envelope, LangGraph checkpointing, HITL interrupt/resume, replayable Runtime Event Log, SSE transport, and a React ActiveTurn reducer.

This audit found **one P0 contract-corruption issue, three major L3 boundary gaps, and three L4 async/RAG hardening gaps**. The correct response is a bounded hardening program, not another broad refactor.

### Findings ledger

| ID | Severity | Finding | Root contract at risk |
| --- | --- | --- | --- |
| HARD-01 | **P0** | Consultation execution identity/provenance is stored on the shared service-level `Runtime`; interrupted/resumed runs can miss or inherit stale identity, and concurrent runs can overwrite each other | Immutable AgentConfiguration identity, execution provenance, replay/rollout correctness |
| HARD-02 | **P1** | StreamEvent network boundaries still use unchecked casts / permissive decode; malformed or wrong-shape events can enter reducers or be silently dropped | Public SSE/event contract, cross-language protocol integrity |
| HARD-03 | **P1** | Consultation run-lifetime semantics are inconsistent: code now survives client transport cancellation in-process, current architecture docs still say disconnect cancels the run, and there is no explicit user-facing run cancel command | Run lifecycle, cancellation, resource ownership, recovery semantics |
| HARD-04 | **P2** | Public event ordering/projection determinism is not fully canonical around out-of-band events and client dedupe; reducer also synthesizes wall-clock time | Replay equivalence, idempotency, deterministic UI projection |
| HARD-05 | **P1** | `KnowledgeLibrary` exposes `async` methods while executing synchronous psycopg queries on the event loop | AI-service concurrency, latency, timeout isolation |
| HARD-06 | **P1** | Local transformer embedding performs synchronous CPU/GPU work directly inside async call paths | Event-loop responsiveness and concurrent Agent/RAG execution |
| HARD-07 | **P2** | Current Treatment faithfulness evaluation is an MVP substring/alias matcher and cannot robustly distinguish evidence-grounded semantic support | RAG quality gate, false pass/fail risk |

### Recommended implementation order

```text
P0 run-scoped execution identity
        ↓
strict stream trust boundaries
        ↓
run lifecycle / explicit cancellation contract
        ↓
event sequencing + deterministic projection cleanup
        ↓
async RAG DB / embedding boundaries
        ↓
grounding / faithfulness Eval v2
        ↓
full release gate + architecture-doc synchronization
```

The order is deliberate: **identity correctness first, performance/eval refinement later**.

---

# 1. Audit evidence and current-system map

## 1.1 Repository state reviewed

At audit time:

```text
local HEAD:   688be4c
origin/main:  89e7d5c
```

The local worktree was behind by five commits, but those commits affected only:

```text
.github/workflows/docker-deploy.yml
.github/workflows/mirror-production-infra.yml
docs/team-collaboration-plan.md
scripts/setup-dev-server.sh
scripts/setup-server.sh
```

No reviewed Consultation/RAG architecture file differs between the two baselines.

Local uncommitted files present before this plan:

```text
.practice-map/maps/bodysense-fundamentals.md
docs/learning/00-unified-roadmap.md
```

These are learning-record changes and must not be mixed into implementation repair commits unless intentionally included.

## 1.2 Validation already run during audit

Focused checks were green before this plan was authored:

```text
AI focused tests: 33 passed
Go service / consultation / repository packages: passed
Web Consultation runtime tests: 27 passed
```

The current system is therefore **not generally broken**. The findings are architecture boundary / concurrency / async correctness issues that existing tests do not fully cover.

## 1.3 Current end-to-end Consultation path

```text
Browser POST /consultation-runs
        ↓
Go Consultation Runtime
  - idempotency
  - conversation/run/turn envelope
  - durable message rows
        ↓
Go → Python NDJSON stream
        ↓
LangGraph Consultation Thread
  - durable Postgres checkpoint
  - immutable consultation manifest in thread state
  - model/tool loop
  - interrupt()
        ↓
Python StreamEvent
        ↓
Go re-sequences public event
        ↓
RuntimeEventService
  - durable runtime_events ledger
        ↓
SSE to browser
        ↓
useSSEProcessor
        ↓
ActiveTurnReducer
        ↓
React streaming projection
```

Recovery path already exists:

```text
transport loss
→ browser remembers maxSeq
→ GET durable events?after_seq=N
→ dispatchReplayEvents(...)
→ same event handlers / reducer
```

HITL path already exists:

```text
LangGraph interrupt(ask_user)
→ AgentInteraction pending
→ run waiting_user
→ user answer durably written to BodyState
→ interaction answered
→ Python Command(resume=answer)
→ same LangGraph thread continues
```

These are protected strengths. The plan should **tighten** them, not replace them.

---

# 2. Protected contracts

All repairs must preserve the following unless the ticket explicitly declares a migration.

## 2.1 Domain ownership

1. Go remains the only durable health/business truth owner.
2. Python LangGraph checkpoints remain runtime protocol state, not BodyState truth.
3. React remains a projection consumer, never a health-state authority.
4. LiteLLM remains infrastructure routing only.

## 2.2 Agent identity and governance

1. Go selects a repository-known immutable Consultation configuration.
2. Python executes the exact configuration selected by Go.
3. Every durable Run must record the exact configuration/execution identity that actually produced it.
4. Identity mismatch must fail closed; rollout observation is not a substitute for runtime authorization.
5. Replay must never compare or attribute a run using another run's provenance.

## 2.3 Consultation runtime

1. Request idempotency remains `(user_id, request_id)` based.
2. One active run per conversation remains enforced.
3. `ask_user` remains a first-class durable interrupt, not a new ordinary chat message.
4. Interaction answers must be committed to durable BodyState before Agent continuation.
5. The same LangGraph `thread_id` continues across interrupt/resume.
6. Public runtime events remain replayable and versioned.

## 2.4 Streaming/public API

1. Existing public event type names remain stable during this hardening batch unless a versioned migration is explicit.
2. `GET .../events?after_seq=N` remains an exclusive lower-bound cursor API.
3. Live and replayed events must feed the same projection behavior.
4. Existing consultation routes/deep links remain compatible.

## 2.5 RAG

1. Diagnosis/Treatment targeted EvidenceGap acquisition remains intact.
2. `user_fact` must never be resolved from generic external RAG.
3. Evidence IDs/source provenance remain stable.
4. Current knowledge publication/review filtering remains enforced.

---

# 3. Finding HARD-01 — P0: shared Consultation execution identity corrupts provenance

## 3.1 Observed code

`apps/api/internal/consultation/runtime.go` stores transient per-run execution identity directly on the shared runtime object:

```go
type Runtime struct {
    ...
    pendingAgentConfigurationID string
    pendingAgentConfiguration   json.RawMessage
    pendingExecutionProvenance  json.RawMessage
}
```

`cmd/server/main.go` constructs one `consultationRuntime` and injects it into the HTTP handler. Go HTTP handlers are concurrent; therefore these fields are service-global mutable state, not request/run-local state.

When Python sends `runtime.agent_configuration`, `handleAIEvent()` mutates those shared fields:

```go
r.pendingAgentConfigurationID = prov.ID
r.pendingAgentConfiguration = payload.AgentConfiguration
r.pendingExecutionProvenance = payload.ExecutionProvenance
```

Later `finishTurn()` persists whatever values happen to be on the shared Runtime:

```go
r.runService.UpdateAgentConfiguration(
    ctx,
    run.ID,
    r.pendingAgentConfigurationID,
    datatypes.JSON(r.pendingAgentConfiguration),
    datatypes.JSON(r.pendingExecutionProvenance),
)
```

## 3.2 Deterministic stale-provenance path: interrupt/resume

The issue is not merely theoretical concurrency.

`stream_thread_turn()` emits `runtime.agent_configuration` **only after the LangGraph run finishes without an interrupt**:

```python
snapshot = await graph.aget_state(config)
if snapshot.interrupts:
    yield state.interaction.required
    return

# only reached for non-interrupted completion
yield runtime.agent_configuration
```

Therefore an interrupted run does **not** emit its configuration identity through this path.

`resume_thread_interrupt()` currently emits resumed model/tool events and eventually `stream.done`, but **does not emit `runtime.agent_configuration` at all**.

At the same time Go's `ResumeInteraction()`:

1. completes the old interrupted `Run`;
2. creates a **new Go Run envelope** for the user answer;
3. resumes the same Python LangGraph thread;
4. calls `finishTurn()` on the new Run.

Because the shared `pendingAgentConfiguration*` fields are not run-local and are not reliably reset, the resumed Run can persist stale identity from a previous completed run.

## 3.3 Concurrent corruption path

Example:

```text
Run A                         Run B
-----                         -----
runtime.agent_configuration
→ shared pending = config-A
                              runtime.agent_configuration
                              → shared pending = config-B
finishTurn(A)
→ persists config-B ❌
```

Potential consequences:

- wrong `agent_configuration_id` on a durable run;
- wrong execution provenance / model lineage;
- incorrect replay attribution;
- incorrect rollout observation;
- false configuration mismatch/match statistics;
- cross-user audit contamination;
- data race under `go test -race` once exercised concurrently.

This violates a universal North-Star invariant documented in `docs/architecture/agent-platform-role-governance.md`:

> durable output must contain correct role-appropriate lineage and identity mismatch must fail closed.

## 3.4 Additional gap: mismatch is observed, not rejected

Current Consultation completion builds:

```go
ConfigurationIdentityMatch:
    persistedID == deployment.ConsultationConfigurationID()
```

but this is passed into rollout observation. It does **not** prevent a mismatched response from being accepted/persisted/delivered.

This differs from Assessment/Posture/Title, which explicitly reject identity mismatch.

## 3.5 North-Star fix

### Decision A — execution identity is run-local data

Remove all mutable `pendingAgentConfiguration*` fields from `consultation.Runtime`.

Introduce a run-local value, for example:

```go
type ConsultationExecutionIdentity struct {
    ConfigurationID     string
    AgentConfiguration  json.RawMessage
    ExecutionProvenance json.RawMessage
}

type streamResult struct {
    ...
    ExecutionIdentity ConsultationExecutionIdentity
}
```

or equivalent run-local `streamState/result` ownership.

The service-level Runtime may contain immutable collaborators only; it must not contain mutable state that belongs to a particular HTTP/run execution.

### Decision B — configuration identity is an early control-plane handshake

The runtime configuration must be established **before ordinary model/tool content is accepted as trusted output**, not emitted at the very end.

Recommended Python order:

```text
Go request carries exact configuration_id
        ↓
Python resolves immutable manifest
        ↓
Python emits internal runtime.agent_configuration FIRST
        ↓
Go validates expected id / role / decision policy / logical model
        ↓
Go persists validated run identity while Run is active
        ↓
semantic stream begins
```

This makes interrupted/failed runs auditable too.

The event remains an internal Go↔Python control event and does not need to become a public Web event.

### Decision C — mismatch fails closed

On the control event, Go must validate at least:

```text
configuration.id == expected Go-selected configuration ID
role == consultation
decision_policy_revision == repository registration
logical_model == repository registration
execution_provenance.logical_model == expected logical model
```

Unknown/missing/mismatched identity terminates the active run through the normal failed-run path before ordinary output is authorized.

### Decision D — resume pins the original interrupted configuration

A HITL resume is continuation of the same logical LangGraph thread, so it must not silently switch to today's champion configuration mid-thread.

Repair flow:

```text
interrupted Run
  └─ durable AgentConfigurationID = C15
        ↓
ResumeInteraction
  └─ load source interrupted Run identity C15
        ↓
Resume request includes configuration_id=C15
        ↓
Python validates checkpoint consultation_manifest.id == C15
        ↓
Python emits early runtime.agent_configuration(C15)
        ↓
new Go Run records C15
```

If the old run predates provenance or the checkpoint configuration cannot be reconciled, resume must fail explicitly and instruct the caller to start a new run. Do not guess current champion.

## 3.6 Required tests

Add characterization/regression tests for:

1. two concurrent runs with different test configuration payloads never cross-persist identity;
2. `go test -race` shows no Runtime provenance race;
3. interrupted run records its configuration before `state.interaction.required` becomes durable/public;
4. resumed run persists the exact same configuration as the interrupted thread;
5. config mismatch fails the run and ordinary content is not authorized;
6. missing `runtime.agent_configuration` fails closed instead of persisting empty/stale provenance;
7. rollout observation reads identity from the completed run/result, never shared process state.

---

# 4. Finding HARD-02 — P1: StreamEvent trust boundaries are not runtime-safe

## 4.1 Web live SSE boundary

`apps/web/src/features/consultation/hooks/useSSEProcessor.ts` currently does:

```ts
const data = JSON.parse(dataStr);
const event = data as StreamEvent;
```

This is a TypeScript assertion, not runtime validation.

Malformed examples can enter application logic:

```json
{"version": 99, "seq": "oops", "type": "message.text.delta", "payload": {}}
```

or:

```json
{"version":1,"seq":1,"type":"state.interaction.required","payload":{"question":17}}
```

TypeScript types disappear at runtime; reducer correctness currently assumes the server always obeys the protocol.

## 4.2 Web durable replay boundary

`apps/web/src/features/consultation/services/consultationService.ts` maps API rows with multiple assertions:

```ts
JSON.parse(item.ids) as StreamEvent["ids"]
JSON.parse(item.payload) as StreamEvent["payload"]
...
} as StreamEvent
```

So live and replay paths both bypass the trust boundary independently.

## 4.3 Go internal Python stream boundary

`apps/api/internal/service/ai_client.go` decodes NDJSON into `dto.StreamEvent`:

```go
if err := json.Unmarshal(line, &event); err != nil {
    continue // skip malformed lines
}
```

Problems:

- malformed internal events are silently discarded;
- envelope values (`version`, `seq`, `channel`, `type`) are not strictly validated here;
- `handleAIEvent()` often calls `event.PayloadAs(...)` while ignoring the returned error;
- an invalid safety/tool/state payload can degrade into zero values instead of explicit protocol failure.

## 4.4 Existing contract assets that should be reused

The repo already has:

```text
packages/contracts/src/stream-events.ts
packages/contracts/schemas/stream-event.v1.schema.json
packages/contracts/fixtures/stream-events.v1.json
packages/contracts/fixtures/stream-event-types.v1.json
apps/ai-service/src/models/stream_event.py
apps/api/internal/dto/stream_event.go
```

This is a good base. The fix should strengthen one contract system rather than create another unrelated protocol representation.

## 4.5 North-Star fix

### Decision A — every network boundary starts from `unknown`

Required flow:

```text
wire bytes
→ JSON parse
→ unknown
→ runtime validator
→ trusted StreamEvent
→ handler/reducer
```

### Decision B — public and internal event vocabularies are distinct

Python has an internal `runtime` channel for `runtime.agent_configuration`; the public Web contract intentionally does not.

Keep that distinction explicit:

```text
InternalRuntimeEvent
  = Go ↔ Python control/runtime protocol

PublicStreamEvent v1
  = replayable Web-facing protocol
```

Do not add `runtime.agent_configuration` to the browser union merely to make schemas match mechanically.

### Decision C — one Web parser for both live and replay paths

Create a single exported parser in `@bodysense/contracts`, e.g.:

```ts
parseStreamEvent(input: unknown): StreamEvent
safeParseStreamEvent(input: unknown): Result<StreamEvent, ContractError>
```

Both:

```text
useSSEProcessor
consultationService.listRunEvents
```

must use the same parser.

Recommended implementation is schema-backed runtime validation in `packages/contracts`, using the existing versioned JSON Schema as the canonical public wire shape. If a validator dependency is introduced, keep it inside the contracts boundary; presentation code must not know the validation library.

The schema should be strengthened from envelope-only validation to event-type-aware payload validation for all public event types.

### Decision D — Go rejects invalid internal stream events explicitly

Add a strict internal validation function before `handleAIEvent()`:

```go
ValidateIncomingConsultationEvent(event) error
```

It should verify:

- version/channel/type vocabulary;
- required IDs for event classes;
- event-specific payload shape;
- internal-only vs public event classification.

Malformed NDJSON should become an explicit stream failure, not `continue`.

Do not rely on zero-value structs after ignored `PayloadAs` errors.

## 4.6 Failure policy

- malformed **internal Python event** → fail active run, persist explicit protocol error, emit safe `stream.error` where possible;
- malformed **live Web event** → terminate/recover stream through durable log, surface protocol error telemetry;
- malformed **durable API event** → treat as server data corruption, do not dispatch to reducer; surface an explicit recovery failure.

## 4.7 Required tests

Add negative fixtures:

- wrong version;
- seq < 1 / non-number;
- channel/type mismatch;
- missing required IDs;
- wrong payload type for `message.text.delta`;
- wrong payload type for `state.interaction.required`;
- unknown public event type;
- internal `runtime.agent_configuration` rejected by public parser;
- malformed Python NDJSON fails run instead of being skipped.

Preserve existing positive fixture tests.

---

# 5. Finding HARD-03 — P1: run-lifetime and cancellation semantics drift

## 5.1 Current code behavior

`apps/api/internal/consultation/runtime.go` now creates the execution context with:

```go
context.WithTimeout(context.WithoutCancel(parent), sseTimeout)
```

This intentionally strips HTTP request cancellation. SSE write errors are logged, while the execution path can continue using the detached context.

The Web also implements durable catch-up after transport failure:

```text
recoverDurableRunEvents()
→ poll GET .../events?after_seq=N
→ finish only after persisted stream.done / stream.error
```

This means current code is moving toward:

```text
transport lifetime != Agent run lifetime
```

within the lifetime of the Go process.

## 5.2 Current authoritative doc says the opposite

`docs/architecture/current-longitudinal-system.md` currently states:

```text
HTTP disconnect cancels the producer,
marks the run failed,
and clears active_run_id.
```

That no longer matches the reviewed code path.

## 5.3 Explicit run cancellation is missing

The repository has cancellation semantics for:

- `AgentInteraction`;
- durable `Job`;
- some stored run/message status representations.

But this audit did not find an explicit Consultation command such as:

```text
POST /api/v1/consultations/:conversationId/runs/:runId/cancel
```

`AbortController` / browser fetch abort must not be treated as durable business cancellation if transport disconnect is intentionally detached.

## 5.4 North-Star decision

Adopt and document the more robust model already implied by the code:

> **Client transport loss does not itself mean the user cancelled the Agent run. Cancellation must be an explicit durable command.**

But define the guarantee precisely:

```text
transport disconnect resilience
= run may continue in the current Go process until terminal state / timeout

NOT YET guaranteed
= automatic execution recovery after Go process crash/redeploy
```

Do not falsely advertise process-crash durable workers unless a separate orchestration project implements them.

## 5.5 Explicit cancellation flow

Recommended contract:

```text
POST /api/v1/consultations/:conversationId/runs/:runId/cancel
```

Go authority checks:

- user owns conversation/run;
- run is cancellable (`running` / possibly `waiting_user`);
- command is idempotent;
- cancellation records a durable terminal transition;
- pending interaction is cancelled if applicable;
- assistant message becomes aborted/cancelled consistently;
- `active_run_id` is cleared;
- a terminal public event is persisted.

Execution cancellation requires a run-scoped cancellation registry in Go, not HTTP request context reuse.

Example:

```go
type ActiveRunController interface {
    Register(runID uuid.UUID, cancel context.CancelFunc)
    Cancel(runID uuid.UUID) bool
    Release(runID uuid.UUID)
}
```

This registry is infrastructure/runtime state only; durable run status remains in the DB.

## 5.6 Terminal event choice

Prefer a first-class versioned event, for example:

```text
run.cancelled
```

rather than overloading `run.failed` with a fake error.

If adding the new public event would expand scope too far, a transitional v1 can persist `run.failed` with an explicit machine-readable cancellation reason, but the preferred North-Star is a distinct lifecycle state/event.

Any public contract change must update fixtures/schema/TS union/Go/Python tests atomically.

## 5.7 Required tests

1. browser transport abort does not automatically mark durable run cancelled;
2. detached run continues producing durable events after SSE writer failure (using a test writer);
3. explicit cancel transitions the durable run exactly once;
4. duplicate cancel is idempotent;
5. cancel closes pending interaction consistently;
6. cancelled run cannot later complete successfully;
7. recovery client terminates correctly on cancellation terminal event;
8. architecture doc matches implemented semantics.

---

# 6. Finding HARD-04 — P2: event sequencing and projection determinism debt

## 6.1 What is already correct

`apps/api/internal/stream/runtime.go` gives each `StreamWriter` a local monotonic sequence:

```go
outSeq := 0
nextSeq := func() int {
    outSeq++
    return outSeq
}
```

Incoming Python events are re-sequenced for the public Go stream.

Database schema enforces:

```sql
UNIQUE (run_id, seq)
```

and durable recovery uses `seq > after_seq`.

This is a good canonical design for ordinary in-band run events.

## 6.2 Out-of-band expiry bypasses the normal sequencer

`RuntimeEventService.RecordInteractionExpired()` creates a synthetic sequence from wall-clock nanoseconds:

```go
seq := int(time.Now().UTC().UnixNano()%1_000_000_000) + 1_000_000
```

This is intentionally outside the StreamWriter allocator.

Risks:

- sequence no longer means one simple per-run append order;
- theoretical `(run_id, seq)` collision remains possible;
- ordering is based on an implementation trick rather than a single owner;
- future out-of-band event types may copy this pattern and make ordering increasingly opaque.

## 6.3 Client dedupe is compensating for unclear identity semantics

`ActiveTurnState` stores:

```ts
lastSeqByType: Partial<Record<StreamEvent["type"], number>>
```

with the comment that backend events mix sequence spaces.

For a clean public protocol, replay identity should be conceptually:

```text
(run_id, seq)
```

not `(event_type, seq)`.

The reducer should not need per-type sequence workarounds once event/run identity is explicit.

## 6.4 Reducer is not fully deterministic

On `state.interaction.required`, reducer code creates:

```ts
created_at: new Date().toISOString()
```

Therefore replaying the same event later can produce a different projection timestamp.

A pure event projection should satisfy:

```text
same initial state + same event history
→ same projection
```

Wall-clock reads belong outside the reducer or inside the server event itself.

## 6.5 North-Star fix

### Decision A — public event identity is `(run_id, seq)`

Document and test this explicitly.

### Decision B — one durable allocator for out-of-band events

Do not derive sequence from random/time arithmetic.

For events emitted after the live StreamWriter is gone, add a repository/service operation that allocates the next sequence under a DB transaction/lock for that run, e.g. conceptually:

```text
lock run / sequence owner
→ current max seq
→ next = max + 1
→ insert event
→ commit
```

The exact SQL implementation should avoid a naked `MAX(seq)+1` race.

A small per-run sequence table/counter is acceptable if it simplifies correctness, but do not introduce event sourcing infrastructure beyond this need.

### Decision C — reducer dedupe becomes run-aware

Recommended client guard:

```text
active run ID changes
→ initialize/reset run projection sequence

same run
→ ignore seq <= lastSeq
```

Live SSE and durable replay must share this logic.

### Decision D — event carries deterministic interaction timestamps

Extend `state.interaction.required` payload with server-owned durable fields such as:

```json
{
  "interaction_id": "...",
  "question": {...},
  "created_at": "...",
  "expires_at": "..."
}
```

or derive the UI timestamp from the separately persisted interaction projection. Do not call `new Date()` inside the reducer for durable state.

## 6.6 Required tests

- monotonically ordered in-band events;
- out-of-band expiry receives exact next durable seq;
- no duplicate `(run_id, seq)` under concurrent append test;
- live + replay duplicates are applied once;
- replaying identical event history yields deep-equal ActiveTurnState;
- sequence reset/transition between distinct run IDs is explicit.

---

# 7. Finding HARD-05 — P1: KnowledgeLibrary blocks the event loop with sync psycopg

## 7.1 Observed code

`apps/ai-service/src/rag/knowledge_library.py` keeps:

```python
self._connection: Optional[psycopg.Connection]
```

and lazily creates it using synchronous:

```python
psycopg.connect(...)
```

But public methods are async:

```python
async def search(...):
    embedding = await self.embedding_generator.generate(query)
    conn = self._get_connection()
    with conn.cursor() as cur:
        cur.execute(...)
```

The same pattern appears in async ingestion/statistics paths.

This is an **async façade around blocking database I/O**.

A Consultation tool call:

```text
LangGraph async runtime
→ search_knowledge async handler
→ KnowledgeLibrary.search()
→ synchronous psycopg cursor/query
→ blocks AI-service event loop
```

Under concurrency this can stall unrelated Agent runs, health endpoints, interrupt/resume requests, and streaming progress.

## 7.2 Existing correct pattern to reuse

The repo already uses:

```python
AsyncConnectionPool
AsyncConnection
AsyncPostgresSaver
```

in `apps/ai-service/src/runtime/checkpointing.py`, with bounded startup timeout and explicit lifespan shutdown.

The installed `pgvector.psycopg` package also exposes `register_vector_async`, so there is no library blocker to a proper async pool.

## 7.3 North-Star fix

Refactor `KnowledgeLibrary` to own/inject an async DB capability instead of one sync singleton connection.

Recommended shape:

```python
class KnowledgeLibrary:
    def __init__(self, pool: AsyncConnectionPool[...], embedding_generator: ...): ...

    async def search(...):
        async with self._pool.connection() as conn:
            async with conn.cursor() as cur:
                await cur.execute(...)
                rows = await cur.fetchall()
```

Initialize/shutdown through FastAPI lifespan, alongside the LangGraph checkpointer lifecycle.

Suggested lifecycle:

```text
app lifespan start
├─ initialize LangGraph checkpoint pool
└─ initialize KnowledgeLibrary async pool

app lifespan stop
├─ close KnowledgeLibrary pool
└─ close checkpoint pool
```

Do not create a new DB connection per search.

## 7.4 Transaction / ingestion considerations

Ingestion currently performs multi-table writes with explicit commits. Preserve atomicity:

```text
knowledge source
+ segments
+ units
+ clips
= one intentional transaction
```

Use async transaction contexts, not many independent autocommitted statements.

Search should use short-lived pooled connections and remain read-only.

## 7.5 Tests

Add tests that prove behavior rather than merely checking `async def` syntax:

- pool is opened once and closed on lifespan shutdown;
- concurrent `search()` calls acquire independent pooled connections;
- search no longer touches sync `psycopg.Connection`;
- ingest rollback keeps source/unit tables consistent on failure;
- pgvector registration happens for async connections;
- connection failure is bounded and explicit.

Optional load characterization:

```text
N concurrent mocked/real local searches
→ event loop remains responsive to an asyncio heartbeat
```

---

# 8. Finding HARD-06 — P1: local embedding blocks async execution

## 8.1 Observed code

`apps/ai-service/src/rag/embedding.py` has:

```python
async def generate(...):
    if self.provider == "local_transformer":
        model = self._get_local_model()
        embedding = model.encode([text])[0]
```

and:

```python
async def generate_batch(...):
    embeddings = model.encode(texts)
```

`SentenceTransformer.encode()` is synchronous CPU/GPU work. Calling it directly inside an async coroutine blocks the event loop for the duration of encoding.

The deterministic hashing provider is also CPU work, though normally much lighter.

## 8.2 North-Star fix

Introduce a narrow execution adapter for blocking embedding providers.

For local transformer:

```python
embeddings = await asyncio.to_thread(model.encode, texts)
```

or a bounded dedicated executor when concurrency control is needed.

Recommended constraints:

- lazy model initialization remains thread-safe;
- cap concurrent local embedding calls to avoid CPU oversubscription / GPU contention;
- remote OpenAI-compatible provider remains truly async through `AsyncOpenAI`;
- deterministic hashing can remain inline if benchmarks prove it is negligible, otherwise use the same blocking executor abstraction.

Avoid creating a full worker queue solely for short interactive embeddings unless measurements show process-level isolation is necessary.

The existing Go durable `JobRuntime` is appropriate for long-running OCR/Posture/ingestion jobs, but **interactive targeted retrieval does not need to be converted into a background Job just to fix event-loop blocking**.

## 8.3 Tests

- local transformer encode runs outside event-loop thread;
- async heartbeat continues while encode stub blocks;
- concurrency limiter prevents unbounded simultaneous encodes;
- generated dimensions/results remain unchanged;
- remote embedding path remains async and retry behavior stays intact.

---

# 9. Finding HARD-07 — P2: faithfulness Eval is too shallow for a production grounding gate

## 9.1 Current implementation

`apps/ai-service/src/services/faithfulness_checker.py` explicitly labels itself MVP and uses:

- substring matching;
- a static `EXERCISE_ALIASES` map;
- no semantic context;
- no semantic similarity / NLP-based support relation.

This can both:

- accept an intervention because the same phrase appears somewhere without actually supporting the prescription;
- reject semantically equivalent wording;
- fail to evaluate dosage/progression/contraindication claims even when the exercise title is grounded.

## 9.2 What should remain deterministic

Do **not** replace the checker with a single opaque LLM Judge.

First keep hard structured rules deterministic:

```text
all cited evidence_ids exist in retrieved/admissible evidence
no evidence ID from another run/source set
user_fact not sourced from external RAG
evidence budget/policy respected
intervention has required structured prescription fields
```

## 9.3 Grounding Eval v2

Recommended layered evaluator:

```text
Layer 1 — deterministic provenance/contract checks
        ↓
Layer 2 — structured semantic support check
        ↓
Layer 3 — optional LLM Judge for nuanced support quality
```

The semantic unit should be an intervention claim, not only an exercise name:

```text
InterventionClaim
├─ kind
├─ title
├─ intended goal
├─ dosage / frequency / duration
├─ progression
├─ stop conditions
└─ supporting evidence IDs
```

Evaluator asks whether the referenced admissible evidence supports the material claim.

Possible v2 implementation:

1. canonicalize intervention fields and evidence excerpts;
2. deterministic exact/alias checks for high-confidence obvious matches;
3. embedding similarity or structured semantic matcher as a candidate-support signal;
4. Judge only uncertain cases, with a fixed rubric and evidence excerpts;
5. report reason codes rather than only `faithful: bool`.

Example result:

```json
{
  "verdict": "degraded",
  "claims": [
    {
      "intervention": "...",
      "support": "partial",
      "evidence_ids": ["E31"],
      "reasons": ["dose_not_supported_by_cited_evidence"]
    }
  ]
}
```

## 9.4 Qualification integration

Do not silently change production Treatment governance because a new evaluator exists.

Treat Grounding Eval v2 as behavior-significant evaluation logic:

```text
build evaluator dataset
→ run against current Champion
→ compare v1/v2 evaluator disagreement
→ manually inspect disagreement slice
→ define accepted thresholds
→ then wire into qualification/governance
```

The first release can be an **eval-only diagnostic** before it becomes a hard production gate.

## 9.5 Required dataset slices

Include at least:

- exact supported exercise;
- alias/synonym supported exercise;
- exercise mentioned but dosage unsupported;
- contraindicated evidence;
- evidence discusses same body part but not intervention;
- multiple sources where only one supports the claim;
- no evidence;
- misleading high lexical overlap;
- Chinese short-token false positive cases;
- progression/stop-condition support.

---

# 10. North-Star architecture after repair

## 10.1 Consultation identity + stream path

```text
Go selects immutable Consultation Configuration C15
        ↓
POST Python turn/resume with expected C15
        ↓
Python resolves exact manifest C15
        ↓
INTERNAL control event: runtime.agent_configuration(C15)
        ↓
Go validates id/role/policy/logical-model
        ↓
Go persists run-local execution identity immediately
        ↓
LangGraph semantic/tool events
        ↓
Go validates internal event contract
        ↓
Go allocates canonical public (run_id, seq)
        ↓
Durable Runtime Event Log
        ↓
SSE
        ↓
Web parse unknown → validated StreamEvent
        ↓
run-aware deterministic ActiveTurnReducer
```

## 10.2 Disconnect / cancellation

```text
browser transport disconnect
        ↓
NO implicit business cancel
        ↓
current Go process continues bounded run
        ↓
durable events append
        ↓
client reconnects with after_seq
```

Explicit user cancellation:

```text
Cancel command
→ Go authorization/idempotency
→ run-scoped cancel func
→ durable cancelled terminal state
→ terminal public event
→ active_run_id cleared
```

Process crash remains a separate availability boundary; this hardening plan does not pretend an in-process goroutine is a durable distributed worker.

## 10.3 RAG execution

```text
EvidenceGap / search_knowledge
        ↓
KnowledgeLibrary
        ├─ async Postgres pool
        └─ EmbeddingGenerator
              ├─ remote API: native async
              └─ local transformer: bounded executor/thread
        ↓
normalized Evidence
        ↓
provenance checks
        ↓
Grounding Eval
```

---

# 11. Phased implementation roadmap

## Phase 0 — Baseline and characterization

### Objective

Turn every verified finding into a failing/characterization test before changing architecture.

### Deliverables

- concurrency test for run-local provenance;
- interrupt/resume provenance test;
- config-mismatch fail-closed test;
- malformed stream-event negative tests;
- cancellation-semantics characterization test;
- event projection determinism test;
- async heartbeat tests for DB/embedding blocking;
- baseline faithfulness disagreement dataset.

### Acceptance

Each issue has a test that either fails on current code or precisely records the current behavior intended to change.

---

## Phase 1 — Consultation execution identity hardening (P0)

### Objective

Restore the invariant:

> **one durable Run ↔ one exact, validated, run-local Agent execution identity**.

### Main changes

- remove `pendingAgentConfiguration*` fields from shared `consultation.Runtime`;
- emit internal configuration handshake before semantic output;
- validate against Go-selected registration;
- persist identity immediately on the run;
- pin resume to interrupted run configuration;
- make resume emit/validate the same identity;
- fail closed when identity is missing/mismatched.

### Exit criterion

No service-global mutable run identity remains, concurrency tests/race detector pass, and every completed/interrupted/resumed durable Run has correct provenance.

---

## Phase 2 — Stream protocol trust boundary

### Objective

No untrusted JSON reaches Agent control logic or React reducers via type assertion / permissive decode.

### Main changes

- runtime parser in `@bodysense/contracts`;
- event-type-aware public schema validation;
- live + durable Web paths reuse parser;
- strict Go validation for internal Python events;
- stop silently dropping malformed NDJSON;
- stop ignoring payload decode errors for authority-relevant events.

### Exit criterion

Negative contract fixtures fail explicitly at the first trust boundary; positive fixtures remain cross-language compatible.

---

## Phase 3 — Run lifecycle, cancellation, event ordering and deterministic projection

### Objective

Make run lifetime, cancel semantics and replay identity explicit and internally consistent.

### Main changes

- declare `transport disconnect != cancellation` as the current runtime contract;
- add explicit durable run-cancel command;
- implement run-scoped cancel controller;
- add canonical out-of-band seq allocation;
- replace per-event-type dedupe with run-aware `(run_id, seq)` projection logic;
- remove reducer wall-clock dependence;
- update architecture docs.

### Exit criterion

Live, replay, disconnect recovery, explicit cancel and HITL tests all agree on one lifecycle model.

---

## Phase 4 — Async RAG resource boundary

### Objective

Remove event-loop blocking from interactive knowledge retrieval.

### Main changes

- async psycopg pool for KnowledgeLibrary;
- FastAPI lifespan ownership;
- async pgvector registration;
- transactional async ingestion;
- local transformer execution in bounded thread/executor;
- concurrency / heartbeat tests.

### Exit criterion

No synchronous DB query or local transformer encode runs directly on the AI-service event loop in interactive RAG paths.

---

## Phase 5 — Grounding / Faithfulness Eval v2

### Objective

Move from lexical exercise-name matching to evidence-linked intervention-claim evaluation without losing deterministic hard gates.

### Main changes

- claim-level evaluator model;
- provenance/ID contract checks;
- semantic support evaluator;
- disagreement dataset and review;
- qualification integration only after evidence.

### Exit criterion

Known lexical false positives/false negatives are covered, and evaluator promotion is evidence-based rather than silently replacing v1.

---

## Phase 6 — Full verification and plan closeout

### Objective

Prove the hardening batch did not regress the successful Agent Platform architecture.

### Release gate

Run in increasing scope:

```bash
# Focused Go
cd apps/api
go test ./internal/consultation ./internal/service ./internal/repository -count=1
go test -race ./internal/consultation ./internal/service -count=1

# Contracts
cd /home/ubuntu/projects/bodysense
pnpm nx run @bodysense/contracts:test
pnpm nx run @bodysense/contracts:typecheck

# Focused Web
pnpm nx test @bodysense/web --run \
  src/features/consultation/runtime/activeTurnReducer.test.ts \
  src/features/consultation/runtime/durableRunRecovery.test.ts \
  src/features/consultation/hooks/useSSEProcessor.test.ts \
  src/features/consultation/services/__tests__/consultationService.test.ts

# Focused AI/RAG
cd apps/ai-service
uv run pytest \
  tests/unit/test_stream_event.py \
  tests/unit/test_checkpointing.py \
  tests/unit/test_embedding.py \
  tests/unit/test_diagnosis_evidence_acquisition.py \
  tests/unit/test_treatment_evidence_acquisition.py \
  tests/unit/test_faithfulness_checker.py -q

# Repository gates
cd /home/ubuntu/projects/bodysense
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm validate:local-deploy
```

If the full Docker/E2E gate is too expensive during an intermediate repair ticket, it may be deferred to phase closeout, but the focused contract tests for that ticket are mandatory before moving on.

---

# 12. Execution-ready tickets

## BS-HARD-001 — Characterize Consultation provenance contamination

**Goal:** Add tests that reproduce service-global provenance risk and interrupt/resume missing identity.

**Why now:** Highest-severity invariant; implementation must be driven by explicit evidence.

**Scope:** `apps/api/internal/consultation/runtime_test.go`, relevant mocks, Python consultation thread tests.

**Out of scope:** Production fix.

**Protected contracts:** Existing run/interaction semantics.

**Implementation:**

1. Build a concurrent test harness around one shared `Runtime` instance.
2. Feed distinct `runtime.agent_configuration` events into two logical runs.
3. Assert each finished Run receives only its own identity; current implementation should expose the defect or race.
4. Add interrupted-run test proving current Python path returns before runtime identity event.
5. Add resume test proving current resume path omits runtime identity.
6. Add `go test -race` target to evidence report.

**Tests:** focused Go + Python runtime tests.

**Acceptance:** Defect is reproducible/characterized without changing production behavior.

**Dependencies:** none.

**Risks:** avoid a timing-flaky concurrency test; coordinate channels/barriers deterministically.

---

## BS-HARD-002 — Replace shared provenance fields with run-local execution identity

**Goal:** `consultation.Runtime` contains no mutable per-run provenance.

**Why now:** Fixes direct cross-run state ownership violation.

**Scope:** `apps/api/internal/consultation/runtime.go`, tests.

**Implementation:**

1. Add a typed run-local execution identity structure.
2. Add it to `streamResult` or another request-local execution object.
3. Capture `runtime.agent_configuration` into that local object.
4. Remove `pendingAgentConfigurationID`, `pendingAgentConfiguration`, `pendingExecutionProvenance` from `Runtime`.
5. Make completion/rollout paths consume the local identity only.
6. Ensure local value initializes empty for every run/resume request.

**Tests:** concurrency test + race detector.

**Acceptance:** no provenance mutable state exists on service singleton; concurrent identity test passes.

**Dependencies:** BS-HARD-001.

**Risks:** incomplete migration could leave rollout path reading old state; grep must prove old fields are gone.

---

## BS-HARD-003 — Add early Consultation configuration handshake and fail-closed validation

**Goal:** Go validates exact Agent identity before accepting ordinary semantic output.

**Scope:** Python `consultation_thread.py`, Go `runtime.go`, deployment registration helpers, tests.

**Implementation:**

1. Emit `runtime.agent_configuration` before LangGraph semantic streaming begins.
2. Include manifest provenance and execution-runtime identity available at start.
3. In Go, validate selected configuration ID, role, policy revision and logical model.
4. Persist validated identity onto active Run immediately.
5. Reject missing/mismatched handshake via failed-run path.
6. Keep actual usage/provider observations additive; update execution provenance at completion if needed without changing immutable configuration identity.

**Tests:** mismatch, missing handshake, interrupted run provenance.

**Acceptance:** an invalid Python runtime identity cannot produce an authorized ordinary reply.

**Dependencies:** BS-HARD-002.

**Risks:** execution provenance fields only known at end; separate immutable config handshake from completion-time observed usage instead of delaying identity validation.

---

## BS-HARD-004 — Pin HITL resume to the interrupted run configuration

**Goal:** a resumed thread continues with the same immutable configuration identity.

**Scope:** `ResumeConsultationInterruptRequest`, Go resume orchestration, Python resume endpoint/thread runtime, RunService accessors/tests.

**Implementation:**

1. Ensure interrupted Run already has configuration identity from early handshake.
2. Load source Run by `interaction.RunID` with user authorization.
3. Add `configuration_id` to resume request.
4. Python compares requested ID with checkpointed `consultation_manifest` identity.
5. Fail closed on mismatch/missing historical identity.
6. Emit early runtime identity for the resumed Go Run and persist it.

**Tests:** champion changes between interrupt and resume; resume must still use source config.

**Acceptance:** config promotion during a waiting-user interval cannot silently change the in-flight logical Agent thread.

**Dependencies:** BS-HARD-003.

**Risks:** legacy interrupted rows without provenance need explicit non-resumable behavior or a carefully scoped compatibility rule; do not infer current champion.

---

## BS-HARD-010 — Create one runtime public StreamEvent parser

**Goal:** both Web live and durable event paths consume validated `StreamEvent` objects.

**Scope:** `packages/contracts`, stream schema/fixtures, package tests.

**Implementation:**

1. Strengthen `stream-event.v1.schema.json` with event-type-specific payload constraints.
2. Add runtime parser API in `@bodysense/contracts`.
3. Add positive/negative fixture suite.
4. Keep internal `runtime` channel outside public schema.
5. Export parser/error types through package index.

**Tests:** contracts unit/typecheck + fixture parity.

**Acceptance:** invalid public event cannot be returned as `StreamEvent` without validation.

**Dependencies:** none after Phase 1 can run in parallel if separate branch.

**Risks:** schema/type drift; fixture tests must prove TS/Python/Go vocabulary stays aligned.

---

## BS-HARD-011 — Enforce parser at live SSE and durable recovery boundaries

**Goal:** remove `JSON.parse(...) as StreamEvent` and durable event casting.

**Scope:** `useSSEProcessor.ts`, `consultationService.ts`, tests.

**Implementation:**

1. Parse JSON to `unknown`.
2. Call shared contract parser.
3. Route parse failures to explicit protocol error/recovery handling.
4. Ensure malformed durable rows do not enter `dispatchReplayEvents`.
5. Remove redundant event-level casts made unnecessary by validation.

**Tests:** malformed live frame, malformed durable page, valid event regression.

**Acceptance:** grep finds no network-boundary `as StreamEvent` bypass in Consultation feature.

**Dependencies:** BS-HARD-010.

**Risks:** avoid treating an unknown future v2 event as silently safe; version mismatch must be explicit.

---

## BS-HARD-012 — Strictly validate Go ← Python consultation events

**Goal:** malformed internal NDJSON is a visible run failure, not a silently skipped event.

**Scope:** `ai_client.go`, `dto`, consultation runtime tests.

**Implementation:**

1. Replace malformed-line `continue` with channel error propagation.
2. Validate internal envelope/channel/type.
3. Validate payload before authority-relevant processing.
4. Stop ignoring `PayloadAs` errors in safety/state/control events.
5. Persist/emit sanitized protocol failure.

**Tests:** malformed JSON, unknown type, malformed red flag, malformed configuration handshake.

**Acceptance:** first invalid internal protocol event deterministically fails the run.

**Dependencies:** BS-HARD-003 for configuration handshake semantics.

**Risks:** distinguish forward-compatible optional events from corrupt required events; define supported vocabulary explicitly.

---

## BS-HARD-020 — Formalize transport-disconnect semantics and explicit run cancellation

**Goal:** one unambiguous lifecycle contract.

**Scope:** Go runtime/controller/service/handler, Web action if UI exposes Stop, contracts/docs/tests.

**Implementation:**

1. Characterize current detached execution behavior.
2. Introduce run-scoped cancellation registry.
3. Add authorized/idempotent cancel command.
4. Persist terminal cancellation state and event.
5. Cancel pending interaction when relevant.
6. Clear active run exactly once.
7. Teach Web recovery path the terminal cancellation event.
8. Keep process-crash recovery explicitly out of scope.

**Tests:** disconnect != cancel, explicit cancel, double cancel, cancel-vs-complete race.

**Acceptance:** transport and business cancellation are no longer conflated.

**Dependencies:** Stream contracts from Phase 2 if adding `run.cancelled`.

**Risks:** cancellation race with completion; use atomic durable transition / compare expected state.

---

## BS-HARD-030 — Canonicalize out-of-band RuntimeEvent sequencing

**Goal:** every durable public event for a run has one clear sequence owner.

**Scope:** RuntimeEvent repository/service/migration only if necessary, expiry worker tests.

**Implementation:**

1. Add transactional next-sequence allocation for out-of-band events.
2. Replace timestamp-derived sequence in `RecordInteractionExpired`.
3. Assert `(run_id, seq)` uniqueness and monotonicity.
4. Document ordering semantics.

**Tests:** concurrent out-of-band allocation, expiry after interrupted run events.

**Acceptance:** no time/random-derived public sequence remains.

**Dependencies:** Phase 2 contract semantics.

**Risks:** do not introduce a heavyweight global event-store abstraction; solve per-run ordering only.

---

## BS-HARD-031 — Make ActiveTurn projection run-aware and deterministic

**Goal:** identical event history produces identical state.

**Scope:** reducer/selectors/tests/event payload.

**Implementation:**

1. Replace `lastSeqByType` workaround with run-aware sequence tracking.
2. Reset/transition sequence state explicitly when run identity changes.
3. Add durable interaction timestamps to server event or consume persisted interaction timestamp.
4. Remove `new Date()` from reducer state construction.
5. Remove debug logging from pure reducer or gate it outside reducer.
6. Add live+replay equivalence test.

**Acceptance:** deep-equal deterministic replay test passes.

**Dependencies:** BS-HARD-030.

**Risks:** resume creates a new Go Run while continuing one LangGraph thread; tests must distinguish thread identity from public run identity.

---

## BS-HARD-040 — Move KnowledgeLibrary to async connection pooling

**Goal:** no sync Postgres I/O on AI-service event loop.

**Scope:** `knowledge_library.py`, FastAPI lifespan, tests.

**Implementation:**

1. Introduce async pool initialization/shutdown.
2. Register pgvector asynchronously.
3. Convert search/list/stats to async cursors.
4. Convert ingestion transaction to async transaction.
5. Make library instance lifecycle-owned rather than an unmanaged sync connection singleton.
6. Add bounded connection startup failure.

**Tests:** pool lifecycle, concurrency, transaction rollback.

**Acceptance:** production interactive RAG path contains no `psycopg.Connection` / sync cursor execution.

**Dependencies:** none on L3; can execute after P0 repair in a separate worktree.

**Risks:** route tests may instantiate library without app lifespan; provide explicit test factory/injection rather than hidden auto-open behavior.

---

## BS-HARD-041 — Offload local embedding execution from event loop

**Goal:** local transformer computation cannot stall unrelated async requests.

**Scope:** `embedding.py`, tests.

**Implementation:**

1. Wrap local transformer encode in bounded blocking executor/to_thread adapter.
2. Add concurrency semaphore/limit.
3. Make lazy model initialization safe under concurrent first use.
4. Preserve remote AsyncOpenAI path unchanged.
5. Benchmark hashing path; offload only if threshold justifies it.

**Tests:** heartbeat responsiveness + dimensional parity.

**Acceptance:** blocking encode runs on non-event-loop thread.

**Dependencies:** can pair with BS-HARD-040.

**Risks:** too many executor threads can hurt CPU/GPU performance; cap concurrency.

---

## BS-HARD-050 — Build Treatment/RAG Grounding Eval v2

**Goal:** evaluate material intervention claims against admissible evidence, not only title substring overlap.

**Scope:** faithfulness checker/evals/test data; no immediate production gate change.

**Implementation:**

1. Define structured `InterventionClaim` and result/reason codes.
2. Add deterministic evidence-ID/provenance checks.
3. Add semantic support stage for claim↔evidence.
4. Add optional Judge only for uncertain cases.
5. Build disagreement dataset against v1.
6. Review false positives/negatives.
7. Decide separately whether v2 becomes qualification-only or production governance.

**Tests:** required dataset slices in §9.5.

**Acceptance:** v2 explains why a claim is unsupported/partial/supported and catches dosage/support mismatches that v1 cannot.

**Dependencies:** async embedding boundary preferred before embedding-based evaluator.

**Risks:** Judge nondeterminism must never replace deterministic evidence-policy invariants.

---

## BS-HARD-060 — Synchronize authoritative architecture and close the plan

**Goal:** docs, tests and current implementation tell one story.

**Scope:** current architecture docs, learning roadmap references if necessary, release validation.

**Implementation:**

1. Update `current-longitudinal-system.md` runtime contract.
2. Update role-governance docs with early configuration handshake / run-local provenance.
3. Document public event `(run_id, seq)` semantics.
4. Document async KnowledgeLibrary lifecycle.
5. Run full release gate.
6. Perform batch review against every finding.
7. Move this plan to archive only after closure evidence is recorded.

**Acceptance:** no open P0/P1 findings; deferred P2 items have explicit reason/evidence.

**Dependencies:** all required repair tickets.

---

# 13. Verification matrix

| Invariant | Focused evidence |
| --- | --- |
| one Run has one exact configuration identity | concurrent Go runtime tests + race detector |
| interrupted/resumed thread keeps same configuration | Go/Python HITL integration tests |
| identity mismatch fails closed | Consultation runtime negative test |
| malformed Web event never reaches reducer | contracts + SSE parser tests |
| malformed Python event never silently disappears | AIClient/runtime protocol tests |
| disconnect is not implicit cancel | test writer / request cancellation test |
| explicit cancel is terminal and idempotent | Go service/runtime tests |
| public `(run_id, seq)` is monotonic | repository/service sequence tests |
| replay projection is deterministic | reducer deep-equality replay test |
| DB search does not block event loop | async heartbeat/concurrency test |
| local encode does not block event loop | thread identity/heartbeat test |
| grounding eval catches unsupported material claims | v2 evaluator dataset |
| no regression to longitudinal flow | `pnpm validate:local-deploy` |

---

# 14. Risk and decision ledger

## R1 — Do not accidentally reopen the whole Agent Platform migration

This plan is hardening only. Diagnosis/Treatment ownership and LiteLLM convergence are protected contracts.

## R2 — Resume identity semantics must be explicit

Recommended decision: a HITL resume continues the configuration pinned by the interrupted thread, even if Champion changes while waiting. Switching configuration mid-checkpoint is a new run/re-analysis concept, not resume.

## R3 — Disconnect resilience is not crash recovery

Detaching from HTTP request cancellation is useful, but it does not make the producer durable across process restart. Do not add worker/job complexity in this plan unless product requirements explicitly demand crash-resumable inference.

## R4 — Contract validator source of truth

Prefer the existing `packages/contracts` schema/fixtures as the public wire authority. Avoid three unrelated handwritten validators drifting independently.

## R5 — Event schema versioning

If `run.cancelled` or interaction timestamps require public contract expansion, either make backward-compatible v1 additions where safe or deliberately introduce v2. Do not silently break older clients.

## R6 — Async pool migration

Knowledge ingestion is transactional; converting to async must preserve atomic source/segment/unit/clip persistence and existing publication visibility filters.

## R7 — Grounding Judge scope

LLM Judge belongs only after deterministic policy/provenance checks. It may grade semantic support; it may not authorize user facts or override hard safety/evidence constraints.

---

# 15. Batch review / repair closure protocol

After each phase:

1. run focused tests;
2. inspect actual diff against the protected contracts;
3. run adjacent tests for the same ownership boundary;
4. classify each finding as `fixed`, `partially fixed`, `not reproduced`, `deferred with reason`, or `new regression`;
5. do not close a finding because the build is green if its behavioral acceptance evidence is missing.

Final review must explicitly re-check:

```text
Input/context
→ Agent configuration identity
→ Python runtime/checkpoint
→ tool / interrupt
→ internal event validation
→ Go public event sequencing/persistence
→ SSE/replay parser
→ React projection
→ cancellation/recovery
→ RAG async resource boundaries
→ Eval/grounding
```

This plan is complete only when the remaining architecture matches the same North-Star principle learned in L1:

> **Keep nondeterministic semantic execution inside explicit, deterministic, run-scoped ownership and contract boundaries.**
