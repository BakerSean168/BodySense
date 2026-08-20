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

The baseline passes 4/4 for `treat-config-85718f8e90ac9d80`. This is a platform qualification baseline, not proof of clinical efficacy or semantic superiority of the intervention content. Future Treatment policy/model changes need richer safety, EvidenceGap and outcome-oriented evals before promotion governance is introduced.

## Protected contracts

- Python never creates or accepts durable Treatment identities.
- Go re-checks Diagnosis eligibility/freshness, candidate-assessment readiness and active safety state before generation and acceptance.
- Acceptance re-reads current BodyState and uses the existing transactional revision guard.
- Accepted Treatment revisions remain immutable.
- Training is still a projection of an accepted revision only.
- LiteLLM remains the sole physical LLM provider/fallback boundary.
