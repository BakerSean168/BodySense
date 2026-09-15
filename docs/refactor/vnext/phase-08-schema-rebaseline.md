# Phase 08 — Database schema rebaseline

Status: **COMPLETE — full Phase 08 acceptance is green; ready for integration**

Branch: `refactor/vnext-08-schema-rebaseline`

Canonical parent before Phase 08: `refactor/bodysense-vnext` at `b924468f8` (Phase 07 merge, PR #180).

## Goal

Replace pre-vNext migration history as active runtime baggage with one clean PostgreSQL 18 baseline that describes current BodySense semantics only. Git history remains the archive for the retired 1–63 evolution chain; runtime databases are reset explicitly because there is no business data that requires migration through the engineering reset.

## Schema audit

The Phase 07 accepted historical chain was first materialized on a fresh `pgvector/pgvector:pg18` database before any baseline was authored:

```text
migration_state=63:false
48 tables including schema_migrations
677 columns
```

Code-reference and ownership review identified five columns with no current runtime authority:

```text
assessment_reports.health_grade
assessment_reports.dimension_scores
consultation_sessions.health_features
thread_projections.health_features
knowledge_units.lifecycle_metadata
```

The two Assessment fields belonged to retired output-v1 semantics. The `health_features` fields belonged to the superseded session/projection health model. `knowledge_units.lifecycle_metadata` was the migration-19 precursor to the current indexed `lifecycle_status` contract introduced in migration 20.

Rollout observation tables are intentionally retained. Assessment, Consultation, Diagnosis and Treatment still wire their current rollout observers/repositories through the Go server, so these tables are current operational evidence rather than migration residue.

Knowledge source/clip path fields are likewise retained: they describe current curated Knowledge artifacts and are not the retired user-upload `file_path` coordinate.

## Canonical baseline

The active migration tree now contains only:

```text
000001_vnext_baseline.up.sql
000001_vnext_baseline.down.sql
checksums.sha256
```

`000001_vnext_baseline.up.sql` was generated from the accepted PostgreSQL 18 schema after physically removing the five dead columns. It contains the current extensions, helper function, 47 application tables, constraints, indexes and triggers. `schema_migrations` remains owned by golang-migrate and is not embedded in the baseline SQL.

The down migration removes the application schema and helper trigger function while intentionally leaving PostgreSQL extensions installed; replay is therefore safe on a disposable validation database without pretending BodySense owns cluster-wide extension lifecycle.

The migration validator no longer accepts `-baseline-version`. With a one-migration baseline it explicitly accepts the nil migration state after `Steps(-1)` and then proves replay to version 1.

Fresh PostgreSQL 18 evidence already passes:

```text
FULL_UP=PASS version=1
LATEST_DOWN=PASS version=nil
LATEST_REPLAY_UP=PASS version=1
BODY_STATE_SEMANTICS=PASS
BODY_REGION_ID_ROUNDTRIP=PASS
TREATMENT_ACTIVATION_ATOMICITY=PASS
OUTCOME_FEEDBACK_ATOMICITY=PASS
DOMAIN_SEMANTICS=PASS
```

## Schema evidence boundaries

Phase 00 evidence remains frozen at `docs/refactor/vnext/baseline/database-schema.json` (`62:false`). Phase 08 does not rewrite the historical starting point.

The current canonical snapshot is independently generated at:

```text
docs/refactor/vnext/schema/database-schema.json
```

Current snapshot:

```text
migration_state=1:false
48 tables including schema_migrations
672 columns
```

It explicitly contains none of the five retired columns. `pnpm refactor:vnext:schema` now regenerates this current snapshot by default; callers can override the destination with `BODYSENSE_SCHEMA_SNAPSHOT_OUT` when a separate evidence capture is required.

## CI and recovery contract

Database CI now tests current semantics instead of historical upgrade compatibility:

1. **PostgreSQL 18 vNext baseline child** — empty DB, baseline up/down/replay, then domain semantics.
2. **PostgreSQL 18 vNext recovery child** — the same clean baseline plus PostgreSQL 18 `pg_dump` / `pg_restore`, restored-domain validation and the production off-host DR algorithm.

The `production-v29.sql` fixture and production-baseline upgrade lane are removed from the active tree. `scripts/validate-migration-history.sh` now requires a contiguous vNext chain beginning at `000001_vnext_baseline` and exact checksum coverage.

Off-host restore no longer exposes `--baseline-version`; restored backups are validated against the current active migration chain. The migration validator now has an explicit destructive-replay switch: disposable empty-schema validation keeps `-replay-latest=true`, while data-bearing restore validation passes `-replay-latest=false` so it may move forward to current schema but can never execute a `down` step against restored user data. This distinction was added after the rebaseline correctly exposed that baseline `1 -> nil` would otherwise drop the entire restored schema. The focused DR integration now proves `schema=1:false data_round_trip=verified`, and the unit suite asserts the non-destructive validator argument.

## Explicit environment reset

A database already stamped `63:false` must never be coerced to migration `1`; every runtime environment enters vNext through a fresh database.

- **Development** switches its named volume to `postgres-dev-vnext-data`.
- **Staging** switches its named volume to `postgres-staging-vnext-data`.
- **Production** switches `POSTGRES_DATA_VOLUME` to `bodysense-postgres-vnext`.

Production uses the existing transactional PostgreSQL reset operator with an important vNext extension: a currently running PostgreSQL 18 instance is reset when its mounted source volume differs from the new target volume. The previous source volume remains available until API/AI/Web health gates pass; commit deletes it, while pre-commit failure can restore the previous runtime/env/volume. This gives the schema rebaseline a real rollback boundary without manufacturing a data migration.

`POSTGRES18_PRODUCTION_CONTRACT=PASS` validates the updated production volume, CI database lanes and bootstrap ordering.

## Final acceptance evidence

Phase 08 completed the full repository and production-shaped acceptance contract on the final implementation diff:

```text
SCHEMA_SNAPSHOT_DETERMINISTIC=PASS
PHASE07_TO_VNEXT_COLUMN_EQUIVALENCE=PASS old=677 new=672 intentional_retirements=5
MIGRATION_SEQUENCE=PASS latest=1
MIGRATION_IMMUTABILITY=PASS
POSTGRES18_PRODUCTION_CONTRACT=PASS
offhost DR unit tests: PASS=88 FAIL=0
```

The application repository is also green:

```text
git diff --check=PASS
go test ./...=PASS
go vet ./...=PASS
pnpm lint=PASS
pnpm typecheck=PASS
pnpm test=PASS
pnpm build=PASS
pnpm contracts:verify=PASS
public REST coverage=95/95
```

`pnpm validate:local-deploy` then built a fresh production-shaped stack on the new vNext database volume and proved the complete vertical from the one-migration baseline:

```text
API_HEALTH=PASS
AI_HEALTH=PASS
WEB_HEALTH=PASS
FULL_UP=PASS version=1
LATEST_DOWN=PASS version=nil
LATEST_REPLAY_UP=PASS version=1
BODY_STATE_SEMANTICS=PASS
BODY_REGION_ID_ROUNDTRIP=PASS
TREATMENT_ACTIVATION_ATOMICITY=PASS
OUTCOME_FEEDBACK_ATOMICITY=PASS
DOMAIN_SEMANTICS=PASS
KNOWLEDGE_PUBLICATION_VERTICAL=PASS
KNOWLEDGE_ROLLBACK_VERTICAL=PASS
Playwright=10/10 PASS
DIAGNOSIS_BASELINE_VALIDATION=PASS current=3 non_current=0 rollout_observations=0
TREATMENT_BASELINE_VALIDATION=PASS current=3 non_current=0 rollout_observations=0
TREATMENT_DECISION_TRACE_VALIDATION=PASS accepted_traces=2
TREATMENT_REPLAY_INPUT_VALIDATION=PASS replay_inputs=3
LOCAL_DEPLOY_VALIDATION=PASS
```

The Phase 07 to vNext structural comparison is intentionally column-scoped: after excluding exactly the five retired migration-era columns, the generated Phase 07 PostgreSQL 18 snapshot and the Phase 08 snapshot are identical at the table/column contract. Constraints, indexes, triggers and operational semantics are independently covered by the pg_dump-derived baseline, migration replay, domain validators, repository tests and production-shaped E2E acceptance above.
