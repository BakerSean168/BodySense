# AI Output Governance

- Status: Current role-specific governance architecture
- Updated: 2026-09-01
- Historical generic-guard design: [`docs/plan/archive/architecture-snapshots/2026-09-01/ai-output-governance.md`](../plan/archive/architecture-snapshots/2026-09-01/ai-output-governance.md)
- Related: [Agent Platform Role Governance](./agent-platform-role-governance.md)

## 1. Governing principle

BodySense does **not** use one generic `AIOutputGuard` as the final authority for every AI role.

Current governance is role-appropriate:

```text
Untrusted model/mechanism output
  -> typed/transport validation
  -> role-specific evidence/domain/safety policy
  -> Go-owned identity & durable-boundary validation
  -> authorized durable projection / delivery
```

Shared governance helpers may exist, but they do not replace the domain-specific contract.

## 2. Universal LLM rules

Every durable LLM-backed role must have:

1. repository-known immutable Agent configuration;
2. Go-owned deployment/configuration selection at application boundaries;
3. exact Python manifest execution;
4. LiteLLM as the only physical provider/model router;
5. execution/configuration provenance appropriate to the durable artifact;
6. fail-closed identity mismatch handling;
7. deterministic policy outside the model wherever a rule can be expressed structurally.

## 3. Current role matrix

| Role                                   | Current governance                                                                                                                                               |
| -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Diagnosis                              | typed output + controlled evidence acquisition + deterministic authority/safety + durable DecisionTrace + replay/rollout                                         |
| Treatment                              | proposal-only generation + acceptance-time reauthorization + durable revision/intervention/outcome boundaries + replay/rollout                                   |
| Assessment                             | selection-only `kind + evidence_ref`; Python and Go independently validate evidence and deterministically render durable prose; no-evidence path skips the model |
| Consultation                           | configuration handshake before semantic output + checkpointed interrupt/resume + runtime event validation/projection + rollout/replay                            |
| Posture                                | immutable perception configuration + deterministic output governance + Go identity validation before `analysis_result` persistence                               |
| Title                                  | immutable utility configuration + exact provenance; cosmetic fallback allowed                                                                                    |
| Knowledge Splitter/Curator             | exact config/lineage + publication/review authority rather than clinical canary                                                                                  |
| OCR / ASR / Embedding / geometric pose | non-LLM mechanisms; require mechanism provenance/input validation but no fake AgentConfiguration                                                                 |

## 4. Assessment as the strictest output-authority example

ADR 0009 deliberately removes durable prose authority from the Assessment model.

```text
authoritative evidence catalog
  -> model selects kind + exactly one ref
  -> Python evidence contract
  -> deterministic source rendering
  -> Go rebuilds catalog
  -> Go revalidates source/kind/ref identity
  -> Go rerenders
  -> unverified BodyState candidate
```

This solves two separate failure modes:

```text
schema-valid output != evidence-grounded output
correct evidence provenance != claim entailment
```

The durable boundary therefore does not trust model-authored Assessment prose at all.

## 5. Generic `guard_structured_output` is a seam, not universal authority

The Python governance module may provide shared structured guard dispatch, but the policies behind it are role-specific. For Assessment, the guard receives the exact runtime evidence catalog. Other roles use their own domain facts and authority rules.

Do not reintroduce the old architecture where every role is expected to become safe by registering generic `SchemaPolicy + FaithfulnessPolicy + SafetyPolicy` in one central pipeline.

The correct reuse level is:

```text
shared result/issues mechanics
+ role-specific executable contract
```

## 6. Durable owner revalidation

When Go owns durable truth, successful Python validation is necessary but not always sufficient.

For high-impact durable artifacts Go should independently verify invariants it can reconstruct, for example:

- selected configuration identity;
- exact evidence references;
- BodyState revision/identity;
- source/kind compatibility;
- deterministic decision policy;
- side-effect authorization.

This is defense in depth across trust boundaries, not duplicated LLM reasoning.

## 7. Governance failures

Expected failure modes are explicit:

```text
malformed/unknown schema     -> reject
unknown configuration        -> reject
identity mismatch            -> reject
unsupported evidence ref     -> reject
forbidden source/kind pair   -> reject
critical authority blocker   -> abstain/review/reject per domain policy
no Assessment evidence       -> deterministic insufficient_information, no model call
```

A validation failure must not be converted into a fabricated healthy/default result merely to keep the request successful.

## 8. Audit and replay

Durable clinical/longitudinal outputs retain enough information to explain:

```text
which configuration was selected
which configuration actually executed
which frozen state/evidence was used
which deterministic policy accepted/rejected the result
which durable mutation followed
```

Replay must pass through the same serving contract where meaningful; it is not a bypass around current durable validation.

## 9. Non-LLM mechanism governance

OCR, ASR, Embedding and geometric pose estimation are not Agents. They still need:

- input/file validation;
- bounded execution and error semantics;
- provider/engine/model/version provenance when their outputs become evidence;
- confidence/quality semantics;
- explicit downstream admissibility policy.

OCR report-indicator admissibility is now deterministic and versioned: completed extraction alone does not grant evidence authority, and current Assessment accepts only `ocr-indicator-admissibility-v1` items marked `admissible`. The remaining OCR gap is mechanism provenance (engine/parser/rendering/extractor identity), tracked in [`2026-09-01-documentation-code-alignment-audit.md`](../plan/active/2026-09-01-documentation-code-alignment-audit.md).

## 10. Definition of done for a new durable AI output

A new output type is not production-ready until the team can answer:

```text
Who owns the input truth?
What may the model author?
What is schema-only vs evidence/domain validation?
What deterministic gate exists before persistence/action?
Who owns configuration selection?
What provenance is durable?
How is replay/eval performed?
What happens when evidence is absent or validation fails?
```

If these answers are only written in a prompt, governance is incomplete.
