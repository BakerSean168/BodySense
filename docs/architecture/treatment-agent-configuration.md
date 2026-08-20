# Treatment Agent Configuration and Qualification

Status: current implementation (2026-08-20)

## Purpose

Treatment already had the durable business boundary before this platform pass: a Python Agent may propose an intervention plan, but Go owns Treatment/TreatmentRevision/Intervention identity, candidate-assessment readiness, safety/freshness gates, acceptance, and the current Treatment pointer. This document adds the missing execution identity around that existing lifecycle.

## Execution path

```text
Go TreatmentService
  | select repository-known Treatment Agent configuration
  | pin BodyState revision + DiagnosisAnalysis + candidate assessments
  v
Python TreatmentAgentService
  | resolve exact immutable manifest
  | build exact PydanticAI Agent revisions
  v
LiteLLM logical model: bodysense-structured
  v
physical provider/model
```

The initial immutable configuration is `treat-config-85718f8e90ac9d80`, derived from `config/agents/treatment-v1.yaml`. Runtime hostnames and credentials are excluded from the behavior fingerprint; prompt, schema, tool, evidence, governance, decision-policy, model-group, logical-model and generation revisions are included.

## Ownership and authority

Go owns the mutable deployment pointer `TREATMENT_AGENT_CONFIGURATION_ID`. Python cannot choose a different configuration for a request. The request carries the exact `configuration_id`, Python resolves only repository manifests, and the response returns `agent_configuration` plus `execution_provenance`.

Before persistence, Go fails closed unless all of these agree with its registration:

- configuration ID;
- role = `treatment`;
- decision policy revision = `treatment-go-acceptance-v1`;
- execution status = `executed`;
- runtime = `pydantic-ai`;
- logical model = `bodysense-structured`.

The AI result remains a proposal even after those checks. Existing deterministic Treatment gates still decide whether generation is allowed, and explicit Go acceptance still decides whether the revision becomes current.

## Durable provenance

Migration `000038_add_treatment_agent_provenance` adds the following to immutable `treatment_revisions`:

```text
agent_configuration_id
agent_configuration
execution_provenance
```

Together with the existing `source_body_state_revision` and `source_diagnosis_analysis_id`, a historical proposal can now answer both “which durable health inputs produced this?” and “which Agent behavior/runtime produced this?”.

## Qualification baseline

`treatment_qualification_v1` is the first deterministic Pydantic Evals qualification suite. It contains four cases across development, holdout, regression and challenge splits and checks:

- typed proposal contract;
- proposal-only/no durable side-effect surface;
- exact Agent configuration and execution provenance;
- exact BodyState revision in Agent context;
- confirmed/unsure candidate-assessment context;
- selected user constraints and existing evidence reaching the run context;
- expected Treatment tool surface with no artificial tool calls.

The baseline passes 4/4 for `treat-config-85718f8e90ac9d80`. This is a platform qualification baseline, not proof of clinical efficacy or semantic superiority of the intervention content.

## EvidenceGap v2 Challenger

Treatment v1 remains immutable and is still the default Go deployment pointer. The additive Challenger `treat-config-f68eec9846664596` changes only the prompt/tool/evidence revisions to `treatment-prompt-v2-evidence-gap`, `treatment-tools-v2`, and `treatment-evidence-gap-v2`. Its logical model, output schema, governance policy and Go acceptance decision policy remain unchanged.

v1 exposes the historical `search_evidence(query)` tool. v2 exposes only `acquire_evidence(EvidenceGap)`. The shared bounded acquisition engine enforces these runtime rules independently of model instructions:

- `user_fact` gaps never perform external RAG and return `user_input_required`;
- `external_knowledge` gaps require a targeted query and rationale;
- one Treatment run has an `EvidenceBudget` of two searches and at most five results per search;
- every attempt records its gap, query, search/no-search decision, evidence identities and explicit stop reason;
- budget exhaustion stops further retrieval and unresolved critical gaps remain in the trace.

Migration `000039_add_treatment_evidence_acquisition_trace` persists that non-authoritative execution trace on each TreatmentRevision proposal as `evidence_acquisition_trace`. The trace explains acquisition behavior; it does not grant acceptance authority.

The v2 Challenger passes the same `treatment_qualification_v1` dataset 4/4 with the same dataset fingerprint as v1, producing zero deterministic regressions and a pass-rate delta of 0.0. The dedicated Treatment EvidenceGap policy suite passes 5/5. This makes v2 eligible for later rollout-governance work, but it does **not** promote v2: all committed Compose definitions continue to default `TREATMENT_AGENT_CONFIGURATION_ID` to v1.

## Go DecisionAuthority and DecisionTrace

Go's existing Treatment generation and acceptance gates are now represented by the pure `treatment-go-acceptance-v1` deny-overrides policy. The policy deliberately excludes model confidence and Agent prose. Its authority facts are Diagnosis status/candidate count, Diagnosis freshness, durable BodyState safety state, candidate-assessment readiness, proposal acceptance state, pinned/current BodyState revisions, and the deterministic material-change review result.

Malformed, unknown or internally inconsistent BodyState safety facts fail closed. A valid proposal stores `generation_decision_trace`; successful acceptance stores `acceptance_decision_trace` in the same repository transaction that changes the revision to accepted and moves the current Treatment pointer. The acceptance trace is computed against an exact BodyState revision, and the existing transaction guard requires that same revision still be current before the transition commits. Both traces include the policy version, phase, outcome/reasons, DiagnosisAnalysis identity, Agent configuration identity and evaluated facts.

This closes deterministic authority provenance for successful Treatment artifacts. An explicit Treatment shadow/canary/promotion policy is still required before any governed production rollout of v2.

## Protected contracts

- Python never creates or accepts durable Treatment identities.
- Go re-checks Diagnosis eligibility/freshness, candidate-assessment readiness and active safety state before generation and acceptance.
- Acceptance re-reads current BodyState and uses the existing transactional revision guard.
- Accepted Treatment revisions remain immutable.
- Training is still a projection of an accepted revision only.
- LiteLLM remains the sole physical LLM provider/fallback boundary.
