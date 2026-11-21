"""Assessment evidence coverage, gaps, status and summary derivation.

Deterministic projections over the evidence catalog: which domains are
available, what is missing, and whether information is sufficient. No model
input is involved.
"""

from __future__ import annotations

from typing import Any

from .assessment_evidence_types import _DOMAIN_ORDER, AssessmentEvidenceItem


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
