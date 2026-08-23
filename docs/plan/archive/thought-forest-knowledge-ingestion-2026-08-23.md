# Thought Forest → BodySense Knowledge Ingestion MVP

**Status:** Completed — 2026-08-23
**Date:** 2026-08-23

## Outcome

Make Thought Forest a governed upstream text source for the existing BodySense Knowledge Library without creating a second RAG system.

The MVP must prove one safe vertical path:

```text
Thought Forest explicit allowlist
→ deterministic health snapshot
→ schema validation in BodySense
→ GeneratedKnowledgePack adapter
→ existing KnowledgeLibrary ingestion
```

Imported units remain `generated` / unpublished by default. This phase does not change user-facing retrieval visibility.

## Current system evidence

- BodySense already owns normalized `knowledge_sources / segments / units / clips` and pgvector retrieval.
- `KnowledgeLibrary.ingest_generated_pack()` is the authoritative ingestion boundary.
- lifecycle columns already exist through migrations `000019` + `000020`.
- online retrieval already has published/reviewed visibility policy.
- the existing schema was originally video-centric, so Thought Forest provenance must be kept in metadata rather than abusing timestamp fields as line numbers.
- Thought Forest now has a curated health North Star and explicit source-authority boundaries.

## Protected contracts

- Existing video `GeneratedKnowledgePack` JSON remains loadable.
- Existing video ingestion behavior and DB columns remain compatible.
- No change to `/api/knowledge/search` response contract in this phase.
- No published-only policy relaxation.
- No automatic ingestion of every `life/health` note; only explicit allowlist entries.
- Git commit + note path + Markdown line range must survive into persisted metadata.

## Phase 0 — Contract + exporter

- Thought Forest explicit allowlist manifest.
- Snapshot schema `bodysense.health.snapshot.v1`.
- Refuse clean exports from dirty worktrees by default.
- Export H2-level human sections, not sentence-level chunks.
- Preserve note hash, section hash, Git commit and absolute Markdown line range.

## Phase 1 — BodySense adapter

- Validate snapshot with Pydantic.
- Convert one Thought Forest note into one existing `GeneratedKnowledgePack` source.
- Convert note sections into knowledge units.
- Persist provenance in source / segment / unit metadata.
- Keep `review_status=generated`, lifecycle default `generated`, quality default `0`.

## Phase 2 — Vertical verification

- Generate a real snapshot from the committed Thought Forest allowlist.
- Run BodySense importer in `--dry-run` mode.
- Run focused Python tests + Ruff.
- Only after review may a later ticket ingest into development Postgres and run unpublished retrieval eval.

## Non-goals

- No automatic machine claim extraction yet.
- No external citation parsing from note prose yet.
- No publication/review UI.
- No production DB ingestion.
- No source-authority promotion of personal notes to primary evidence.
- No citation UI changes.

## Acceptance

1. Export is deterministic for a fixed Git commit + manifest apart from `generated_at`.
2. Every exported section has a stable key, content hash, note path and absolute line range.
3. BodySense rejects malformed snapshot schema or invalid line ranges.
4. Adapter preserves provenance metadata through `GeneratedKnowledgePack`.
5. Existing video pack tests still pass.
6. Dry-run succeeds on the real Thought Forest snapshot.


## Implementation Result

The MVP vertical slice is complete.

Verified evidence:

- Thought Forest committed exporter source: `8fa5393` (`feat(kb): export BodySense health snapshot`).
- real snapshot schema: `bodysense.health.snapshot.v1`.
- allowlist: 11 curated health notes.
- exported H2-level units: 115 sections.
- snapshot identity binds the Thought Forest Git commit and manifest hash.
- each unit preserves `repository + git_commit + note path + absolute Markdown line range + content hash`.
- BodySense Pydantic adapter converts the snapshot into the existing `GeneratedKnowledgePack` boundary.
- existing video pack constructors remain backward compatible through additive metadata defaults.
- development PostgreSQL migrated successfully through version 49.
- development ingestion result: 11 Thought Forest sources / 115 segments / 115 units / 0 clips.
- all imported units remained `lifecycle_status=generated` and `review_status=generated`.
- default retrieval returned 0 Thought Forest units; `include_unpublished=True` retrieved the pilot units.
- persisted DB metadata was inspected and retained the Markdown source locator.
- AI-service Ruff passed and the complete unit suite passed: **305/305**.

The development database was used only for validation; publication was not performed.

## Follow-up Boundary

The next phase should not expand this importer blindly. It should add the evidence-normalization layer:

```text
Thought Forest section
→ typed machine claim candidate
→ cited source / authority evidence
→ admissibility review
→ reviewed snapshot / publication batch
→ retrieval + grounding eval
```

Key follow-ups:

1. define `claim_kind`, certainty, authority tier and external-source provenance contract;
2. extract / resolve external citations instead of treating personal synthesis as primary evidence;
3. expose text-source locators in `SearchResult` / normalized Agent evidence without reusing video timestamps;
4. build an unpublished retrieval eval set before promotion;
5. only then implement review / publication for selected Thought Forest units.
