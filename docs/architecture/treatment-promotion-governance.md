# Treatment Promotion, Shadow, Canary, and Rollback

Status: Current rollout mechanism. Treatment v2 became the repository Champion on 2026-09-01 under ADR 0010; v1 is the explicit rollback/historical target and there is no active Challenger by default.

## North-star rule

A qualified Treatment Agent configuration does not become production because its
manifest or code is merged. Go owns the mutable deployment state; Python executes
only the exact immutable configuration selected by Go.

The governed v1 -> v2 state machine is:

```text
champion
  -> shadow
  -> canary 5%
  -> canary 25%
  -> canary 50%
  -> promoted 100%

any rollout stage -> rollback
```

All committed Compose files default to `champion`, which now serves Treatment v2.
`TREATMENT_AGENT_CONFIGURATION_ID` is retired as serving authority; Champion,
optional Challenger and rollback target use separate explicit pointers.

## Promotion evidence

`apps/ai-service/data/evals/treatment_promotion_policy.json` is the machine-readable
promotion record `treatment_promotion_v1`. Its evaluator requires:

- v1 Champion qualification 4/4;
- v2 Challenger qualification 4/4 on the same dataset fingerprint;
- v1 -> v2 non-inferiority with zero deterministic regressions;
- Treatment EvidenceGap policy qualification 5/5;
- repository-known immutable Champion and Challenger manifests;
- the predeclared rollout thresholds below.

The generated evidence artifact is
`data/evals/reports/treatment_promotion_readiness.json`.

No interaction experiment is required for this pair because v2 changes only the
prompt/tool/evidence-acquisition revisions. Model group, output schema, governance
policy and Go decision policy remain unchanged. A future configuration that mixes
several such changes must not silently inherit this waiver.

## Runtime admission

Current clean-environment baseline:

```text
TREATMENT_CHAMPION_CONFIGURATION_ID=treat-config-f68eec9846664596
TREATMENT_CHALLENGER_CONFIGURATION_ID=
TREATMENT_ROLLBACK_CONFIGURATION_ID=treat-config-85718f8e90ac9d80
TREATMENT_ROLLOUT_STAGE=champion
```

`treatment_promotion_v1` remains immutable evidence for the historical v1 -> v2
transition. A future non-Champion stage requires a distinct Challenger and a new
approved record for that exact pair. Canary still admits only the predeclared
basis-point steps once such a pair exists.

## Stable serving assignment

Treatment uses the same deterministic assignment primitive as Diagnosis:

```text
bucket = uint64(SHA256(rollout_salt + NUL + stable_user_id)[0:8]) mod 10000
challenger iff bucket < canary_bps
```

For one rollout salt the same user stays in the same bucket. During `shadow`, v1
serves and v2 is paired read-only. During canary, the assigned configuration serves
and the opposite configuration runs as the paired shadow. `promoted` serves the active Challenger only; current `rollback` serves the explicit v1 rollback target.

Route selection lives in `TreatmentService`, not the HTTP handler, so both direct
proposal generation and internal Training-feedback regeneration use the same
stable control plane.

## Shadow side-effect boundary

The served run is the only path allowed to create `TreatmentRevision`. After that
immutable proposal is persisted, rollout invokes `TreatmentReplayService` through
a read-only revision-source interface and runs the opposite configuration against
the exact frozen `replay_input`.

Shadow/counterfactual execution cannot accept/reject Treatment, move the current
pointer, create TrainingPlan/Outcome, or mutate BodyState/Diagnosis. A shadow error
is recorded as operational evidence and never changes the already-served proposal.

The served TreatmentRevision stores `rollout_provenance` with stage, stable bucket,
served/shadow configuration identities, canary basis points and promotion record.

## Durable observations

Migration `000042_create_treatment_rollout_observations` adds anonymous
`treatment_rollout_observations` and `treatment_revisions.rollout_provenance`.
Each paired observation records:

- source TreatmentRevision identity, but no user identity;
- stage, subject bucket and canary step;
- Champion/Challenger/served/shadow config identities and promotion record;
- hard/semantic/presentation replay comparison;
- unsafe authority relaxation, forbidden side-effect surface, configuration
  mismatch and shadow execution error signals.

Artifact integrity is always evaluated against the source configuration's own Go
generation decision. The target configuration has a separate generation decision;
this prevents a future decision-policy change from being mistaken for source
artifact corruption.

## Stop and progression policy

`treatment-rollout-policy-v1` is deny-first:

- any unsafe authority relaxation -> `rollback`;
- any forbidden Treatment side-effect surface -> `rollback`;
- any configuration identity mismatch -> `rollback`;
- any shadow execution error -> `pause`;
- after 20 samples, hard mismatch rate > 10% -> `pause`;
- after 20 samples, semantic mismatch rate > 25% -> `pause`.

A clean stage needs 20 observations before progression:

```text
shadow (20) -> canary 500 bps
500 bps (20) -> 2500 bps
2500 bps (20) -> 5000 bps
5000 bps (20) -> promoted 10000 bps
```

The evaluator and operator command never mutate deployment state. Operators inspect
the deterministic recommendation and explicitly change deployment configuration.

From `apps/api`:

```bash
go run ./cmd/treatment-rollout-status -stage shadow -canary-bps 0
```

## Hermetic deployment proof

`local-deploy-validate.sh` now validates the promoted baseline directly. Longitudinal
E2E must persist Treatment v2 revisions, persist zero newly served v1 revisions,
and create zero rollout observations because no Challenger is active. Historical
v1 -> v2 rollout mechanics remain covered by focused Go tests and the immutable
promotion-policy evaluator.
