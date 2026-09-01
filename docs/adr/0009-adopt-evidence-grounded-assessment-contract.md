# ADR 0009: Adopt an evidence-grounded Assessment contract

- Status: Accepted
- Date: 2026-09-01
- Scope: Assessment Agent, evidence governance, BodyState projection, replay, API contract
- Supersedes for new reports: model-authored Assessment prose, health grades, and numeric dimension scores

## Context

A real Assessment report was generated from a frozen input with no images, no completed posture analysis and no report indicators, yet the model emitted shoulder-protraction and spinal-curvature observations. The same output assigned a posture score while also admitting that posture detail was missing.

The first fix exposed a second, deeper problem. A later end-to-end run correctly attached a `lifestyle.activity` evidence ref to an observation, but expanded the exact source fact “日常活动以久坐为主” into a new claim that this “可能对整体健康产生长期影响”. Exact source provenance therefore did **not** guarantee claim-level entailment while the model still authored durable prose.

The failures exposed coupled design flaws rather than a prompt-only defect:

1. `assessment-output-v1` validated JSON shape but not whether claims were grounded in available evidence.
2. `health_grade` and mandatory 0-100 `dimension_scores` forced pseudo-precise values even when no deterministic scoring rubric existed.
3. report-level `completed` was entangled with “must emit at least one observation”, creating generation pressure unrelated to evidence availability.
4. free-form model summary/advice could cross the observation-only boundary.
5. even a valid evidence ref could coexist with model-authored prose that exceeded the source.
6. Python and Go trusted structurally valid model output too far into the durable write path.
7. architecture documentation described evidence classes but did not encode them as executable contracts.

## Decision

### 1. Model authority is reduced to evidence selection and classification

For the serving `assessment-output-v2` model schema, one generated item contains only:

```text
kind
exactly one evidence_ref
```

The model cannot author durable:

- `label`;
- `description`;
- `body_region`;
- severity/confidence;
- report status;
- health grade;
- numeric dimension scores;
- overall summary;
- information-gap semantics;
- recommendation, treatment or training text.

Unknown/extra fields fail typed validation. Empty model selections are valid.

### 2. Durable observation prose is rendered deterministically from trusted evidence

After governance validates `kind + evidence_ref`, Python renders the candidate from the authoritative evidence snapshot. Go independently reconstructs the same evidence catalog and renders it again before BodyState persistence. Upstream prose is therefore non-authoritative even if a buggy or malicious caller supplies it.

Examples:

```text
BodyState lifestyle.exercise = 健身；频率：1-2
-> 运动记录
-> 来源记录：健身；频率：1-2。

Posture finding evidence = 右侧肩峰位置略高
-> 肩部对称性待复核
-> 体态分析记录：右侧肩峰位置略高。
```

The durable text is a source restatement, not a model interpretation. This structurally removes the claim-expansion failure mode instead of adding a second LLM judge.

### 3. The application owns the authoritative evidence catalog

The serving catalog is built only from frozen health inputs that may support a health observation:

```text
body_state:fact:<fact-uuid>
body_state:observation:<observation-uuid>
report:upload:<upload-uuid>:indicator:<index>
posture:upload:<upload-uuid>:finding:<index>
posture:upload:<upload-uuid>:summary
```

Position-based refs exist only as deterministic fallback when legacy/minimal replay data lacks a durable identity.

`profile` is **not** health-observation evidence. Gender, birth date and derived age can remain stable identity context for other use-cases, but their existence cannot manufacture a BodyState/Assessment health claim.

Unverified, rejected, inactive or `excluded_from_reasoning` BodyState items are not selectable evidence.

### 4. Posture is the sole visual-perception authority

The serving Assessment contract rejects raw image inputs. Posture owns image → governed finding. Assessment may only select completed `posture_analysis` evidence. This removes duplicate visual-inference authority and makes Assessment replay reproducible without retaining raw image bytes.

Unmodeled `rag_context` is also rejected by the serving contract. Every source able to support a durable Assessment observation must first enter the explicit evidence model.

### 5. Observation taxonomy has executable source policy

The serving taxonomy is:

```text
posture_alignment
posture_asymmetry
lifestyle_pattern
exercise_pattern
report_indicator
anthropometry
```

Rules include:

```text
posture_alignment / posture_asymmetry -> posture_analysis
exercise_pattern                       -> BodyState lifestyle.exercise
lifestyle_pattern                      -> BodyState lifestyle.* except exercise
report_indicator                       -> report
anthropometry                          -> BodyState anthropometry.*
```

Each selection must reference exactly one real evidence item. The same evidence ref cannot be selected twice in one output. Unknown kinds, nonexistent refs and incompatible source/kind pairs fail closed.

### 6. Evidence domains replace pseudo health-scoring dimensions

The application derives coverage across six independent **evidence domains**:

```text
posture
exercise
lifestyle
anthropometry
health_report
injury_symptoms
```

Each domain is `available` or `missing` and lists the exact refs behind that state. Overall coverage is:

- `complete`: all six domains have evidence;
- `partial`: at least one, but not all, domains have evidence;
- `insufficient`: no usable health evidence exists.

A generic lab report is `health_report` evidence; it is not automatically `injury_symptoms` evidence. This prevents the previous `injury_safety <- any report` semantic conflation.

This is evidence coverage, **not health quality**. If a future product requires a score, it must first define an explicit deterministic and clinically justified rubric as a separate contract.

### 7. Report status, gaps and summary are deterministic projections

For new reports:

```text
coverage == insufficient -> status = insufficient_information
otherwise                -> status = completed
```

When the evidence catalog is empty, Python short-circuits before resolving or calling a model. The report is derived deterministically with `execution_provenance.status=skipped_no_evidence`, zero model requests/tokens, and Go records `generation_decision_trace.status=derived_without_model`, `phase=deterministic_derivation`, `model_executed=false`. This avoids paying a model to answer a question the application already knows has no admissible evidence.

Evidence gaps describe missing coverage only and carry `required: false`; they are not diagnoses, clinical requirements, or instructions to collect every possible data class.

The summary is deterministic, for example:

```text
当前资料支持 1 项待审核观察；1/6 个证据领域已有资料，5/6 个领域当前未提供资料。
```

### 8. Python and Go both enforce and render the contract

Python is the primary generation governance seam because it owns PydanticAI output plus the exact runtime catalog. Go is the durable truth owner and independently:

1. rebuilds the catalog from the frozen request;
2. validates output schema/contract/governance identity;
3. validates exact ref existence, uniqueness and kind/source compatibility;
4. derives report status/coverage/gaps/summary;
5. deterministically renders observation prose;
6. only then projects an unverified observation into BodyState.

The durable invariant is:

```text
model can select/classify evidence
model cannot author durable health prose
unsupported selection -> rejected
accepted selection -> deterministic source rendering -> BodyState
```

Counterfactual replay for the serving contract passes through the same Go evidence validation/rendering before comparison, so replay is not a contract bypass.

### 9. Immutable configuration history is preserved

`assessment-v1` and `assessment-v2` remain repository-known historical configurations and are not allowed to serve new durable reports.

`assessment-v3` is the serving configuration and binds:

```text
prompt_revision: assessment-prompt-v3-evidence-contract
output_schema_revision: assessment-output-v2
evidence_policy_revision: assessment-evidence-contract-v2
governance_policy_revision: assessment-governance-v2
decision_policy_revision: assessment-go-generation-v2
```

Because this v3 contract is introduced atomically in this change, its immutable configuration identity represents the final selection-only behavior. Future behavior-significant changes require a new revision/configuration identity rather than silently mutating a published contract.

## Persistence and compatibility

Migration `000060_upgrade_assessment_evidence_contract` adds:

- `contract_revision`;
- `evidence_coverage`;
- `evidence_gaps`;
- nullable legacy `health_grade` / `dimension_scores`.

Historical `assessment-output-v1` rows keep their original grade/score semantics. New `assessment-output-v2` rows do not write them. API/Web types distinguish the legacy and evidence-grounded contracts instead of pretending the historical empty coverage object is a valid v2 coverage projection.

The migration down path is fail-closed: if any v2 report exists, schema downgrade is rejected rather than inventing a legacy `D` grade or synthetic score payload. A clean database with no v2 rows still supports `60 -> 59 -> 60` migration validation.

Assessment regression export is versioned as `assessment_qualification_v2` and records expected contract revision, evidence coverage status, evidence-gap count, whether the model actually executed, and v2-forbidden legacy fields. Counterfactual replay of a v2 target passes through the same Go evidence validation/rendering before comparison.

## Qualification

The contract has a deterministic Pydantic Evals policy suite (`assessment-evidence-contract-v2`) that exercises profile-only input, source-only BodyState rendering, unverified evidence exclusion, incompatible source/kind selection, duplicate refs, governed Posture findings, report-vs-injury domain separation, and BodyState-kind compatibility. The initial qualification is **8/8 passed** and is exposed as the Nx target `eval:assessment-evidence-contract`. Real Playwright E2E separately verifies both model-executed grounded persistence and the zero-evidence no-model path.

## Consequences

### Positive

- a hallucinated model sentence can no longer become durable Assessment prose;
- valid provenance alone is no longer mistaken for claim-level entailment;
- no fake `0`, `50`, arbitrary grade or arbitrary 0-100 score is needed to mean “unknown”;
- stable profile identity cannot manufacture health evidence;
- lab/report evidence is no longer conflated with injury/symptom evidence;
- every durable new observation traces to exactly one authoritative evidence item;
- Python, Go and replay share the same fail-closed trust boundary;
- future observation kinds must explicitly define admissible evidence.

### Trade-offs

- Assessment observations are intentionally conservative source restatements. Interpretation belongs to later Diagnosis/Treatment workflows with their own evidence contracts.
- the model has less expressive freedom; this is intentional because Assessment writes candidates into longitudinal state.
- legacy v1/v2 replay remains available for historical analysis but cannot serve new durable reports.
- health-grade UI concepts cannot return without a separate scoring specification.

## Rejected alternatives

### Prompt-only hardening

Rejected because the original prompt already prohibited unsupported observations. Prompt obedience is probabilistic and cannot protect durable truth.

### Validate only evidence source/ref, keep model-authored prose

Rejected after end-to-end testing demonstrated a correctly referenced BodyState fact could still be expanded into an unsupported health-effect claim.

### Add a second LLM entailment judge

Rejected as the primary durable-state solution. It would turn one probabilistic trust problem into two. Deterministic rendering removes the free-text authority entirely.

### Filter only `kind.startswith("posture")`

Rejected because taxonomy is broader than one prefix and unsupported claims can occur outside posture.

### Keep 0-100 scores and use `null`

Rejected because nullability fixes missingness but not the absence of a defined scoring rubric or misleading precision.

### Trust model-declared provenance

Rejected because the model cannot define whether evidence exists. Evidence truth is reconstructed from frozen application inputs.
