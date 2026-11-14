"""Adapter for MedlinePlus health-topic XML into reviewed knowledge units.

The adapter is deliberately limited to deterministic transformations:

    raw topic XML -> plain paragraphs -> rendered Markdown (frozen line numbers)
    -> claim-sized units -> reviewed-pack artifacts

It never writes ``knowledge_units`` and never publishes. Go remains the only
publication authority.
"""

from __future__ import annotations

import hashlib
import html
import json
import re
import xml.etree.ElementTree as ElementTree
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from .knowledge_pack import (
    GeneratedKnowledgePack,
    KnowledgeUnitCandidate,
    SourceVideoMetadata,
    TranscriptSegment,
)

SOURCE_TYPE = "medlineplus_topic"
SOURCE_SNAPSHOT_SCHEMA_VERSION = "bodysense.source-snapshot.medlineplus.v1"
RENDERING_SCHEMA_VERSION = "bodysense.medlineplus-rendering.v1"
UNIT_SPEC_SCHEMA_VERSION = "bodysense.medlineplus-units.v1"
SNAPSHOT_REPOSITORY = "bodysense-knowledge-snapshots"

_ANCHOR_RE = re.compile(r"(?is)<a\b[^>]*>(.*?)</a>")
_SPAN_RE = re.compile(r'(?is)<span\b[^>]*class="[^"]*qt\d[^"]*"[^>]*>(.*?)</span>')
_TAG_RE = re.compile(r"(?s)<[^>]+>")
_WHITESPACE_RE = re.compile(r"[ \t\r\f\v]+")


@dataclass(frozen=True)
class TopicRecord:
    """The structured subset of one MedlinePlus health-topic record we import."""

    slug: str
    title: str
    canonical_url: str
    topic_id: str
    language: str
    date_created: str
    meta_description: str
    paragraphs: list[str]
    see_references: list[str] = field(default_factory=list)
    related_topics: list[str] = field(default_factory=list)
    mesh_descriptors: list[str] = field(default_factory=list)
    groups: list[str] = field(default_factory=list)


@dataclass(frozen=True)
class MarkdownSection:
    """One heading-delimited section of the rendered Markdown."""

    heading: str
    heading_path: list[str]
    line_start: int
    line_end: int
    body_markdown: str


def sha256_bytes(payload: bytes) -> str:
    return hashlib.sha256(payload).hexdigest()


def sha256_text(value: str) -> str:
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


def _collapse(value: str) -> str:
    return _WHITESPACE_RE.sub(" ", value.replace("\n", " ")).strip()


def strip_highlight_markup(fragment: str) -> str:
    """Remove keyword-in-context highlight wrappers while keeping their text.

    ``rettype=brief`` search responses wrap matched terms in ``span.qt0``/
    ``span.qt1``; ``rettype=topic`` records do not. Stripping is defensive so a
    future capture mode cannot leak markup into unit bodies or hashes.
    """
    previous = None
    value = fragment
    while previous != value:
        previous = value
        value = _SPAN_RE.sub(r"\1", value)
    return value


def _plain_text(fragment: str) -> str:
    value = strip_highlight_markup(fragment)
    value = html.unescape(value)
    value = _ANCHOR_RE.sub(r"\1", value)
    value = _TAG_RE.sub(" ", value)
    value = html.unescape(value)
    return _collapse(value)


def summary_paragraphs(summary_fragment: str) -> list[str]:
    """Split the HTML-escaped full summary into normalized paragraphs."""
    value = html.unescape(summary_fragment or "")
    value = strip_highlight_markup(value)
    blocks = re.split(r"(?i)</p\s*>", value)
    paragraphs: list[str] = []
    for block in blocks:
        text = _plain_text(block)
        if text:
            paragraphs.append(text)
    return paragraphs


def load_topic_xml(path: str | Path, slug: str) -> TopicRecord:
    """Read one captured topic XML document without assuming unobserved nodes."""
    resolved = Path(path)
    root = ElementTree.parse(resolved).getroot()
    topics = list(root.iter("health-topic"))
    if len(topics) != 1:
        raise ValueError(
            f"topic capture must contain exactly one health-topic record, found {len(topics)}"
        )
    topic = topics[0]
    summary = topic.findtext("full-summary") or ""
    if not summary.strip():
        raise ValueError(f"topic capture {resolved} has no full-summary text")
    record = TopicRecord(
        slug=slug,
        title=(topic.get("title") or "").strip(),
        canonical_url=(topic.get("url") or "").strip(),
        topic_id=(topic.get("id") or "").strip(),
        language=(topic.get("language") or "").strip(),
        date_created=(topic.get("date-created") or "").strip(),
        meta_description=(topic.get("meta-desc") or "").strip(),
        paragraphs=summary_paragraphs(summary),
        see_references=[_plain_text(item.text or "") for item in topic.findall("see-reference")],
        related_topics=[_plain_text(item.text or "") for item in topic.findall("related-topic")],
        mesh_descriptors=[
            _plain_text(item.get("id") or "") for item in topic.findall("mesh-heading/descriptor")
        ],
        groups=[_plain_text(item.text or "") for item in topic.findall("group")],
    )
    if not record.title or not record.canonical_url:
        raise ValueError(f"topic capture {resolved} is missing title or canonical URL")
    if not record.paragraphs:
        raise ValueError(f"topic capture {resolved} has no readable summary paragraphs")
    return record


def load_rendering_plan(path: str | Path) -> dict[str, Any]:
    payload = json.loads(Path(path).read_text(encoding="utf-8"))
    if payload.get("schema_version") != RENDERING_SCHEMA_VERSION:
        raise ValueError(f"unsupported rendering plan schema at {path}")
    return payload


def load_unit_spec(path: str | Path) -> dict[str, Any]:
    payload = json.loads(Path(path).read_text(encoding="utf-8"))
    if payload.get("schema_version") != UNIT_SPEC_SCHEMA_VERSION:
        raise ValueError(f"unsupported unit spec schema at {path}")
    return payload


def _normalized_source_text(record: TopicRecord) -> str:
    return _collapse(" ".join([record.title, *record.paragraphs])).lower()


def validate_rendering_plan(record: TopicRecord, plan: dict[str, Any]) -> None:
    """Fail closed when the source moved under a reviewed rendering plan.

    Every section must quote text that still exists in the source, and the plan
    must cover the whole summary. A drifted capture is a re-materialization
    event, never a silent rewrite.
    """
    source_text = _normalized_source_text(record)
    expected_blocks = int(plan.get("source_block_count") or 0)
    if expected_blocks != len(record.paragraphs):
        raise ValueError(
            "rendering plan source_block_count does not match the captured summary "
            f"(plan={expected_blocks} source={len(record.paragraphs)})"
        )
    sections = plan.get("sections") or []
    if not sections:
        raise ValueError("rendering plan has no sections")
    seen_headings: set[str] = set()
    for index, section in enumerate(sections):
        heading = str(section.get("heading") or "").strip()
        paragraphs = [str(item).strip() for item in section.get("paragraphs") or []]
        quote = str(section.get("source_quote_en") or "").strip()
        if not heading or not paragraphs or not quote:
            raise ValueError(f"rendering section {index} is incomplete")
        if heading in seen_headings:
            raise ValueError(f"rendering plan repeats heading {heading!r}")
        seen_headings.add(heading)
        if _collapse(quote).lower() not in source_text:
            raise ValueError(
                f"rendering section {heading!r} quotes text that is absent from the capture; "
                "re-capture and re-materialize instead of patching line numbers"
            )


def render_markdown(
    record: TopicRecord,
    plan: dict[str, Any],
    *,
    raw_sha256: str,
    xml_path: str,
) -> str:
    """Render the topic into byte-stable Markdown with stable line numbering."""
    validate_rendering_plan(record, plan)
    lines: list[str] = [
        f"# {plan['title_zh']}",
        "",
        "> 来源：MedlinePlus.gov（美国国家医学图书馆，NLM）。",
        f"> 原文：{record.canonical_url}",
        f"> 来源快照：{xml_path}（SHA-256 {raw_sha256}）",
        "",
    ]
    for section in plan["sections"]:
        lines.append(f"## {section['heading']}")
        lines.append("")
        for paragraph in section["paragraphs"]:
            lines.append(paragraph.strip())
            lines.append("")
    return "\n".join(lines).rstrip("\n") + "\n"


def parse_markdown_sections(markdown: str) -> list[MarkdownSection]:
    """Slice rendered Markdown at level-2 headings with 1-based line numbers."""
    lines = markdown.split("\n")
    if lines and lines[-1] == "":
        lines = lines[:-1]
    document_title = ""
    for line in lines:
        if line.startswith("# "):
            document_title = line[2:].strip()
            break
    sections: list[MarkdownSection] = []
    current_heading: str | None = None
    current_start = 0
    for index, line in enumerate(lines, start=1):
        if line.startswith("## "):
            if current_heading is not None:
                sections.append(
                    _section_slice(lines, document_title, current_heading, current_start, index - 1)
                )
            current_heading = line[3:].strip()
            current_start = index
    if current_heading is not None:
        sections.append(
            _section_slice(lines, document_title, current_heading, current_start, len(lines))
        )
    return sections


def _section_slice(
    lines: list[str],
    document_title: str,
    heading: str,
    line_start: int,
    line_end: int,
) -> MarkdownSection:
    body_lines = lines[line_start - 1 : line_end]
    while body_lines and not body_lines[-1].strip():
        body_lines.pop()
    line_end = line_start + len(body_lines) - 1
    return MarkdownSection(
        heading=heading,
        heading_path=[document_title, heading],
        line_start=line_start,
        line_end=line_end,
        body_markdown="\n".join(body_lines),
    )


def claim_id_for(unit_key: str, claim_content_hash: str, prefix: str = "knee") -> str:
    digest = sha256_text(f"{unit_key}\n{claim_content_hash}")[:12]
    return f"mlc-{prefix}-{digest}"


def snapshot_id_for(content_commit: str, raw_sha256: str) -> str:
    return f"medlineplus-topics:{content_commit}:{raw_sha256[:12]}"


def build_units(
    record: TopicRecord,
    plan: dict[str, Any],
    spec: dict[str, Any],
    markdown: str,
    *,
    content_commit: str,
    raw_sha256: str,
    markdown_path: str,
) -> list[KnowledgeUnitCandidate]:
    """Materialize the shard's units from the committed Markdown sections."""
    validate_rendering_plan(record, plan)
    sections = {section.heading: section for section in parse_markdown_sections(markdown)}
    snapshot_id = snapshot_id_for(content_commit, raw_sha256)
    claim_prefix = str(spec.get("claim_prefix") or "knee")
    planned_headings = [section["heading"] for section in plan["sections"]]
    units: list[KnowledgeUnitCandidate] = []
    seen_keys: set[str] = set()
    for item in spec.get("units") or []:
        heading = str(item.get("section") or "").strip()
        section = sections.get(heading)
        if section is None:
            raise ValueError(
                f"unit spec references section {heading!r} that is absent from the "
                "rendered Markdown"
            )
        if heading not in planned_headings:
            raise ValueError(f"unit spec section {heading!r} is not part of the rendering plan")
        unit_key = str(item.get("unit_key") or "").strip()
        if not unit_key or unit_key in seen_keys:
            raise ValueError(f"unit spec has empty or duplicate unit_key {unit_key!r}")
        seen_keys.add(unit_key)
        content_hash = sha256_text(section.body_markdown)
        claim_id = claim_id_for(unit_key, content_hash, claim_prefix)
        problem_slug = str(item.get("problem_slug") or record.slug).strip()
        knowledge_kinds = list(item.get("knowledge_kinds") or [])
        locator = {
            "locator_type": "markdown_lines",
            "repository": SNAPSHOT_REPOSITORY,
            "git_commit": content_commit,
            "path": markdown_path,
            "line_start": section.line_start,
            "line_end": section.line_end,
            "heading_path": section.heading_path,
            "source_time_applicable": False,
        }
        units.append(
            KnowledgeUnitCandidate(
                unit_key=unit_key,
                problem_slug=problem_slug,
                problem_display_name=str(plan.get("problem_display_name") or plan["title_zh"]),
                category=str(item.get("category") or "knee-pain-explanation"),
                unit_type=str(item.get("unit_type") or "reference"),
                title=f"{plan['title_zh']} · {section.heading}",
                summary=str(item.get("summary") or "").strip(),
                body_markdown=section.body_markdown,
                source_start_sec=float(section.line_start),
                source_end_sec=float(section.line_end),
                evidence_segment_indices=[0],
                tags=list(item.get("tags") or []),
                transcript_excerpt=section.body_markdown,
                content_hash=content_hash,
                metadata={
                    "snapshot_id": snapshot_id,
                    "snapshot_schema_version": SOURCE_SNAPSHOT_SCHEMA_VERSION,
                    "source_type": SOURCE_TYPE,
                    "source_locator": locator,
                    "canonical_url": record.canonical_url,
                    "publisher": "MedlinePlus (NLM)",
                    "attribution": "Source: MedlinePlus, National Library of Medicine",
                    "raw_xml_sha256": raw_sha256,
                    "knowledge_kinds": knowledge_kinds,
                    "translation": {
                        "from": "en",
                        "to": "zh",
                        "method": plan.get("translation_method", ""),
                    },
                    "claim_candidate": {
                        "claim_id": claim_id,
                        "claim_kind": str(item.get("claim_kind") or "definition"),
                        "candidate_scope": "section",
                        "authority_tier": "C",
                        "certainty": "unreviewed",
                        "evidence_level": "unresolved",
                        "external_evidence_status": "unresolved",
                        "population": "unspecified",
                    },
                },
            )
        )
    if not units:
        raise ValueError("unit spec produced no units")
    return units


def build_external_reference(record: TopicRecord, line: int) -> dict[str, Any]:
    return {
        "reference_id": f"medlineplus:{record.slug}",
        "label": record.title,
        "url": record.canonical_url,
        "scope": "section_direct",
        "line": line,
    }


def build_pack(
    record: TopicRecord,
    units: list[KnowledgeUnitCandidate],
    *,
    content_commit: str,
    markdown_path: str,
    raw_sha256: str,
) -> GeneratedKnowledgePack:
    """Assemble the generated pack consumed by the existing ingest path."""
    snapshot_id = snapshot_id_for(content_commit, raw_sha256)
    segments = [
        TranscriptSegment(
            segment_index=index,
            start_sec=float(unit.source_start_sec),
            end_sec=float(unit.source_end_sec),
            text=unit.body_markdown,
            metadata={
                "source_locator": unit.metadata["source_locator"],
                "content_hash": unit.content_hash,
            },
        )
        for index, unit in enumerate(units)
    ]
    source = SourceVideoMetadata(
        source_key=f"medlineplus:health-topic:{record.slug}",
        source_type=SOURCE_TYPE,
        title=record.title,
        author="MedlinePlus / U.S. National Library of Medicine",
        problem_slug=str(units[0].problem_slug),
        problem_display_name=str(units[0].problem_display_name),
        original_file_path=markdown_path,
        language="zh",
        duration_sec=None,
        transcript_provider="medlineplus-topic-xml",
        transcript_model=None,
        transcript_file_path=None,
        metadata={
            "snapshot_id": snapshot_id,
            "snapshot_schema_version": SOURCE_SNAPSHOT_SCHEMA_VERSION,
            "canonical_url": record.canonical_url,
            "topic_id": record.topic_id,
            "raw_xml_sha256": raw_sha256,
            "source_time_applicable": False,
            "publisher": "MedlinePlus (NLM)",
            "attribution": "Source: MedlinePlus, National Library of Medicine",
        },
    )
    return GeneratedKnowledgePack(
        source=source,
        artifact_dir=f"medlineplus://{snapshot_id}/{markdown_path}",
        transcript_segments=segments,
        units=units,
        clips=[],
    )
