# ADR 0001: Deepen Runtime Modules Around Streaming AI Workflows

## Status

Accepted — runtime-module principle retained; Agent runtime ownership details superseded by ADR 0002

## Context

BodySense uses React, Go, and Python as separate runtimes. At the time of this decision the system had ContextBuilder, StreamRuntime, ToolRuntime, JobRuntime, and KnowledgeLifecycle concepts, but orchestration details were still spread across large caller modules. ADR 0002 later replaced the transitional Go/Python Agent-runtime ownership split while preserving the broader principle of deep runtime modules.

## Decision

Keep the three-service architecture and deepen runtime Modules instead of replacing the stack with a new agent framework.

- Go remains the source of truth for conversations, messages, runs, jobs, and persisted user state.
- Python remains the source of truth for AI reasoning, RAG, and tool execution.
- `packages/contracts` is the shared stream-event interface for Web, Go, and Python.
- Chat and agent workflows should expose small runtime interfaces while keeping event mapping, persistence side effects, and recovery logic local to their implementations.

## Consequences

- Handlers and graph nodes should adapt inputs and delegate orchestration to runtime Modules.
- New stream events require contract fixtures or equivalent parity tests across affected runtimes.
- Durable background work must be recoverable through JobRuntime rather than only attached to request-time goroutines.
