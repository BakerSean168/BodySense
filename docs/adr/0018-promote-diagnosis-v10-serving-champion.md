# ADR 0018: Promote Diagnosis v10 as the Current Serving Champion

## Status

Accepted.

## Date

2026-09-30

## Context

Diagnosis v3 (`diag-config-5a4a13627e14b4cf`) was the long-lived serving baseline while the structured-safety successor line was developed and qualified. ADR 0017 introduced typed SafetyEnvelope semantics and preserved v3-v7 as immutable historical configurations. DGS-SAFE-080 then established a standalone v10 acceptance contract, and DGS-SAFE-090 proved that v3-v7 remain replayable and that serving-pointer rollback does not rewrite historical Diagnosis rows.

The production promotion gate was intentionally separate from staging acceptance because the physical provider route is owned by LiteLLM deployment rather than by the immutable Agent configuration fingerprint. Before promotion, production was migrated from the retired MiMo route to `openai/gemini-3.7-flash` behind the generic `PRIMARY_LLM_*` credential boundary. Both isolated production-candidate qualification and a second run against the real production LiteLLM gateway completed 20/20 with zero provider errors, contract failures, governance rejections, or configuration mismatches.

Release `v0.14.0` was published from exact revision `1f71f76622ce2cb06120ab5f2ecca84d470faa4c` using the canonical release-manifest/digest promotion path. The production deploy watcher created a validated database backup, reconciled the coherent release set, and reported a successful deployment before any Diagnosis serving-pointer change.

## Decision

Diagnosis v10 (`diag-config-3f64de162dc937ee`) is the current serving Champion and repository default.

The current default is aligned across:

- the Go control plane `defaultDiagnosisConfigurationID`;
- Python `get_default_diagnosis_configuration()`;
- dev, staging, and production Compose defaults;
- production-shaped local deployment validation.

Diagnosis v3 remains an immutable historical rollback/replay identity. Historical promotion records and paired-evaluation fixtures continue to bind explicitly to the configuration IDs that existed when those experiments ran; they do not inherit the mutable/current default.

Production `bodysense-diagnosis` routes through LiteLLM to `openai/gemini-3.7-flash`. Provider qualification is a release-readiness gate and does not change the Agent configuration fingerprint.

The production promotion sequence is:

1. qualify immutable v10 deterministically and against the real provider;
2. complete DGS-SAFE-090 replay/rollback operational acceptance;
3. require production provider readiness = `PROMOTE`;
4. publish and deploy one coherent immutable application release;
5. switch only the Diagnosis Champion pointer from v3 to v10;
6. prove the public route serves v10;
7. rehearse v10 -> v3 -> v10 without changing the application release;
8. verify the protected historical root is unchanged after rollback and restore.

## Production acceptance evidence

The canonical audit artifact is:

`apps/ai-service/data/evals/reports/diagnosis_v10_production_promotion_audit.json`

It records:

- release `v0.14.0` and its canonical manifest digest;
- production provider readiness with Gemini 3.7 Flash and zero fallback;
- public v3 pre-promotion serving evidence;
- public v10 promotion evidence;
- a real v10 -> v3 -> v10 rollback rehearsal;
- a protected-history baseline of 2 Diagnosis analyses and 1 candidate;
- identical protected root
  `355831b2f7ecad395bd0d80e450a0aa0eb93a7c75b01bd308ae5fa10c779ffb1`
  after rollback and restore;
- final production state: v10 Champion, no Challenger, no promotion record, rollout stage `champion`;
- all application services healthy on release revision
  `1f71f76622ce2cb06120ab5f2ecca84d470faa4c`.

The synthetic public probe also reproduced the migration behavior that motivated the structured-safety successor: under v3 the bounded test case was `safety_blocked / rejected`, while under v10 the same durable state completed with accepted governance and a candidate. This observation is evidence of the serving change, not a rule that every v3/v10 pair must differ in this way.

## Consequences

- New clean environments serve v10 even without an operator override.
- Removing `DIAGNOSIS_CHAMPION_CONFIGURATION_ID` no longer silently falls back to v3.
- Historical v3-v7 manifests, analyses, replay inputs, promotion policies, and observations remain unchanged and resolvable.
- Emergency rollback to v3 remains an operator serving-pointer action; rollback does not rewrite old analyses.
- Historical paired-evaluation tools must name their historical baseline explicitly instead of relying on the repository serving default.
- A future Diagnosis successor must repeat immutable qualification, provider/runtime readiness, coherent release deployment, explicit owner promotion, and rollback verification. It must not mutate v10 in place.
