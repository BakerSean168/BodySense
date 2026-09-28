"""Deterministic qualification for the v8 Diagnosis local-boundary policy."""

from __future__ import annotations

from pathlib import Path
from typing import Any

from src.evals.diagnosis_negation_policy import (
    build_negation_policy_task,
    load_negation_policy_dataset,
)

SERVICE_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_NEGATION_POLICY_DATASET_PATH = SERVICE_ROOT / "data/evals/diagnosis_negation_policy_v5.yaml"
DIAGNOSIS_V8_CONFIGURATION_ID = "diag-config-d041da102ba90b81"
DIAGNOSIS_V8_GOVERNANCE_POLICY_REVISION = "diagnosis-governance-v8-negation-list-local-claims"
DIAGNOSIS_V8_DETECTOR_REVISION = "red-flag-detector-negation-list-local-v5"


def run_negation_policy_qualification(path: Path = DEFAULT_NEGATION_POLICY_DATASET_PATH) -> Any:
    return load_negation_policy_dataset(path).evaluate_sync(
        build_negation_policy_task(), progress=False, name="diagnosis-negation-policy-v5"
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
        "configuration_id": DIAGNOSIS_V8_CONFIGURATION_ID,
        "governance_policy_revision": DIAGNOSIS_V8_GOVERNANCE_POLICY_REVISION,
        "detector_revision": DIAGNOSIS_V8_DETECTOR_REVISION,
    }
