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
    assert (
        KnowledgeLibrary._has_meaningful_lexical_anchor("疼痛是不是等于组织损伤程度？", result)
        is True
    )
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
    assert (
        KnowledgeLibrary._passes_published_relevance_gate(
            "unrelated query", video, embedding_provider="hashing"
        )
        is True
    )


def _published_pain_result(
    *,
    unit_key: str,
    heading: str,
    claim_kind: str,
    body: str,
    similarity: float,
):
    from src.rag.knowledge_library import SearchResult

    return SearchResult(
        id=100,
        unit_key=unit_key,
        problem_slug="pain-and-nociception",
        category="pain_science",
        unit_type="reference",
        title=f"疼痛与伤害感受 · {heading}",
        summary=body[:120],
        body_markdown=body,
        similarity=similarity,
        source_key="thought-forest:z/pain-and-nociception.md",
        source_type="thought_forest_note",
        source_title="疼痛与伤害感受",
        source_author="Thought Forest",
        source_start_sec=0.0,
        source_end_sec=0.0,
        lifecycle_status="published",
        review_status="reviewed",
        quality_score=0.96,
        unit_metadata={
            "claim_candidate": {"claim_kind": claim_kind},
            "source_locator": {
                "heading_path": ["疼痛与伤害感受", heading],
            },
        },
    )


def test_published_hashing_rerank_prefers_section_specific_definition_heading():
    library = KnowledgeLibrary(database_url="postgresql://test")
    pain_definition = _published_pain_result(
        unit_key="pain-definition",
        heading="疼痛（Pain）是什么",
        claim_kind="definition",
        body="IASP 将 pain 定义为不愉快感觉与情绪体验。",
        similarity=0.0515,
    )
    nociception_definition = _published_pain_result(
        unit_key="nociception-definition",
        heading="伤害感受（Nociception）是什么",
        claim_kind="definition",
        body="Nociception 是神经系统编码 noxious stimulus 的过程。",
        similarity=0.0579,
    )

    query = "什么是疼痛？"
    assert library._published_hashing_rerank_score(
        query, pain_definition
    ) > library._published_hashing_rerank_score(query, nociception_definition)


def test_published_hashing_rerank_prefers_interpretation_boundary_for_relation_query():
    library = KnowledgeLibrary(database_url="postgresql://test")
    boundary = _published_pain_result(
        unit_key="boundary",
        heading="疼痛（Pain）与伤害感受（Nociception）的核心解释边界",
        claim_kind="interpretation_boundary",
        body="Pain ≠ Nociception。不能只根据感觉神经元活动推断一个人是否疼痛。",
        similarity=0.5752,
    )
    nociception_definition = _published_pain_result(
        unit_key="nociception-definition",
        heading="伤害感受（Nociception）是什么",
        claim_kind="definition",
        body="Nociception 是神经系统编码 noxious stimulus 的过程。",
        similarity=0.6249,
    )

    query = "疼痛和 nociception 是同一现象吗？"
    assert library._published_hashing_rerank_score(
        query, boundary
    ) > library._published_hashing_rerank_score(query, nociception_definition)


def test_published_hashing_rerank_prefers_boundary_for_inference_and_damage_questions():
    library = KnowledgeLibrary(database_url="postgresql://test")
    boundary = _published_pain_result(
        unit_key="boundary",
        heading="疼痛（Pain）与伤害感受（Nociception）的核心解释边界",
        claim_kind="interpretation_boundary",
        body=(
            "不能只根据感觉神经元活动推断一个人是否疼痛。"
            "没有明确实际或威胁性组织损伤证据时也可能存在 pain。"
        ),
        similarity=0.04,
    )
    pain_definition = _published_pain_result(
        unit_key="pain-definition",
        heading="疼痛（Pain）是什么",
        claim_kind="definition",
        body="IASP 将 pain 定义为不愉快感觉与情绪体验。",
        similarity=0.11,
    )

    for query in [
        "只看感觉神经元活动能判断一个人疼不疼吗？",
        "没有明确组织损伤证据还可能有疼痛吗？",
    ]:
        assert library._published_hashing_rerank_score(
            query, boundary
        ) > library._published_hashing_rerank_score(query, pain_definition)


def test_published_hashing_rerank_keeps_nociception_definition_specific():
    library = KnowledgeLibrary(database_url="postgresql://test")
    boundary = _published_pain_result(
        unit_key="boundary",
        heading="疼痛（Pain）与伤害感受（Nociception）的核心解释边界",
        claim_kind="interpretation_boundary",
        body="Pain 与 Nociception 是不同现象。",
        similarity=0.63,
    )
    nociception_definition = _published_pain_result(
        unit_key="nociception-definition",
        heading="伤害感受（Nociception）是什么",
        claim_kind="definition",
        body="Nociception 是神经系统编码 noxious stimulus 的过程。",
        similarity=0.70,
    )

    query = "nociception 是什么？"
    assert library._published_hashing_rerank_score(
        query, nociception_definition
    ) > library._published_hashing_rerank_score(query, boundary)


def test_published_hashing_rerank_specific_body_evidence_beats_generic_boundary_intent():
    library = KnowledgeLibrary(database_url="postgresql://test")
    boundary = _published_pain_result(
        unit_key="boundary",
        heading="疼痛（Pain）与伤害感受（Nociception）的核心解释边界",
        claim_kind="interpretation_boundary",
        body="Pain 与 Nociception 是不同现象。",
        similarity=0.43,
    )
    nociception = _published_pain_result(
        unit_key="nociception-definition",
        heading="伤害感受（Nociception）是什么",
        claim_kind="definition",
        body=(
            "Nociception 可能伴随 withdrawal reflex 和 autonomic response，"
            "但并不意味着主观 pain experience 一定存在。"
        ),
        similarity=0.45,
    )

    query = "withdrawal reflex 一定代表主观疼痛吗？"
    assert library._published_hashing_rerank_score(
        query, nociception
    ) > library._published_hashing_rerank_score(query, boundary)
