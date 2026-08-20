# Treatment DecisionTrace Archive

Completed 2026-08-20.

This batch made Treatment generation/acceptance authority an explicit pure Go policy, hardened durable SafetyState parsing to fail closed, and persisted generation/acceptance DecisionTrace on immutable TreatmentRevision artifacts. Acceptance trace is written in the same transaction that accepts the revision and moves the current Treatment pointer.

- [Implementation plan](./treatment-decision-trace-2026-08-20.md)
- [Current Treatment Agent architecture](../../../architecture/treatment-agent-configuration.md)
