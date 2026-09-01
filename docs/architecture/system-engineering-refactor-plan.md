# System Engineering Refactor Plan — Historical Redirect

- Status: Historical plan; no longer a current implementation backlog
- Historical snapshot: [`docs/plan/archive/architecture-snapshots/2026-09-01/system-engineering-refactor-plan.md`](../plan/archive/architecture-snapshots/2026-09-01/system-engineering-refactor-plan.md)

The original system-engineering plan contained migration-era targets such as a Go `ContextBuilder`, linear `HealthJourneyWorkflow`, session-level health truth and a `MedicalRecord` terminal artifact. Those targets were superseded and should not remain mixed into current architecture documentation.

Current source-of-truth documents are:

- [Current Longitudinal System](./current-longitudinal-system.md)
- [Longitudinal BodyState Domain Model](./longitudinal-body-state-domain.md)
- [Longitudinal Health Loop](./longitudinal-health-loop.md)
- [Agent Platform Role Governance](./agent-platform-role-governance.md)
- [Model Gateway Routing](./model-gateway-routing.md)
- [Deployment Architecture](./deployment-architecture.md)
- [ADR 0002](../adr/0002-agent-runtime-ownership.md)
- [ADR 0004](../adr/0004-adopt-longitudinal-body-state-model.md)
- [ADR 0005](../adr/0005-adopt-standalone-litellm-model-gateway.md)
- [ADR 0007](../adr/0007-separate-stable-profile-from-longitudinal-lifestyle.md)
- [ADR 0009](../adr/0009-adopt-evidence-grounded-assessment-contract.md)

Remaining implementation work must be tracked in active plans, especially [`2026-09-01-documentation-code-alignment-audit.md`](../plan/active/2026-09-01-documentation-code-alignment-audit.md), rather than resurrecting this migration-era plan.
