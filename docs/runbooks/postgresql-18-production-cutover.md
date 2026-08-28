# PostgreSQL 18 Production Reset

BodySense supports PostgreSQL / pgvector **18 only** for development, staging, CI and production.

## Production transition decision

The Alibaba production PostgreSQL 16 database contains no production data that needs to be retained. The transition therefore does **not** migrate, restore or preserve that database. The release watcher performs a destructive legacy reset and lets the Go API initialize the fresh PostgreSQL 18 database through the canonical migration history.

The old PostgreSQL data directory is never mounted into PostgreSQL 18.

## One-time release flow

For the release that performs the transition:

1. normal deploy preflight requires zero actively leased Consultation runs;
2. the coherent release runtime is synchronized;
3. Caddy, API and AI are stopped so no writes occur during the database switch;
4. the existing legacy PostgreSQL container is replaced by `pgvector:pg18` on `bodysense-postgres-pg18`;
5. PostgreSQL 18 must become healthy and report major `18`;
6. AI, API and Web are started; API bootstrap runs the canonical migrations on the fresh database;
7. after all application health gates pass, the legacy PostgreSQL volume is deleted immediately;
8. only then is Caddy exposed and the public `/api/health` check executed.

The old volume exists only during steps 4–6 as a short infrastructure rollback window in case the new PostgreSQL service itself cannot start. It is not a data-retention mechanism and is deleted by the same successful deployment transaction.

## Steady-state volume

```text
bodysense-postgres-pg18 -> /var/lib/postgresql
```

After successful cutover, no PostgreSQL 16 runtime or data volume is part of the BodySense production contract.

## Verification

```bash
ssh ali-bodysense

docker exec docker-postgres-1 psql -U bodysense -d bodysense -Atc 'show server_version'
docker exec docker-postgres-1 psql -U bodysense -d bodysense -Atc \
  "select version::text || ':' || dirty::text from schema_migrations order by version desc limit 1"
cat /opt/bodysense/.postgres18-reset-state
cat /opt/bodysense/.deploy-state
docker volume ls --format '{{.Name}}' | grep -E 'postgres|pg18'
curl -fsS https://body.bakersean.top/api/health
```

Acceptance requires PostgreSQL major 18, clean latest migration state, only the PG18 production data volume, coherent application revisions and public health success.

## CI contract

CI keeps two database scenarios and both use PostgreSQL 18:

- `PostgreSQL 18 current-history replay` — empty database through the current migration history plus replay validation;
- `PostgreSQL 18 production-baseline upgrade` — historical schema-shape fixture normalized under PG18 through latest migrations, domain semantics and PG18 dump/restore recovery.

Neither job tests or supports the PostgreSQL 16 engine.
