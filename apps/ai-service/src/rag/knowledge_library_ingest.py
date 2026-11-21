"""Transactional ingestion of generated knowledge packs."""

from __future__ import annotations

from typing import Any

from psycopg.types.json import Jsonb

from .knowledge_library_base import KnowledgeLibraryBase
from .knowledge_pack import GeneratedKnowledgePack


class KnowledgeIngestMixin(KnowledgeLibraryBase):
    """Write paths: overwrite preflight and atomic pack ingestion."""

    async def assert_sources_overwritable(self, source_keys: list[str]) -> None:
        """Fail before batch writes if any source is protected by publication state."""
        normalized = sorted({key for key in source_keys if key})
        if not normalized:
            return
        pool = self._require_pool()
        async with pool.connection() as conn:
            async with conn.cursor() as cur:
                await cur.execute(
                    """
                    SELECT ks.source_key
                    FROM knowledge_sources ks
                    WHERE ks.source_key = ANY(%s)
                      AND EXISTS (
                          SELECT 1
                          FROM knowledge_units ku
                          WHERE ku.source_id = ks.id
                            AND (
                                ku.lifecycle_status = 'published'
                                OR ku.publication_id IS NOT NULL
                            )
                      )
                    ORDER BY ks.source_key
                    """,
                    (normalized,),
                )
                rows = await cur.fetchall()
        if rows:
            protected = ", ".join(str(row[0]) for row in rows)
            raise RuntimeError(
                "cannot overwrite knowledge batch; published/publication-linked sources: "
                f"{protected}"
            )

    async def ingest_generated_pack(
        self,
        pack: GeneratedKnowledgePack,
        overwrite_source: bool = False,
    ) -> dict[str, Any]:
        """Insert a generated pack atomically using an async transaction."""
        embedding_inputs = [
            "\n".join([unit.title, unit.summary, unit.body_markdown]) for unit in pack.units
        ]
        embeddings = await self.embedding_generator.generate_batch(embedding_inputs)
        if len(embeddings) != len(pack.units):
            raise RuntimeError("embedding count does not match generated knowledge unit count")
        embedding_identity = self.embedding_generator.identity()
        expected_dimension = int(embedding_identity["dimension"])
        if any(len(embedding) != expected_dimension for embedding in embeddings):
            raise RuntimeError("embedding dimension does not match embedding identity")
        pool = self._require_pool()

        async with pool.connection() as conn:
            async with conn.transaction():
                async with conn.cursor() as cur:
                    await cur.execute(
                        """
                        SELECT id, ingest_status, source_type, title, author, problem_slug,
                               problem_display_name, language, license_status, content_hash,
                               source_version, provenance, registered_by, registered_at
                        FROM knowledge_sources
                        WHERE source_key = %s
                        """,
                        (pack.source.source_key,),
                    )
                    row = await cur.fetchone()
                    if row is None:
                        raise RuntimeError("knowledge source must be registered before ingestion")

                    source_id = row[0]
                    ingest_status = row[1]
                    registered_identity = (row[2], row[3], row[4], row[5], row[6], row[7])
                    pack_identity = (
                        pack.source.source_type,
                        pack.source.title,
                        pack.source.author,
                        pack.source.problem_slug,
                        pack.source.problem_display_name,
                        pack.source.language,
                    )
                    if registered_identity != pack_identity:
                        raise RuntimeError(
                            "generated pack identity does not match registered knowledge source"
                        )
                    if (
                        row[8] not in {"verified_reuse", "citation_only", "owned", "public_domain"}
                        or not row[9]
                        or not row[10]
                        or not row[11]
                        or not row[12]
                        or row[13] is None
                    ):
                        raise RuntimeError(
                            "registered knowledge source is missing "
                            "license/content/provenance authority"
                        )

                    if pack.source.source_type == "thought_forest_note":
                        snapshot_id = str(pack.source.metadata.get("snapshot_id") or "")
                        note_content_hash = str(
                            pack.source.metadata.get("note_content_hash") or ""
                        ).lower()
                        repository = pack.source.metadata.get("repository")
                        repository_git_commit = (
                            str(repository.get("git_commit") or "")
                            if isinstance(repository, dict)
                            else ""
                        )
                        provenance = row[11] if isinstance(row[11], dict) else {}
                        if (
                            not snapshot_id
                            or not note_content_hash
                            or row[9].lower() != note_content_hash
                            or row[10] != snapshot_id
                            or provenance.get("snapshot_id") != snapshot_id
                            or provenance.get("git_commit") != repository_git_commit
                            or provenance.get("path") != pack.source.original_file_path
                            or provenance.get("note_content_hash") != note_content_hash
                        ):
                            raise RuntimeError(
                                "Thought Forest pack does not match registered snapshot identity"
                            )

                    if ingest_status != "registered" and not overwrite_source:
                        return {
                            "source_id": source_id,
                            "source_key": pack.source.source_key,
                            "status": "already_exists",
                        }
                    if ingest_status != "registered":
                        await cur.execute(
                            """
                            SELECT COUNT(*)
                            FROM knowledge_units
                            WHERE source_id = %s
                              AND (lifecycle_status = 'published' OR publication_id IS NOT NULL)
                            """,
                            (source_id,),
                        )
                        protected_row = await cur.fetchone()
                        protected_count = int(protected_row[0]) if protected_row is not None else 0
                        if protected_count > 0:
                            raise RuntimeError(
                                "cannot overwrite a knowledge source with published/"
                                "publication-linked units; rollback or create a new "
                                "immutable source version first"
                            )
                        await cur.execute(
                            "DELETE FROM knowledge_clips WHERE source_id = %s",
                            (source_id,),
                        )
                        await cur.execute(
                            "DELETE FROM knowledge_segments WHERE source_id = %s",
                            (source_id,),
                        )
                        await cur.execute(
                            "DELETE FROM knowledge_units WHERE source_id = %s",
                            (source_id,),
                        )

                    await cur.execute(
                        """
                        UPDATE knowledge_sources
                        SET duration_sec = %s,
                            transcript_provider = %s,
                            transcript_model = %s,
                            transcript_file_path = %s,
                            ingest_status = 'ingested',
                            metadata = COALESCE(metadata, '{}'::jsonb) || %s
                        WHERE id = %s
                        """,
                        (
                            pack.source.duration_sec,
                            pack.source.transcript_provider,
                            pack.source.transcript_model,
                            pack.source.transcript_file_path,
                            Jsonb(pack.source.metadata),
                            source_id,
                        ),
                    )

                    for segment in pack.transcript_segments:
                        await cur.execute(
                            """
                            INSERT INTO knowledge_segments (
                                source_id, segment_index, start_sec, end_sec,
                                transcript, normalized_transcript, confidence, metadata
                            )
                            VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
                            """,
                            (
                                source_id,
                                segment.segment_index,
                                segment.start_sec,
                                segment.end_sec,
                                segment.text,
                                segment.text,
                                segment.confidence,
                                Jsonb(segment.metadata),
                            ),
                        )

                    unit_id_by_key: dict[str, int] = {}
                    for unit, embedding in zip(pack.units, embeddings, strict=False):
                        await cur.execute(
                            """
                            INSERT INTO knowledge_units (
                                source_id, unit_key, problem_slug, category, unit_type,
                                title, summary, body_markdown, source_start_sec,
                                source_end_sec, evidence_segment_indices, tags,
                                transcript_excerpt, review_status, lifecycle_status,
                                quality_score, content_hash, embedding, metadata
                            )
                            VALUES (
                                %s, %s, %s, %s, %s, %s, %s, %s,
                                %s, %s, %s, %s, %s, %s, %s, %s, %s, %s::vector, %s
                            )
                            RETURNING id
                            """,
                            (
                                source_id,
                                unit.unit_key,
                                unit.problem_slug,
                                unit.category,
                                unit.unit_type,
                                unit.title,
                                unit.summary,
                                unit.body_markdown,
                                unit.source_start_sec,
                                unit.source_end_sec,
                                unit.evidence_segment_indices,
                                unit.tags,
                                unit.transcript_excerpt,
                                unit.review_status,
                                unit.lifecycle_status,
                                unit.quality_score,
                                unit.content_hash,
                                embedding,
                                Jsonb(
                                    {
                                        **unit.metadata,
                                        "problem_display_name": unit.problem_display_name,
                                        "embedding_identity": embedding_identity,
                                    }
                                ),
                            ),
                        )
                        row = await cur.fetchone()
                        if row is None:
                            raise RuntimeError("Failed to persist knowledge unit row")
                        unit_id_by_key[unit.unit_key] = row[0]

                    for clip in pack.clips:
                        await cur.execute(
                            """
                            INSERT INTO knowledge_clips (
                                source_id, source_unit_id, clip_key, clip_type, title,
                                file_path, start_sec, end_sec, transcript_excerpt, notes, metadata
                            )
                            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                            """,
                            (
                                source_id,
                                unit_id_by_key.get(clip.source_unit_key),
                                clip.clip_key,
                                clip.clip_type,
                                clip.title,
                                clip.file_path,
                                clip.start_sec,
                                clip.end_sec,
                                clip.transcript_excerpt,
                                clip.notes,
                                Jsonb({}),
                            ),
                        )

        return {
            "source_id": source_id,
            "source_key": pack.source.source_key,
            "status": "ingested",
            "segments": len(pack.transcript_segments),
            "units": len(pack.units),
            "clips": len(pack.clips),
        }
