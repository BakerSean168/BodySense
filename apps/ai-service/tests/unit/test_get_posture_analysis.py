"""Tests for the get_posture_analysis consultation tool (P4 / Phase 3-B1)."""

from __future__ import annotations

import pytest

from src.services.agent.tools.get_posture_analysis import (
    format_posture_analysis_for_llm,
    handle_get_posture_analysis,
    read_posture_analysis,
)

SAMPLE_SUMMARY = {
    "has_analysis": True,
    "views": [
        {
            "upload_id": "u1",
            "view": "side",
            "file_type": "photo_side",
            "analysis_status": "completed",
            "analysis": {
                "schema_version": 1,
                "view": "side",
                "overall_confidence": "medium",
                "findings": [
                    {
                        "key": "forward_head",
                        "label": "头前移倾向",
                        "severity": "moderate",
                        "confidence": "medium",
                        "evidence": "耳垂位于肩峰前方",
                    }
                ],
                "red_flags": [],
                "summary_markdown": "侧面观可见头前移倾向。",
                "disclaimer": "仅供参考",
            },
        }
    ],
    "findings": [
        {
            "key": "forward_head",
            "label": "头前移倾向",
            "severity": "moderate",
        }
    ],
    "summaries": ["侧面观可见头前移倾向。"],
}


def test_read_posture_analysis_returns_findings_from_stored_result():
    result = read_posture_analysis(SAMPLE_SUMMARY)

    assert result["has_analysis"] is True
    assert "头前移倾向" in result["result_text"]
    assert "analysis_result" in result["result_text"] or "已存储" in result["result_text"]
    assert result["summary"]["views"][0]["view"] == "side"


def test_read_posture_analysis_empty_when_missing():
    result = read_posture_analysis(None)

    assert result["has_analysis"] is False
    assert "没有已完成" in result["result_text"]


def test_read_posture_analysis_filters_by_view():
    result = read_posture_analysis(SAMPLE_SUMMARY, view="front")
    assert result["has_analysis"] is False
    assert "front" in result["result_text"]

    side = read_posture_analysis(SAMPLE_SUMMARY, view="side")
    assert side["has_analysis"] is True
    assert "头前移" in side["result_text"]


@pytest.mark.asyncio
async def test_handle_get_posture_analysis_uses_injected_context():
    result = await handle_get_posture_analysis({"_posture_analysis": SAMPLE_SUMMARY})
    assert result["has_analysis"] is True
    assert "耳垂" in result["result_text"]


def test_format_mentions_no_recompute():
    text = format_posture_analysis_for_llm(SAMPLE_SUMMARY)
    assert "重新视觉分析" in text or "已存储" in text
