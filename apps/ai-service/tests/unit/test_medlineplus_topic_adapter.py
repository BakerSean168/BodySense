from __future__ import annotations

import copy
from pathlib import Path

import pytest

from src.rag.medlineplus_topic import (
    RENDERING_SCHEMA_VERSION,
    build_units,
    claim_id_for,
    load_topic_xml,
    parse_markdown_sections,
    render_markdown,
    sha256_bytes,
    sha256_text,
    summary_paragraphs,
)

FIXTURE = Path(__file__).resolve().parents[1] / "fixtures" / "medlineplus" / "fixture-topic.xml"

PLAN = {
    "schema_version": RENDERING_SCHEMA_VERSION,
    "title_zh": "测试主题",
    "problem_display_name": "测试问题",
    "source_block_count": 2,
    "translation_method": "fixture",
    "sections": [
        {
            "heading": "关节由什么组成",
            "paragraphs": ["膝关节由骨骼和软骨组成。"],
            "source_quote_en": "The knee joint is made of bone and cartilage.",
        },
        {
            "heading": "会引起疼痛吗",
            "paragraphs": ["膝关节问题可以引起疼痛。"],
            "source_quote_en": "Knee problems can cause pain.",
        },
    ],
}

SPEC = {
    "units": [
        {
            "unit_key": "mlp-test-001",
            "section": "关节由什么组成",
            "problem_slug": "knee-pain",
            "unit_type": "definition",
            "claim_kind": "definition",
            "summary": "膝关节由骨骼和软骨组成。",
            "tags": ["膝关节"],
            "knowledge_kinds": ["definition"],
        },
        {
            "unit_key": "mlp-test-002",
            "section": "会引起疼痛吗",
            "problem_slug": "knee-pain",
            "unit_type": "impact",
            "claim_kind": "association",
            "summary": "膝关节问题可以引起疼痛。",
            "tags": ["疼痛"],
            "knowledge_kinds": ["anatomy-mechanism"],
        },
    ]
}

COMMIT = "a127b7e642d929072bb78557a87d17646fb0571e"


def _record():
    return load_topic_xml(FIXTURE, "fixture-topic")


def _render():
    record = _record()
    return render_markdown(
        record,
        PLAN,
        raw_sha256=sha256_bytes(FIXTURE.read_bytes()),
        xml_path="topics/fixture-topic.xml",
    )


def test_summary_paragraphs_strip_highlight_markup_and_anchors():
    record = _record()

    assert record.paragraphs[0] == "The knee joint is made of bone and cartilage."
    assert all("span" not in paragraph for paragraph in record.paragraphs)


def test_summary_paragraphs_keep_text_when_highlight_markup_is_nested():
    fragment = '<p>A <span class="qt0">knee</span> and <a href="/x">joint</a>.</p>'

    assert summary_paragraphs(fragment) == ["A knee and joint."]


def test_render_is_byte_stable_across_runs():
    first = _render()
    second = _render()

    assert first == second
    assert first.endswith("\n")
    assert "https://medlineplus.gov/fixturetopic.html" in first


def test_render_keeps_canonical_url_and_attribution():
    markdown = _render()

    assert "> 原文：https://medlineplus.gov/fixturetopic.html" in markdown
    assert "MedlinePlus" in markdown
    assert "c7625abe" not in markdown  # hash is supplied by the caller, not hard-coded


def test_parse_sections_line_ranges_address_the_rendered_body():
    markdown = _render()
    sections = parse_markdown_sections(markdown)
    lines = markdown.split("\n")

    assert [section.heading for section in sections] == ["关节由什么组成", "会引起疼痛吗"]
    for section in sections:
        slice_ = "\n".join(lines[section.line_start - 1 : section.line_end])
        assert slice_ == section.body_markdown
        assert section.heading_path == ["测试主题", section.heading]


def test_unit_hash_equals_body_hash_and_claim_id_is_deterministic():
    markdown = _render()
    units = build_units(
        _record(),
        PLAN,
        SPEC,
        markdown,
        content_commit=COMMIT,
        raw_sha256=sha256_bytes(FIXTURE.read_bytes()),
        markdown_path="topics/fixture-topic.md",
    )

    for unit in units:
        assert unit.content_hash == sha256_text(unit.body_markdown)
        assert unit.metadata["claim_candidate"]["claim_id"] == claim_id_for(
            unit.unit_key, str(unit.content_hash)
        )
        assert unit.metadata["source_locator"]["git_commit"] == COMMIT
        assert unit.review_status == "generated"


def test_one_character_source_change_invalidates_the_rendering_plan():
    record = _record()
    drifted_plan = copy.deepcopy(PLAN)
    drifted_plan["sections"][0]["source_quote_en"] = "The knee joint is made of bone only."

    with pytest.raises(ValueError, match="absent from the capture"):
        render_markdown(
            record,
            drifted_plan,
            raw_sha256="0" * 64,
            xml_path="topics/fixture-topic.xml",
        )


def test_changed_paragraph_count_is_a_hard_failure():
    record = _record()
    drifted_plan = copy.deepcopy(PLAN)
    drifted_plan["source_block_count"] = 3

    with pytest.raises(ValueError, match="source_block_count"):
        render_markdown(
            record,
            drifted_plan,
            raw_sha256="0" * 64,
            xml_path="topics/fixture-topic.xml",
        )


def test_unit_spec_section_must_exist_in_the_rendered_markdown():
    markdown = _render()
    drifted_spec = copy.deepcopy(SPEC)
    drifted_spec["units"][0]["section"] = "不存在的标题"

    with pytest.raises(ValueError, match="absent from the rendered Markdown"):
        build_units(
            _record(),
            PLAN,
            drifted_spec,
            markdown,
            content_commit=COMMIT,
            raw_sha256="0" * 64,
            markdown_path="topics/fixture-topic.md",
        )


def test_duplicate_unit_keys_are_rejected():
    markdown = _render()
    drifted_spec = copy.deepcopy(SPEC)
    drifted_spec["units"][1]["unit_key"] = drifted_spec["units"][0]["unit_key"]

    with pytest.raises(ValueError, match="duplicate unit_key"):
        build_units(
            _record(),
            PLAN,
            drifted_spec,
            markdown,
            content_commit=COMMIT,
            raw_sha256="0" * 64,
            markdown_path="topics/fixture-topic.md",
        )


def test_claim_prefix_defaults_to_knee_and_follows_the_unit_spec():
    markdown = _render()
    spec = copy.deepcopy(SPEC)
    spec["claim_prefix"] = "back"

    units = build_units(
        _record(),
        PLAN,
        spec,
        markdown,
        content_commit=COMMIT,
        raw_sha256=sha256_bytes(FIXTURE.read_bytes()),
        markdown_path="topics/fixture-topic.md",
    )

    assert all(
        unit.metadata["claim_candidate"]["claim_id"].startswith("mlc-back-") for unit in units
    )
    assert claim_id_for("mlp-test-001", "0" * 64) == claim_id_for("mlp-test-001", "0" * 64, "knee")
    assert claim_id_for("mlp-test-001", "0" * 64, "hip").startswith("mlc-hip-")
