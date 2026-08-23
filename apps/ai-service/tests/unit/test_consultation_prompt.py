from src.prompts.consultation import build_rag_context


def test_build_rag_context_prefers_body_markdown_and_includes_clip_metadata():
    result = build_rag_context(
        [
            {
                "title": "什么是肘外翻",
                "summary": "肘外翻是肘关节外偏过大。",
                "body_markdown": "## 定义\n肘外翻是...",
                "category": "posture.cubitus-valgus",
                "source_title": "肘外翻",
                "source_timestamp": "00:00-00:18",
                "clips": [
                    {
                        "title": "肘外翻基础观察演示",
                        "source_timestamp": "00:00-00:18",
                    }
                ],
            }
        ]
    )

    assert "## 相关知识库参考" in result
    assert "什么是肘外翻" in result
    assert "## 定义\n肘外翻是..." in result
    assert "摘要：肘外翻是肘关节外偏过大。" in result
    assert "动作演示：肘外翻基础观察演示（00:00-00:18）" in result


def test_consultation_prompt_requires_explicit_published_answer_attribution():
    from src.prompts.consultation import SYSTEM_PROMPT

    assert "record_answer_attribution" in SYSTEM_PROMPT
    assert "Published Evidence Ref" in SYSTEM_PROMPT
    assert "不要猜测" in SYSTEM_PROMPT
