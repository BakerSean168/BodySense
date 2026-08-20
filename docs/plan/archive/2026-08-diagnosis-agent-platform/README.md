# Diagnosis Agent Platform Refactor Archive

Completed: 2026-08-20

This archive contains the implementation program and deletion ledger for the
Diagnosis Agent platform refactor completed across Phases 3–10.

Completed platform capabilities include:

- immutable AgentConfiguration identity and Go-owned deployment selection;
- Pydantic Evals qualification, slices, critical gates, and paired non-inferiority;
- typed EvidenceGap / EvidenceBudget / EvidenceAttempt acquisition;
- deterministic Go DecisionAuthority / SafetyEnvelope;
- durable DecisionTrace and execution/configuration/evidence provenance;
- historical and counterfactual replay plus reviewed regression export;
- shadow / stable canary / promotion / rollback governance;
- one repository-wide LiteLLM logical-model routing topology;
- physical retirement of the legacy application-owned provider/router/fallback stack.

The immutable v1/v2/v3 Diagnosis Agent manifests remain repository-versioned for
Champion/rollback/replay and audit. Their continued existence is not a parallel
provider-routing architecture: all model execution now uses the same LiteLLM
gateway boundary.

Current architecture references:

- [`../../../architecture/model-gateway-routing.md`](../../../architecture/model-gateway-routing.md)
- [`../../../architecture/diagnosis-evidence-acquisition.md`](../../../architecture/diagnosis-evidence-acquisition.md)
- [`../../../architecture/diagnosis-decision-authority.md`](../../../architecture/diagnosis-decision-authority.md)
- [`../../../architecture/diagnosis-audit-provenance.md`](../../../architecture/diagnosis-audit-provenance.md)
- [`../../../architecture/diagnosis-replay-comparison.md`](../../../architecture/diagnosis-replay-comparison.md)
- [`../../../architecture/diagnosis-promotion-governance.md`](../../../architecture/diagnosis-promotion-governance.md)
