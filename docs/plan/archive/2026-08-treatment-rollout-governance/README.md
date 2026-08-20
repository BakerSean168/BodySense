# Treatment Rollout Governance Archive

Completed 2026-08-20.

This batch promoted Treatment v2 from qualification/replay-ready to **governed rollout-ready** without changing production serving. It added repository-versioned `treatment_promotion_v1`, Go-owned stable Champion/Challenger routing, read-only paired shadow/canary execution, anonymous durable rollout observations, deny-first stop/progression policy, an operator status command, and migration 42 rollout provenance.

Hermetic deployment proof served Treatment v1, executed v2 only as a read-only shadow, recorded three paired observations with zero blockers, and persisted zero v2 TreatmentRevisions. All committed Compose defaults remain v1 / `champion`.

- [Implementation plan](./treatment-rollout-governance-2026-08-20.md)
- [Treatment promotion governance](../../../architecture/treatment-promotion-governance.md)
- [Treatment Agent architecture](../../../architecture/treatment-agent-configuration.md)
