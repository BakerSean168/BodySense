# Treatment Replay / Comparison Archive

Completed 2026-08-20.

This batch added private frozen Treatment Agent inputs, deterministic historical replay, read-only counterfactual replay across immutable Treatment configurations, layered hard/semantic/presentation comparison, privacy-sanitized regression export/import, and prod-like validation that replay remains side-effect free before later acceptance/Training/Outcome transitions.

Treatment v1 remains the production default. Treatment v2 is replay-ready and qualification-ready, but is still only a Challenger; production rollout requires a separate governed shadow/canary/promotion policy.

- [Implementation plan](./treatment-replay-comparison-2026-08-20.md)
- [Current Treatment Agent architecture](../../../architecture/treatment-agent-configuration.md)
