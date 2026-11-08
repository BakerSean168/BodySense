# MedlinePlus Health Topic XML adapter

Imported third-party knowledge enters BodySense only through a reviewed pack
that ends at the Go publication authority. This adapter is the first one for
source `S1` of
[knowledge-expansion-stage-b-inventory.md](./knowledge-expansion-stage-b-inventory.md)
(§3, §6). It converts one MedlinePlus health-topic XML record into versioned
Markdown, then into claim-sized Chinese units with the four reviewed artifacts
the existing publication gate already requires.

Adapters never write `knowledge_units`. They stop at
`reviewed-knowledge-snapshot.v1`; `knowledge-publication-manager
publish-reviewed` is the only writer of publication state.

## 1. Captured source

| Field | Value |
| --- | --- |
| Source | MedlinePlus Health Topic XML (U.S. National Library of Medicine) |
| Topic | Knee Injuries and Disorders |
| Canonical URL | https://medlineplus.gov/kneeinjuriesanddisorders.html |
| Raw snapshot | `topics/knee-injuries-and-disorders.xml`, 26,710 bytes |
| Raw SHA-256 | `c7625abe24df3f4056faf77d2a3e152d6d8b740a8f86ff9612ecb77e44d8dd85` |
| Capture request | one `GET https://wsearch.nlm.nih.gov/ws/query?db=healthTopics&term=title%3A%22Knee+Injuries+and+Disorders%22&rettype=topic&retmax=1&tool=BodySenseKnowledgeAdapter`, HTTP 200 |
| Snapshot repository | `~/projects/bodysense-knowledge-snapshots`, capture commit `cb4bc8cfba1b56ccc8805d545a333f41f0f4de6e` |

Capture rules applied: exactly one request per topic, a constant `tool=`
identifier, no crawling, and no request rate above NLM's published limit of 85
requests/minute/IP. NLM recommends caching responses for 12–24 hours; the
committed snapshot is that cache.

## 2. Observed XML structure (measured, not assumed)

Measured from the captured bytes with a structural scan; do not extend the
parser to elements that are absent here without a new capture.

```text
nlmSearchResult
├── term, file, server, count, retstart, retmax
└── list[@num,@start,@per]
    └── document[@rank,@url]
        └── content[@name="healthTopic"]
            └── health-topic[@title,@url,@id,@language,@date-created,@meta-desc]
                ├── full-summary                  # the only body text
                ├── group[@id,@url]               # x2 for this topic
                ├── mesh-heading/descriptor[@id]
                ├── see-reference                 # vocabulary only, x6
                ├── related-topic[@id,@url]       # x2
                ├── language-mapped-topic[@id,@language,@url]
                ├── other-language[@url,@vernacular-name]
                ├── primary-institute[@url]
                └── site[@title,@url,@language-mapped-url]   # x60
                    ├── information-category
                    ├── organization
                    └── standard-description
```

Facts that shape the parser:

1. `full-summary` is **HTML-escaped markup** (`<p>`, `<a href>`), not plain
   text. The renderer unescapes it, unwraps anchors to their visible label, and
   keeps the topic's canonical URL from `health-topic/@url`.
2. `rettype=topic` records carry **no** `span.qt0` highlight markup; spotlight
   markup appears only in `rettype=brief`. The renderer strips any
   `<span class="qt*">` wrapper defensively so a future capture mode cannot
   leak markup into unit text.
3. `site` is a link list. Its `@title` is the label; `information-category`,
   `organization` and `standard-description` are its children. Link entries are
   **not** imported as unit text.
4. `see-reference` and `related-topic` are vocabulary, not prose. They are used
   only as retrieval tags, never as claim evidence.
5. The topic record contains no copyright element. The topic page carries no
   third-party copyright label either.

## 3. Licence finding

Inspected on 2026-11-08:

- https://medlineplus.gov/about/using/usingcontent/ states that summaries on
  health topic pages are in the public domain and may be reproduced and
  redistributed with attribution, and that copyrighted material is labelled
  near the bottom of the page with the copyright holder and date.
- https://medlineplus.gov/kneeinjuriesanddisorders.html shows no such label for
  the topic summary; the only "Copyright" link is the site-wide NLM footer.

Consequences:

- The source registry records `license_status = public_domain` with the
  required attribution inside `provenance`.
- The external-evidence manifest uses `verified_reuse`, because the topic page
  is the actual supporting source for these claims (the schema admits only
  `citation_only` or `verified_reuse`).
- Each unit carries `canonical_url`, `provenance.license_basis` and the snapshot
  commit, so a takedown request is computable without a side table.

No third-party notice was found. If a future capture does carry one, stop:
`citation_only` does not permit storing the text as units.

## 4. Content budget the source actually supports

The captured record contains **one** `full-summary` of 1,255 characters and no
per-section prose. It supports explanatory units about knee structures,
the mechanism that produces knee problems, common-ness and age range, activity
impact, osteoarthritis and cartilage wear, ligament/tendon and ACL injury, and
the boundary that treatment depends on the cause.

It does **not** contain direct support for medial-compartment anatomy or for the
iliotibial band, and MedlinePlus publishes no health-topic record for either.
Those units are therefore not authored from this source. Authoring them from
secondary knowledge would require marking `support_scope: direct` on a relation
the source does not state, which the reviewed-pack contract forbids.

## 5. Pipeline

```text
topics/<slug>.xml                  raw capture, hashed
  -> rendering/<slug>.zh.json      reviewed zh rendering plan (heading -> text, source quote guard)
  -> topics/<slug>.md              deterministic Markdown, line numbers frozen by commit
  -> units/<slug>.<shard>.json     unit spec (slug, type, claim kind, section key)
  -> docs/knowledges/evidence/medlineplus-<...>-{evidence,claim-review,reviewed}.json
  -> knowledge-source-manager register-source
  -> ingest_medlineplus_snapshot.py
  -> knowledge-publication-manager publish-reviewed
  -> run_published_knowledge_eval.py
```

Commands (disposable dev database only):

```bash
# 1. render (idempotent, byte-stable)
uv run python scripts/medlineplus_topic_adapter.py render \
  --snapshot-dir ~/projects/bodysense-knowledge-snapshots \
  --topic knee-injuries-and-disorders

# 2. materialize the reviewed artifacts for one shard
uv run python scripts/medlineplus_topic_adapter.py materialize \
  --snapshot-dir ~/projects/bodysense-knowledge-snapshots \
  --topic knee-injuries-and-disorders --shard k1 \
  --source-commit "$(git -C ~/projects/bodysense-knowledge-snapshots rev-parse HEAD)" \
  --output-dir ../../docs/knowledges/evidence

# 3. register the source (idempotent; a changed hash conflicts instead of mutating)
go run ./cmd/knowledge-source-manager register-source \
  --snapshot ~/projects/bodysense-knowledge-snapshots/manifests/knee-injuries-and-disorders.snapshot.json \
  --operator-id <operator-uuid>

# 4. ingest the reviewed pack (no publication)
uv run python scripts/ingest_medlineplus_snapshot.py \
  --snapshot-dir ~/projects/bodysense-knowledge-snapshots \
  --topic knee-injuries-and-disorders --shard k1 \
  --evidence-review-manifest ../../docs/knowledges/evidence/medlineplus-knee-k1-evidence-review.json \
  --claim-review-manifest ../../docs/knowledges/evidence/medlineplus-knee-k1-claim-review.json

# 5. publish, evaluate, observe, roll back
go run ./cmd/knowledge-publication-manager publish-reviewed \
  --publication-key medlineplus-knee-k1 \
  --batch-key medlineplus-knee \
  --reviewed-snapshot ../../docs/knowledges/evidence/medlineplus-knee-k1-reviewed-knowledge.json \
  --published-by <operator-uuid> --summary "K1 knee cohort"
```

## 6. Takedown

A rights-holder request is executed in this order:

1. Identify the affected units: every unit whose
   `metadata.source_locator.git_commit` equals the snapshot commit, or whose
   `metadata.snapshot_id` equals the reviewed `source_snapshot_id`.
2. Roll back the publications that contain them:
   `knowledge-publication-manager rollback --publication-key <key>
   --rollback-key <key>-takedown --rolled-back-by <operator-uuid> --reason takedown`.
3. Delete or revert the snapshot commit in
   `~/projects/bodysense-knowledge-snapshots`. Because every hash and locator is
   bound to that commit, the removal set is exactly computable; no product
   migration is needed.
4. The repository README carries the public takedown contact notice.

## 7. Limits

- The adapter is one source type only. It does not generalize to other
  publishers without a new capture, licence finding and rendering plan.
- Units are Chinese rewrites of an English summary. A changed source or a
  changed rendering plan changes line numbers, so locators must be
  re-materialized, never patched.
- Retrieval keeps the `hashing` embedding. Semantic retrieval is a corpus-wide
  migration, not an adapter feature.
