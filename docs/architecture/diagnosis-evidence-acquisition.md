# Diagnosis Evidence Acquisition Architecture

> Status: implemented Challenger boundary in Phase 5; production promotion remains Phase 9 governance work.

## Ownership

Diagnosis reasoning may identify that the current durable BodyState is insufficient, but the model does not own arbitrary retrieval. The Python Agent runtime owns typed gap declaration and bounded acquisition. Go remains the owner of durable health truth and final business authority.

```text
BodyState revision
  -> Diagnosis Agent
  -> typed EvidenceGap
  -> acquire_evidence tool
  -> DiagnosisEvidenceAcquirer
       -> source policy
       -> EvidenceBudget
       -> KnowledgeEvidenceSearcher (external_knowledge only)
  -> EvidenceAttempt + stop_reason
  -> run EvidenceAcquisitionTrace
  -> governed Diagnosis output
```

There is no Diagnosis HTTP field for injecting preloaded RAG context/results. New retrieval must pass through the acquisition runtime.

## EvidenceGap

Each acquisition request declares:

- `gap_id`: run-local identity for the missing information;
- `kind`: `user_fact` or `external_knowledge`;
- `description`: what is missing;
- `rationale`: why resolving it can materially change Diagnosis reasoning;
- `critical`: whether the run must preserve the gap if acquisition cannot resolve it;
- `query`: required only for `external_knowledge`.

`user_fact` explicitly forbids a search query. This prevents a retrieval result from being used as a substitute for a fact about the user.

## Budget and stopping

The v2 policy currently allows at most two external searches per Diagnosis run and at most five results per search. These values are behavior owned by `diagnosis-evidence-gap-v2`; changing them requires a new evidence-policy revision and therefore a new Agent configuration identity.

Every requested gap produces an `EvidenceAttempt` with one of these stopping reasons:

- `evidence_returned`;
- `user_input_required`;
- `budget_exhausted`;
- `search_unavailable`;
- `no_results`.

A critical gap whose attempt remains unresolved is merged back into final `information_gaps` even if the model omitted it after the tool call. This is a runtime invariant, not merely a prompt instruction.

## Immutable configurations

The original Champion remains unchanged:

```text
diag-config-f492eb1c0c6676ae
prompt:   diagnosis-prompt-v3
tools:    diagnosis-tools-legacy-v1
evidence: diagnosis-evidence-legacy-v1
```

The Phase 5 Challenger is:

```text
diag-config-20fbfc23ca09cbab
prompt:   diagnosis-prompt-v4-evidence-gap
tools:    diagnosis-evidence-acquisition-tools-v2
evidence: diagnosis-evidence-gap-v2
```

`DiagnosisService` no longer accepts a preconstructed Agent. Tests and deterministic execution may inject a model, but the Agent itself is always created from the exact selected manifest. This prevents configuration provenance from diverging from executed prompt/tool policy.

## Qualification

The general Diagnosis qualification set is tool-policy neutral: it asks the runtime to expose exactly the tool surface declared by the selected immutable configuration instead of hard-coding the Champion's legacy tool name.

On the same dataset fingerprint, the Phase 5 Challenger passes all seven Diagnosis cases and is paired non-inferior to the v1 Champion with zero critical regressions and a pass-rate delta of `+0.000`.

A second deterministic Pydantic Evals suite covers the acquisition policy itself:

1. no gap -> no search;
2. user fact -> no RAG and `user_input_required`;
3. targeted external gap -> one attributable search;
4. critical gap with zero budget -> `budget_exhausted` and preserved critical gap;
5. second critical gap after budget consumption -> no second search and preserved critical gap.

The Challenger is qualified but is not promoted by Phase 5. The Go production deployment pointer remains on the v1 Champion until the explicit Shadow/Canary/Promotion phase so architecture migration does not bypass its own release-governance model.
