"""Deterministic evidence contract for Assessment.

The model is only a classifier over an authoritative evidence catalog. It never
owns durable observation prose. After exact-ref validation, this module renders
safe observations directly from frozen evidence values and derives evidence
coverage/status/gaps without model-authored health scores.

Implementation is split by responsibility across sibling modules; this module
keeps the stable public surface:

- ``assessment_evidence_types``: policy revisions, contract tables, item type
- ``assessment_evidence_catalog``: catalog construction and prompt projection
- ``assessment_evidence_observations``: ref validation and observation rendering
- ``assessment_evidence_coverage``: coverage, gaps, status and summary
"""

from __future__ import annotations

from .assessment_evidence_catalog import (
    build_assessment_evidence_catalog,
    evidence_catalog_for_prompt,
)
from .assessment_evidence_coverage import (
    build_assessment_evidence_coverage,
    build_assessment_evidence_gaps,
    build_assessment_summary,
    derive_assessment_status,
)
from .assessment_evidence_observations import (
    assessment_evidence_issues,
    render_assessment_observations,
)
from .assessment_evidence_types import (
    ASSESSMENT_EVIDENCE_POLICY_REVISION,
    ASSESSMENT_EVIDENCE_POLICY_REVISION_V2,
    ASSESSMENT_EVIDENCE_POLICY_REVISION_V3,
    ASSESSMENT_EVIDENCE_POLICY_REVISION_V4,
    OCR_INDICATOR_ADMISSIBILITY_POLICY_REVISION,
    AssessmentEvidenceItem,
)

__all__ = [
    "ASSESSMENT_EVIDENCE_POLICY_REVISION",
    "ASSESSMENT_EVIDENCE_POLICY_REVISION_V2",
    "ASSESSMENT_EVIDENCE_POLICY_REVISION_V3",
    "ASSESSMENT_EVIDENCE_POLICY_REVISION_V4",
    "OCR_INDICATOR_ADMISSIBILITY_POLICY_REVISION",
    "AssessmentEvidenceItem",
    "assessment_evidence_issues",
    "build_assessment_evidence_catalog",
    "build_assessment_evidence_coverage",
    "build_assessment_evidence_gaps",
    "build_assessment_summary",
    "derive_assessment_status",
    "evidence_catalog_for_prompt",
    "render_assessment_observations",
]
