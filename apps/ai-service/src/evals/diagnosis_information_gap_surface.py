"""Deterministic qualification for the v7 Diagnosis claim-surface boundary."""

from __future__ import annotations

import hashlib
import json
from typing import Any

from src.runtime.governance import (
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V6,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V7,
    guard_structured_output,
)

DIAGNOSIS_V6_CONFIGURATION_ID = "diag-config-4377355ba2012ce8"
DIAGNOSIS_V7_CONFIGURATION_ID = "diag-config-0206f70742d8a7a1"
DIAGNOSIS_V7_GOVERNANCE_POLICY_REVISION = "diagnosis-governance-v7-information-gap-surface"
DIAGNOSIS_V7_DETECTOR_REVISION = "red-flag-detector-negation-bridge-v3"


def _payload() -> dict[str, Any]:
    return {
        "summary": "当前信息更符合轻度颈肩负荷模式。",
        "candidates": [
            {
                "name": "颈肩负荷模式",
                "basis": "用户报告久坐后颈肩僵硬，活动后缓解。",
                "impact": "目前影响轻微。",
                "reasoning_summary": "当前证据仅支持轻度负荷相关表现。",
                "typical_symptoms": "可能包括放射痛、头晕、外伤或发热。",
                "differential": "若出现放射痛需进一步鉴别。",
            }
        ],
        "information_gaps": ["仍需确认是否存在放射痛、头晕、外伤或发热。"],
    }


def _cases() -> list[dict[str, Any]]:
    cases: list[dict[str, Any]] = []

    historical = guard_structured_output(
        "diagnosis", _payload(), policy_revision=DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V6
    )
    cases.append(
        {
            "name": "v6-information-gaps-remain-scanned",
            "expected": "rejected",
            "actual": historical.verdict,
        }
    )

    v7 = guard_structured_output(
        "diagnosis", _payload(), policy_revision=DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V7
    )
    cases.append(
        {
            "name": "v7-top-level-information-gaps-excluded",
            "expected": "accepted",
            "actual": v7.verdict,
        }
    )

    for name, location in (
        ("summary-current-claim", ("summary",)),
        ("candidate-basis-current-claim", ("candidates", 0, "basis")),
        ("candidate-impact-current-claim", ("candidates", 0, "impact")),
        ("candidate-reasoning-current-claim", ("candidates", 0, "reasoning_summary")),
        ("unknown-top-level-current-claim", ("future_current_claim",)),
        ("unknown-candidate-current-claim", ("candidates", 0, "future_current_claim")),
        ("nested-information-gaps-are-not-top-level", ("candidates", 0, "information_gaps")),
    ):
        payload = _payload()
        target: Any = payload
        for key in location[:-1]:
            target = target[key]
        target[location[-1]] = "用户当前出现放射痛。"
        result = guard_structured_output(
            "diagnosis", payload, policy_revision=DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V7
        )
        cases.append({"name": name, "expected": "rejected", "actual": result.verdict})

    return cases


def information_gap_surface_summary() -> dict[str, Any]:
    cases = _cases()
    passed = sum(case["expected"] == case["actual"] for case in cases)
    fingerprint = hashlib.sha256(
        json.dumps(cases, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
    ).hexdigest()
    return {
        "name": "diagnosis-information-gap-surface-v7",
        "configuration_id": DIAGNOSIS_V7_CONFIGURATION_ID,
        "governance_policy_revision": DIAGNOSIS_V7_GOVERNANCE_POLICY_REVISION,
        "detector_revision": DIAGNOSIS_V7_DETECTOR_REVISION,
        "dataset_fingerprint": fingerprint,
        "passed": passed,
        "total": len(cases),
        "failed": len(cases) - passed,
        "cases": [{**case, "passed": case["expected"] == case["actual"]} for case in cases],
    }
