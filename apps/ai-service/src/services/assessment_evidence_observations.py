"""Assessment observation validation and deterministic rendering.

Validates model-selected evidence refs against the catalog and renders durable
observation prose solely from trusted evidence values.
"""

from __future__ import annotations

import json
from typing import Any

from .assessment_evidence_types import (
    _BODY_STATE_LABELS,
    _KIND_ALLOWED_SOURCES,
    AssessmentEvidenceItem,
    _clean_mapping,
)
from .governance.types import GovernanceIssue, IssueSeverity


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
    if isinstance(data.get("statement"), str) and data["statement"]:
        text = data["statement"]
    elif item.kind.startswith("anthropometry."):
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
