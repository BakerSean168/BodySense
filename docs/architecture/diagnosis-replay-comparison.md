# Diagnosis Replay and Configuration Comparison

Status: Historical Phase-8 implementation checkpoint; historical/counterfactual replay is implemented.

## Purpose

Diagnosis replay answers two different questions and does not blur them into one
"rerun the LLM" operation:

1. **Historical replay** — can the system deterministically reproduce the stored
   behavioral invariants of a past Diagnosis from the exact frozen input and
   recorded configuration/decision facts?
2. **Counterfactual replay** — what would the same frozen case do **today** under
   an explicitly selected immutable Agent configuration?

Token-for-token model reproduction is not a goal.

## Frozen replay input

Migration `000036_add_diagnosis_replay_input` adds a private `replay_input` JSONB
column to immutable `diagnosis_analyses`. New Diagnosis runs freeze exactly the
production-shaped values used for the run:

```text
body_state_revision
body_state
relevant_history
profile
```

The snapshot is written on normal, pre-agent safety-blocked, and Go
DecisionAuthority-blocked paths. It is not exposed by the ordinary Diagnosis read
model and never becomes a mutable BodyState source.

Historical analyses created before this snapshot existed are **not reconstructed
from today's BodyState**. Replay fails closed with
`DIAGNOSIS_REPLAY_INPUT_UNAVAILABLE` for those rows.

## Historical replay

`POST /api/v1/diagnosis-analyses/:analysisId/replay`

```json
{"mode":"historical"}
```

Historical replay performs no model call. It:

- loads the immutable raw output and frozen input;
- verifies BodyState revision, durable status, and Agent configuration identity;
- resolves the recorded decision-policy revision;
- re-evaluates the Go `DiagnosisDecisionPolicy` where applicable;
- compares recomputed behavior with the stored result.

This makes historical safety/authority behavior reproducible even when model text
is nondeterministic.

## Counterfactual replay

```json
{
  "mode": "counterfactual",
  "configuration_id": "diag-config-..."
}
```

Counterfactual replay:

- requires a repository-known immutable configuration;
- sends the frozen BodyState/history/profile to the existing typed Diagnosis AI
  boundary;
- verifies the returned configuration identity;
- applies the target configuration's Go DecisionAuthority revision;
- produces a comparison report;
- does **not** persist a DiagnosisAnalysis, Evidence, Hypothesis, safety state, or
  consultation phase transition.

External knowledge retrieval may reflect today's knowledge library. That is
intentional: the question is "what would this config do today?" Historical replay
remains the deterministic path for explaining what happened then.

## Three comparison layers

### Hard invariants

Deterministic equality is required for:

- top-level Diagnosis status;
- final delivery/authority outcome;
- absence of Treatment/Training side effects.

A hard mismatch is release-governance evidence, not a wording difference.

### Semantic invariants

Structured semantic drift is reported for:

- candidate cardinality;
- candidate `concern_key` coverage;
- referenced Fact / Observation / Evidence / counterevidence identities.

This layer intentionally avoids pretending that text equality is semantic
identity.

### Presentation

Exact text changes are tracked separately for:

- summary text;
- candidate display names.

Presentation mismatch alone does not imply a hard regression.

## Regression dataset export

`GET /api/v1/diagnosis-analyses/:analysisId/regression-export` returns an envelope
whose nested `case` matches `diagnosis_qualification_v1` case structure. It uses a
synthetic eval `user_id` and never exports the durable user UUID.

Because BodyState/profile content can still contain sensitive user-provided data,
the export must be reviewed/redacted before source control.

After review:

```bash
cd apps/ai-service
uv run python scripts/import_diagnosis_regression_case.py \
  --input /path/to/reviewed-export.json
```

The importer validates the Pydantic case schema, requires `split=regression`,
rejects duplicate case names, and then appends the case to the qualification
dataset.

## Protected contracts

Phase 8 preserves:

- immutable Diagnosis/candidate identities;
- existing public Diagnosis history/detail semantics;
- BodyState ownership and revision semantics;
- Go-only final DecisionAuthority;
- immutable AgentConfiguration identity;
- ordinary serving pointer (still unchanged until Phase 9).

Replay is a read-only analysis plane. It is not a second production Diagnosis
write path.
