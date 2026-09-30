# Diagnosis replay, rollback, and production-promotion runbook

Status: Active operational contract for DGS-SAFE-090.

## Purpose

This runbook governs Diagnosis historical replay, serving-pointer rollback, immutable-history verification, and the later production-promotion decision.

The rollback contract changes **future serving only**. It must never rewrite an existing `diagnosis_analyses` or `diagnosis_candidates` row, replay input, raw output, decision trace, execution provenance, or Agent configuration provenance.

Current staging Champion after DGS-SAFE-080:

```text
diag-config-3f64de162dc937ee  # immutable Diagnosis v10
```

Historical rollback target retained for operational recovery:

```text
diag-config-5a4a13627e14b4cf  # Diagnosis v3
```

Historical v3-v7 identities remain immutable replay identities even when they are no longer normal serving targets.

## Safety rules

- Acquire the staging/production deployment lock before changing a serving pointer.
- Change only rollout/serving configuration. Do not update historical Diagnosis rows.
- Clear Challenger and promotion identity during an emergency Champion rollback unless a separately qualified rollout explicitly requires them.
- Keep the application image/revision fixed during a serving-pointer rehearsal. Rollback is not a code rollback.
- Use the public Diagnosis endpoint after each pointer change and verify `decision_trace.rollout_provenance.served_configuration_id`.
- Capture a Diagnosis history snapshot **before** rollback and verify it after rollback and after restore.
- Do not treat a missing historical SafetyEnvelope as `absent` safety evidence. Old replay input remains old replay input.
- A production provider that differs from the staging-accepted physical provider requires its own provider acceptance evidence before promotion.

## 1. Historical identity audit

From `apps/ai-service`:

```bash
uv run --extra dev python scripts/run_diagnosis_historical_identity_audit.py
```

Required result:

```text
Diagnosis historical identity audit: ACCEPTED
identities: 5/5
```

The audit pins v3-v7 manifest fingerprints, configuration IDs, decision-policy revisions, governance-policy revisions, and the shared Diagnosis v1 manifest contract.

## 2. Historical database replay audit

The API image contains `/app/domain-validator`.

Inside the API runtime:

```bash
/app/domain-validator -mode diagnosis-replay-audit
```

The audit performs model-free historical replay for every replayable v3-v7 database analysis, up to the configured per-configuration limit. A configuration with no database samples is reported as:

```text
identity_and_synthetic_regression_only
```

and remains covered by repository regression tests plus the historical identity audit.

Required result:

```text
DIAGNOSIS_HISTORICAL_REPLAY_AUDIT=PASS
```

Any database replay failure blocks DGS-SAFE-090.

## 3. Capture immutable history before rollback

Inside the API runtime:

```bash
/app/domain-validator -mode diagnosis-history-snapshot > /tmp/diagnosis-history-before.json
```

The snapshot contains only opaque row IDs plus SHA-256 hashes. It does not export Diagnosis prose or other raw health content.

Persist the snapshot outside the API container before recreating the API container.

A baseline self-check may be performed with:

```bash
/app/domain-validator \
  -mode diagnosis-history-verify \
  -snapshot-file /tmp/diagnosis-history-before.json
```

Required:

```text
DIAGNOSIS_HISTORY_IMMUTABILITY=PASS
```

## 4. Serving-pointer rollback

For an emergency rollback from v10 to v3, set:

```text
DIAGNOSIS_CHAMPION_CONFIGURATION_ID=diag-config-5a4a13627e14b4cf
DIAGNOSIS_CHALLENGER_CONFIGURATION_ID=
DIAGNOSIS_ROLLOUT_STAGE=champion
DIAGNOSIS_PROMOTION_RECORD=
```

Recreate **only the API service** with the already deployed API image. Do not rebuild images and do not restart PostgreSQL.

Wait for API health to become `healthy`.

Then issue a real public Diagnosis request using a synthetic/operator test subject and require:

```text
status = completed
governance = accepted
decision_trace.rollout_provenance.stage = champion
decision_trace.rollout_provenance.served_configuration_id = diag-config-5a4a13627e14b4cf
```

Copy the pre-rollback history snapshot into the recreated API container and verify it:

```bash
/app/domain-validator \
  -mode diagnosis-history-verify \
  -snapshot-file /tmp/diagnosis-history-before.json
```

New analyses created by the rollback smoke are allowed. Any missing or mutated protected row is a rollback failure.

## 5. Restore v10

Restore:

```text
DIAGNOSIS_CHAMPION_CONFIGURATION_ID=diag-config-3f64de162dc937ee
DIAGNOSIS_CHALLENGER_CONFIGURATION_ID=
DIAGNOSIS_ROLLOUT_STAGE=champion
DIAGNOSIS_PROMOTION_RECORD=
```

Again recreate only the API service with the same deployed image.

Repeat the public Diagnosis smoke and require:

```text
served_configuration_id = diag-config-3f64de162dc937ee
```

Run the history verification again. The protected pre-rollback root must still equal the baseline root.

## 6. Production-promotion readiness

From `apps/ai-service`:

```bash
uv run --extra dev python scripts/run_diagnosis_production_promotion_readiness.py
```

This command is a decision report. It may validly return `HOLD`.

To make a CI/operator gate fail when production is not ready:

```bash
uv run --extra dev python \
  scripts/run_diagnosis_production_promotion_readiness.py \
  --require-ready
```

Promotion requires all of:

- Diagnosis v10 final acceptance is green.
- v3-v7 historical identity audit is green.
- DGS-SAFE-090 operational audit is green.
- a production-candidate provider report exists for the exact production physical model;
- provider sample count and pass rate satisfy the production-promotion policy;
- provider errors, contract failures, governance rejections, and configuration mismatches are zero.

A staging provider acceptance does not automatically authorize a different production provider. The production-only preflight and isolated 20-sample workflow are defined in [`diagnosis-production-provider-qualification.md`](./diagnosis-production-provider-qualification.md).

## Current production decision

DGS-SAFE-090 closed while production still used a now-retired MiMo route. The follow-on provider migration aligns the tracked production Diagnosis route with the staging-accepted physical model:

```text
staging target           = openai/gemini-3.7-flash
production target        = openai/gemini-3.7-flash
credential boundary      = PRIMARY_LLM_BASE_URL / PRIMARY_LLM_API_KEY
```

Production promotion remains **HOLD** until the updated production runtime is deployed, its logical route probe attests `openai/gemini-3.7-flash` with zero fallback, and the isolated 20-sample production-candidate provider acceptance succeeds. The pre-migration production probe found the retired MiMo credential missing and the OpenRouter fallback credential expired; that evidence is historical and must not be treated as the current route after migration.

This HOLD is not a failure of DGS-SAFE-090 or Diagnosis v10. It is the intended fail-closed production promotion decision until the replacement production provider route is qualified.

## Evidence artifacts

- `apps/ai-service/data/evals/reports/diagnosis_historical_identity_audit.json`
- `apps/ai-service/data/evals/reports/diagnosis_dgs_safe_090_operational_audit.json`
- `apps/ai-service/data/evals/reports/diagnosis_production_promotion_readiness.json`
- `apps/ai-service/data/evals/reports/diagnosis_v10_final_acceptance.json`

Operational reports are evidence, not mutable control-plane state.
