#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

: "${COMPOSE_PROJECT_NAME:=bodysense-validator}"
: "${DB_USER:=bodysense}"
: "${DB_NAME:=bodysense}"
compose=(docker compose -f docker/docker-compose.yml --profile dev)

operator_id="00000000-0000-4000-8000-000000000221"
member_id="00000000-0000-4000-8000-000000000222"
source_key="validator-publication-source-v1"
unit_key="validator-publication-unit-v1"
batch_key="validator-publication-batch"
publication_key="validator-publication-v1"
rollback_key="validator-publication-v1-rollback"
denied_key="validator-publication-denied"
artifact="$(mktemp)"

cleanup() {
  rm -f "$artifact"
  "${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 >/dev/null <<SQL || true
DELETE FROM knowledge_publications WHERE publication_batch_key = '$batch_key' OR publication_key = '$denied_key';
DELETE FROM knowledge_sources WHERE source_key = '$source_key';
DELETE FROM users WHERE id IN ('$operator_id'::uuid, '$member_id'::uuid);
SQL
}
trap cleanup EXIT
cleanup
trap cleanup EXIT

read -r body_hash embedding_fingerprint < <(python3 - <<'PY'
import hashlib
body = "## Validator Definition\n\nReviewed publication validator claim."
identity = "hashing\nbodysense-hashing-ngram\n1536\nsha256-char-word-ngram-v1"
print(hashlib.sha256(body.encode()).hexdigest(), hashlib.sha256(identity.encode()).hexdigest())
PY
)

python3 - "$artifact" "$body_hash" <<'PY'
import json
import sys
from pathlib import Path
path = Path(sys.argv[1])
content_hash = sys.argv[2]
artifact = {
    "schema_version": "bodysense.reviewed-knowledge-snapshot.v1",
    "reviewed_snapshot_id": "reviewed-knowledge:validator-v1",
    "source_snapshot_id": "thought-forest:validatorcommit:manifest",
    "source_git_commit": "validatorcommit",
    "external_evidence_review_id": "validator-evidence-review-v1",
    "claim_review_id": "validator-claim-review-v1",
    "units": [{
        "unit_key": "validator-publication-unit-v1",
        "claim_id": "validator-claim-v1",
        "claim_content_hash": content_hash,
        "review_status": "reviewed",
        "lifecycle_status": "reviewed",
        "quality_score": 0.96,
        "publication_eligible": True,
        "source_locator": {
            "locator_type": "markdown_lines",
            "repository": "thought-forest",
            "git_commit": "validatorcommit",
            "path": "z/validator-publication.md",
            "line_start": 10,
            "line_end": 20,
            "heading_path": ["Validator", "Definition"],
        },
        "claim_review": {
            "review_id": "validator-claim-review-v1",
            "decision": "approved",
            "review_status": "reviewed",
            "quality_score": 0.96,
            "external_evidence_review_id": "validator-evidence-review-v1",
        },
    }],
}
path.write_text(json.dumps(artifact, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
PY

metadata="$(python3 - "$embedding_fingerprint" <<'PY'
import json
import sys
fingerprint = sys.argv[1]
print(json.dumps({
    "embedding_identity": {
        "provider": "hashing",
        "model": "bodysense-hashing-ngram",
        "dimension": 1536,
        "revision": "sha256-char-word-ngram-v1",
        "fingerprint": fingerprint,
    },
    "snapshot_id": "thought-forest:validatorcommit:manifest",
    "claim_candidate": {"claim_id": "validator-claim-v1"},
    "claim_admissibility": {"status": "claim_reviewed", "publication_eligible": True},
    "claim_review": {
        "review_id": "validator-claim-review-v1",
        "decision": "approved",
        "review_status": "reviewed",
        "external_evidence_review_id": "validator-evidence-review-v1",
    },
    "external_evidence_candidates": [{
        "support_status": "reviewed_support",
        "admissibility_status": "admissible_for_claim_review",
        "external_review_id": "validator-evidence-review-v1",
        "license_status": "citation_only",
    }],
    "source_locator": {
        "locator_type": "markdown_lines",
        "repository": "thought-forest",
        "git_commit": "validatorcommit",
        "path": "z/validator-publication.md",
        "line_start": 10,
        "line_end": 20,
        "heading_path": ["Validator", "Definition"],
    },
}, separators=(",", ":")))
PY
)"

"${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 \
  -v operator_id="$operator_id" -v member_id="$member_id" -v source_key="$source_key" \
  -v unit_key="$unit_key" -v body_hash="$body_hash" -v metadata="$metadata" >/dev/null <<'SQL'
INSERT INTO users (id, email, password_hash, role)
VALUES
  (:'operator_id'::uuid, 'knowledge-validator-operator@invalid.local', 'not-a-login-hash', 'operator'),
  (:'member_id'::uuid, 'knowledge-validator-member@invalid.local', 'not-a-login-hash', 'member');

INSERT INTO knowledge_sources (
  source_key, source_type, title, author, problem_slug, problem_display_name,
  original_file_path, language, ingest_status, metadata, license_status,
  content_hash, canonical_url, source_version, provenance, registered_by, registered_at
)
VALUES (
  :'source_key', 'thought_forest_note', 'Validator publication source', 'Thought Forest',
  'validator', 'Validator', 'z/validator-publication.md', 'en', 'ingested', '{}'::jsonb,
  'citation_only', repeat('b', 64), NULL, 'v1', '{"origin":"synthetic-validator"}'::jsonb,
  :'operator_id'::uuid, now()
);

INSERT INTO knowledge_units (
  source_id, unit_key, problem_slug, category, unit_type, title, summary, body_markdown,
  source_start_sec, source_end_sec, evidence_segment_indices, tags, transcript_excerpt,
  review_status, lifecycle_status, quality_score, content_hash, embedding, metadata
)
SELECT
  ks.id, :'unit_key', 'validator', 'reference', 'reference', 'Validator definition',
  'Reviewed validator claim.', E'## Validator Definition\n\nReviewed publication validator claim.',
  0, 0, '{}', '{}', E'## Validator Definition\n\nReviewed publication validator claim.',
  'reviewed', 'reviewed', 0.96, :'body_hash',
  array_fill(0.0::real, ARRAY[1536])::vector, :'metadata'::jsonb
FROM knowledge_sources ks WHERE ks.source_key = :'source_key';
SQL

# The production lifecycle CLI must not expose raw unit-key publication.
if "${compose[@]}" exec -T api /app/knowledge-publication-manager publish >/dev/null 2>&1; then
  echo "KNOWLEDGE_LEGACY_PUBLISH_DISABLED=FAIL" >&2
  exit 1
fi
echo "KNOWLEDGE_LEGACY_PUBLISH_DISABLED=PASS"

# A normal member cannot become the lifecycle actor merely by supplying its UUID.
if "${compose[@]}" exec -T api /app/knowledge-publication-manager publish-reviewed \
  --publication-key "$denied_key" --batch-key "$batch_key" --reviewed-snapshot /tmp/reviewed.json \
  --published-by "$member_id" >/dev/null 2>&1; then
  echo "KNOWLEDGE_OPERATOR_GATE=FAIL" >&2
  exit 1
fi
if [[ "$("${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -Atc "SELECT count(*) FROM knowledge_publications WHERE publication_key='$denied_key';")" != "0" ]]; then
  echo "KNOWLEDGE_OPERATOR_GATE=FAIL publication leaked" >&2
  exit 1
fi
echo "KNOWLEDGE_OPERATOR_GATE=PASS"

# Copy the exact reviewed artifact into the API container without relying on host mounts.
docker cp "$artifact" "${COMPOSE_PROJECT_NAME}-api-1:/tmp/reviewed.json"

"${compose[@]}" exec -T api /app/knowledge-publication-manager publish-reviewed \
  --publication-key "$publication_key" --batch-key "$batch_key" --reviewed-snapshot /tmp/reviewed.json \
  --published-by "$operator_id" --summary "synthetic local publication validation" >/dev/null

published_state="$("${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -Atc "
SELECT concat_ws('|', ku.lifecycle_status, ku.review_status, ku.published_version::text,
  (ku.publication_id IS NOT NULL)::text,
  (kp.metadata->'embedding_fingerprints' ? '$embedding_fingerprint')::text,
  kp.status, kp.published_by)
FROM knowledge_units ku
JOIN knowledge_publications kp ON kp.id = ku.publication_id
WHERE ku.unit_key='$unit_key' AND kp.publication_key='$publication_key';")"
expected_published="published|reviewed|1|true|true|published|$operator_id"
if [[ "$published_state" != "$expected_published" ]]; then
  echo "KNOWLEDGE_PUBLICATION_VERTICAL=FAIL state=$published_state" >&2
  exit 1
fi
echo "KNOWLEDGE_PUBLICATION_VERTICAL=PASS"

"${compose[@]}" exec -T api /app/knowledge-publication-manager rollback \
  --publication-key "$publication_key" --rollback-key "$rollback_key" \
  --rolled-back-by "$operator_id" --reason "synthetic local rollback validation" >/dev/null

rollback_state="$("${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -Atc "
SELECT concat_ws('|', ku.lifecycle_status, ku.review_status,
  (ku.publication_id IS NULL)::text, (ku.published_version IS NULL)::text,
  original.status, rb.status, rb.published_by)
FROM knowledge_units ku
JOIN knowledge_publications original ON original.publication_key='$publication_key'
JOIN knowledge_publications rb ON rb.publication_key='$rollback_key'
WHERE ku.unit_key='$unit_key';")"
expected_rollback="reviewed|reviewed|true|true|rolled_back|rollback|$operator_id"
if [[ "$rollback_state" != "$expected_rollback" ]]; then
  echo "KNOWLEDGE_ROLLBACK_VERTICAL=FAIL state=$rollback_state" >&2
  exit 1
fi
echo "KNOWLEDGE_ROLLBACK_VERTICAL=PASS"
