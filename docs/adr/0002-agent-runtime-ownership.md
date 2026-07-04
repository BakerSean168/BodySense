# ADR 0002: Make Python the Agent Runtime Owner and Go the Durable Ledger Owner

## Status

Accepted

## Context

BodySense currently splits Agent runtime truth across three runtimes:

- Python owns LLM invocation, tool registration, and partial orchestration.
- Go owns conversations, runs, SSE forwarding, and partial tool/interruption state.
- Web owns additional resume semantics by turning answered interactions back into ordinary user messages.

This creates shallow seams around the most complex workflow in the system: tool-calling consultation turns with human interruption and resume. The same concept must currently be understood in `consultation_graph.py`, Go chat runtime/context building, and the React consultation runtime.

The result is low locality:

- tool-call protocol history is not durably represented in one place
- interrupt and resume semantics are split between Python, Go, and Web
- the Web invents runtime behavior by sending a follow-up user turn
- Go reconstructs LLM context from text projections rather than runtime truth

The project already uses LangGraph and assistant-ui, but only partially. The current architecture pays the framework complexity cost without receiving the full leverage of native checkpointed runtime state, interrupts, resumptions, and projection-driven UI.

## Decision

Adopt the following final ownership model:

- **Python is the owner of Agent Thread runtime truth.**
  - LangGraph is used as the real Agent Runtime, not only as a graph-shaped request handler.
  - Python owns checkpointed message state, tool-call sequencing, interrupt semantics, resume semantics, and thread-level runtime identity.
- **Go is the owner of durable business truth and Runtime Event Log truth.**
  - Go owns user auth, conversation ownership, runs, event persistence, projections, and public stream delivery.
  - Go does not reconstruct LLM runtime history from text-only messages.
- **Web is a projection consumer only.**
  - Web renders assistant-ui thread projections, pending interrupts, tool state, citations, and run lifecycle.
  - Web submits intents such as `submit_user_message` and `submit_interrupt_answer`.
  - Web does not invent resume semantics or emit synthetic follow-up chat turns.

This implies the following architectural commitments:

1. LangGraph checkpoint + interrupt + resume become first-class runtime features.
2. The durable audit model in Go becomes an append-only Runtime Event Log plus explicit projections.
3. Public SSE/Web stream events are derived from runtime events and remain the only frontend contract.
4. `messages` become a projection for UI and search, not the source of Agent runtime truth.
5. Existing text-only context rebuilding and frontend-driven resume flows are deleted rather than preserved.

## Consequences

### Positive

- The deepest complexity sits behind one real seam: the Python Agent Runtime Module.
- Interrupt/resume becomes native behavior instead of simulated behavior.
- Go gains leverage as a durable ledger and projection runtime rather than a partial agent runtime.
- Web becomes simpler, more deterministic, and easier to test because it only consumes projections and emits intents.
- LangGraph and assistant-ui are used fully rather than being surrounded by custom replacement logic.

### Negative

- This is a non-incremental architecture change and will require deletion of current chat/runtime/context plumbing.
- New runtime APIs, event persistence, and projections must be designed together before code movement starts.
- The existing documentation around tool runtime and stream runtime must be updated to reflect final ownership instead of transitional ownership.

### Follow-up

- Define the final Agent Runtime API between Go and Python.
- Define the final Runtime Event Log schema and projection schema.
- Replace current consultation chat runtime, context builder, and interaction resume implementation with the final model.
