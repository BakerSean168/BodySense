"""Generate deterministic evidence for Diagnosis rollout structured-authority policy v2."""

from __future__ import annotations

import json
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
REPO = ROOT.parents[1]
OUTPUT = ROOT / "data/evals/reports/diagnosis_rollout_authority_policy_v1.json"
PROMOTION_V7 = ROOT / "data/evals/diagnosis_promotion_policy_v7.json"
PROMOTION_V8 = ROOT / "data/evals/diagnosis_promotion_policy_v8.json"
QUALIFICATION_V9 = ROOT / "data/evals/reports/diagnosis_structured_safety_v9.json"

CONFIGURATION_ID = "diag-config-ba10b8e6820c3691"
ROLLOUT_POLICY = "diagnosis-rollout-policy-v2-structured-authority"
POLICY_NAME = "diagnosis-rollout-authority-policy-v1"

GO_TESTS = {
    "authorized_legacy_false_positive_removal": (
        "TestStructuredAuthorityRolloutAuthorizesProvenLegacyFalsePositiveRemoval"
    ),
    "unproven_relaxations_fail_closed": (
        "TestStructuredAuthorityRolloutStillRejectsUnprovenRelaxations"
    ),
    "historical_v1_cohort_isolated": (
        "TestStructuredAuthorityPolicyKeepsHistoricalV1CohortSeparate"
    ),
    "canary_direction_normalized": (
        "TestStructuredAuthorityClassificationNormalizesCanaryDirection"
    ),
    "legacy_v1_semantics_preserved": "TestLegacyRolloutPolicyStillTreatsBlockToAllowAsUnsafe",
    "runtime_authority_evidence_extracted": (
        "TestDiagnosisReplayAuthorityEvidenceRecognizesLegacyProseFalsePositive"
    ),
    "mixed_governance_not_authorized": (
        "TestLegacyProseAuthorityEvidenceRejectsMixedGovernanceIssues"
    ),
    "legacy_category_requires_exact_absence_proof": (
        "TestStructuredAuthorityRolloutRejectsUncoveredLegacyRedFlagCategory"
    ),
    "confirmed_absence_requires_every_covered_source": (
        "TestDiagnosisReplayAuthorityEvidenceRequiresConfirmedAbsentCoverage"
    ),
    "promotion_registry_binds_v2": (
        "TestStructuredSafetyPromotionPolicyV8BindsStructuredAuthorityRollout"
    ),
    "shadow_errors_stay_in_v2_cohort": (
        "TestStructuredAuthorityShadowErrorStaysInV2PromotionCohort"
    ),
    "missing_reports_stay_in_v2_cohort": (
        "TestStructuredAuthorityMissingReportStaysInV2PromotionCohort"
    ),
    "promotion_cohorts_remain_separate": ("TestStructuredAuthorityPromotionCohortsRemainSeparate"),
}


def run_go_test(name: str) -> bool:
    proc = subprocess.run(
        ["go", "test", "./internal/service", "-run", f"^{name}$", "-count=1"],
        cwd=REPO / "apps/api",
        capture_output=True,
        text=True,
        check=False,
    )
    return proc.returncode == 0


def main() -> int:
    checks: dict[str, bool] = {}
    for check, test_name in GO_TESTS.items():
        checks[check] = run_go_test(test_name)

    v7 = json.loads(PROMOTION_V7.read_text(encoding="utf-8"))
    v8 = json.loads(PROMOTION_V8.read_text(encoding="utf-8"))
    qualification = json.loads(QUALIFICATION_V9.read_text(encoding="utf-8"))

    checks["v7_historical_policy_remains_implicit_v1"] = (
        "policy_revision" not in v7["rollout"] and v7["name"] == "diagnosis_promotion_v7"
    )
    checks["v8_explicitly_binds_structured_authority_policy"] = (
        v8["name"] == "diagnosis_promotion_v8"
        and v8["champion_configuration_id"] == "diag-config-5a4a13627e14b4cf"
        and v8["challenger_configuration_id"] == CONFIGURATION_ID
        and v8["rollout"].get("policy_revision") == ROLLOUT_POLICY
    )
    checks["stop_rules_remain_fail_closed"] = v8["rollout"].get("stop_rules") == {
        "unsafe_relaxations": 0,
        "forbidden_side_effects": 0,
        "configuration_mismatches": 0,
        "challenger_errors_before_pause": 1,
        "rate_gate_min_samples": 20,
        "max_hard_mismatch_rate": 0.1,
        "max_semantic_mismatch_rate": 0.25,
    }
    checks["qualified_dataset_identity_matches"] = (
        qualification.get("configuration_id") == CONFIGURATION_ID
        and qualification.get("dataset", {}).get("fingerprint")
        == "7ff22d4eaa9b1f6e8402f7df5647da9d77315b18da6a8a7809afb44d4e4b3876"
        and qualification.get("qualification", {}).get("qualified") is True
    )

    result = {
        "name": POLICY_NAME,
        "configuration_id": CONFIGURATION_ID,
        "rollout_policy_revision": ROLLOUT_POLICY,
        "dataset_fingerprint": qualification.get("dataset", {}).get("fingerprint"),
        "passed": sum(checks.values()),
        "total": len(checks),
        "checks": checks,
        "go_tests": GO_TESTS,
    }
    OUTPUT.write_text(
        json.dumps(result, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(f"{result['passed']}/{result['total']} Diagnosis rollout authority checks passed")
    return 0 if result["passed"] == result["total"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
