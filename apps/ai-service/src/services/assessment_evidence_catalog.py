"""Assessment evidence catalog construction.

Normalizes frozen health inputs into the selectable evidence catalog and
projects it for prompt consumption. Evidence authority decisions (reasoning
eligibility, OCR admissibility, reviewed provenance) live here unchanged.
"""

from __future__ import annotations

from typing import Any
from uuid import UUID

from .assessment_evidence_types import (
    _LEGACY_REPORT_EVIDENCE_POLICIES,
    ASSESSMENT_EVIDENCE_POLICY_REVISION,
    ASSESSMENT_EVIDENCE_POLICY_REVISION_V3,
    ASSESSMENT_EVIDENCE_POLICY_REVISION_V4,
    OCR_INDICATOR_ADMISSIBILITY_POLICY_REVISION,
    AssessmentEvidenceItem,
    _clean_mapping,
)


def _reasoning_eligible_body_state_item(item: dict[str, Any]) -> bool:
    if "assertion_status" in item and item["assertion_status"] != "asserted":
        return False
    if item.get("excluded_from_reasoning") is True:
        return False
    review_state = str(item.get("review_state") or "").strip()
    if review_state and review_state != "confirmed":
        return False
    lifecycle_state = str(item.get("lifecycle_state") or "").strip()
    if lifecycle_state and lifecycle_state != "active":
        return False
    return True


def _body_state_category(item: dict[str, Any]) -> str:
    """Classify coverage without rewriting the native record or source grade."""
    kind = str(item.get("kind") or "")
    if kind == "lifestyle":
        habit = _clean_mapping(item.get("details")).get("habit_type")
        if isinstance(habit, str) and habit:
            return f"lifestyle.{habit}"
    if kind in {"height", "weight"}:
        return f"anthropometry.{kind}"
    return kind


def _compact_body_state_value(item: dict[str, Any]) -> dict[str, Any]:
    return {
        key: item[key]
        for key in (
            "id",
            "source_evidence_id",
            "created_revision",
            "kind",
            "value",
            "details",
            "body_region",
            "method",
            "review_state",
            "statement",
            "raw_value",
            "raw_unit",
            "authorship",
            "observed_at",
            "source",
            "temporal",
        )
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
            _body_state_category(item) or "fact",
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
            _body_state_category(item) or "observation",
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
