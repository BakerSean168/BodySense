#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

pick_port() {
  python3 - <<'PYPORT'
import socket
with socket.socket() as sock:
    sock.bind(("127.0.0.1", 0))
    print(sock.getsockname()[1])
PYPORT
}

: "${THOUGHT_FOREST_REPO:=$(dirname "$repo_root")/thought-forest}"
: "${COMPOSE_PROJECT_NAME:=bodysense-kb-qualifier-$$}"
: "${DB_USER:=bodysense}"
: "${DB_PASSWORD:=bodysense-kb-validator}"
: "${DB_NAME:=bodysense}"
: "${DB_PORT:=$(pick_port)}"
: "${REDIS_PORT:=$(pick_port)}"
: "${LITELLM_PORT:=$(pick_port)}"
: "${API_PORT:=$(pick_port)}"
: "${AI_SERVICE_PORT:=$(pick_port)}"
: "${JWT_SECRET_KEY:=bodysense-kb-validator-jwt-secret}"
: "${LITELLM_MASTER_KEY:=sk-bodysense-kb-validator}"
: "${EMBEDDING_PROVIDER:=hashing}"

export COMPOSE_PROJECT_NAME DB_USER DB_PASSWORD DB_NAME DB_PORT REDIS_PORT LITELLM_PORT API_PORT AI_SERVICE_PORT
export JWT_SECRET_KEY LITELLM_MASTER_KEY EMBEDDING_PROVIDER

compose=(docker compose -p "$COMPOSE_PROJECT_NAME" -f docker/docker-compose.yml --profile dev)
reviewed_artifact="$repo_root/docs/knowledges/evidence/reviewed-knowledge-pain-nociception-cohort.json"
evidence_review="$repo_root/docs/knowledges/evidence/external-evidence-review-pain-nociception-cohort.json"
claim_review="$repo_root/docs/knowledges/evidence/claim-review-pain-nociception-cohort.json"
eval_cases="$repo_root/docs/knowledges/eval/published-knowledge-pain-nociception-cohort.jsonl"
eval_thresholds="$repo_root/docs/knowledges/eval/published-knowledge-pain-nociception-cohort.thresholds.json"
operator_id="00000000-0000-4000-8000-000000000231"
publication_key="pain-nociception-local-qualified-v1"
batch_key="pain-nociception-local-qualified"
rollback_key="pain-nociception-local-qualified-v1-rollback"

if [[ ! -d "$THOUGHT_FOREST_REPO/.git" && ! -f "$THOUGHT_FOREST_REPO/.git" ]]; then
  echo "THOUGHT_FOREST_SOURCE=FAIL repo=$THOUGHT_FOREST_REPO" >&2
  exit 1
fi
if [[ ! -x "$THOUGHT_FOREST_REPO/node_modules/.bin/tsx" ]]; then
  echo "THOUGHT_FOREST_SOURCE=FAIL missing tsx dependency" >&2
  exit 1
fi

read -r source_commit expected_snapshot_id < <(python3 - "$reviewed_artifact" <<'PY'
import json, sys
payload=json.load(open(sys.argv[1], encoding="utf-8"))
print(payload["source_git_commit"], payload["source_snapshot_id"])
PY
)

worktree="$(mktemp -d /tmp/bodysense-tf-candidate-XXXXXX)"
snapshot="$worktree/generated/bodysense-health/health-snapshot.json"
report_host="$(mktemp /tmp/bodysense-kb-qualification-XXXXXX.json)"

cleanup() {
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  if git -C "$THOUGHT_FOREST_REPO" worktree list --porcelain 2>/dev/null | grep -Fq "worktree $worktree"; then
    git -C "$THOUGHT_FOREST_REPO" worktree remove --force "$worktree" >/dev/null 2>&1 || true
  else
    rm -rf "$worktree"
  fi
  rm -f "$report_host"
}
trap cleanup EXIT

"${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true

git -C "$THOUGHT_FOREST_REPO" cat-file -e "$source_commit^{commit}"
git -C "$THOUGHT_FOREST_REPO" worktree add --detach "$worktree" "$source_commit" >/dev/null
rm -rf "$worktree/node_modules"
ln -s "$THOUGHT_FOREST_REPO/node_modules" "$worktree/node_modules"
(
  cd "$worktree"
  ./node_modules/.bin/tsx scripts/bodysense-export/export-health-snapshot.ts \
    --allow-dirty \
    --output generated/bodysense-health/health-snapshot.json >/tmp/bodysense-kb-export.log
)
actual_snapshot_id="$(python3 - "$snapshot" <<'PY'
import json, sys
payload=json.load(open(sys.argv[1], encoding="utf-8"))
print(payload["snapshot_id"])
PY
)"
if [[ "$actual_snapshot_id" != "$expected_snapshot_id" ]]; then
  echo "KNOWLEDGE_SOURCE_SNAPSHOT=FAIL expected=$expected_snapshot_id actual=$actual_snapshot_id" >&2
  exit 1
fi
echo "KNOWLEDGE_SOURCE_SNAPSHOT=PASS id=$actual_snapshot_id"

"${compose[@]}" up -d --build --wait --wait-timeout 180 postgres-dev redis-dev litellm-gateway ai-service api

version="$("${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -Atc "SELECT version, dirty FROM schema_migrations;")"
if [[ "$version" != "56|f" ]]; then
  echo "KNOWLEDGE_CANDIDATE_SCHEMA=FAIL state=$version" >&2
  exit 1
fi
echo "KNOWLEDGE_CANDIDATE_SCHEMA=PASS state=$version"

"${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO users (id, email, password_hash, role)
VALUES ('$operator_id'::uuid, 'knowledge-candidate-operator@invalid.local', 'not-a-login-hash', 'operator');
SQL

api_cid="$("${compose[@]}" ps -q api)"
ai_cid="$("${compose[@]}" ps -q ai-service)"
docker cp "$snapshot" "$api_cid:/tmp/health-snapshot.json"
docker cp "$snapshot" "$ai_cid:/tmp/health-snapshot.json"
docker cp "$reviewed_artifact" "$api_cid:/tmp/reviewed-knowledge.json"
docker cp "$evidence_review" "$ai_cid:/tmp/external-evidence-review.json"
docker cp "$claim_review" "$ai_cid:/tmp/claim-review.json"
docker cp "$eval_cases" "$ai_cid:/tmp/published-cases.jsonl"
docker cp "$eval_thresholds" "$ai_cid:/tmp/published-thresholds.json"

registration_json="$("${compose[@]}" exec -T api /app/knowledge-source-manager register-thought-forest \
  --snapshot /tmp/health-snapshot.json --operator-id "$operator_id")"
python3 - "$registration_json" <<'PY'
import json, sys
report=json.loads(sys.argv[1])
assert report["total_sources"] == 11, report
assert report["registered"] == 11, report
assert report["existing_validated"] == 0, report
print("KNOWLEDGE_SOURCE_REGISTRATION=PASS registered=11")
PY

"${compose[@]}" exec -T ai-service python scripts/ingest_thought_forest_snapshot.py \
  /tmp/health-snapshot.json \
  --evidence-review-manifest /tmp/external-evidence-review.json \
  --claim-review-manifest /tmp/claim-review.json >/tmp/bodysense-kb-ingest.log

prepublish_state="$("${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -Atc "
SELECT concat_ws('|',
  (SELECT count(*) FROM knowledge_sources WHERE source_type='thought_forest_note')::text,
  (SELECT count(*) FROM knowledge_units ku JOIN knowledge_sources ks ON ks.id=ku.source_id WHERE ks.source_type='thought_forest_note')::text,
  (SELECT count(*) FROM knowledge_units ku JOIN knowledge_sources ks ON ks.id=ku.source_id WHERE ks.source_type='thought_forest_note' AND ku.lifecycle_status='reviewed')::text,
  (SELECT count(*) FROM knowledge_units ku JOIN knowledge_sources ks ON ks.id=ku.source_id WHERE ks.source_type='thought_forest_note' AND ku.lifecycle_status='published')::text,
  (SELECT count(*) FROM knowledge_units ku JOIN knowledge_sources ks ON ks.id=ku.source_id WHERE ks.source_type='thought_forest_note' AND ku.metadata ? 'embedding_identity')::text
);")"
if [[ "$prepublish_state" != "11|115|3|0|115" ]]; then
  echo "KNOWLEDGE_CANDIDATE_PREPUBLISH=FAIL state=$prepublish_state" >&2
  exit 1
fi
echo "KNOWLEDGE_CANDIDATE_PREPUBLISH=PASS state=$prepublish_state"

# Re-registering the exact snapshot after ingestion must validate identity rather than mutate it.
reregistration_json="$("${compose[@]}" exec -T api /app/knowledge-source-manager register-thought-forest \
  --snapshot /tmp/health-snapshot.json --operator-id "$operator_id")"
python3 - "$reregistration_json" <<'PY'
import json, sys
report=json.loads(sys.argv[1])
assert report["registered"] == 0, report
assert report["existing_validated"] == 11, report
print("KNOWLEDGE_SOURCE_REREGISTRATION=PASS existing_validated=11")
PY

# Default online retrieval is published-only and therefore must be empty before publication.
"${compose[@]}" exec -T ai-service python - <<'PY'
import asyncio
from src.rag.knowledge_library import KnowledgeLibrary
async def main():
    library=KnowledgeLibrary()
    await library.initialize()
    try:
        results=await library.search("什么是疼痛？", top_k=3, source_type="thought_forest_note", min_quality_score=0.90)
        assert results == [], results
        print("KNOWLEDGE_PREPUBLISH_VISIBILITY=PASS")
    finally:
        await library.close()
asyncio.run(main())
PY

"${compose[@]}" exec -T api /app/knowledge-publication-manager publish-reviewed \
  --publication-key "$publication_key" \
  --batch-key "$batch_key" \
  --reviewed-snapshot /tmp/reviewed-knowledge.json \
  --published-by "$operator_id" \
  --summary "Pain/Nociception local qualification candidate" >/tmp/bodysense-kb-publish.log

published_count="$("${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -Atc "
SELECT count(*) FROM knowledge_units ku
JOIN knowledge_publications kp ON kp.id=ku.publication_id
WHERE ku.lifecycle_status='published' AND kp.publication_key='$publication_key';")"
if [[ "$published_count" != "3" ]]; then
  echo "KNOWLEDGE_CANDIDATE_PUBLICATION=FAIL count=$published_count" >&2
  exit 1
fi
echo "KNOWLEDGE_CANDIDATE_PUBLICATION=PASS published=3"

"${compose[@]}" exec -T ai-service python scripts/run_published_knowledge_eval.py \
  --cases /tmp/published-cases.jsonl \
  --thresholds /tmp/published-thresholds.json \
  --publication-key "$publication_key" \
  --top-k 3 \
  --json-report /tmp/qualification-report.json

docker cp "$ai_cid:/tmp/qualification-report.json" "$report_host"
python3 - "$report_host" <<'PY'
import json, sys
report=json.load(open(sys.argv[1], encoding="utf-8"))
summary=report["summary"]
qualification=report.get("qualification") or {}
assert qualification.get("passed") is True, qualification
assert summary == {
    "cases": 9,
    "passed": 9,
    "pass_rate": 1.0,
    "positive_hits": 6,
    "negative_rejections": 3,
    "citation_valid": 6,
    "grounding_supported": 6,
}, summary
print("KNOWLEDGE_CANDIDATE_QUALIFICATION=PASS cases=9/9 positives=6 negatives=3")
PY

"${compose[@]}" exec -T api /app/knowledge-publication-manager rollback \
  --publication-key "$publication_key" \
  --rollback-key "$rollback_key" \
  --rolled-back-by "$operator_id" \
  --reason "local qualification rollback" >/tmp/bodysense-kb-rollback.log

postrollback_state="$("${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -Atc "
SELECT concat_ws('|',
  (SELECT count(*) FROM knowledge_units ku JOIN knowledge_sources ks ON ks.id=ku.source_id WHERE ks.source_type='thought_forest_note' AND ku.lifecycle_status='reviewed')::text,
  (SELECT count(*) FROM knowledge_units ku JOIN knowledge_sources ks ON ks.id=ku.source_id WHERE ks.source_type='thought_forest_note' AND ku.lifecycle_status='published')::text
);")"
if [[ "$postrollback_state" != "3|0" ]]; then
  echo "KNOWLEDGE_CANDIDATE_ROLLBACK=FAIL state=$postrollback_state" >&2
  exit 1
fi
echo "KNOWLEDGE_CANDIDATE_ROLLBACK=PASS state=$postrollback_state"

echo "KNOWLEDGE_CANDIDATE_VALIDATION=PASS"
