# PostgreSQL 18 Production Cutover

BodySense supports PostgreSQL / pgvector **18 only** for development, staging, CI and the steady-state production runtime.
The API/DR runtime also carries PostgreSQL 18 client tooling (`psql`, `pg_dump`, `pg_restore`, `createdb`, `dropdb`), so backups and restore drills never cross a server/client major-version boundary.

## Why a guarded cutover exists

The Alibaba production host was still running PostgreSQL 16 when the PG18-only contract was adopted. PostgreSQL data directories are not forward-compatible across major versions, and the PG18 container layout uses a versioned `PGDATA` below `/var/lib/postgresql`. Therefore production must never change only the image tag while reusing the PG16 volume.

The release watcher owns the one-time 16 -> 18 transition:

1. require zero actively leased Consultation runs;
2. create and validate the normal pre-deploy logical backup;
3. synchronize the coherent release runtime;
4. stop Caddy, API and AI so no external writes can occur during the database cutover;
5. create a final custom-format PG16 dump and SHA-256 sidecar;
6. preserve the old PG16 Docker volume untouched;
7. start PG18 on the independent `bodysense-postgres-pg18` volume mounted at `/var/lib/postgresql`;
8. restore with PG18 tooling, then verify schema state, pgvector presence and an exact public-table row-count digest;
9. start LiteLLM, AI, API and Web and wait for internal health;
10. commit PG18 as the durable database boundary;
11. only then expose Caddy and run the public health check.

Before step 10, a failure can recreate PG16 from the archived previous Compose/runtime and its untouched source volume without losing writes, because Caddy has remained down. After step 10 the watcher never moves back to PG16; a later application failure may roll back application images, but the PG18 runtime and data remain authoritative.

## Volumes

Steady-state PG18 uses:

```text
bodysense-postgres-pg18 -> /var/lib/postgresql
```

The previous `docker_postgres-prod-data` PG16 volume is deliberately not deleted by the cutover. Retain it as a rollback/forensic artifact until the operator explicitly closes the migration retention window. It is not part of the PG18 Compose contract and must not receive new writes after cutover.

## Verification

After cutover:

```bash
ssh ali-bodysense

docker exec docker-postgres-1 psql -U bodysense -d bodysense -Atc 'show server_version'
docker exec docker-postgres-1 psql -U bodysense -d bodysense -Atc \
  "select version::text || ':' || dirty::text from schema_migrations order by version desc limit 1"
cat /opt/bodysense/.postgres-major-upgrade-state
cat /opt/bodysense/.deploy-state
curl -fsS https://body.bakersean.top/api/health
```

Acceptance requires PostgreSQL major 18, `schema_migrations` clean at the repository latest version, a committed major-upgrade state for the release that performed the cutover, coherent application revisions and public health success.

## CI contract

CI intentionally keeps two database **scenarios**, but both run PostgreSQL 18:

- `PostgreSQL 18 current-history replay`: empty database -> full current migration history -> replay validation;
- `PostgreSQL 18 production-baseline upgrade`: production-v29 schema fixture -> latest migrations -> domain semantics -> dump/restore recovery.

The second scenario protects production upgrade semantics. It is not PostgreSQL 16 compatibility testing.
