"""Revision constants and per-kind tables for the governance seam.

These values bind each output kind to the policy revision it was reviewed under
and to the safety fallback text emitted when a payload is rejected.
"""

from __future__ import annotations

from typing import Literal

from ..services.red_flag_detector import (
    RED_FLAG_DETECTOR_REVISION_LITERAL_V1,
    RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2,
    RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3,
    RED_FLAG_DETECTOR_REVISION_NEGATION_LIST_V4,
)

OutputKind = Literal["diagnosis", "treatment", "posture", "assessment"]

DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V3 = "diagnosis-governance-v3"
DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4 = "diagnosis-governance-v4-claim-surface"
DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V5 = "diagnosis-governance-v5-negation-aware-claims"
DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V6 = "diagnosis-governance-v6-negation-bridge-claims"
DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V7 = "diagnosis-governance-v7-negation-list-claims"
DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V8 = "diagnosis-governance-v8-structured-safety"
DIAGNOSIS_GOVERNANCE_POLICY_REVISION = DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4
TREATMENT_GOVERNANCE_POLICY_REVISION = "treatment-governance-v1"
ASSESSMENT_GOVERNANCE_POLICY_REVISION = "assessment-governance-v2"

DIAGNOSIS_RED_FLAG_DETECTOR_REVISION_BY_POLICY = {
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V3: RED_FLAG_DETECTOR_REVISION_LITERAL_V1,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4: RED_FLAG_DETECTOR_REVISION_LITERAL_V1,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V5: RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V6: RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V7: RED_FLAG_DETECTOR_REVISION_NEGATION_LIST_V4,
}

# Fields that must be present for each structured kind.
_REQUIRED_FIELDS: dict[OutputKind, list[str]] = {
    "diagnosis": ["candidates"],
    "treatment": ["goal", "interventions"],
    "posture": ["view", "findings", "summary_markdown", "disclaimer"],
    "assessment": ["observations"],
}

_SAFETY_FALLBACK: dict[OutputKind, str] = {
    "diagnosis": (
        "出于安全考虑，本次未能生成可下发的诊断结论。"
        "请补充更具体的症状信息，或前往专业医疗机构进一步评估。"
        "本系统不构成医疗诊断。"
    ),
    "treatment": (
        "出于安全考虑，本次未能生成可下发的训练方案。"
        "请确认诊断结论后重试，或咨询专业康复/医疗人员获取个性化方案。"
        "本系统不构成医疗处方。"
    ),
    "posture": (
        "出于安全考虑，本次未能生成可下发的体态分析。"
        "请重新上传清晰的站姿照片，或咨询专业人士评估。"
        "本分析不构成医疗诊断。"
    ),
    "assessment": (
        "本次 Assessment 输出未通过证据一致性校验，因此未写入任何健康观察。请补充可验证资料后重试。"
    ),
}
