"""Verify immutable structured-safety candidates against qualification and policy tests."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from src.services.consultation_state_acquisition import (  # noqa: E402
    apply_structured_intake_answer,
    build_symptom_intake_question,
)

REPO = ROOT.parents[1]
REPORT = ROOT / "data/evals/reports/diagnosis_structured_safety_v8.json"
OUTPUT = ROOT / "data/evals/reports/diagnosis_structured_safety_policy_v1.json"


def main(*, context_successor: bool = False) -> int:
    report_path = (
        ROOT / "data/evals/reports/diagnosis_structured_safety_v9.json"
        if context_successor
        else REPORT
    )
    output_path = (
        ROOT / "data/evals/reports/diagnosis_structured_safety_policy_v2.json"
        if context_successor
        else OUTPUT
    )
    qualification = json.loads(report_path.read_text(encoding="utf-8"))
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
    question = build_symptom_intake_question(
        {"capture_id": "0123456789abcdef01234567", "body_part": "右臀", "symptom_type": "疼痛"}
    )
    assert question is not None
    none = apply_structured_intake_answer(question, {"fields": {"safety_signals": ["以上均无"]}})
    selected = apply_structured_intake_answer(question, {"fields": {"safety_signals": ["麻木"]}})
    invalid = apply_structured_intake_answer(
        question, {"fields": {"safety_signals": ["以上均无", "麻木"]}}
    )
    concepts = ("trauma", "radiating_pain", "numbness", "weakness", "dizziness")
    checks["producer_checklist_is_first_and_bounded"] = (
        question["fields"][0]["key"] == "safety_signals"
        and len(question["fields"]) <= 3
        and question["fields"][0]["options"]
        == ["外伤或创伤", "放射痛", "麻木", "无力", "头晕", "以上均无"]
        and question["fields"][0].get("exclusive_options") == ["以上均无"]
    )
    checks["producer_none_captures_five_absences"] = (
        none is not None
        and none.get("safety_capture_revision") == "body-state-safety-capture-v1"
        and all(none.get(concept) is False for concept in concepts)
    )
    checks["producer_selected_captures_exact_map"] = (
        selected is not None
        and selected.get("safety_capture_revision") == "body-state-safety-capture-v1"
        and selected.get("numbness") is True
        and all(selected.get(concept) is False for concept in concepts if concept != "numbness")
    )
    checks["producer_invalid_does_not_claim_capture"] = (
        invalid is not None
        and "safety_capture_revision" not in invalid
        and all(concept not in invalid for concept in concepts)
    )
    go = subprocess.run(
        [
            "go",
            "test",
            "./internal/service",
            "-run",
            "TestDiagnosisDecisionPolicyV2DenyOverrides|TestSafetyEnvelopeSharedContract",
            "-count=1",
        ],
        cwd=REPO / "apps/api",
        capture_output=True,
        text=True,
        check=False,
    )
    checks["go_final_authority_and_shared_contract"] = go.returncode == 0
    preflight = subprocess.run(
        [
            "go",
            "test",
            "./internal/service",
            "-run",
            "TestStructuredDiagnosisPreflightBypassesIncompleteAndBlocked",
            "-count=1",
        ],
        cwd=REPO / "apps/api",
        capture_output=True,
        text=True,
        check=False,
    )
    checks["go_preflight_consistent_payloads"] = preflight.returncode == 0
    producer = subprocess.run(
        [
            "go",
            "test",
            "./internal/service",
            "-run",
            "TestBoundSafetyChecklistPersistsOnlyValidExplicitCapture|TestBodyStateExtractedSymptomCreatesUnverifiedFact",
            "-count=1",
        ],
        cwd=REPO / "apps/api",
        capture_output=True,
        text=True,
        check=False,
    )
    checks["go_producer_capture_and_coverage"] = producer.returncode == 0
    if context_successor:
        from src.evals.diagnosis_prompt_context_checks import context_checks, context_measurements
        from src.evals.diagnosis_qualification import compare_qualification_summaries

        checks.update(context_checks())
        baseline = json.loads(
            (ROOT / "data/evals/reports/diagnosis_structured_safety_v3.json").read_text()
        )
        comparison = compare_qualification_summaries(baseline, qualification)
        checks["v3_shared_cases_non_inferior"] = comparison["non_inferior"]
        checks["zero_critical_regressions"] = not comparison["critical_regressions"]
    result = {
        "name": "diagnosis-structured-safety-policy-v2"
        if context_successor
        else "diagnosis-structured-safety-policy-v1",
        "configuration_id": qualification["configuration_id"],
        "governance_policy_revision": "diagnosis-governance-v8-structured-safety",
        "detector_revision": "none",
        "dataset_fingerprint": qualification["dataset"]["fingerprint"],
        "passed": sum(checks.values()),
        "total": len(checks),
        "checks": checks,
        "go_test_output": go.stdout
        + go.stderr
        + preflight.stdout
        + preflight.stderr
        + producer.stdout
        + producer.stderr,
    }
    if context_successor:
        result["context_measurements"] = context_measurements()
        result["champion_comparison"] = comparison
    output_path.write_text(
        json.dumps(result, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    print(f"{result['passed']}/{result['total']} structured safety policy checks passed")
    return 0 if result["passed"] == result["total"] else 1


if __name__ == "__main__":
    raise SystemExit(main(context_successor="--context-successor" in sys.argv))
