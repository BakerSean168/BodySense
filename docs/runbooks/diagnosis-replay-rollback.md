# Diagnosis Historical Replay and Rollback Runbook

> Owner: Diagnosis release governance  
> Scope: DGS-SAFE-090  
> Safety rule: rollback changes the future serving pointer only. It must never rewrite historical `diagnosis_analyses`, `diagnosis_candidates`, replay inputs, raw outputs, DecisionTrace, or execution provenance.

## 1. Canonical identities

Current structured-safety Champion:

```text
Diagnosis v10
diag-config-3f64de162dc937ee
```

Historical rollback Champion:

```text
Diagnosis v3
diag-config-5a4a13627e14b4cf
```

Historical replay identities retained by contract:

| Version | Configuration ID |
| --- | --- |
| v3 | `diag-config-5a4a13627e14b4cf` |
| v4 | `diag-config-4a517fea19cb6c49` |
| v5 | `diag-config-375187050b203078` |
| v6 | `diag-config-4377355ba2012ce8` |
| v7 | `diag-config-4eb948f419994367` |

These identities remain audit/replay artifacts after they leave the serving set. Do not mutate or delete their manifests as runtime cleanup.

## 2. Required invariants

```text
historical v3-v7 manifests resolve to their original IDs
old replay JSON without SafetyEnvelope remains decodable
historical replay uses frozen input and does not persist a replacement analysis
pre-existing DiagnosisAnalysis rows remain unchanged
pre-existing DiagnosisCandidate rows remain unchanged
new requests use the selected Champion after pointer movement
final state restores v10 Champion with no Challenger/promotion record
```

## 3. Historical identity audit

Repository/CI audit:

```bash
cd apps/ai-service
uv run --extra dev python scripts/run_diagnosis_historical_identity_audit.py
```

Required result: 5/5 v3-v7 identities accepted. The committed report is `apps/ai-service/data/evals/reports/diagnosis_historical_identity_audit.json`.

## 4. Historical database replay audit

`domain-validator` is already included in the API runtime image. DGS-SAFE-090 adds a read-only historical replay mode:

```bash
docker exec bodysense-staging-api-1 \
  /app/domain-validator \
  -mode diagnosis-replay-audit \
  -replay-limit-per-configuration 1000 \
  > /tmp/diagnosis-replay-audit.json
```

The audit considers v3-v7 only, reads analyses that contain frozen replay input, calls `HistoricalReplay` with no AI client, requires artifact/hard/semantic/presentation replay invariants to match, and never persists a replacement analysis.

A historical version with no database rows is not fabricated. Its identity and replay reachability remain covered by repository fixtures plus the v3-v7 identity audit.

## 5. Capture the immutable history boundary

Before rollback pointer movement, capture a host-side snapshot:

```bash
docker exec bodysense-staging-api-1 \
  /app/domain-validator \
  -mode diagnosis-history-snapshot \
  > /tmp/diagnosis-history-before.json
```

The snapshot contains only analysis/candidate UUIDs, SHA-256 hashes, counts, and an aggregate root. It does not export health text, replay input, raw output, or profile data.

Keep the snapshot on the operator host. Do not store it only inside the API container because that container is recreated during rollback.

## 6. Acquire the deployment lock

```bash
LOCK=/home/dev/.local/state/bodysense/staging-deploy.lock
exec 9>"$LOCK"
flock -w 60 9
```

Expected normal state:

```text
DIAGNOSIS_ROLLOUT_STAGE=champion
DIAGNOSIS_CHAMPION_CONFIGURATION_ID=diag-config-3f64de162dc937ee
DIAGNOSIS_CHALLENGER_CONFIGURATION_ID=
DIAGNOSIS_PROMOTION_RECORD=
```

## 7. Roll back the serving pointer to v3

Change only these Diagnosis rollout keys:

```text
DIAGNOSIS_CHAMPION_CONFIGURATION_ID=diag-config-5a4a13627e14b4cf
DIAGNOSIS_CHALLENGER_CONFIGURATION_ID=
DIAGNOSIS_ROLLOUT_STAGE=champion
DIAGNOSIS_PROMOTION_RECORD=
```

Do not change provider credentials, database state, release tags, or historical manifests. Recreate only API using the already deployed immutable API image; do not rebuild source or move the coherent release pointer.

## 8. Public Diagnosis smoke after rollback

Use the dedicated synthetic staging subject through the normal authenticated public API. Required evidence:

```text
status = completed
governance.verdict = accepted
decision_trace.rollout_provenance.stage = champion
agent_configuration_id = diag-config-5a4a13627e14b4cf
served_configuration_id = diag-config-5a4a13627e14b4cf
champion_configuration_id = diag-config-5a4a13627e14b4cf
```

An env-only check is not sufficient; a public Diagnosis request must prove routing changed.

## 9. Verify historical immutability while v3 serves

```bash
cat /tmp/diagnosis-history-before.json | \
docker exec -i bodysense-staging-api-1 \
  /app/domain-validator \
  -mode diagnosis-history-verify \
  -snapshot-file - \
  > /tmp/diagnosis-history-verify-v3.json
```

Required result:

```text
unchanged = true
missing_analysis_ids = []
mutated_analysis_ids = []
missing_candidate_ids = []
mutated_candidate_ids = []
```

`added_analysis_count` and `added_candidate_count` may be positive because the smoke creates a new immutable analysis. Any missing or mutated protected row is an immediate failure.

## 10. Restore v10

Restore only the pointer:

```text
DIAGNOSIS_CHAMPION_CONFIGURATION_ID=diag-config-3f64de162dc937ee
DIAGNOSIS_CHALLENGER_CONFIGURATION_ID=
DIAGNOSIS_ROLLOUT_STAGE=champion
DIAGNOSIS_PROMOTION_RECORD=
```

Recreate only API with the same coherent release image, wait for health, then repeat the public Diagnosis smoke. The served and Champion IDs must both be v10.

Run history verification again against the same pre-rollback snapshot. All protected rows must remain unchanged.

## 11. Final expected state

```text
stage = champion
Champion = diag-config-3f64de162dc937ee
Challenger = empty
PromotionRecord = empty
API = healthy
AI service = healthy
LiteLLM gateway = healthy
```

The pre-rollback root and protected post-restore root must match.

## 12. Production promotion is a separate decision

DGS-SAFE-090 does not automatically move production to v10.

```bash
cd apps/ai-service
uv run --extra dev python scripts/run_diagnosis_production_promotion_readiness.py
```

At the time of DGS-SAFE-090:

```text
staging bodysense-diagnosis -> openai/gemini-3.7-flash
production bodysense-diagnosis -> openai/mimo-v2.5-pro
```

Staging Gemini acceptance cannot be silently reused as MiMo production evidence. Before production can become PROMOTE, produce paced production-candidate evidence:

```bash
python scripts/run_diagnosis_provider_acceptance.py \
  --samples 20 \
  --min-start-interval-seconds 5 \
  --environment production-candidate \
  --physical-model openai/mimo-v2.5-pro \
  --json-output data/evals/reports/diagnosis_v10_production_provider_acceptance.json
```

The production readiness gate requires v10 final acceptance, historical identity audit, DGS-SAFE-090 operational audit, 20/20 production-provider execution, zero provider/contract/governance/configuration failures, and a physical model matching `docker/litellm/config.yaml`.

Until that evidence exists, the correct production decision is HOLD.

## 13. Forbidden operations

Never use rollback to:

- update `agent_configuration_id` on old analyses;
- regenerate or overwrite `raw_output`;
- backfill missing historical SafetyEnvelope assertions;
- rewrite `replay_input`;
- delete failed rollout observations;
- mutate v3-v7 manifests while retaining their IDs;
- move production pointers merely because staging is green;
- bypass the coherent release watcher;
- rebuild an image during rollback.

Rollback is pointer movement plus verification, not historical migration.

## 14. Required DGS-SAFE-090 evidence

The work item closes only when the repository retains:

```text
diagnosis_historical_identity_audit.json
diagnosis_dgs_safe_090_operational_audit.json
diagnosis_production_promotion_readiness.json
```

The operational report must record only non-sensitive operational evidence: historical replay counts, failures, protected counts/root hash, public v10/v3/v10 smokes, immutability checks after rollback and restore, final Champion identity, and coherent runtime revision.
