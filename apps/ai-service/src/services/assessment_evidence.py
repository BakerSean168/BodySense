"""Deterministic evidence contract for Assessment.

The model is only a classifier over an authoritative evidence catalog. It never
owns durable observation prose. After exact-ref validation, this module renders
safe observations directly from frozen evidence values and derives evidence
coverage/status/gaps without model-authored health scores.
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Any
from uuid import UUID

from ..models.assessment import AssessmentEvidenceSource
from .governance.types import GovernanceIssue, IssueSeverity

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


def _reasoning_eligible_body_state_item(item: dict[str, Any]) -> bool:
    if item.get("excluded_from_reasoning") is True:
        return False
    review_state = str(item.get("review_state") or "").strip()
    if review_state and review_state != "confirmed":
        return False
    lifecycle_state = str(item.get("lifecycle_state") or "").strip()
    if lifecycle_state and lifecycle_state != "active":
        return False
    return True


def _compact_body_state_value(item: dict[str, Any]) -> dict[str, Any]:
    return {
        key: item[key]
        for key in ("kind", "value", "details", "body_region", "method", "review_state")
        if key in item
    }


def _admissible_report_value(value_mapping: dict[str, Any]) -> bool:
    admissibility = _clean_mapping(value_mapping.get("evidence_admissibility"))
    return (
        admissibility.get("policy_revision") == OCR_INDICATOR_ADMISSIBILITY_POLICY_REVISION
        and admissibility.get("status") == "admissible"
    )


_REVIEWED_ACTIONS = frozenset({"confirm", "correct"})


def _durable_id(value: Any) -> str:
    text = str(value or "").strip()
    if not text or text == "00000000-0000-0000-0000-000000000000":
        return ""
    return text


def _reviewed_uuid(value: Any) -> str:
    text = str(value or "").strip()
    try:
        parsed = UUID(text)
    except (ValueError, TypeError, AttributeError):
        return ""
    return str(parsed) if parsed.int else ""


def _reviewed_source_refs(value: Any) -> list[str] | None:
    if not isinstance(value, list) or not value:
        return None
    refs: list[str] = []
    for raw in value:
        if not isinstance(raw, str) or not raw.strip():
            return None
        ref = raw.strip()
        if ref not in refs:
            refs.append(ref)
    return refs or None


def _reviewed_provenance_complete(item: dict[str, Any]) -> bool:
    """Fail closed unless immutable review + source provenance is exact."""
    if item.get("reviewed") is not True:
        return False
    if not _reviewed_uuid(item.get("upload_id")) or not _reviewed_uuid(
        item.get("extraction_run_id")
    ):
        return False
    if not _reviewed_uuid(item.get("review_id")):
        return False
    indicator_id = str(item.get("indicator_id") or "").strip()
    if not indicator_id:
        return False
    if str(item.get("action") or "").strip() not in _REVIEWED_ACTIONS:
        return False
    indicator_index = item.get("indicator_index")
    if type(indicator_index) is not int or indicator_index < 0:
        return False
    value_mapping = _clean_mapping(item.get("value"))
    if not value_mapping or str(value_mapping.get("indicator_id") or "").strip() != indicator_id:
        return False
    if _reviewed_source_refs(item.get("source_refs")) is None:
        return False
    if not isinstance(item.get("page_ref"), dict):
        return False
    return True


def _attach_review_provenance(
    value_mapping: dict[str, Any], item: dict[str, Any]
) -> dict[str, Any]:
    result = dict(value_mapping)
    result["reviewed"] = True
    result["review_provenance"] = {
        "action": str(item.get("action") or "").strip(),
        "review_id": _reviewed_uuid(item.get("review_id")),
        "reviewer_user_id": _reviewed_uuid(item.get("reviewer_user_id")),
        "upload_id": _reviewed_uuid(item.get("upload_id")),
        "extraction_run_id": _reviewed_uuid(item.get("extraction_run_id")),
        "indicator_id": str(item.get("indicator_id") or "").strip(),
        "indicator_index": item.get("indicator_index"),
        "source_refs": _reviewed_source_refs(item.get("source_refs")) or [],
        "page_ref": dict(item.get("page_ref") or {}),
    }
    return result


def _reviewed_indicator_value(item: dict[str, Any]) -> dict[str, Any] | None:
    value_mapping = _clean_mapping(item.get("value"))
    if not value_mapping:
        return None
    return _attach_review_provenance(value_mapping, item)


def _body_state_ref(prefix: str, item: dict[str, Any], index: int) -> str:
    identity = _durable_id(item.get("id"))
    return f"{prefix}:{identity}" if identity else f"{prefix}:{index}"


def build_assessment_evidence_catalog(
    *,
    profile: dict[str, Any],
    body_state: dict[str, Any],
    report_indicators: list[Any],
    posture_analysis: dict[str, Any],
    reviewed_report_evidence: list[Any] | None = None,
    evidence_policy_revision: str = ASSESSMENT_EVIDENCE_POLICY_REVISION,
) -> dict[str, AssessmentEvidenceItem]:
    """Build the v2 selectable evidence catalog from frozen health inputs.

    Stable profile fields are deliberately excluded: identity/demographics are
    context, not health observations. Raw images are also excluded; governed
    Posture analysis is the sole visual evidence authority.
    """

    _ = profile
    catalog: dict[str, AssessmentEvidenceItem] = {}
    reviewed_report_evidence = reviewed_report_evidence or []

    for index, raw in enumerate(body_state.get("facts") or []):
        item = _clean_mapping(raw)
        if not item or not _reasoning_eligible_body_state_item(item):
            continue
        ref = _body_state_ref("body_state:fact", item, index)
        catalog[ref] = AssessmentEvidenceItem(
            ref,
            "body_state",
            str(item.get("kind") or "fact"),
            _compact_body_state_value(item),
        )

    for index, raw in enumerate(body_state.get("observations") or []):
        item = _clean_mapping(raw)
        if not item or not _reasoning_eligible_body_state_item(item):
            continue
        ref = _body_state_ref("body_state:observation", item, index)
        catalog[ref] = AssessmentEvidenceItem(
            ref,
            "body_state",
            str(item.get("kind") or "observation"),
            _compact_body_state_value(item),
        )

    allowed_policies = frozenset(
        {ASSESSMENT_EVIDENCE_POLICY_REVISION_V3, ASSESSMENT_EVIDENCE_POLICY_REVISION_V4}
    )
    if evidence_policy_revision not in _LEGACY_REPORT_EVIDENCE_POLICIES and (
        evidence_policy_revision not in allowed_policies
    ):
        raise ValueError(
            f"unsupported Assessment evidence policy revision: {evidence_policy_revision}"
        )

    # Machine-admissible report indicators: extraction completion never implies
    # evidence authority, so v3 and v4 both require the exact supported
    # OCR-admissibility decision (ocr-indicator-admissibility-v1 + admissible).
    # Review support never silently reinterprets a machine decision.
    for index, raw in enumerate(report_indicators):
        item = _clean_mapping(raw)
        upload_id = _durable_id(item.get("upload_id")) if item else ""
        indicator_index = item.get("indicator_index") if item else None
        value = item.get("value") if item and "value" in item else raw

        if evidence_policy_revision in (
            ASSESSMENT_EVIDENCE_POLICY_REVISION_V3,
            ASSESSMENT_EVIDENCE_POLICY_REVISION_V4,
        ):
            if not _admissible_report_value(_clean_mapping(value)):
                continue
        if upload_id and isinstance(indicator_index, int):
            ref = f"report:upload:{upload_id}:indicator:{indicator_index}"
        else:
            ref = f"report:{index}"
        catalog[ref] = AssessmentEvidenceItem(ref, "report", "report_indicator", value)

    # Reviewed report evidence is a distinct durable lane carried from the
    # append-only review projection (never from mutating OCRResult). Under v4 a
    # confirmed/corrected latest review may enter the catalog with exact
    # upload/extraction-run/review/indicator provenance. Rejected, unresolved,
    # superseded and provenance-less entries fail closed.
    if evidence_policy_revision == ASSESSMENT_EVIDENCE_POLICY_REVISION_V4:
        for raw in reviewed_report_evidence:
            item = _clean_mapping(raw)
            if not item or not _reviewed_provenance_complete(item):
                continue
            upload_id = _durable_id(item.get("upload_id"))
            indicator_index = item.get("indicator_index")
            if not upload_id or not isinstance(indicator_index, int) or indicator_index < 0:
                continue
            value_mapping = _reviewed_indicator_value(item)
            if value_mapping is None:
                continue
            ref = f"report:upload:{upload_id}:indicator:{indicator_index}"
            catalog[ref] = AssessmentEvidenceItem(ref, "report", "report_indicator", value_mapping)

    views = posture_analysis.get("views") if isinstance(posture_analysis, dict) else None
    seen_posture_summaries: set[str] = set()
    for view_index, raw_view in enumerate(views or []):
        view = _clean_mapping(raw_view)
        analysis = _clean_mapping(view.get("analysis"))
        upload_id = _durable_id(view.get("upload_id"))
        view_ref = f"posture:upload:{upload_id}" if upload_id else f"posture:view:{view_index}"
        for finding_index, raw_finding in enumerate(analysis.get("findings") or []):
            finding = _clean_mapping(raw_finding)
            if not finding:
                continue
            evidence_value = dict(finding)
            evidence_value.setdefault("view", view.get("view") or view.get("file_type") or "")
            ref = f"{view_ref}:finding:{finding_index}"
            catalog[ref] = AssessmentEvidenceItem(
                ref,
                "posture_analysis",
                str(finding.get("key") or "posture_finding"),
                evidence_value,
            )
        summary = str(analysis.get("summary_markdown") or "").strip()
        if summary:
            ref = f"{view_ref}:summary"
            catalog[ref] = AssessmentEvidenceItem(
                ref, "posture_analysis", "posture_summary", summary
            )
            seen_posture_summaries.add(summary)

    for index, summary in enumerate(posture_analysis.get("summaries") or []):
        summary_text = str(summary or "").strip()
        if not summary_text or summary_text in seen_posture_summaries:
            continue
        ref = f"posture:summary:{index}"
        catalog[ref] = AssessmentEvidenceItem(
            ref, "posture_analysis", "posture_summary", summary_text
        )

    return catalog


def evidence_catalog_for_prompt(
    catalog: dict[str, AssessmentEvidenceItem],
) -> dict[str, dict[str, Any]]:
    return {ref: item.to_prompt_dict() for ref, item in catalog.items()}


def _body_state_kind_allowed(observation_kind: str, evidence_kind: str) -> bool:
    if observation_kind == "exercise_pattern":
        return evidence_kind == "lifestyle.exercise"
    if observation_kind == "lifestyle_pattern":
        return evidence_kind.startswith("lifestyle.") and evidence_kind != "lifestyle.exercise"
    if observation_kind == "anthropometry":
        return evidence_kind.startswith("anthropometry.")
    return True


def assessment_evidence_issues(
    payload: dict[str, Any],
    catalog: dict[str, AssessmentEvidenceItem],
) -> list[GovernanceIssue]:
    issues: list[GovernanceIssue] = []
    observations = payload.get("observations")
    if not isinstance(observations, list):
        return issues

    used_refs: set[str] = set()
    for index, raw in enumerate(observations):
        if not isinstance(raw, dict):
            issues.append(
                GovernanceIssue(
                    policy="assessment_evidence_contract",
                    severity=IssueSeverity.ERROR,
                    message=f"Assessment observation {index} is not an object",
                )
            )
            continue

        kind = str(raw.get("kind") or "")
        allowed = _KIND_ALLOWED_SOURCES.get(kind)
        if allowed is None:
            issues.append(
                GovernanceIssue(
                    policy="assessment_evidence_contract",
                    severity=IssueSeverity.ERROR,
                    message=f"Unsupported Assessment observation kind: {kind or '<empty>'}",
                    details={"observation_index": index, "kind": kind},
                )
            )
            continue

        refs = raw.get("evidence_refs")
        if not isinstance(refs, list) or len(refs) != 1:
            issues.append(
                GovernanceIssue(
                    policy="assessment_evidence_contract",
                    severity=IssueSeverity.ERROR,
                    message=(
                        f"Assessment observation {index} must reference exactly one evidence item"
                    ),
                    details={"observation_index": index, "kind": kind},
                )
            )
            continue

        ref = str(refs[0])
        if ref in used_refs:
            issues.append(
                GovernanceIssue(
                    policy="assessment_evidence_contract",
                    severity=IssueSeverity.ERROR,
                    message=f"Assessment evidence {ref} was selected more than once",
                    details={"observation_index": index, "kind": kind, "evidence_ref": ref},
                )
            )
            continue
        used_refs.add(ref)

        item = catalog.get(ref)
        if item is None:
            issues.append(
                GovernanceIssue(
                    policy="assessment_evidence_contract",
                    severity=IssueSeverity.ERROR,
                    message=f"Assessment observation {index} references unavailable evidence {ref}",
                    details={"observation_index": index, "kind": kind, "evidence_ref": ref},
                )
            )
            continue
        if item.source not in allowed:
            issues.append(
                GovernanceIssue(
                    policy="assessment_evidence_contract",
                    severity=IssueSeverity.ERROR,
                    message=(
                        f"Assessment observation {index} kind {kind} cannot use "
                        f"{item.source} evidence"
                    ),
                    details={
                        "observation_index": index,
                        "kind": kind,
                        "evidence_ref": ref,
                        "source": item.source,
                    },
                )
            )
            continue
        if item.source == "body_state" and not _body_state_kind_allowed(kind, item.kind):
            issues.append(
                GovernanceIssue(
                    policy="assessment_evidence_contract",
                    severity=IssueSeverity.ERROR,
                    message=(
                        f"Assessment observation {index} kind {kind} is not supported "
                        f"by BodyState kind {item.kind}"
                    ),
                    details={
                        "observation_index": index,
                        "kind": kind,
                        "evidence_ref": ref,
                        "evidence_kind": item.kind,
                    },
                )
            )

    return issues


def _json_text(value: Any) -> str:
    if isinstance(value, str):
        return value.strip()
    if value is None:
        return ""
    if isinstance(value, (int, float, bool)):
        return str(value)
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def _measurement_text(value: Any) -> str:
    if isinstance(value, dict) and "value" in value:
        raw = _json_text(value.get("value"))
        unit = str(value.get("unit") or "").strip()
        return f"{raw} {unit}".strip()
    return _json_text(value)


def _render_body_state(item: AssessmentEvidenceItem) -> tuple[str, str, str]:
    data = _clean_mapping(item.value)
    raw_value = data.get("value")
    body_region = str(data.get("body_region") or "").strip()
    label = _BODY_STATE_LABELS.get(item.kind, "身体状态记录")
    if item.kind.startswith("anthropometry."):
        text = _measurement_text(raw_value)
    else:
        text = _json_text(raw_value)
    description = f"来源记录：{text}。" if text else "来源记录已存在。"
    return label, description, body_region


def _render_posture(item: AssessmentEvidenceItem) -> tuple[str, str, str]:
    if isinstance(item.value, dict):
        label = str(item.value.get("label") or "体态分析观察").strip()
        evidence = str(item.value.get("evidence") or label).strip()
        body_region = str(item.value.get("body_region") or "").strip()
        return label, f"体态分析记录：{evidence}。", body_region
    text = _json_text(item.value)
    return "体态分析摘要", f"体态分析记录：{text}", ""


def _render_report(item: AssessmentEvidenceItem) -> tuple[str, str, str]:
    value = _clean_mapping(item.value)
    name = str(value.get("name") or "报告指标").strip()
    measured = str(value.get("value") or "").strip()
    unit = str(value.get("unit") or "").strip()
    reference = str(value.get("reference_range") or "").strip()
    measured_text = f"{measured} {unit}".strip()
    parts = [f"{name}={measured_text}" if measured_text else name]
    if reference:
        parts.append(f"参考范围={reference}")
    return name, "报告记录：" + "；".join(parts) + "。", ""


def render_assessment_observations(
    selections: list[dict[str, Any]],
    catalog: dict[str, AssessmentEvidenceItem],
) -> list[dict[str, Any]]:
    """Render durable observation prose solely from trusted evidence values."""

    rendered: list[dict[str, Any]] = []
    for selection in selections:
        kind = str(selection["kind"])
        ref = str(selection["evidence_refs"][0])
        item = catalog[ref]
        if item.source == "posture_analysis":
            label, description, body_region = _render_posture(item)
        elif item.source == "report":
            label, description, body_region = _render_report(item)
        else:
            label, description, body_region = _render_body_state(item)
        rendered.append(
            {
                "kind": kind,
                "body_region": body_region,
                "label": label,
                "description": description,
                "evidence_refs": [ref],
            }
        )
    return rendered


def _domain_refs(catalog: dict[str, AssessmentEvidenceItem], domain: str) -> list[str]:
    refs: list[str] = []
    for ref, item in catalog.items():
        if domain == "posture" and item.source == "posture_analysis":
            refs.append(ref)
        elif (
            domain == "exercise"
            and item.source == "body_state"
            and item.kind == "lifestyle.exercise"
        ):
            refs.append(ref)
        elif domain == "lifestyle" and item.source == "body_state":
            if item.kind.startswith("lifestyle.") and item.kind != "lifestyle.exercise":
                refs.append(ref)
        elif domain == "anthropometry" and item.source == "body_state":
            if item.kind.startswith("anthropometry."):
                refs.append(ref)
        elif domain == "health_report" and item.source == "report":
            refs.append(ref)
        elif domain == "injury_symptoms" and item.source == "body_state":
            if any(
                token in item.kind
                for token in ("injury", "symptom", "pain", "discomfort", "history")
            ):
                refs.append(ref)
    return sorted(refs)


def build_assessment_evidence_coverage(
    catalog: dict[str, AssessmentEvidenceItem],
) -> dict[str, Any]:
    domains: dict[str, dict[str, Any]] = {}
    available = 0
    for domain in _DOMAIN_ORDER:
        refs = _domain_refs(catalog, domain)
        status = "available" if refs else "missing"
        if refs:
            available += 1
        domains[domain] = {"status": status, "evidence_refs": refs}

    if available == len(_DOMAIN_ORDER):
        status = "complete"
    elif available == 0:
        status = "insufficient"
    else:
        status = "partial"

    return {
        "status": status,
        "available_sources": sorted({item.source for item in catalog.values()}),
        "domains": domains,
    }


def build_assessment_evidence_gaps(coverage: dict[str, Any]) -> list[dict[str, Any]]:
    specs: dict[str, tuple[str, list[str]]] = {
        "posture": ("当前未提供已完成的体态分析。", ["posture_analysis"]),
        "exercise": ("当前未提供运动方式或频率记录。", ["body_state"]),
        "lifestyle": ("当前未提供其它生活方式记录。", ["body_state"]),
        "anthropometry": ("当前未提供身体测量记录。", ["body_state"]),
        "health_report": ("当前未提供结构化健康报告指标。", ["report"]),
        "injury_symptoms": ("当前未提供伤病史或症状记录。", ["body_state"]),
    }
    domains = coverage.get("domains") or {}
    gaps: list[dict[str, Any]] = []
    for domain in _DOMAIN_ORDER:
        state = domains.get(domain) or {}
        if state.get("status") != "missing":
            continue
        description, needed_sources = specs[domain]
        gaps.append(
            {
                "dimension": domain,
                "description": description,
                "needed_sources": needed_sources,
                "required": False,
            }
        )
    return gaps


def derive_assessment_status(coverage: dict[str, Any]) -> str:
    return "insufficient_information" if coverage.get("status") == "insufficient" else "completed"


def build_assessment_summary(
    observation_count: int,
    coverage: dict[str, Any],
) -> str:
    domains = coverage.get("domains") or {}
    available = sum(
        1
        for value in domains.values()
        if isinstance(value, dict) and value.get("status") == "available"
    )
    total = len(_DOMAIN_ORDER)
    return (
        f"当前资料支持 {observation_count} 项待审核观察；"
        f"{available}/{total} 个证据领域已有资料，"
        f"{total - available}/{total} 个领域当前未提供资料。"
    )
