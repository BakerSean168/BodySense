# Diagnosis Promotion, Shadow, Canary, and Rollback

Status: Current rollout mechanism. Diagnosis v3 remains the repository Champion;
v4 remains immutable historical qualification evidence, and v5 is the current
qualified successor Challenger, ready for shadow under `diagnosis_promotion_v3`.
There is no active Challenger by default and no promotion/deployment has occurred.

## North-star rule

A qualified Agent configuration does not become production merely because its
code exists. Go owns the mutable deployment state; Python only executes the exact
immutable configuration selected by Go.

The rollout state machine is:

```text
champion
  -> shadow
  -> canary 500 bps (5%)
  -> canary 2500 bps (25%)
  -> canary 5000 bps (50%)
  -> promoted 100%

any rollout stage -> rollback
```

Repository/default production state remains `champion`, with v3 as the current
Champion. v4 is a distinct qualified Challenger; `shadow/canary/promoted` are only
meaningful after an explicit rollout selection and matching promotion record. The
historical v1 -> v3 and v3 -> v4 records remain immutable evidence for their
transitions. ADR0010 forbids silently treating qualification as promotion.

## Promotion evidence

`apps/ai-service/data/evals/diagnosis_promotion_policy.json` is the historical
`diagnosis_promotion_v1` specification. The v3 -> v4 successor specification is
`data/evals/diagnosis_promotion_policy_v2.json`; v3's successor specification is
`data/evals/diagnosis_promotion_policy_v3.json`. The same
`run_diagnosis_promotion_eval.py` validates these immutable policies:

- v1 Champion qualification: 7/7;
- v2 EvidenceGap Challenger vs v1: non-inferior, promotion-eligible, no critical regression;
- v3 DecisionAuthority Challenger vs v2: non-inferior, promotion-eligible, no critical regression;
- v4 claim-surface Challenger vs v3: 7/7 qualified, non-inferior, promotion-eligible, no critical regression;
- v5 negation-aware Challenger vs v4: 7/7 qualified, non-inferior, promotion-eligible, no critical regression;
- one shared qualification dataset fingerprint across the chain;
- EvidenceGap policy suite: 5/5;
- dedicated negation policy suite: 10/10, identity-bound to v5 and detector v2;
- the declared immutable Champion and final Challenger both resolve from repository manifests.

The historical generated evidence artifact is
`data/evals/reports/diagnosis_promotion_readiness.json`; the v4 successor
artifact is `data/evals/reports/diagnosis_promotion_readiness_v2.json`; the v5
successor artifact is `data/evals/reports/diagnosis_promotion_readiness_v3.json`.

No interaction experiment is required for v5 because its explicit-negation
interpretation is deterministic and is exercised on the actual pre-agent and
post-agent service path; provider interaction cannot change that decision. The
focused regression suite covers explicit negation, mixed clauses, positive and
ambiguous phrases, and immutable v4 literal behavior. The cumulative
Challengers otherwise changed one governed boundary at a time: v2 isolates
EvidenceGap; v3 isolates Go DecisionAuthority; v4 isolates the Diagnosis
claim-surface scan.
If a future change combines model, prompt, tools, or policy changes such that
attribution is ambiguous, a promotion policy
must explicitly require the interaction experiment instead of reusing this waiver.

## Runtime admission

Current clean-environment baseline:

```text
DIAGNOSIS_CHAMPION_CONFIGURATION_ID=diag-config-5a4a13627e14b4cf
DIAGNOSIS_CHALLENGER_CONFIGURATION_ID=
DIAGNOSIS_ROLLOUT_STAGE=champion
DIAGNOSIS_CANARY_BPS=500
DIAGNOSIS_ROLLOUT_SALT=diagnosis-rollout-v1
```

The operator canary steps are exactly 500 -> 2500 -> 5000 basis points. A clean
baseline defaults to the first step, 500 bps. Staging Compose must propagate both
`DIAGNOSIS_CANARY_BPS` and `DIAGNOSIS_ROLLOUT_SALT` into the API container.

The v3 -> v4 pair and `diagnosis_promotion_v2` remain immutable historical
qualification evidence. v5 may be selected explicitly as the Challenger for
`shadow` only with the approved `diagnosis_promotion_v3` record. Qualification
alone does not promote v5. For every non-Champion Diagnosis stage, Go admits
only a repository-known promotion record whose exact Champion -> Challenger
pair matches the selected immutable IDs; a nonempty label is not sufficient.
`DIAGNOSIS_AGENT_CONFIGURATION_ID` remains retired.

## Stable canary assignment

Canary assignment is deterministic:

```text
bucket = uint64(SHA256(rollout_salt + NUL + stable_user_id)[0:8]) mod 10000
challenger iff bucket < canary_bps
```

The same subject remains in the same bucket for a fixed rollout salt. Canary
stages accept only the predeclared 500, 2500, and 5000 basis-point steps; 100%
cannot be smuggled in as a canary and must use the explicit `promoted` stage.

During `shadow`, Champion serves and Challenger is paired read-only. During
`canary`, the assigned config serves and the opposite config runs as the paired
shadow. `promoted` serves Challenger only. `rollback` serves the explicit rollback target, which is separate from the current Champion.

## Shadow/canary side-effect boundary

The served run is the only path allowed to create DiagnosisAnalysis, Evidence,
Hypothesis, BodyState safety state, or consultation phase changes.

The paired run reuses the Phase-8 frozen input and counterfactual path. It may read
knowledge, but it never creates those business artifacts. A target v4 pre-agent
safety block is recomputed in Go and bypasses the model, preserving Phase-6
semantics even in shadow.

Historical governance-rejected responses are deliberately not forced into a new
DiagnosisAnalysis shape. The current v3 Champion can still be paired against v4
through the frozen counterfactual path so `Champion block -> Challenger allow`
remains observable as an unsafe relaxation without changing durable semantics.

## Durable rollout observations

Migration `000037_create_diagnosis_rollout_observations` stores anonymous
operational evidence:

- stage and canary basis-point step;
- stable subject bucket, but not user id;
- Champion / Challenger / served / shadow configuration identities;
- hard / semantic / presentation comparison;
- unsafe authority relaxation;
- forbidden Diagnosis side effect;
- configuration mismatch;
- shadow execution error;
- source analysis id when a durable source exists.

`source_analysis_id` is nullable for the legacy rejected-baseline case; the system
never invents a fake DiagnosisAnalysis identity just to satisfy rollout telemetry.

The served Diagnosis `DecisionTrace` also includes `rollout_provenance`, so a
historical artifact can explain the stage, bucket, served config, opposite config,
canary percentage, and promotion record that selected it.

## Stop and rollback policy

`diagnosis-rollout-policy-v1` is deny-first:

- any unsafe authority relaxation -> `rollback`;
- any forbidden Diagnosis side effect -> `rollback`;
- any configuration identity mismatch -> `rollback`;
- any shadow execution error -> `pause`;
- after 20 samples, hard mismatch rate > 10% -> `pause`;
- after 20 samples, semantic mismatch rate > 25% -> `pause`.

These thresholds are mirrored in Go and the repository promotion JSON, with a
cross-language test that fails if they drift.

A clean stage needs 20 observations before progression:

```text
shadow (20) -> canary 500 bps
500 bps (20) -> 2500 bps
2500 bps (20) -> 5000 bps
5000 bps (20) -> promoted 10000 bps
```

The evaluator never mutates deployment state. Operators inspect the deterministic
recommendation and change deployment configuration explicitly.

## Operator command

From `apps/api` with database environment variables set:

```bash
go run ./cmd/diagnosis-rollout-status \
  -stage shadow \
  -canary-bps 0
```

It prints the observation summary, stop gate, and next progression action, and
returns non-zero when the stop gate says pause/rollback.

## Hermetic deployment proof

`local-deploy-validate.sh` now runs the disposable production-shaped stack on the
same baseline as clean environments: Diagnosis v3 serves directly in `champion`
with no active Challenger. After longitudinal E2E, PostgreSQL must contain v3
Diagnosis artifacts, zero non-Champion Diagnosis artifacts and zero rollout
observations. Historical v1 -> v3 and v3 -> v4 mechanics remain covered by
focused Go tests and the immutable promotion-policy evaluators.
