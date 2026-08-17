"""Targeted evidence retrieval shared by Diagnosis and Treatment agents."""

from __future__ import annotations

import uuid
from dataclasses import dataclass
from typing import Any

from ..rag import get_knowledge_library


@dataclass(slots=True)
class KnowledgeEvidenceSearcher:
    """Adapter over the normalized knowledge library.

    Retrieval is invoked by an Agent tool only when the model identifies an
    evidence gap; Diagnosis no longer performs broad Go-side RAG on every run.
    """

    user_id: str

    async def search(self, query: str, *, top_k: int = 5) -> list[dict[str, Any]]:
        library = get_knowledge_library()
        results = await library.search(query=query, top_k=max(1, min(top_k, 10)))
        return [normalize_evidence(self.user_id, result) for result in results]


def normalize_evidence(user_id: str, result: Any) -> dict[str, Any]:
    source_type = "knowledge_unit"
    source_key = f"knowledge-unit:{result.id}"
    source_version = str(getattr(result, "source_timestamp", "") or "")
    identity = ":".join(["bodysense:evidence", user_id, source_type, source_key, source_version])
    evidence_id = str(uuid.uuid5(uuid.NAMESPACE_URL, identity))
    return {
        "evidence_id": evidence_id,
        "source_type": source_type,
        "source_key": source_key,
        "source_version": source_version,
        "title": result.title,
        "summary": result.summary,
        "excerpt": result.body_markdown,
        "problem_slug": result.problem_slug,
        "category": result.category,
        "unit_type": result.unit_type,
        "similarity": result.similarity,
        "source_title": result.source_title,
        "source_author": result.source_author,
        "source_timestamp": result.source_timestamp,
        "tags": result.tags,
    }
