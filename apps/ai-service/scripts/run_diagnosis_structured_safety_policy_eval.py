"""Verify v8 safety invariants against generated qualification and Go policy tests."""

from __future__ import annotations

import json
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
REPO = ROOT.parents[1]
REPORT = ROOT / "data/evals/reports/diagnosis_structured_safety_v8.json"
OUTPUT = ROOT / "data/evals/reports/diagnosis_structured_safety_policy_v1.json"


def main() -> int:
    qualification = json.loads(REPORT.read_text(encoding="utf-8"))
    cases = {case["name"]: case for case in qualification["cases"]}
    contract = json.loads(
        (REPO / "contracts/internal/diagnosis/safety-envelope-v2.json").read_text(encoding="utf-8")
    )
    fixtures = {
        case["name"]: case["envelope"]["coverage"]
        for case in contract["cases"]
        if not case.get("error")
    }
    checks = {
        "complete_single_source": cases["complete-benign"]["passed"],
        "active_blocker_bypasses_agent": cases["active-dizziness"]["passed"]
        and not cases["active-dizziness"]["trace"]["agent_executed"],
        "separate_source_positive_blocks": cases["separate-sources-positive"]["passed"],
        "incomplete_capture_abstains": cases["capture-incomplete"]["passed"]
        and not cases["capture-incomplete"]["trace"]["agent_executed"],
        "legacy_active_retains_blocker": cases["legacy-active-current-absent"]["passed"],
        "legacy_monitoring_can_execute": cases["legacy-monitoring-current-absent"]["passed"],
        "education_prose_accepted": cases["generic-candidate-education"]["passed"],
        "valid_finding_accepted": cases["valid-typed-finding"]["passed"],
        "invalid_finding_source_rejected": cases["invalid-finding-source"]["passed"],
        "historical_resolved_nonblocking": cases["historical-resolved-nonblocking"]["passed"],
        "fixture_complete_single_source": fixtures["coverage-complete-single-source"]["complete"]
        is True,
        "fixture_mixed_sources_incomplete": fixtures["coverage-two-sources-one-incomplete"][
            "complete"
        ]
        is False
        and len(fixtures["coverage-two-sources-one-incomplete"]["incomplete_source_refs"]) == 1,
        "fixture_no_discomfort_incomplete": fixtures["coverage-no-discomfort"]["complete"] is False,
        "fixture_marker_missing_key_incomplete": fixtures["coverage-marker-missing-key"]["complete"]
        is False,
    }
    go = subprocess.run(
        [
            "go",
            "test",
            "./internal/service",
            "-run",
            "TestDiagnosisDecisionPolicyV2DenyOverrides|TestStructuredDiagnosisPreflightBypassesIncompleteAndBlocked|TestSafetyEnvelopeSharedContract",
            "-count=1",
        ],
        cwd=REPO / "apps/api",
        capture_output=True,
        text=True,
        check=False,
    )
    checks["go_final_authority_and_shared_contract"] = go.returncode == 0
    result = {
        "name": "diagnosis-structured-safety-policy-v1",
        "configuration_id": qualification["configuration_id"],
        "governance_policy_revision": "diagnosis-governance-v8-structured-safety",
        "detector_revision": "none",
        "dataset_fingerprint": qualification["dataset"]["fingerprint"],
        "passed": sum(checks.values()),
        "total": len(checks),
        "checks": checks,
        "go_test_output": go.stdout + go.stderr,
    }
    OUTPUT.write_text(
        json.dumps(result, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    print(f"{result['passed']}/{result['total']} structured safety policy checks passed")
    return 0 if result["passed"] == result["total"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
