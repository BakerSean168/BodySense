# Diagnosis DecisionTrace and Provenance

> Status: Historical Phase-7 implementation checkpoint; durable DecisionTrace/provenance is implemented.

## Purpose

A DiagnosisAnalysis must be explainable after process restart without reconstructing mutable runtime state. `raw_output` remains the immutable source capture, but current code no longer depends on parsing it to recover configuration or execution identity.

Migration `000035_add_diagnosis_decision_trace` adds first-class immutable columns:

- `agent_configuration_id` — indexed immutable configuration identity;
- `agent_configuration` — prompt/schema/tool/evidence/governance/decision/model-group revisions;
- `execution_provenance` — runtime-observed model/provider adapter, PydanticAI run/conversation IDs and usage when a model executed, or an explicit bypass reason;
- `evidence_acquisition_trace` — EvidenceBudget plus typed EvidenceAttempt records and evidence IDs;
- `decision_trace` — the audit envelope that ties the exact BodyState revision, configuration, execution, evidence, Python governance evidence and Go authority decision together.

## DecisionTrace v1

```text
trace_revision: diagnosis-decision-trace-v1
body_state_revision
agent_configuration
authority_mode
execution_provenance
evidence_acquisition
agent_governance
decision_authority
```

`authority_mode` is explicit. A cumulative Phase-6 configuration records `go-decision-policy`; still-serving historical/pre-promotion configurations record `pre-envelope-compatibility`. This avoids retroactively pretending that Go made a decision it did not make.

## Execution provenance

PydanticAI exposes runtime-observed metadata on `AgentRunResult.response`; Diagnosis records only fields actually observed:

- logical model + model-group revision selected by BodySense;
- gateway-reported model name;
- PydanticAI provider adapter name;
- Agent run ID and conversation ID;
- request/input/output/total token usage.

No physical provider or fallback hop is invented when LiteLLM does not expose it through the OpenAI-compatible response. The schema is additive so richer gateway metadata can be persisted later when available.

Pre-agent safety gates explicitly record `status=bypassed` and a reason rather than fabricating model execution metadata.

## Historical data

Migration 35 backfills configuration/execution/evidence columns only from fields that already existed in historical immutable `raw_output`. It does not manufacture missing DecisionTrace facts. New writes construct DecisionTrace deterministically during `PersistAIResult`.
