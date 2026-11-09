# Knowledge Expansion — Stage B Inventory, Candidate Catalog, and First Adapter Plan

> Status: Stage B first-leg deliverable for
> [GitHub Issue #510](https://github.com/BakerSean168/BodySense/issues/510)
> (`Part of #499`). This is a read-only inventory plus a dispatchable adapter
> plan, not an implementation record and not a task ledger.
>
> Baseline: this repository at `main` commit `89d582e7`. Every repository fact
> below was read from a file at that commit; file and line references are
> included so the numbers can be re-checked. Anything that exists only in a
> database or a running environment is listed in §2 and is **not** estimated
> here.
>
> Stage A context: [knowledge-expansion-stage-a.md](./knowledge-expansion-stage-a.md).
> Lifecycle mechanics remain owned by
> [knowledge-lifecycle-architecture.md](./knowledge-lifecycle-architecture.md);
> product semantics remain owned by [docs/product](../product/README.md).

## 0. Scope, owner decision, and non-goals

### 0.1 Owner decision recorded (2026-11-08)

The product owner formally waived the two admission gates that Stage A §8.3
and §8.4 required before any real ingestion:

| Gate (Stage A §8) | Original requirement | Owner decision 2026-11-08 |
| --- | --- | --- |
| §8.3 License adjudication per source | Written per-source license value, evidence, and `citation_only` fallback on the issue | **Waived** for this line; sources proceed to import without a per-source license table |
| §8.4 Named human clinical reviewer | Named reviewer for claim review and external-evidence review | **Waived** for this line; no named reviewer is required |

The decision is recorded on #510 (owner comment `2026-11-08`,
`issuecomment-6716196711`) and supersedes the "Dependencies" section of that
issue. The compensating control is a repository README takedown notice: a
rights holder can ask the maintainer to remove knowledge content.

This document therefore does **not** list those two gates as blockers. It does
record, in §7, the residual risk that the waiver creates: without per-source
adjudication, provenance and attribution must be carried *inside each unit*
rather than in a side table, otherwise a takedown request cannot be executed
precisely.

### 0.2 What the waiver does not change

The owner waived *admission review*, not the code's fail-closed gates. The
following still reject bad input at runtime and must be satisfied by any
adapter (all verified in §1):

- the registry's closed license vocabulary (`verified_reuse`, `citation_only`,
  `owned`, `public_domain`); `unknown` is rejected at registration;
- the SHA-256 `content_hash` and non-empty `provenance` requirement;
- the publication quality floor of `0.90`, reviewed lifecycle/review status,
  `body_markdown` hash freshness, a stored embedding, a complete
  `embedding_identity` fingerprint, an approved claim review, and reviewed
  direct external support;
- the reviewed-snapshot validator, which requires `markdown_lines` locators
  bound to the same git commit as the snapshot.

### 0.3 Non-goals of this leg

No code, no data import, no database write, no staging or production access, no
new vector store, no safety/red-flag content, and no clinical claims. §2 exists
precisely because this leg cannot produce environment numbers.

## 1. Repository-side baseline

### 1.1 Knowledge data model and migrations

All knowledge tables are created by the baseline migration; **no later
migration (`000002`–`000007`) references `knowledge`** (verified by grepping
`apps/api/migrations/*.up.sql`). Canonical schema definition:

`apps/api/migrations/000001_vnext_baseline.up.sql`.

| Table | Grain | Columns that matter for expansion | Definition |
| --- | --- | --- | --- |
| `knowledge_sources` | one registered source artifact | `source_key` (unique identity), `source_type`, `problem_slug`, `language` (default `zh`), `license_status` (default `unknown`), `content_hash`, `canonical_url`, `source_version` (default `v1`), `provenance` jsonb, `registered_by`, `registered_at` | lines 830–855 |
| `knowledge_segments` | one transcript segment of a source | `source_id`, `segment_index`, `start_sec`/`end_sec`, `transcript`, `normalized_transcript` | lines 793–804 |
| `knowledge_units` | one searchable, reviewable claim-sized unit | `source_id`, `unit_key`, `problem_slug`, `category`, `unit_type`, `title`, `summary`, `body_markdown`, `source_start_sec`/`source_end_sec`, `evidence_segment_indices`, `review_status`, `lifecycle_status`, `quality_score`, `content_hash`, `publication_id`, `published_version`, `metadata` jsonb, `embedding vector(1536)` | lines 881–906 |
| `knowledge_clips` | optional media clip attached to a unit | `source_unit_id`, `clip_key`, `file_path`, time range | lines 706–721 |
| `knowledge_publications` | one immutable publication or rollback batch | `publication_key`, `publication_batch_key`, `published_version`, `status`, `unit_count`, `rollback_of`, `metadata` | lines 769–786 |
| `knowledge_publication_observations` | one post-publication eval observation | `publication_id`, `observation_kind`, `retrieval_status`, `citation_status`, `grounding_status`, `identity_status`, `provenance_status` | lines 747–762 |

Structural facts that constrain any adapter:

1. `embedding` is a fixed `vector(1536)` column (line 897). An embedding
   provider with a different dimension cannot be used for this column without
   a migration.
2. There is **no pgvector index** in the baseline migration — no `ivfflat` or
   `hnsw` definition appears anywhere in the migrations. Retrieval is a
   sequential cosine-distance scan (`apps/ai-service/src/rag/knowledge_library.py:552,572`).
   That is adequate at pilot scale and is a known scaling limit, not a bug to
   fix in this leg.
3. `source_start_sec`/`source_end_sec` on `knowledge_units` are `NOT NULL`
   doubles (lines 891–892). Text sources must therefore supply a non-null
   numeric span; the existing Thought Forest path satisfies this with line
   ordinals, not seconds.
4. `language` defaults to `zh` at both the table (line 839) and the registry
   (below), so an English source is the exceptional case and must say so
   explicitly.

### 1.2 Where the reviewed/published governance actually lives

| Stage | Implementation | Enforcement read from the file |
| --- | --- | --- |
| Source registration | `apps/api/internal/service/knowledge_source_registry.go` | license vocabulary at lines 56–62; `unknown` rejected at 95–97; identity fields, 64-hex `content_hash`, non-empty `provenance` at 91–104 |
| Source registration (Thought Forest variant) | `apps/api/internal/service/thought_forest_source_registry.go` | requires `schema_version == bodysense.health.snapshot.v3` (line 144) and `source_type == thought_forest_note` (line 167) |
| Ingestion (Go job) | `apps/api/internal/service/knowledge_ingestion_service.go` | single job type `knowledge.ingest_video` (line 25); rejects any non-`video` source (line 155) |
| Text ingestion (Python path) | `apps/ai-service/scripts/ingest_thought_forest_snapshot.py`, `apps/ai-service/src/rag/knowledge_library.py` | source must already be registered; registered identity must equal pack identity; license/content/provenance authority required (`knowledge_library.py:298–345`) |
| External evidence review | `apps/ai-service/src/rag/external_evidence.py` | schema `bodysense.external-evidence-review.v1`; source `license_status` is a **two-value** literal (`citation_only` or `verified_reuse`) (lines 33–46) |
| Claim review | `apps/ai-service/src/rag/claim_review.py` | schema `bodysense.claim-review.v1`; exact-unit/claim/hash-keyed decisions; `decision ∈ {approved,rejected}`; quality in `[0,1]` (lines 12–47) |
| Reviewed snapshot | `apps/ai-service/scripts/materialize_thought_forest_reviewed_snapshot.py` (producer) and `apps/api/internal/service/knowledge_reviewed_snapshot.go` (validator) | schema `bodysense.reviewed-knowledge-snapshot.v1` (line 15); validation at 68–127 |
| Publication | `apps/api/internal/service/knowledge_publication_service.go` | quality floor `0.90` (line 21); gate at 156–211; immutable reviewed-snapshot cross-check at 209–255 |
| Publication CLI | `apps/api/cmd/knowledge-publication-manager/main.go` | `publish-reviewed`, `rollback`, `observe-eval`, `status` (lines 40–51) |
| Retrieval | `apps/ai-service/src/rag/knowledge_library.py`; HTTP surface `apps/ai-service/src/api/routes/knowledge.py` | published-only SQL predicate (840–851); stats/sources endpoints (routes lines 234, 244) |
| Evaluation | `apps/ai-service/scripts/run_published_knowledge_eval.py` | schema `bodysense.published-knowledge-eval.v1`; hit cases require unit key + claim id + evidence terms (lines 136–151) |

### 1.3 Reviewed pack contract, precisely

The "reviewed pack" is not one file but a three-artifact chain that ends in the
Go publication authority:

```text
source snapshot (git commit)
  -> external-evidence-review.v1   (external_evidence.py)
  -> claim-review.v1              (claim_review.py)
  -> reviewed-knowledge-snapshot.v1 (materialize script -> Go validator)
  -> knowledge-publication-manager publish-reviewed
```

`reviewed-knowledge-snapshot.v1` fields, from
`apps/api/internal/service/knowledge_reviewed_snapshot.go:47–55` and
`docs/knowledges/evidence/reviewed-knowledge-pilot.json`:

| Field | Type | Required | Validator rule |
| --- | --- | --- | --- |
| `schema_version` | string | yes | must equal `bodysense.reviewed-knowledge-snapshot.v1` (line 72) |
| `reviewed_snapshot_id` | string | yes | non-empty (75) |
| `source_snapshot_id` | string | yes | must contain `:<source_git_commit>:` (82) |
| `source_git_commit` | string | yes | non-empty (78) |
| `external_evidence_review_id` | string | yes | must match every unit's `claim_review.external_evidence_review_id` (121) |
| `claim_review_id` | string | yes | must match every unit's `claim_review.review_id` (118) |
| `units[].unit_key` | string | yes | non-empty, unique (91, 98) |
| `units[].claim_id` | string | yes | with hash, unique per claim identity (101–105) |
| `units[].claim_content_hash` | 64-char hex | yes | decodes as hex (91–96) |
| `units[].review_status` / `lifecycle_status` | string | yes | both exactly `reviewed` (106) |
| `units[].quality_score` | float | yes | equals `claim_review.quality_score` (124) |
| `units[].publication_eligible` | bool | yes | must be `true` (106) |
| `units[].source_locator.locator_type` | string | yes | must be `markdown_lines` (109) |
| `units[].source_locator.repository` / `path` / `heading_path` | strings | yes | non-empty; `heading_path` non-empty (110–114) |
| `units[].source_locator.git_commit` | string | yes | equals top-level `source_git_commit` (111) |
| `units[].source_locator.line_start` / `line_end` | int | yes | `line_start > 0`, `line_end >= line_start` (114–115) |
| `units[].claim_review.decision` | string | yes | exactly `approved` (119) |

Two further gates that only appear at publication time
(`knowledge_publication_service.go:156–211`):

- `content_hash` must equal `sha256(body_markdown)` computed at publication
  time (172–176) — an adapter must hash exactly the stored Markdown body, not
  a pre-normalized form.
- `metadata.embedding_identity` must contain `provider`, `model`, `dimension`,
  `revision`, `fingerprint`, and the fingerprint must equal
  `sha256(provider + "\n" + model + "\n" + dimension + "\n" + revision)`
  (121–137). This is checked in the publication transaction, not merely at
  ingestion.
- For `thought_forest_note` only, the publication gate additionally requires a
  `markdown_lines` locator (201–206). The snapshot validator is stricter and
  applies the locator rule to **all** source types, so a new source type must
  still materialize Markdown with line ranges.

Adapter consequences worth stating plainly:

1. A structured XML/API source cannot be published as-is. It must first become
   versioned Markdown text with stable line numbering and a git commit.
2. The evidence-source license literal (`citation_only | verified_reuse`) does
   not accept `public_domain` or `owned`. Even if the *source registry* records
   `public_domain`, the external-evidence manifest must express its support
   relation as one of the two allowed values.
3. The `claim_content_hash` in the reviewed snapshot must equal the DB unit's
   `content_hash`, and the DB unit's `content_hash` must equal
   `sha256(body_markdown)`. Any reformatting between review and publication
   aborts the batch.

### 1.4 In-repo knowledge content: location and countable size

Repository knowledge content lives entirely under `docs/knowledges/`. There is
no SQL seed, no test fixture pack, and no import bundle elsewhere: a search for
JSON files containing `unit_key` under `apps/**/test*` returns nothing, and the
only `*knowledge*` directories are the two Go CLIs and `docs/knowledges/`.

| Path | Kind | Files | Countable content |
| --- | --- | --- | --- |
| `docs/knowledges/README.md` | orientation | 1 | states the directory is reference/eval material, not live data |
| `docs/knowledges/glossary.md` | glossary | 1 | — |
| `docs/knowledges/testing-guide.md` | testing guide | 1 | — |
| `docs/knowledges/ai-agent-impl/unified-consultation-run-sse-pipeline.md` | pipeline reference | 1 | — |
| `docs/knowledges/issue-13-video-knowledge-pipeline.md` | pipeline reference | 1 | — |
| `docs/knowledges/issue-13-manual-video-ingestion-runbook.md` | operator runbook | 1 | — |
| `docs/knowledges/eval/` | eval datasets | 4 | 27 cases total |
| `docs/knowledges/evidence/` | review artifacts | 6 | 4 reviewed units |

Totals, counted from the files themselves:

- **16 files** under `docs/knowledges/` (3 top-level documents + 1
  `ai-agent-impl` document + 2 Issue #13 documents + 4 eval + 6 evidence).
- **4 reviewed units**: 1 in `reviewed-knowledge-pilot.json`, 3 in
  `reviewed-knowledge-pain-nociception-cohort.json`.
- **4 claim-review decisions** (1 + 3) and **2 external-evidence manifests**
  covering **3 external sources** and **5 claim-support relations** (1 source /
  1 relation in the pilot; 2 sources / 4 relations in the cohort).
- **27 eval cases**: 15 in published-knowledge datasets (9 `hit`, 6
  `no_result`; pilot 3/3, cohort 6/3) and **12 unpublished demand probes** in
  `thought-forest-unpublished.jsonl`, which have no `expect` field and are
  demand signals rather than coverage.
- **1 thresholds file** pinning the cohort dataset by SHA-256
  (`6ed384433b007cb564abcced6e07e343d5e153e26efca3f7095e985922a81878`),
  requiring pass rate `1.0`, ≥6 positive hits, ≥3 negative rejections, ≥6
  citation-valid, ≥6 grounding-supported, and publication identity.

All 4 reviewed units are on one topic (pain / nociception), backed by one
publisher (IASP). That is the whole in-repo corpus; it is example/qualification
material, not the live corpus.

### 1.5 Embedding implementation and identity

| Concern | Verified value | Basis |
| --- | --- | --- |
| Provider default | `EMBEDDING_PROVIDER=hashing` | `apps/ai-service/src/rag/embedding.py:28`; `.env.example:54`; `.env.production:64`; all three compose files default to `hashing` |
| Hashing model identity | model `bodysense-hashing-ngram`, revision `sha256-char-word-ngram-v1` | `embedding.py:11–12` |
| Default semantic model (when provider ≠ hashing) | `paraphrase-multilingual-MiniLM-L12-v2`, overridable by `EMBEDDING_MODEL` | `embedding.py:29`; `.env.example:59` |
| Dimension | `EMBEDDING_DIMENSIONS`, default `1536`; must match `vector(1536)` | `embedding.py:30`; migration line 897; runbook warns explicitly that 1536 is mandatory |
| Identity shape | `{provider, model, dimension, revision, fingerprint}` where fingerprint = `sha256("provider\nmodel\ndimension\nrevision")` | `embedding.py:108–137`; Go re-computes and verifies it at `knowledge_publication_service.go:121–137` |
| Where identity is stored | `knowledge_units.metadata.embedding_identity` | written at `knowledge_library.py:483`; read by `scripts/knowledge-baseline.sql:60–68` |
| Embedding input | `title + "\n" + summary + "\n" + body_markdown` | `knowledge_library.py:283–286` |
| Dimension guard | ingestion aborts if any vector length ≠ identity dimension | `knowledge_library.py:287–290` |
| Retrieval distance | cosine via `<=>`; no ANN index | `knowledge_library.py:552,572` |
| Retrieval policy under hashing | published Thought Forest results require an independent lexical anchor; other source types bypass that gate | `knowledge_library.py:788–805` |

Two planning consequences that matter for topic and eval design:

1. **Hashing is lexical.** It is a deterministic char/word n-gram hash, not a
   semantic model. It therefore cannot bridge languages. English source text
   with Chinese queries (the product default language) will not retrieve
   reliably, and paraphrase-only queries are weak even within one language.
2. **Provider identity is corpus-wide in effect.** A query is embedded with the
   currently configured generator and compared against stored vectors. Changing
   the provider/model/dimension without re-embedding every unit leaves
   incomparable vectors in one column; there is no per-unit provider filter in
   the search SQL. Any provider change is a corpus-wide re-embedding operation,
   not a config flip.

A third, narrower finding: the deny-first lexical gate is keyed to
`source_type == "thought_forest_note"`. A new source type publishing under
hashing would be returned on raw cosine similarity with no lexical anchor,
which is exactly the failure mode the gate exists to prevent. This is a
deliberate integration decision for the first adapter (§6.6), not something to
silently inherit.

### 1.6 What this baseline establishes, and what it cannot

Established from files: the full governance chain exists and is enforced in
code; the reviewed pack contract is narrow and hash-bound; the in-repo corpus is
4 reviewed units on one topic; the embedding layer is a 1536-dimensional
lexical hash by default with a fingerprinted identity; and there is no
text/API adapter in the Go ingestion service.

Not established here: how many sources or units exist in any deployed
environment, which publication states exist, which categories are present, and
which embedding provider is actually running. Those are §2.

## 2. Target-environment checklist (staging)

The numbers below cannot be produced from this worktree. They must be collected
by the main line on the host with read-only access to **staging**, labelled
explicitly, and recorded on #510. Nothing in this section was executed by this
leg.

Rules for whoever runs them:

- Run inside a trusted host/container with an independently confirmed DSN.
  Never paste a DSN, password, or API key into the issue or a PR.
- Every block starts with `BEGIN TRANSACTION READ ONLY;` and a statement
  timeout, and touches only `knowledge_*` tables — no user, conversation, or
  health-record tables.
- Label the environment in the output (`-v environment=staging`). A dev number
  must never be reported as staging (Stage A §3, and the Oracle2-dev result on
  #510 already showed why: dev was all zeros).
- Prefer the supported interfaces where they exist: `GET /api/knowledge/stats`
  and `GET /api/knowledge/sources` on the AI service, and
  `knowledge-publication-manager status`.

Wrapper used by the SQL items (adjust only the compose file/service/DSN
identifier, never the read-only guard):

```bash
docker compose -f docker/docker-compose.staging.yml exec -T postgres \
  psql -X -qAt -v ON_ERROR_STOP=1 -v environment=staging \
       -U "${DB_USER:-bodysense}" -d "${DB_NAME:-bodysense}" -f - < <SQL>
```

| # | Must be measured | Why it decides something | Executable check |
| --- | --- | --- | --- |
| E1 | Environment identity and schema version | Prevents dev/staging mix-ups; publication gates depend on schema columns | `SELECT current_database(), current_user, current_setting('transaction_read_only'), (SELECT max(version) FROM public.schema_migrations);` inside a read-only transaction |
| E2 | Registered sources by `source_type` × `license_status` × `ingest_status` (Stage A G1) | Shows whether anything is pre-registered in staging and in which license states; determines whether the first adapter registers or reuses a source | `SELECT source_type, license_status, ingest_status, count(*) FROM knowledge_sources GROUP BY 1,2,3 ORDER BY 1,2,3;` |
| E3 | Units by `lifecycle_status` × `review_status` (G2) | The reviewed→published funnel; tells whether staging holds only generated units, which would collide with new `unit_key`s | `SELECT lifecycle_status, review_status, count(*) FROM knowledge_units GROUP BY 1,2 ORDER BY 1,2;` |
| E4 | Publication batches, published units, latest version (G3/G4) | The real live corpus size; sets the baseline that the first cohort must not overwrite | `SELECT status, count(*) AS batches, coalesce(sum(unit_count),0) AS units, max(published_version) AS latest_version FROM knowledge_publications GROUP BY 1 ORDER BY 1;` and `SELECT count(*) FROM knowledge_units WHERE lifecycle_status='published';` |
| E5 | Unit category distribution | Determines whether the knee cohort introduces new `category` values or must reuse existing ones, and whether the eval's negative set is already occupied | `SELECT category, count(*) FROM knowledge_units GROUP BY 1 ORDER BY 2 DESC;` |
| E6 | Problem-slug distribution | Needed to pick non-colliding `problem_slug` values and to see existing demand coverage | `SELECT problem_slug, count(*) FROM knowledge_units GROUP BY 1 ORDER BY 2 DESC LIMIT 50;` |
| E7 | Stored embedding identity distribution (G7 stored) | If staging already holds units under a different provider/model, mixing in this cohort creates incomparable vectors | `SELECT coalesce(metadata->'embedding_identity'->>'provider','not_recorded') AS provider, coalesce(metadata->'embedding_identity'->>'model','not_recorded') AS model, coalesce(metadata->'embedding_identity'->>'revision','not_recorded') AS revision, count(*) FROM knowledge_units GROUP BY 1,2,3 ORDER BY 4 DESC;` |
| E8 | Vector dimension and embedding coverage | Confirms the column really is 1536 and finds units that could never publish (null embedding) | `SELECT vector_dims(embedding) AS dims, count(*) FROM knowledge_units WHERE embedding IS NOT NULL GROUP BY 1;` and `SELECT count(*) FILTER (WHERE embedding IS NULL) AS missing_embedding, count(*) AS total FROM knowledge_units;` |
| E9 | Provenance/completeness nulls | A unit without `content_hash`, `provenance`, `canonical_url`, or `source_version` cannot satisfy the registry authority check if re-registered | `SELECT count(*) FILTER (WHERE content_hash IS NULL) AS missing_hash, count(*) FILTER (WHERE provenance='{}'::jsonb) AS empty_provenance, count(*) FILTER (WHERE canonical_url IS NULL) AS missing_url, count(*) FILTER (WHERE registered_at IS NULL) AS unregistered FROM knowledge_sources;` |
| E10 | Observation gate distribution (G5) | Says whether staging is at `continue` or `hold` and how many samples exist — context for judging any retrieval change | `SELECT observation_kind, retrieval_status, citation_status, grounding_status, count(*), min(created_at), max(created_at) FROM knowledge_publication_observations GROUP BY 1,2,3,4 ORDER BY 1,2;` |
| E11 | Latest publications and rollback history | Establishes the last known-good version to roll back to if the first cohort misbehaves | `SELECT publication_key, publication_batch_key, published_version, unit_count, status, rollback_of, published_at FROM knowledge_publications ORDER BY published_at DESC LIMIT 10;` and `SELECT count(*) FROM knowledge_publications WHERE rollback_of IS NOT NULL;` |
| E12 | Runtime embedding configuration (G7 effective) | Stored metadata is not proof of the running value; provider changes are corpus-wide (see §1.5) | `docker compose -f docker/docker-compose.staging.yml exec -T ai-service sh -lc 'printf "provider=%s\nmodel=%s\ndims=%s\nrevision=%s\n" "$EMBEDDING_PROVIDER" "$EMBEDDING_MODEL" "$EMBEDDING_DIMENSIONS" "$EMBEDDING_REVISION"'` — prints no key material |
| E13 | Live retrieval hit rate on real traffic (G6) | Stage A §3 explicitly marks this unmeasurable from stored observations; it must be reported as unknown or derived from real traffic telemetry, never inferred from the eval cohort | No SQL. Use the staging traffic/telemetry source the operators already trust; if none exists, record `unknown`. `knowledge_publication_observations` covers published eval runs only (`scripts/knowledge-baseline.sql:58–59`). |

The existing aggregate script already covers E2–E4, E7, and E10 in one
transaction and is regression-tested against a disposable PG18 instance:

```bash
docker compose -f docker/docker-compose.staging.yml exec -T postgres \
  psql -X -qAt -v ON_ERROR_STOP=1 -v environment=staging \
       -U "${DB_USER:-bodysense}" -d "${DB_NAME:-bodysense}" \
       -f /path/to/repo/scripts/knowledge-baseline.sql
```

Run the script first; use E1, E5, E6, E8, E9, E11, E12, E13 for the gaps it
deliberately leaves (category, slug, vector coverage, runtime identity). Report
all thirteen (or mark an item `not available`) — a partial report is useful, a
guessed number is not.

## 3. Candidate source catalog

Twelve candidates, consolidated from Stage A §5–§6 and the #510 research
comment. "Volume" is given only where a number has a citable basis; otherwise
it is explicitly `not obtained`, together with how to size it.

| # | Candidate | Type | Access | Volume estimate and basis | Registry license candidate |
| --- | --- | --- | --- | --- | --- |
| S1 | MedlinePlus Health Topic XML | public health-topic documents | downloadable XML per topic; NLM attribution required | 31 matching topics for `knee pain` from the one real probe (#510, 2026-11-05); per-topic size not obtained — capture one topic XML and count sections | `public_domain` if the captured topic carries no third-party notice, else `citation_only` (no bulk copy) |
| S2 | MedlinePlus Web Service (`wsearch.nlm.nih.gov`) | search/brief XML API | unauthenticated GET; rate limit 85 req/min/IP, 12–24h caching recommended (#510 probe) | 15,302-byte response, `retmax=3`, total matches 31 (#510); catalogue-wide size not obtained | same as S1 for stored text; citations otherwise |
| S3 | Europe PMC REST | bibliographic + OA full text | REST; per-record license field | not obtained; size by querying a topic and counting `resultList.resultList` entries with reusable licenses | `verified_reuse` only for records whose license field is a reusable CC license; otherwise `citation_only` |
| S4 | PMC Open Access Subset | OA article corpus | OA service / OAI-PMH / E-utilities / BioC only; no bulk scraping | not obtained; size by counting OA records for the topic | per-article; `verified_reuse` for CC-BY-like records, `citation_only` otherwise |
| S5 | PubMed E-utilities | bibliographic metadata/abstracts | API; 3 req/s without key | not obtained; size by `esearch` count for the topic | `citation_only` (abstracts remain publisher-copyrighted) |
| S6 | IASP terminology pages (already in use) | publisher reference pages | public HTML | 4 reviewed units already in-repo (2 IASP artifacts in `docs/knowledges/evidence/`) | `citation_only` |
| S7 | Thought Forest owned notes | owned Markdown | in-house repository, git commit | repository-side: 1 reviewed unit from commit `4990c58…`; lifecycle doc §11.7 records an 11-note / 115-unit dev slice (`docs/architecture/knowledge-lifecycle-architecture.md`) | `owned` |
| S8 | OpenStax Anatomy & Physiology 2e | open textbook | web/PDF | not obtained; book-scale (dozens of chapters) | currently **blocked for AI ingestion** by its CC BY-NC-SA + no-LLM clause (Stage A §6, #510) |
| S9 | NICE syndication API | guideline syndication | organization application/licence | not obtained | **blocked**: UK-only licence, AI use requires NICE approval and fees (Stage A §6) |
| S10 | Open Concept Lab terminology API | terminology service | REST | not obtained; source/version structure only | code-system dependent; OCL software licence ≠ terminology data licence (Stage A §5.2) |
| S11 | WHO SMART Guidelines / DAK | guideline framework | web/publications | not obtained; no MSK DAK exists (Stage A §5.3) | WHO copyright; not public domain |
| S12 | Physiopedia | professional wiki | web | not obtained | `citation_only` pending explicit reuse/AI/commercial permission (#510) |

### 3.1 Five-candidate comparison

The five below are the realistic "first stored-text" candidates once the owner
waiver is applied: the two MedlinePlus surfaces, the two Europe PMC/PMC
surfaces, and Physiopedia. Basis tags: **[A]** Stage A text, **[I]** #510
research/probe comment, **[R]** repository file read at `89d582e7`.

| Dimension | S1 MedlinePlus Topic XML | S2 MedlinePlus Web Service | S3 Europe PMC REST | S4 PMC OA Subset | S12 Physiopedia |
| --- | --- | --- | --- | --- | --- |
| Reuse license form | NLM attribution requirement; per-topic third-party notices possible **[I]** | Same NLM terms; API terms add rate limit and caching guidance **[I]** | Per-record license field; OA subset is the cleared portion **[A §6]** | Per-article CC terms; "OA" ≠ everything reusable **[A §5.1, §6]** | No explicit reuse/AI permission recorded; citation only **[I]** |
| Storage form | Structured XML → must be rendered to versioned Markdown with line ranges to satisfy the contract **[R: knowledge_reviewed_snapshot.go:109–117]** | Brief/search XML contains highlight markup (`span.qt0`) that must be stripped; not a document body **[I]** | Metadata + abstract or OA full text records **[A §6]** | Article XML/full text per record **[A §6]** | HTML pages **[A §5.1 note on general web corpora]** |
| Embedding suitability | Expository lay-language prose; good chunk material once translated **[R: embedding input is title+summary+body, knowledge_library.py:283–286]** | Brief titles only; too thin to embed as units **[I]** | Abstracts are dense, citation-heavy; usable but noisier per unit **[A §6]** | Full text is long; needs sectioning into claim-sized units **[A §7 step 1]** | Clinician-oriented prose, uneven length per page **[A §5.1]** |
| Translation need | **Yes** — source is English, product default language is `zh` **[R: migration:839; registry default]** | Yes, but n/a for unit storage | **Yes** for stored units; abstract-level again | **Yes** | **Yes** |
| AI/automation restrictions | Rate limit 85 req/min/IP, 12–24h caching, attribution **[I]** | Same, plus "reference links first, no automatic clinical recommendation publication" per #510 | API availability ≠ reuse approval; automate only via approved channels **[A §6]** | Bulk scraping prohibited; must use PMC Cloud/OAI-PMH/E-utilities/BioC **[I]** | No scrape/import approved as of #510 research |

Reading the comparison:

- **S1 is the only candidate that is simultaneously (a) a real document corpus,
  (b) reachable without registration, (c) already probed successfully, and
  (d) small enough to convert deterministically.** S2 is a discovery aid, not a
  unit source: its brief XML is a title list with highlight markup.
- S3/S4 are better for citation-grade evidence than for lay-user explanatory
  units, and their per-record licensing makes bulk ingestion a licensing
  lottery even under a waiver.
- S12 has no recorded permission for storage at all; it stays citation-only.

## 4. First topic: knee pain and peri-knee load

### 4.1 Selection

**Topic:** knee pain and peri-knee load — the explanatory layer a user needs
before any training or diagnosis conversation.

**Bounded problem slugs (3):**

| Problem slug | Scope of the cohort's units | Demand basis |
| --- | --- | --- |
| `knee-pain` | what knee pain is, which structures are commonly involved, how load and activity relate to symptoms | No existing probe; adjacent to the pilot's negative ankle-sprain case **[A §2.2]** |
| `medial-knee-pain` | medial compartment anatomy and common benign explanations, without diagnosis | Existing unpublished probe: `膝内侧疼痛需要鉴别哪些情况？` **[R: docs/knowledges/eval/thought-forest-unpublished.jsonl]** |
| `iliotibial-band-syndrome` | what the IT band is, why the "friction" story is contested, where symptoms are usually felt | Existing unpublished probes: `髂胫束综合征是不是髂胫束摩擦造成的？` plus Stage A N6 **[R: same file; A §4]** |

**Target size: 18 knowledge units** (within the required 10–30). Budget:

| Unit type | `knee-pain` | `medial-knee-pain` | `iliotibial-band-syndrome` | Total |
| --- | --- | --- | --- | --- |
| definition | 2 | 1 | 2 | 5 |
| anatomy / mechanism | 4 | 2 | 2 | 8 |
| load & activity relation | 2 | 1 | 1 | 4 |
| expectation / boundary | 1 | 0 | 0 | 1 |
| **Total** | **9** | **4** | **5** | **18** |

### 4.2 Why this topic first

1. **Demand is already recorded, not assumed.** Two of the twelve in-repo
   demand probes are knee/ITB questions; Stage A ranks N6 (knee & ankle) P1 and
   notes the pilot deliberately expects `no_result` for ankle-sprain care
   **[A §2.2, §4]**.
2. **It is explanatory, not safety-critical.** The cohort contains no
   red-flag, referral, contraindication, or medication content, satisfying
   Stage A §8.6 (now a scope rule rather than a gate).
3. **It is reachable from S1.** The one real MedlinePlus probe returned "Knee
   Injuries and Disorders" as the first result for `knee pain` **[I]**, so the
   first adapter has a real target document rather than a hypothetical one.
4. **It maps to existing product surfaces.** Body Region Ontology has a knee
   region, and the consultation flow already asks about pain location and
   laterality; knee is the natural first region where "explain what this is"
   precedes "what should I do".
5. **It does not flip an existing negative.** The pilot/cohort `no_result`
   cases are ankle sprain, anterior pelvic tilt, and PostgreSQL. None is in
   this cohort, so the existing qualification dataset stays valid and the new
   dataset is additive.

### 4.3 Explicit exclusions

No ACL/meniscus surgical indications, no "when to see a doctor" lists, no
medication, no post-operative protocols, no diagnosis of a named condition from
symptoms. Those belong to N2/N7 and to clinical review that this line does not
have.

## 5. Evaluation cases for the knee cohort

Design follows the existing evaluator exactly
(`apps/ai-service/scripts/run_published_knowledge_eval.py`): a `hit` case must
supply `expected_unit_key`, `expected_claim_id`, and `expected_evidence_terms`
(lines 136–151); a `no_result` case must return no result at all; `top_k`
defaults to 3; every observed result must also pass citation, grounding,
identity, and provenance checks, and the publication key must match
(`--publication-key` is required).

Unit keys follow `mlp-knee-<nnn>`; claim ids follow `mlc-knee-<12-hex>`.
Concrete keys and hashes are frozen when the units are authored — the table
below fixes the *case*, not a hash.

### 5.1 Positive cases (expect `hit`)

| Case id | Query (zh) | Expected unit | Expected evidence terms | Why it must hit |
| --- | --- | --- | --- | --- |
| `knee-pos-01` | 膝盖疼通常和哪些结构有关？ | `mlp-knee-001` (definition/overview) | 膝关节, 结构, 疼痛 | Tests the top-of-funnel question; plain lexical overlap after translation |
| `knee-pos-02` | 膝内侧疼可能是什么原因？ | `mlp-knee-010` (medial anatomy) | 内侧, 半月板, 内侧副韧带 | Matches the existing demand probe with different wording |
| `knee-pos-03` | 髂胫束综合征是摩擦造成的吗？ | `mlp-knee-014` (ITB mechanism) | 髂胫束, 摩擦, 争议 | The probe question is itself a misconception; the unit must answer the mechanism, not repeat the myth |
| `knee-pos-04` | 跑步以后膝盖外侧疼是怎么回事？ | `mlp-knee-015` (ITB presentation) | 外侧, 跑步, 负荷 | Paraphrase-only case; no shared character n-grams beyond 膝盖/外侧, so it tests real retrieval rather than exact string matching |
| `knee-pos-05` | 膝盖的负荷和活动量有什么关系？ | `mlp-knee-006` (load relation) | 负荷, 活动量, 适应 | Cross-slug case: the answer lives in `knee-pain`, not the query's surface wording |
| `knee-pos-06` | 膝盖疼是不是说明软骨磨损了？ | `mlp-knee-009` (expectation/boundary) | 软骨, 磨损, 不能判断 | Interpretation-boundary unit; tests that the corpus can say "symptoms ≠ tissue state" without diagnosing |

### 5.2 Negative cases (expect `no_result`)

| Case id | Query (zh) | Why it must not hit | Judgment |
| --- | --- | --- | --- |
| `knee-neg-01` | 前交叉韧带撕裂一定要手术吗？ | Treatment/surgical decision is out of cohort scope (§4.3) | Any returned citation is a failure, even if topically adjacent |
| `knee-neg-02` | 膝盖红肿发热还发烧，需要马上去医院吗？ | Red-flag/referral content is excluded (N2/N7) | Must not be answered from this cohort; the safety path owns it |
| `knee-neg-03` | 腰椎间盘突出会引起膝盖疼吗？ | Radicular/other-region content is not in scope | A knee unit that accidentally answers this is a precision failure |
| `knee-neg-04` | 膝关节置换术后第三天怎么康复？ | Post-operative protocol, out of scope | No unit may match on 膝关节 alone |
| `knee-neg-05` | 布洛芬一次吃多少毫克？ | Medication dosing is prohibited | Must never be grounded in this cohort |
| `knee-neg-06` | PostgreSQL 索引为什么会失效？ | Off-domain control, mirrors the existing pilot negative | Guards against a hashing collision being surfaced as knowledge |

### 5.3 Pass criteria

Proposed thresholds file (`docs/knowledges/eval/published-knowledge-knee-cohort.thresholds.json`),
same schema as the pain cohort:

```json
{
  "schema_version": "bodysense.published-knowledge-qualification-thresholds.v1",
  "dataset": "docs/knowledges/eval/published-knowledge-knee-cohort.jsonl",
  "dataset_sha256": "<computed at authoring time; never hand-written>",
  "expected_cases": 12,
  "min_pass_rate": 1.0,
  "min_positive_hits": 6,
  "min_negative_rejections": 6,
  "min_citation_valid": 6,
  "min_grounding_supported": 6,
  "require_publication_identity": true
}
```

Rules:

- A positive case passes only if the top-3 includes the exact
  `expected_unit_key`, the citation is valid, grounding is supported, and the
  publication key matches the cohort publication.
- A negative case passes only if retrieval returns zero results — not "a result
  with low similarity". This is the existing evaluator's semantics and is
  deliberately strict.
- If an English-only corpus is used for the first pass, the dataset must be
  English and a separate zh dataset must be deferred explicitly. Do not mix
  languages in one dataset and claim a pass rate: under hashing, that measures
  translation coverage, not knowledge coverage.
- The dataset hash is computed after the JSONL is final. A hand-written hash is
  a defect.

## 6. First adapter implementation plan (dispatchable)

### 6.1 Scope of the next leg

One narrow adapter: **MedlinePlus Health Topic XML → versioned Markdown
snapshot → existing generated-pack and review path → `publish-reviewed` in a
disposable dev database.** No staging or production write. No second source. No
safety content. No new vector store.

The adapter must not write `knowledge_units` directly. Stage A §7 step 2 is
binding: adapters stop at reviewed-snapshot artifacts; Go remains the only
publication authority.

### 6.2 Input: where the format sample comes from

- The only verified sample so far is the #510 probe: `GET
  https://wsearch.nlm.nih.gov/ws/query?db=healthTopics&term=knee%20pain&retmax=3&rettype=brief&tool=BodySenseResearch`,
  HTTP 200, 15,302-byte XML, 31 total matches, brief-title results containing
  `span.qt0` highlight markup. That is a **search** response, not a topic
  document.
- Task 0 of the next leg must capture **one full topic XML** for the chosen
  topic (start from "Knee Injuries and Disorders", the first probe result) into
  `~/artifacts/bodysense/510-knee-adapter/raw/` with its SHA-256, and record the
  observed element structure. Do not guess element names in code before this
  capture exists.
- The XML fetch must use a descriptive `tool=` parameter, a single request,
  and a cache file; NLM's limit is 85 req/min/IP with 12–24h caching
  recommended **[I]**. No crawling.

### 6.3 Output contract

**Artifact 1 — source snapshot (new, versioned in its own git repository):**

```json
{
  "schema_version": "bodysense.source-snapshot.medlineplus.v1",
  "snapshot_id": "medlineplus-topics:<git_commit>:<content_hash_prefix>",
  "source": {
    "source_key": "medlineplus:health-topic:knee-injuries-and-disorders",
    "source_type": "medlineplus_topic",
    "title": "<topic title>",
    "author": "MedlinePlus / U.S. National Library of Medicine",
    "problem_slug": "knee-pain",
    "problem_display_name": "膝关节疼痛",
    "language": "zh",
    "original_file_path": "topics/knee-injuries-and-disorders.xml",
    "canonical_url": "https://medlineplus.gov/<topic>.html",
    "source_version": "<topic revision date if present, else retrieved-YYYY-MM-DD>",
    "content_hash": "<sha256 of the raw XML bytes>",
    "provenance": {
      "publisher": "MedlinePlus (NLM)",
      "retrieved_at": "<ISO-8601 UTC>",
      "raw_xml_sha256": "<same as content_hash>",
      "markdown_sha256": "<sha256 of rendered Markdown>",
      "attribution": "Content sourced from MedlinePlus.gov, U.S. National Library of Medicine",
      "license_basis": "<the exact evidence inspected for this topic>",
      "translation": {"from": "en", "to": "zh", "method": "<tool + human spot-check record>"}
    }
  },
  "documents": [
    {"path": "topics/knee-injuries-and-disorders.md", "sha256": "<markdown sha256>",
     "line_count": 0, "sections": [{"heading_path": ["..."], "line_start": 1, "line_end": 0}]}
  ]
}
```

Rules baked into this shape:

- `content_hash` is over the **raw** source bytes, so re-fetching a changed
  topic is detectable; the Markdown hash travels separately in provenance.
  The registry's hash-pinning and `RegisterOrValidate` identity comparison then
  work as designed.
- The snapshot must be committed to git, because `source_snapshot_id` must
  contain the commit and every unit locator must carry that same
  `git_commit` (`knowledge_reviewed_snapshot.go:82,111`).
- Every unit's locator `path` + `line_start`/`line_end` must address lines in
  the committed Markdown. Line numbers are computed once, at materialization,
  and frozen with the commit.

**Artifact 2 — external evidence review (`bodysense.external-evidence-review.v1`):**

- One `sources[]` entry per MedlinePlus topic page:
  `canonical_key`, `canonical_url`, `authority_tier`, `evidence_level`,
  `license_status`, `review_status`, `reviewed_by`, `review_basis`.
- `license_status` must be `citation_only` or `verified_reuse` — the literal
  does not admit `public_domain`. Use `verified_reuse` only if the topic page
  is the actual supporting source for the claim; otherwise `citation_only`.
  This is a schema limitation, not a licensing statement about the corpus
  (which the registry records separately).
- `relations[]` must bind each `claim_id` + `claim_content_hash` to a
  `canonical_key` with `support_status: reviewed_support` and
  `support_scope: direct`.

**Artifact 3 — claim review (`bodysense.claim-review.v1`):**

- One decision per unit, keyed by `(unit_key, claim_id, claim_content_hash)`,
  with `decision: approved`, `review_status: reviewed`, `reviewed_by`,
  `review_basis`, `quality_score >= 0.90`, `certainty`, `population: general`.
- `reviewed_by` must name the role that actually performed the check under the
  waiver (for example `maintainer-adapter-self-review`). Do not invent a
  clinician identity; that would falsify the provenance the takedown control
  depends on.

**Artifact 4 — reviewed snapshot (`bodysense.reviewed-knowledge-snapshot.v1`):**
exactly the fields in §1.3, produced by a new materializer or by generalizing
`materialize_thought_forest_reviewed_snapshot.py` (the current script is
Thought-Forest-shaped, so a sibling function is the lower-risk change).

**Database effect:** units are inserted by the Python ingest path with
`lifecycle_status='reviewed'` only when a claim-review manifest is applied;
`publication_eligible=true`; `metadata.embedding_identity` from
`EmbeddingGenerator.identity()`; `metadata.source_locator` matching the
snapshot; `metadata.claim_candidate`, `metadata.claim_review`, and
`metadata.external_evidence_candidates` present because the publication gate
reads all three.

### 6.4 Shard order

Three shards, smallest-risk first, each independently reviewable and
publishable:

| Shard | Content | Units | Order rationale |
| --- | --- | --- | --- |
| K1 | `iliotibial-band-syndrome` | 5 | Smallest, most self-contained, directly answers an existing probe; proves the pipeline end to end |
| K2 | `medial-knee-pain` | 4 | Second probe; tests a second heading structure and a second problem slug |
| K3 | `knee-pain` overview + load + boundary | 9 | Largest and most cross-cutting; last because it depends on the vocabulary established in K1/K2 |

Per-shard definition of done: XML captured and hashed → Markdown rendered and
committed → units materialized → evidence + claim manifests → reviewed
snapshot validates → ingested into disposable dev DB → eval subset passes →
publish + rollback verified. Do not start the next shard until the previous
one publishes and rolls back cleanly.

### 6.5 Tests the adapter must ship with

- Fixture XML → deterministic Markdown and deterministic unit list (byte-stable
  across runs).
- Highlight markup (`span.qt0` and any equivalent) is stripped from titles and
  bodies, while canonical URLs survive.
- `content_hash` equals SHA-256 of the committed raw XML; `claim_content_hash`
  equals SHA-256 of `body_markdown`; a one-character edit aborts publication.
- Re-running the adapter on an identical input is idempotent; a changed input
  produces a registry conflict rather than a silent mutation.
- A snapshot with an ineligible unit (quality < 0.90, missing embedding
  identity, missing external support) fails the existing Go validator and the
  publication gate. These are regression tests of §1.3, not new policy.
- Translated output retains heading structure and line ranges; a translation
  that changes line counts invalidates the frozen locators and must be
  re-materialized, not patched.

### 6.6 Decisions the next leg must make explicitly (do not inherit silently)

1. **Snapshot repository location.** Default recommendation: a dedicated
   snapshot repository (imported third-party text should be independently
   removable for takedown, and should not bloat the product repo). If it stays
   in this repository, say so and accept the takedown/revert coupling.
2. **Deny-first lexical gate for the new source type.** Today the gate applies
   only to `thought_forest_note` (`knowledge_library.py:788–805`). Under
   hashing, a `medlineplus_topic` unit is returned on raw cosine similarity
   with no lexical anchor. Either extend the gate to a set of governed text
   source types (small, tested change) or document that the first cohort runs
   unguarded and cover it with the eval. Recommend extending the gate.
3. **Translation.** Source is English, product default is `zh`, hashing is not
   cross-lingual. The cohort must be published in `zh` (or the eval must be
   explicitly English-only). Translation quality becomes a first-class review
   artifact because the claim-review gate is waived.
4. **Embedding provider.** Keep `hashing` for the first cohort (matches all
   existing data and requires no re-embedding), and record that semantic
   retrieval is a separate corpus-wide migration, not an adapter feature.
5. **Registry license value for S1.** Confirm from the captured topic whether a
   third-party notice exists. If none: `public_domain` with attribution in
   provenance. If one exists: stop and pick another topic, because
   `citation_only` does not permit storing the text as units.

## 7. Residual risks after the owner waiver

| Risk | Consequence | Mitigation carried by this plan |
| --- | --- | --- |
| No per-source license adjudication | A source could be stored under a value that later proves wrong | Per-unit `canonical_url` + `provenance.license_basis` + snapshot commit make removal precise; README takedown notice is the published control |
| No named clinical reviewer | `quality_score >= 0.90` and `decision: approved` are self-attested | `reviewed_by` records the actual role; no clinician identity is fabricated; safety content stays excluded |
| Machine translation of medical text | Meaning drift that the gates cannot detect | Translation is a reviewed artifact; boundary units are phrased as "symptoms do not equal tissue state", not as diagnosis |
| New source type bypasses the lexical gate under hashing | Hash-collision citations | §6.6 item 2 — extend the gate before publishing, or accept + measure via eval |
| Staging numbers still unknown | Cohort sizing and collision checks are provisional | §2 checklist; no import decision should be finalized before E2–E8 are reported |
| Takedown without a reviewer process | Removal request may be slow or incomplete | Snapshot commit + unit locator make the removal set computable; document the one-command rollback path in the adapter README |

## References

In-repo: `docs/architecture/knowledge-expansion-stage-a.md`;
`docs/architecture/knowledge-lifecycle-architecture.md`;
`docs/knowledges/` (README, eval, evidence);
`apps/api/migrations/000001_vnext_baseline.up.sql`;
`apps/api/internal/service/knowledge_source_registry.go`,
`knowledge_ingestion_service.go`, `knowledge_reviewed_snapshot.go`,
`knowledge_publication_service.go`, `thought_forest_source_registry.go`;
`apps/api/cmd/knowledge-publication-manager/main.go`;
`apps/ai-service/src/rag/{embedding,knowledge_library,claim_review,external_evidence,thought_forest_snapshot}.py`;
`apps/ai-service/src/api/routes/knowledge.py`;
`apps/ai-service/scripts/{ingest_thought_forest_snapshot,materialize_thought_forest_reviewed_snapshot,run_published_knowledge_eval}.py`;
`scripts/knowledge-baseline.sql`, `scripts/test-knowledge-baseline.sh`.

Issue: #510 (including the 2026-11-05 source research comment, the MedlinePlus
probe, and the 2026-11-08 owner waiver comment).
