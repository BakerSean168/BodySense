"""Deterministic qualification for the v6 Diagnosis negation bridge."""

from __future__ import annotations

from pathlib import Path
from typing import Any

from src.evals.diagnosis_negation_policy import (
    build_negation_policy_task,
    load_negation_policy_dataset,
)

SERVICE_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_NEGATION_POLICY_DATASET_PATH = SERVICE_ROOT / "data/evals/diagnosis_negation_policy_v3.yaml"
DIAGNOSIS_V6_CONFIGURATION_ID = "diag-config-4377355ba2012ce8"
DIAGNOSIS_V6_GOVERNANCE_POLICY_REVISION = "diagnosis-governance-v6-negation-bridge-claims"
DIAGNOSIS_V6_DETECTOR_REVISION = "red-flag-detector-negation-bridge-v3"


def run_negation_policy_qualification(
    path: Path = DEFAULT_NEGATION_POLICY_DATASET_PATH,
) -> Any:
    return load_negation_policy_dataset(path).evaluate_sync(
        build_negation_policy_task(),
        progress=False,
        name="diagnosis-negation-policy-v3",
    )


def negation_policy_summary(report: Any) -> dict[str, Any]:
    cases: list[dict[str, Any]] = []
    passed = 0
    for case in report.cases:
        assertions = {name: bool(result.value) for name, result in case.assertions.items()}
        case_passed = bool(assertions) and all(assertions.values()) and not case.evaluator_failures
        passed += int(case_passed)
        cases.append({"name": case.name, "passed": case_passed, "assertions": assertions})
    return {
        "name": report.name,
        "passed": passed,
        "total": len(report.cases),
        "failed": len(report.cases) - passed,
        "cases": cases,
        "configuration_id": DIAGNOSIS_V6_CONFIGURATION_ID,
        "governance_policy_revision": DIAGNOSIS_V6_GOVERNANCE_POLICY_REVISION,
        "detector_revision": DIAGNOSIS_V6_DETECTOR_REVISION,
    }
