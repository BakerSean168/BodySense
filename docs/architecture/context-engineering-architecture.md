# Context Engineering Architecture — Superseded Design Redirect

- Status: Superseded by ADR 0002
- Current runtime authority: [ADR 0002 — Agent Runtime Ownership](../adr/0002-agent-runtime-ownership.md)
- Historical snapshot: [`docs/plan/archive/architecture-snapshots/2026-09-01/context-engineering-architecture.md`](../plan/archive/architecture-snapshots/2026-09-01/context-engineering-architecture.md)

The former Go `ContextBuilder` architecture is no longer an implementation target and has been removed from the current architecture document set.

## Current ownership

```text
Python LangGraph checkpoint
  = Agent Thread runtime truth

Go Runtime Event Log + domain repositories
  = durable ledger / business truth

Web
  = projection consumer + intent producer
```

Go does not rebuild model history from a text-only `messages` projection. Web does not synthesize runtime resume semantics. Context needed for durable Diagnosis/Treatment decisions is reconstructed from explicit pinned domain inputs such as BodyState revisions and governed evidence, not from an implicit giant conversation bundle.

For the current system read:

- [Current Longitudinal System](./current-longitudinal-system.md)
- [Agent Platform Role Governance](./agent-platform-role-governance.md)
- [Longitudinal BodyState Domain Model](./longitudinal-body-state-domain.md)
- [ADR 0002](../adr/0002-agent-runtime-ownership.md)

The archived snapshot is retained only to explain the architectural transition and must not be used as a current implementation guide.
