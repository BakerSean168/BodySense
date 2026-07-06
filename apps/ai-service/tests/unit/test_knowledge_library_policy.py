from src.rag.knowledge_library import KnowledgeLibrary


def test_published_visibility_filter_requires_review_or_publication():
    clause = KnowledgeLibrary._published_visibility_filter()

    assert "ku.lifecycle_status IN ('published', 'reviewed')" in clause
    assert "ku.review_status IN ('reviewed', 'approved', 'curated')" in clause
    assert "COALESCE(ku.quality_score, 0.0) >= %s" in clause
