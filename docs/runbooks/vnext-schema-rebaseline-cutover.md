# BodySense vNext Schema Rebaseline Cutover

BodySense supports PostgreSQL / pgvector **18 only**. Phase 08 replaces the pre-user migration history with one clean vNext baseline and deliberately resets runtime databases instead of simulating an upgrade of data that does not need preservation.

## Cutover decision

The application has no business data that requires migration across the engineering reset. The active migration directory therefore starts at `000001_vnext_baseline`, while Git history remains the archive for migrations 1–63.

The production release switches PostgreSQL 18 to a **new named volume**:

```text
bodysense-postgres-vnext -> /var/lib/postgresql
```

Changing the target volume is the explicit reset signal. This works even when the currently running source database is already PostgreSQL 18: the reset operator compares the mounted source volume with `POSTGRES_DATA_VOLUME`, not only the server major version.

## Transactional release flow

For the release that performs the rebaseline:

1. deploy preflight requires zero actively leased Consultation runs;
2. the coherent release runtime is synchronized;
3. Caddy, API and AI are stopped so no writes occur during the database switch;
4. the current PostgreSQL source volume remains untouched while a fresh `bodysense-postgres-vnext` volume is created;
5. PostgreSQL must become healthy and report major `18`;
6. API starts first and bootstraps `000001_vnext_baseline`; AI starts only after API health proves the schema exists;
7. Web and the remaining application health gates run against the new database;
8. if a gate fails before commit, the reset operator can restore the previous Compose/env snapshot and source volume;
9. after all health gates pass, the reset is committed and the previous PostgreSQL volume is deleted;
10. only then is Caddy exposed and public health checked.

The previous volume is a short rollback mechanism, not a data-migration source and not a long-lived backup.

## Expected schema state

After bootstrap:

```text
schema_migrations = 1:false
```

The vNext baseline intentionally excludes migration-era columns that have no current runtime authority, including the retired Assessment score columns, the two `health_features` projection columns, and the obsolete Knowledge `lifecycle_metadata` column.

## Verification

```bash
ssh ali-bodysense

docker exec docker-postgres-1 psql -U bodysense -d bodysense -Atc 'show server_version'
docker exec docker-postgres-1 psql -U bodysense -d bodysense -Atc \
  "select version::text || ':' || dirty::text from schema_migrations order by version desc limit 1"
cat /opt/bodysense/.postgres18-reset-state
cat /opt/bodysense/.deploy-state
docker volume ls --format '{{.Name}}' | grep -E 'postgres|vnext'
curl -fsS https://body.bakersean.top/api/health
```

Acceptance requires PostgreSQL major 18, migration state `1:false`, `bodysense-postgres-vnext` as the active production data volume, coherent application revisions and public health success.

## CI contract

CI keeps two database scenarios, both on PostgreSQL 18:

- `PostgreSQL 18 vNext baseline child` — empty database through baseline `up -> down -> replay up`, followed by domain semantics;
- `PostgreSQL 18 vNext recovery child` — the same clean baseline plus `pg_dump` / `pg_restore`, restored-domain validation and the production off-host DR algorithm.

There is no active production-v29 fixture and no pre-vNext schema-upgrade lane.
