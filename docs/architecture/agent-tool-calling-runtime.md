# Agent Tool Calling Runtime — Current Ownership

- Status: Current Consultation tool/interrupt architecture
- Updated: 2026-09-01
- Supersedes: the old Go-centered `ChatHandler`/resume design
- Historical snapshot: [`docs/plan/archive/architecture-snapshots/2026-09-01/agent-tool-calling-runtime.md`](../plan/archive/architecture-snapshots/2026-09-01/agent-tool-calling-runtime.md)
- Authority: [ADR 0002 — Agent Runtime Ownership](../adr/0002-agent-runtime-ownership.md)

## 1. Ownership

```text
Python LangGraph Agent Runtime
  -> model/tool protocol history
  -> tool invocation sequencing
  -> checkpoint / interrupt / resume semantics

Go
  -> auth and conversation/run ownership
  -> Agent configuration handshake validation
  -> durable Runtime Event Log
  -> tool-call audit persistence
  -> AgentInteraction persistence / answer command
  -> domain side-effect authority

Web
  -> render tool/interaction projections
  -> submit user intent/interaction answer
```

Web and Go do not reconstruct the LLM tool protocol from text messages.

## 2. Current implemented pieces

| Capability                                                                        | Status |
| --------------------------------------------------------------------------------- | ------ |
| LangGraph tool execution/checkpoint semantics                                     | ✅     |
| `ask_user` interaction contract                                                   | ✅     |
| durable `agent_interactions`                                                      | ✅     |
| resume through `POST /api/v1/consultations/:id/interrupts/:interactionId/answers` | ✅     |
| durable `agent_tool_calls` audit model/repository/service                         | ✅     |
| public tool call/result StreamEvents                                              | ✅     |
| `AskUserCard` and interrupted/resuming UI                                         | ✅     |
| durable recovery/replay of interaction/tool state                                 | ✅     |
| BodyState write authority outside the model                                       | ✅     |

The old `POST /api/v1/chat/send`, `/api/chat/resume`, Go `ToolExecutor` ownership and migration-era AskUserCard/tool-audit phase checklist are historical and no longer belong in the current architecture.

## 3. `ask_user` lifecycle

```text
Agent decides more user evidence is needed
  -> Python creates interrupt/checkpoint semantics
  -> internal state.interaction.required
  -> Go validates/persists interaction + public event
  -> Run becomes interrupted/waiting
  -> Web renders AskUserCard
  -> user answers explicit interaction endpoint
  -> Go validates ownership/interaction state
  -> Python resumes the same checkpointed thread/config identity
  -> new Run/public stream continues
```

An answer is resume input, not an ordinary synthetic chat message.

## 4. Tool-call audit vs business authority

`agent_tool_calls` records what the Agent attempted and what result was observed. It does **not** grant the model arbitrary domain mutation authority.

High-impact domain changes still pass Go-owned application/domain boundaries. For example, model-authored lifestyle extraction is persisted as an unverified/excluded candidate until the appropriate review/acceptance path authorizes it.

```text
Tool proposal/execution trace
  != durable health truth authority
```

## 5. Public events

Tool UI consumes versioned public events such as `tool.call`, `tool.result` and `state.interaction.required`. Internal provider or LangGraph events are mapped/validated before they become public StreamEvents.

See [StreamEvent Contract Runtime](./stream-event-contract-runtime.md).

## 6. Adding a new tool

A new tool should declare:

- input schema and runtime validation;
- whether it is read-only, evidence acquisition, user interrupt, or domain mutation;
- idempotency/side-effect semantics;
- which runtime owns execution;
- which durable audit/provenance is needed;
- whether a human/authority gate is required before a real-world/domain side effect;
- public projection events/UI if user-visible.

Do not reintroduce a second Go-side Agent orchestrator merely to add a tool; keep Agent Thread protocol ownership in Python and durable business authority in Go.
