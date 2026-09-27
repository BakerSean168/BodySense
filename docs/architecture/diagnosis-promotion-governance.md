# Diagnosis Promotion, Shadow, Canary, and Rollback

Status: Current rollout mechanism. Diagnosis v4 is the repository Champion after
the claim-surface successor qualification; v3 is retired to historical/replay
evidence and there is no active Challenger by default.

## North-star rule

A qualified Agent configuration does not become production merely because its
code exists. Go owns the mutable deployment state; Python only executes the exact
immutable configuration selected by Go.

The rollout state machine is:

```text
champion
  -> shadow
  -> canary 5%
  -> canary 25%
  -> canary 50%
  -> promoted 100%

any rollout stage -> rollback
```

Repository/default production state remains `champion`, but Champion now means the
latest qualified v4 baseline. `shadow/canary/promoted` are only meaningful after a
future distinct Challenger and matching promotion record exist. The historical
v1 -> v3 and v3 -> v4 records remain immutable evidence for their transitions.

## Promotion evidence

`apps/ai-service/data/evals/diagnosis_promotion_policy.json` is the historical
`diagnosis_promotion_v1` specification. The v3 -> v4 successor specification is
`data/evals/diagnosis_promotion_policy_v2.json`; the same
`run_diagnosis_promotion_eval.py` validates both policies:

- v1 Champion qualification: 7/7;
- v2 EvidenceGap Challenger vs v1: non-inferior, promotion-eligible, no critical regression;
- v3 DecisionAuthority Challenger vs v2: non-inferior, promotion-eligible, no critical regression;
- v4 claim-surface Challenger vs v3: 7/7 qualified, non-inferior, promotion-eligible, no critical regression;
- one shared qualification dataset fingerprint across the chain;
- EvidenceGap policy suite: 5/5;
- the declared immutable Champion and final Challenger both resolve from repository manifests.

The historical generated evidence artifact is
`data/evals/reports/diagnosis_promotion_readiness.json`; the v4 successor
artifact is `data/evals/reports/diagnosis_promotion_readiness_v2.json`.

No interaction experiment is required for this chain because the cumulative
Challengers changed one governed boundary at a time: v2 isolates EvidenceGap;
v3 isolates Go DecisionAuthority; v4 isolates the Diagnosis claim-surface scan.
If a future change combines model, prompt, tools, or policy changes such that
attribution is ambiguous, a promotion policy
must explicitly require the interaction experiment instead of reusing this waiver.

## Runtime admission

Current clean-environment baseline:

```text
DIAGNOSIS_CHAMPION_CONFIGURATION_ID=diag-config-4a517fea19cb6c49
DIAGNOSIS_CHALLENGER_CONFIGURATION_ID=
DIAGNOSIS_ROLLOUT_STAGE=champion
```

The v1 -> v3 and v3 -> v4 pairs are immutable qualification evidence and may be
used by hermetic/history tests; that does not make a retired configuration a
serving target. A future
non-Champion stage requires a distinct Challenger and an approved record for that
exact pair. `DIAGNOSIS_AGENT_CONFIGURATION_ID` remains retired.

## Stable canary assignment

Canary assignment is deterministic:

```text
bucket = uint64(SHA256(rollout_salt + NUL + stable_user_id)[0:8]) mod 10000
challenger iff bucket < canary_bps
```

The same subject remains in the same bucket for a fixed rollout salt. Canary
stages are only valid between 1 and 9999 basis points; 100% cannot be smuggled in
as a canary and must use the explicit `promoted` stage.

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

Legacy v1/v3 governance-rejected responses are deliberately not forced into a new
DiagnosisAnalysis shape. Instead, a transient frozen baseline can still be paired
against v4 so `Champion block -> Challenger allow` remains observable as an
unsafe relaxation without changing the user's characterized v1 response.

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
same baseline as clean environments: Diagnosis v4 serves directly in `champion`
with no active Challenger. After longitudinal E2E, PostgreSQL must contain v4
Diagnosis artifacts, zero newly served retired artifacts and zero rollout
observations. Historical v1 -> v3 and v3 -> v4 mechanics remain covered by
focused Go tests and the immutable promotion-policy evaluators.
