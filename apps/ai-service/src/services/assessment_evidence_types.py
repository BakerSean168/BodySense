"""Shared Assessment evidence contract primitives.

Policy revisions, source/kind and domain tables, the frozen evidence item, and
the generic mapping guard used by the catalog, observation and coverage
modules. Kept separate so the contract vocabulary is defined exactly once.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from ..models.assessment import AssessmentEvidenceSource

ASSESSMENT_EVIDENCE_POLICY_REVISION_V2 = "assessment-evidence-contract-v2"
ASSESSMENT_EVIDENCE_POLICY_REVISION_V3 = "assessment-evidence-contract-v3"
ASSESSMENT_EVIDENCE_POLICY_REVISION_V4 = "assessment-evidence-contract-v4"
ASSESSMENT_EVIDENCE_POLICY_REVISION = ASSESSMENT_EVIDENCE_POLICY_REVISION_V4
OCR_INDICATOR_ADMISSIBILITY_POLICY_REVISION = "ocr-indicator-admissibility-v1"

_LEGACY_REPORT_EVIDENCE_POLICIES = frozenset(
    {
        "assessment-evidence-reuse-v1",
        ASSESSMENT_EVIDENCE_POLICY_REVISION_V2,
    }
)

_VISUAL_SOURCES = frozenset({"posture_analysis"})
_KIND_ALLOWED_SOURCES: dict[str, frozenset[str]] = {
    "posture_alignment": _VISUAL_SOURCES,
    "posture_asymmetry": _VISUAL_SOURCES,
    "lifestyle_pattern": frozenset({"body_state"}),
    "exercise_pattern": frozenset({"body_state"}),
    "report_indicator": frozenset({"report"}),
    "anthropometry": frozenset({"body_state"}),
}

_DOMAIN_ORDER = (
    "posture",
    "exercise",
    "lifestyle",
    "anthropometry",
    "health_report",
    "injury_symptoms",
)

_BODY_STATE_LABELS = {
    "lifestyle.activity": "日常活动记录",
    "lifestyle.sleep": "睡眠作息记录",
    "lifestyle.exercise": "运动记录",
    "lifestyle.nutrition": "饮食节律记录",
    "lifestyle.substances": "相关摄入记录",
    "lifestyle.recovery": "恢复与压力记录",
    "anthropometry.height": "身高记录",
    "anthropometry.weight": "体重记录",
    "history.injury_summary": "既往伤病记录",
    "discomfort": "不适记录",
    "symptom": "症状记录",
}


@dataclass(frozen=True, slots=True)
class AssessmentEvidenceItem:
    ref: str
    source: AssessmentEvidenceSource
    kind: str
    value: Any

    def to_prompt_dict(self) -> dict[str, Any]:
        return {
            "ref": self.ref,
            "source": self.source,
            "kind": self.kind,
            "value": self.value,
        }


def _clean_mapping(value: Any) -> dict[str, Any]:
    return value if isinstance(value, dict) else {}
