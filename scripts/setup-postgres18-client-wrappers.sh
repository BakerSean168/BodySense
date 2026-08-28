#!/usr/bin/env bash
set -Eeuo pipefail

OUT_DIR="${1:?usage: setup-postgres18-client-wrappers.sh <output-dir>}"
IMAGE="${POSTGRES18_CLIENT_IMAGE:-pgvector/pgvector:pg18}"
REPO_ROOT="$(git rev-parse --show-toplevel)"
mkdir -p "$OUT_DIR"

cat > "$OUT_DIR/.pg18-client-dispatch" <<SCRIPT
#!/usr/bin/env bash
set -Eeuo pipefail
tool="\$(basename "\$0")"
exec docker run --rm --network host \\
  -e PGHOST -e PGPORT -e PGDATABASE -e PGUSER -e PGPASSWORD -e PGSSLMODE \\
  -v "$REPO_ROOT:$REPO_ROOT" -v /tmp:/tmp -w "\$PWD" \\
  "$IMAGE" "\$tool" "\$@"
SCRIPT
chmod 0755 "$OUT_DIR/.pg18-client-dispatch"
for tool in psql pg_dump pg_restore createdb dropdb; do
  ln -sf .pg18-client-dispatch "$OUT_DIR/$tool"
done

"$OUT_DIR/pg_dump" --version | grep -Eq '^pg_dump \(PostgreSQL\) 18\.'
echo "POSTGRES18_CLIENT_WRAPPERS=PASS dir=$OUT_DIR image=$IMAGE"
