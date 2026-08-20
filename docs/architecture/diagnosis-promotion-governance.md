# Diagnosis Promotion, Shadow, Canary, and Rollback

Status: Phase 9 implementation checkpoint (2026-08-20)

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

Repository/default production state remains `champion`. Advancing a real
environment requires explicit rollout environment settings and the approved
promotion record identity.

## Promotion evidence

`apps/ai-service/data/evals/diagnosis_promotion_policy.json` is the machine-readable
promotion specification. `run_diagnosis_promotion_eval.py` validates:

- v1 Champion qualification: 7/7;
- v2 EvidenceGap Challenger vs v1: non-inferior, promotion-eligible, no critical regression;
- v3 DecisionAuthority Challenger vs v2: non-inferior, promotion-eligible, no critical regression;
- one shared qualification dataset fingerprint across the chain;
- EvidenceGap policy suite: 5/5;
- the declared immutable Champion and final Challenger both resolve from repository manifests.

The generated evidence artifact is
`data/evals/reports/diagnosis_promotion_readiness.json`.

No interaction experiment is required for this chain because the cumulative
Challengers changed one governed boundary at a time: v2 isolates EvidenceGap;
v3 isolates Go DecisionAuthority. If a future change combines model, prompt,
tools, or policy changes such that attribution is ambiguous, a promotion policy
must explicitly require the interaction experiment instead of reusing this waiver.

## Runtime admission

Non-Champion stages require all of the following:

```text
DIAGNOSIS_CHAMPION_CONFIGURATION_ID=diag-config-f492eb1c0c6676ae
DIAGNOSIS_CHALLENGER_CONFIGURATION_ID=diag-config-5a4a13627e14b4cf
DIAGNOSIS_PROMOTION_RECORD=diagnosis_promotion_v1
DIAGNOSIS_ROLLOUT_STAGE=shadow|canary|promoted
```

A missing/wrong promotion record or different config pair fails service startup.
The old `DIAGNOSIS_AGENT_CONFIGURATION_ID` is only a Phase-10 compatibility alias
for the Champion pointer.

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
shadow. `promoted` serves Challenger only. `rollback` serves Champion only.

## Shadow/canary side-effect boundary

The served run is the only path allowed to create DiagnosisAnalysis, Evidence,
Hypothesis, BodyState safety state, or consultation phase changes.

The paired run reuses the Phase-8 frozen input and counterfactual path. It may read
knowledge, but it never creates those business artifacts. A target v3 pre-agent
safety block is recomputed in Go and bypasses the model, preserving Phase-6
semantics even in shadow.

Legacy v1 governance-rejected responses are deliberately not forced into a new
DiagnosisAnalysis shape. Instead, a transient frozen baseline can still be paired
against v3 so `Champion block -> Challenger allow` remains observable as an
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

`local-deploy-validate.sh` explicitly enables shadow only inside the disposable
validator stack. The longitudinal browser suite runs through the normal serving
path, then the script queries PostgreSQL and requires at least one v1/v3 shadow
observation with zero unsafe-relaxation, forbidden-side-effect, config-mismatch,
or shadow-error blockers.

This proves the real Docker composition and HTTP application path while keeping
production Compose defaults on Champion.
