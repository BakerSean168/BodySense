# Final Agent Runtime Architecture Proposal
> ✅ **已完成并归档**（2026-07-29）。实施落地见对应代码与测试；本文件移入 archive 仅作历史记录。


**Status**: Proposed final state  
**Audience**: Go API, Python AI runtime, Web consultation workbench  
**Decision anchor**: [ADR 0002](../adr/0002-agent-runtime-ownership.md)

## 1. Goal

Replace the current mixed runtime design with a fully engineered final state that:

- uses **LangGraph as the real Agent Runtime**
- uses **assistant-ui as the real thread rendering runtime**
- uses **Go as the durable Runtime Event Log and projection owner**
- deletes text-only history reconstruction, synthetic resume messages, and duplicated runtime semantics

There is no compatibility mode in this proposal. The target is the end-state architecture only.

## 2. Final ownership model

### 2.1 Python Agent Runtime Module

Python owns the **Agent Thread**.

It is the single owner of:

- LangGraph state
- message state used by the LLM
- tool-call sequencing
- checkpoint persistence
- interrupt semantics
- resume semantics
- thread-level runtime identity

Python does **not** own:

- user auth
- conversation ownership
- durable business records
- public SSE delivery
- UI projections

### 2.2 Go Runtime Ledger Module

Go owns the **Runtime Event Log** and all durable business truth.

It is the single owner of:

- conversations
- runs
- event persistence
- projection persistence
- user interaction ownership checks
- public stream delivery
- idempotency
- audit/replay

Go does **not** own:

- LangGraph message state
- tool-loop control
- LLM protocol reconstruction
- runtime resume semantics

### 2.3 Web Projection Module

Web is a pure projection consumer and intent emitter.

It owns:

- assistant-ui thread rendering
- ask-user cards and future interrupt cards
- local optimistic interaction states
- intent submission

Web does **not** own:

- chat history reconstruction
- resume graph behavior
- tool/result pairing logic beyond presentation needs

## 3. Final system architecture

```txt
Web (assistant-ui)
  -> Go API
      -> Runtime Ledger Module
      -> Projection Module
      -> Stream Contract Module
      -> Python Agent Runtime API
          -> LangGraph Runtime
          -> Checkpointer
          -> Tool Registry / Tool Executor
          -> Model Provider Adapter
```

### 3.1 Final request flows

#### Start turn

```txt
Web submit_user_message
  -> Go creates run
  -> Go appends run.started
  -> Go calls Python start_turn(thread_id, input, run_context)
  -> Python runs LangGraph
  -> Python emits runtime events
  -> Go appends runtime events to Runtime Event Log
  -> Go updates projections
  -> Go streams public events to Web
```

#### Resume interrupt

```txt
Web submit_interrupt_answer
  -> Go validates conversation + interrupt ownership
  -> Go appends interrupt.answered
  -> Go creates resumed run
  -> Go calls Python resume_interrupt(thread_id, interrupt_id, answer, run_context)
  -> Python resumes from checkpoint
  -> Python emits runtime events
  -> Go appends runtime events
  -> Go updates projections
  -> Go streams public events to Web
```

## 4. Final data model

### 4.1 Durable tables in Go

The durable truth becomes:

- `conversations`
- `runs`
- `runtime_events`
- `runtime_interrupts`
- `thread_projections`
- `thread_projection_messages`
- `thread_projection_tool_calls`

`messages` is no longer the runtime truth source. It may survive only as a projection table or be replaced entirely by thread projection tables.

### 4.2 Runtime Event Log

Every user-visible or resume-relevant runtime state change is recorded as an append-only event:

- `run.started`
- `run.completed`
- `run.failed`
- `run.interrupted`
- `assistant.message.started`
- `assistant.text.delta`
- `assistant.tool_call.created`
- `assistant.tool_result.created`
- `interrupt.required`
- `interrupt.answered`
- `projection.message.materialized`

This log is the durable replay surface for Go.

### 4.3 Agent Thread checkpoint

Python owns the checkpoint payload. Go stores only a checkpoint reference or opaque blob metadata when needed.

Preferred final model:

- checkpoint storage lives behind LangGraph checkpointer
- Go stores `thread_id`, `latest_checkpoint_id`, and interrupt metadata
- Go never parses checkpoint internals unless there is an explicit operational need

## 5. Final protocol model

### 5.1 LLM protocol truth

The LLM protocol model is no longer mirrored manually in Go.

Canonical ownership:

- Python owns `HumanMessage`, `AIMessage`, `ToolMessage`
- Python owns `AIMessage.tool_calls`
- Python owns `ToolMessage.tool_call_id`

Go never rebuilds these objects from UI transcript text.

### 5.2 Public stream contract truth

Public event truth remains in `packages/contracts`.

The public contract should be expanded to fully reflect run and interrupt lifecycle:

- `run.started`
- `run.interrupted`
- `run.resumed`
- `run.completed`
- `run.failed`
- `message.created`
- `message.text.delta`
- `message.completed`
- `tool.call`
- `tool.result`
- `interrupt.required`
- `interrupt.answered`

These are UI-facing projections of the Runtime Event Log, not raw LangGraph internals.

## 6. Final Module decomposition

### 6.1 Go final Modules

```txt
apps/api/internal/runtime_ledger/
  runtime_event.go
  runtime_event_repository.go
  runtime_event_service.go
  run_service.go
  interrupt_service.go
  projection_service.go
  projection_repository.go
  replay_service.go

apps/api/internal/agent_runtime_client/
  client.go
  dto.go
  stream_decoder.go

apps/api/internal/stream_contract/
  mapper.go
  validator.go
  writer.go

apps/api/internal/consultation_runtime/
  send_message_handler.go
  resume_interrupt_handler.go
  thread_query_handler.go
```

#### Go Modules to delete

Delete entire shallow Modules and paths that exist only because Go was partially acting as an agent runtime:

- `apps/api/internal/context/context_builder.go`
- text-only chat context builder tests and seams
- current consultation chat runtime that stores assistant parts while reconstructing protocol semantics inline
- current frontend-driven interaction resume semantics

Specifically delete or collapse these responsibilities:

- `BuildChatContext`
- `getMessageTextContent`
- text-only `service.ChatMessage`
- manual synthetic tool/result pairing as runtime truth

### 6.2 Python final Modules

```txt
apps/ai-service/src/runtime/
  thread_runtime.py
  runtime_api.py
  event_mapper.py
  checkpointing.py
  interrupts.py
  thread_state.py

apps/ai-service/src/runtime/graphs/
  consultation_graph.py
  nodes/
    safety_check.py
    classify_intent.py
    generate_response.py
    decide_phase.py
    generate_diagnosis.py
    generate_treatment.py

apps/ai-service/src/runtime/tools/
  registry.py
  executor.py
  policies.py
  types.py
  handlers/
    ask_user.py
    search_knowledge.py
    extract_symptom_info.py
    save_extracted_info.py
    finish_consultation.py

apps/ai-service/src/runtime/providers/
  openai_compatible.py
```

#### Python final expectations

- LangGraph uses a real checkpointer
- `interrupt()` is used for HITL pauses
- `Command(resume=...)` is used for answer submission
- one thread id maps to one durable Agent Thread

#### Python Modules to delete

Delete partial-runtime seams that only exist because the graph is currently treated as a request-time helper:

- ad-hoc `/api/chat/resume` shape that rebuilds messages manually
- duplicate orchestration layers that only convert text history into `ChatMessage`
- request-time graph state assembly that pretends to be resume without checkpoint ownership

### 6.3 Web final Modules

```txt
apps/web/src/features/consultation/runtime/
  threadProjectionAdapter.ts
  eventReducer.ts
  interruptReducer.ts
  streamClient.ts

apps/web/src/features/consultation/components/
  AssistantThread.tsx
  InterruptCard.tsx
  AskUserCard.tsx
  ToolTimeline.tsx
  CitationPanel.tsx

apps/web/src/features/consultation/services/
  consultationRuntimeApi.ts
```

#### Web final expectations

- assistant-ui is the primary thread/message abstraction
- active turn state is derived from public runtime events and projections
- interrupt answering is a first-class intent, not a synthetic chat send

#### Web Modules to delete

Delete custom behavior that exists only to compensate for missing backend runtime semantics:

- automatic "interaction answer -> send new user message"
- hidden interaction-answer messages
- resume markers in message metadata
- thread reconstruction logic that depends on `assistant` message status hacks like `aborted` to recover interrupts

## 7. Final API design

### 7.1 Go public API

```txt
POST /api/v1/consultations/:id/messages
POST /api/v1/consultations/:id/interrupts/:interruptId/answers
GET  /api/v1/consultations/:id/thread
GET  /api/v1/consultations/:id/runtime-events?after_seq=
```

### 7.2 Go -> Python runtime API

```txt
POST /runtime/threads/:thread_id/turns
POST /runtime/threads/:thread_id/interrupts/:interrupt_id/resume
GET  /runtime/threads/:thread_id
```

Start turn request:

```json
{
  "run_id": "uuid",
  "conversation_id": "uuid",
  "user_id": "uuid",
  "input": {
    "type": "user_message",
    "text": "..."
  },
  "business_context": {
    "profile": {},
    "consultation_snapshot": {}
  }
}
```

Resume interrupt request:

```json
{
  "run_id": "uuid",
  "conversation_id": "uuid",
  "user_id": "uuid",
  "interrupt_id": "uuid",
  "answer": {
    "text": "...",
    "selected": ["..."]
  },
  "business_context": {
    "profile": {},
    "consultation_snapshot": {}
  }
}
```

## 8. Final framework usage policy

### 8.1 LangGraph

Use LangGraph fully for:

- thread state
- checkpointing
- interrupt/resume
- message accumulation
- graph execution

Do **not**:

- rebuild synthetic graph state in Go
- treat LangGraph as just a nicer function wrapper
- manually emulate checkpoint/resume while importing LangGraph

### 8.2 assistant-ui

Use assistant-ui fully for:

- thread rendering
- assistant/user message abstraction
- structured assistant message parts
- runtime-driven streaming updates

Do **not**:

- maintain a parallel, custom chat runtime abstraction that reimplements assistant-ui concepts
- hide fake messages to compensate for backend protocol gaps

## 9. Engineering standards for the final state

### 9.1 Strong seams

Every major Module must have:

- one small Interface
- multiple tests at that Interface
- no leakage of provider-private protocol across seams

### 9.2 Projection discipline

UI reads from projections, not raw runtime internals.

Examples:

- thread page reads `thread projection`
- interrupt sidebar reads `pending interrupt projection`
- analytics reads runtime event aggregates

### 9.3 No duplicated ownership

For every concept, there must be exactly one owner:

- Agent Thread -> Python
- Runtime Event Log -> Go
- Public Stream Contract -> contracts package
- Thread rendering -> Web assistant-ui layer

### 9.4 Delete shallow code aggressively

If a Module only exists to translate one shallow representation into another because ownership is split, delete it rather than preserve it.

## 10. Final delete list

The following design patterns are explicitly forbidden in the end state:

- text-only `ChatMessage` between Go and Python
- Go-side reconstruction of LLM protocol messages from UI transcript text
- frontend-triggered synthetic follow-up user messages for interrupt answers
- storing runtime truth only in assistant message `parts`
- using assistant message status hacks to recover pending interrupts
- partial LangGraph usage without checkpoint ownership
- custom chat abstractions that duplicate assistant-ui runtime semantics

## 11. Suggested migration cut shape

Although this document does not define intermediate compatibility states, the final cut should be executed as one coherent refactor across these surfaces together:

- Go runtime ledger + public API
- Python runtime API + LangGraph checkpoint semantics
- contracts expansion
- Web thread projection + interrupt answer flow

If cut piecemeal, ownership drift will reappear.

## 12. Success criteria

The final architecture is successful when all of the following are true:

1. A pending `ask_user` answer resumes the same Agent Thread without creating a synthetic user turn.
2. Go can rebuild the full UI thread entirely from projections and Runtime Event Log without reading LangGraph internals.
3. Python can resume an interrupted consultation from checkpoint without receiving reconstructed text history from Go.
4. Web can refresh at any point and re-render the exact pending interrupt and tool timeline from projections.
5. No module outside Python needs to understand `AIMessage.tool_calls` or `ToolMessage.tool_call_id` in order to keep runtime correctness.
