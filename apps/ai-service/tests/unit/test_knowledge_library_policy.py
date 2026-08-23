from src.rag.knowledge_library import KnowledgeLibrary


def test_published_visibility_filter_requires_explicit_publication_and_review():
    clause = KnowledgeLibrary._published_visibility_filter()

    assert "ku.lifecycle_status = 'published'" in clause
    assert "ku.publication_id IS NOT NULL" in clause
    assert "ku.published_version IS NOT NULL" in clause
    assert "ku.review_status IN ('reviewed', 'approved', 'curated')" in clause
    assert "COALESCE(ku.quality_score, 0.0) >= %s" in clause
    assert " OR " not in clause


def test_claim_kind_intent_boost_supports_thought_forest_reference_units():
    from src.rag.knowledge_library import SearchResult

    library = KnowledgeLibrary(database_url="postgresql://test")
    safety = SearchResult(
        id=1,
        problem_slug="musculoskeletal-safety",
        category="assessment",
        unit_type="reference",
        title="Safety Netting",
        summary="summary",
        body_markdown="出现红旗或紧急风险时需要升级就医。",
        similarity=0.05,
        source_title="Safety",
        source_author="Thought Forest",
        source_start_sec=0.0,
        source_end_sec=0.0,
        unit_metadata={"claim_candidate": {"claim_kind": "safety_guidance"}},
    )
    plain = SearchResult(
        id=2,
        problem_slug="other",
        category="assessment",
        unit_type="reference",
        title="Other",
        summary="summary",
        body_markdown="body",
        similarity=0.05,
        source_title="Other",
        source_author="Thought Forest",
        source_start_sec=0.0,
        source_end_sec=0.0,
    )

    assert library._intent_boost("出现红旗时什么时候升级就医", safety) >= 0.10
    assert library._intent_boost("出现红旗时什么时候升级就医", plain) < 0.10


def test_generic_movement_word_does_not_trigger_intervention_claim_boost():
    from src.rag.knowledge_library import SearchResult

    library = KnowledgeLibrary(database_url="postgresql://test")
    intervention = SearchResult(
        id=3,
        problem_slug="training",
        category="exercise",
        unit_type="reference",
        title="动作说明",
        summary="动作说明",
        body_markdown="动作说明",
        similarity=0.05,
        source_title="Training",
        source_author="Thought Forest",
        source_start_sec=0.0,
        source_end_sec=0.0,
        unit_metadata={"claim_candidate": {"claim_kind": "intervention_option"}},
    )

    assert library._intent_boost("视频动作角度能不能判断关节力矩", intervention) < 0.10


def test_hashing_published_relevance_gate_requires_meaningful_lexical_anchor():
    from src.rag.knowledge_library import SearchResult

    result = SearchResult(
        id=10,
        unit_key="tfu-pain-definition",
        problem_slug="pain-science",
        category="pain-science",
        unit_type="reference",
        title="疼痛与伤害感受 · 一句话定义",
        summary="疼痛与伤害感受不是同一现象。",
        body_markdown=(
            "IASP 将 pain 定义为与实际或潜在组织损伤相关的不愉快感觉与情绪体验；"
            "nociception 是神经系统对有害刺激进行编码的过程。"
        ),
        similarity=0.05,
        source_key="thought-forest:z/pain-and-nociception.md",
        source_type="thought_forest_note",
        source_title="疼痛与伤害感受",
        source_author="Thought Forest",
        source_start_sec=0.0,
        source_end_sec=0.0,
    )

    assert KnowledgeLibrary._has_meaningful_lexical_anchor("什么是疼痛？", result) is True
    assert KnowledgeLibrary._has_meaningful_lexical_anchor(
        "疼痛是不是等于组织损伤程度？", result
    ) is True
    assert KnowledgeLibrary._has_meaningful_lexical_anchor("脚踝扭伤怎么处理？", result) is False
    assert (
        KnowledgeLibrary._has_meaningful_lexical_anchor("怎么设置 PostgreSQL 索引？", result)
        is False
    )
    assert KnowledgeLibrary._has_meaningful_lexical_anchor("组织架构怎么设计？", result) is False


def test_published_hashing_gate_only_targets_thought_forest_results():
    from src.rag.knowledge_library import SearchResult

    video = SearchResult(
        id=11,
        problem_slug="shoulder",
        category="exercise",
        unit_type="exercise",
        title="Video exercise",
        summary="summary",
        body_markdown="body",
        similarity=0.01,
        source_type="video",
        source_title="Video",
        source_author="Author",
        source_start_sec=0.0,
        source_end_sec=1.0,
    )
    assert KnowledgeLibrary._passes_published_relevance_gate(
        "unrelated query", video, embedding_provider="hashing"
    ) is True
