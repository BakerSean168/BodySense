"""Adapter for governed Thought Forest health-knowledge snapshots."""

from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Literal

from pydantic import BaseModel, Field, model_validator

from .external_evidence import (
    ExternalEvidenceReviewManifest,
    apply_external_evidence_review,
    build_claim_admissibility,
    resolve_external_reference,
)
from .knowledge_pack import (
    GeneratedKnowledgePack,
    KnowledgeUnitCandidate,
    SourceVideoMetadata,
    TranscriptSegment,
)

SNAPSHOT_SCHEMA_VERSION_V1 = "bodysense.health.snapshot.v1"
SNAPSHOT_SCHEMA_VERSION_V2 = "bodysense.health.snapshot.v2"
SNAPSHOT_SCHEMA_VERSION_V3 = "bodysense.health.snapshot.v3"
SUPPORTED_SNAPSHOT_SCHEMA_VERSIONS = frozenset(
    {SNAPSHOT_SCHEMA_VERSION_V1, SNAPSHOT_SCHEMA_VERSION_V2, SNAPSHOT_SCHEMA_VERSION_V3}
)


class ThoughtForestRepository(BaseModel):
    name: str
    git_commit: str = Field(min_length=1)


class ThoughtForestClaimCandidate(BaseModel):
    claim_id: str = Field(min_length=1, max_length=200)
    claim_kind: Literal[
        "definition",
        "measurement_guidance",
        "interpretation_boundary",
        "association",
        "mechanism_hypothesis",
        "safety_guidance",
        "intervention_option",
        "dosage_guidance",
        "outcome_reassessment",
        "general_reference",
    ]
    candidate_scope: Literal["section"]
    authority_tier: Literal["C"]
    certainty: Literal["unreviewed"]
    evidence_level: Literal["unresolved"]
    external_evidence_status: Literal["unresolved"]
    population: Literal["unspecified"]


class ThoughtForestEvidenceReferenceCandidate(BaseModel):
    reference_id: str = Field(min_length=1, max_length=200)
    label: str = Field(min_length=1)
    url: str = Field(min_length=1)
    scope: Literal["section_direct", "note_bibliography"]
    line: int = Field(ge=1)
    source_resolution_status: Literal["unresolved"]
    support_status: Literal["unreviewed"]


class ThoughtForestSection(BaseModel):
    section_key: str = Field(min_length=1, max_length=200)
    title: str = Field(min_length=1)
    heading_path: list[str]
    line_start: int = Field(ge=1)
    line_end: int = Field(ge=1)
    markdown: str = Field(min_length=1)
    content_hash: str = Field(min_length=16)
    claim_candidate: ThoughtForestClaimCandidate | None = None
    evidence_reference_candidates: list[ThoughtForestEvidenceReferenceCandidate] | None = None

    @model_validator(mode="after")
    def validate_line_range(self) -> "ThoughtForestSection":
        if self.line_end < self.line_start:
            raise ValueError("line_end must be greater than or equal to line_start")
        return self


class ThoughtForestNote(BaseModel):
    source_key: str = Field(min_length=1, max_length=200)
    source_type: str
    path: str = Field(min_length=1)
    title: str = Field(min_length=1)
    aliases: list[str] = Field(default_factory=list)
    description: str = Field(min_length=1)
    tags: list[str]
    note_type: str | None = None
    status: str | None = None
    updated: str | None = None
    problem_slug: str = Field(min_length=1, max_length=100)
    knowledge_kinds: list[str] = Field(min_length=1)
    content_hash: str = Field(min_length=16)
    bibliography_reference_candidates: list[ThoughtForestEvidenceReferenceCandidate] | None = None
    sections: list[ThoughtForestSection] = Field(min_length=1)

    @model_validator(mode="after")
    def validate_source_type(self) -> "ThoughtForestNote":
        if self.source_type != "thought_forest_note":
            raise ValueError(f"unsupported Thought Forest source_type: {self.source_type}")
        if not self.path.startswith("z/") or not self.path.endswith(".md"):
            raise ValueError("Thought Forest note path must be a z/*.md path")
        return self


class ThoughtForestHealthSnapshot(BaseModel):
    schema_version: str
    snapshot_id: str = Field(min_length=1)
    generated_at: str = Field(min_length=1)
    authority_role: str = Field(min_length=1)
    repository: ThoughtForestRepository
    notes: list[ThoughtForestNote] = Field(min_length=1)

    @model_validator(mode="after")
    def validate_schema(self) -> "ThoughtForestHealthSnapshot":
        if self.schema_version not in SUPPORTED_SNAPSHOT_SCHEMA_VERSIONS:
            raise ValueError(f"unsupported Thought Forest snapshot schema: {self.schema_version}")
        if self.repository.name != "thought-forest":
            raise ValueError(f"unexpected snapshot repository: {self.repository.name}")
        if self.schema_version in {SNAPSHOT_SCHEMA_VERSION_V2, SNAPSHOT_SCHEMA_VERSION_V3}:
            missing = [
                section.section_key
                for note in self.notes
                for section in note.sections
                if section.claim_candidate is None
            ]
            if missing:
                raise ValueError(
                    "claim_candidate is required for snapshot "
                    f"{self.schema_version} sections: {missing}"
                )
        if self.schema_version == SNAPSHOT_SCHEMA_VERSION_V3:
            missing_reference_fields = [
                section.section_key
                for note in self.notes
                for section in note.sections
                if section.evidence_reference_candidates is None
            ]
            missing_bibliography = [
                note.path for note in self.notes if note.bibliography_reference_candidates is None
            ]
            if missing_reference_fields:
                raise ValueError(
                    "evidence_reference_candidates is required for snapshot v3 sections: "
                    f"{missing_reference_fields}"
                )
            if missing_bibliography:
                raise ValueError(
                    "bibliography_reference_candidates is required for snapshot v3 notes: "
                    f"{missing_bibliography}"
                )
        return self


def load_thought_forest_snapshot(path: str | Path) -> ThoughtForestHealthSnapshot:
    resolved = Path(path).resolve()
    payload = json.loads(resolved.read_text(encoding="utf-8"))
    return ThoughtForestHealthSnapshot.model_validate(payload)


def _plain_summary(markdown: str, fallback: str) -> str:
    lines: list[str] = []
    in_fence = False
    for raw in markdown.splitlines():
        stripped = raw.strip()
        if stripped.startswith("```"):
            in_fence = not in_fence
            continue
        if in_fence or not stripped or stripped.startswith("#"):
            if lines:
                break
            continue
        if stripped.startswith("> [!") or stripped.startswith("[["):
            continue
        text = re.sub(
            r"\[\[([^\]|]+)(?:\|([^\]]+))?\]\]",
            lambda match: match.group(2) or match.group(1),
            stripped,
        )
        text = re.sub(r"[*_`>#-]+", " ", text)
        text = " ".join(text.split())
        if text:
            lines.append(text)
        if len(" ".join(lines)) >= 420:
            break
    summary = " ".join(lines).strip() or fallback.strip()
    return summary[:500]


def build_generated_packs(
    snapshot: ThoughtForestHealthSnapshot,
    review_manifest: ExternalEvidenceReviewManifest | None = None,
) -> list[GeneratedKnowledgePack]:
    if (
        review_manifest is not None
        and review_manifest.snapshot_git_commit != snapshot.repository.git_commit
    ):
        raise ValueError(
            "external evidence review snapshot_git_commit does not match Thought Forest snapshot"
        )
    packs: list[GeneratedKnowledgePack] = []
    for note in snapshot.notes:
        segments: list[TranscriptSegment] = []
        units: list[KnowledgeUnitCandidate] = []
        for index, section in enumerate(note.sections):
            locator = {
                "locator_type": "markdown_lines",
                "repository": snapshot.repository.name,
                "git_commit": snapshot.repository.git_commit,
                "path": note.path,
                "line_start": section.line_start,
                "line_end": section.line_end,
                "heading_path": section.heading_path,
                "source_time_applicable": False,
            }
            direct_external_references = [
                resolve_external_reference(reference)
                for reference in (section.evidence_reference_candidates or [])
                if reference.scope == "section_direct"
            ]
            if review_manifest is not None and section.claim_candidate is not None:
                direct_external_references = apply_external_evidence_review(
                    claim_id=section.claim_candidate.claim_id,
                    claim_content_hash=section.content_hash,
                    resolved_references=direct_external_references,
                    review_manifest=review_manifest,
                )
            claim_admissibility = build_claim_admissibility(direct_external_references)
            segments.append(
                TranscriptSegment(
                    segment_index=index,
                    start_sec=0.0,
                    end_sec=0.0,
                    text=section.markdown,
                    metadata={"source_locator": locator, "content_hash": section.content_hash},
                )
            )
            units.append(
                KnowledgeUnitCandidate(
                    unit_key=section.section_key,
                    problem_slug=note.problem_slug,
                    problem_display_name=note.title,
                    category=note.knowledge_kinds[0],
                    unit_type="reference",
                    title=f"{note.title} · {section.title}",
                    summary=_plain_summary(section.markdown, note.description),
                    body_markdown=section.markdown,
                    source_start_sec=0.0,
                    source_end_sec=0.0,
                    evidence_segment_indices=[index],
                    tags=note.tags,
                    transcript_excerpt=section.markdown,
                    review_status="generated",
                    metadata={
                        "source_locator": locator,
                        "snapshot_id": snapshot.snapshot_id,
                        "snapshot_schema_version": snapshot.schema_version,
                        "authority_role": snapshot.authority_role,
                        "knowledge_kinds": note.knowledge_kinds,
                        "note_type": note.note_type,
                        "note_status": note.status,
                        "note_content_hash": note.content_hash,
                        "section_content_hash": section.content_hash,
                        **(
                            {"claim_candidate": section.claim_candidate.model_dump()}
                            if section.claim_candidate is not None
                            else {}
                        ),
                        **(
                            {
                                "external_evidence_candidates": direct_external_references,
                                "claim_admissibility": claim_admissibility,
                            }
                            if snapshot.schema_version == SNAPSHOT_SCHEMA_VERSION_V3
                            else {}
                        ),
                    },
                )
            )

        bibliography_references = [
            resolve_external_reference(reference)
            for reference in (note.bibliography_reference_candidates or [])
            if reference.scope == "note_bibliography"
        ]

        source = SourceVideoMetadata(
            source_key=note.source_key,
            source_type="thought_forest_note",
            title=note.title,
            author="Thought Forest",
            problem_slug=note.problem_slug,
            problem_display_name=note.title,
            original_file_path=note.path,
            language="zh",
            duration_sec=None,
            transcript_provider="thought-forest-export",
            transcript_model=None,
            transcript_file_path=None,
            metadata={
                "snapshot_id": snapshot.snapshot_id,
                "snapshot_schema_version": snapshot.schema_version,
                "authority_role": snapshot.authority_role,
                "repository": snapshot.repository.model_dump(),
                "note_path": note.path,
                "note_updated": note.updated,
                "note_content_hash": note.content_hash,
                "knowledge_kinds": note.knowledge_kinds,
                "source_time_applicable": False,
                **(
                    {"bibliography_reference_candidates": bibliography_references}
                    if snapshot.schema_version == SNAPSHOT_SCHEMA_VERSION_V3
                    else {}
                ),
            },
        )
        packs.append(
            GeneratedKnowledgePack(
                source=source,
                artifact_dir=f"thought-forest://{snapshot.snapshot_id}/{note.path}",
                transcript_segments=segments,
                units=units,
                clips=[],
            )
        )
    return packs
