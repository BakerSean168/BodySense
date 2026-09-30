"""Generate deterministic evidence for versioned Diagnosis rollout authority policies."""

from __future__ import annotations

import json
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
REPO = ROOT.parents[1]
OUTPUT_V1 = ROOT / "data/evals/reports/diagnosis_rollout_authority_policy_v1.json"
OUTPUT_V2 = ROOT / "data/evals/reports/diagnosis_rollout_authority_policy_v2.json"
OUTPUT_V3 = ROOT / "data/evals/reports/diagnosis_rollout_authority_policy_v3.json"
PROMOTION_V7 = ROOT / "data/evals/diagnosis_promotion_policy_v7.json"
PROMOTION_V8 = ROOT / "data/evals/diagnosis_promotion_policy_v8.json"
PROMOTION_V9 = ROOT / "data/evals/diagnosis_promotion_policy_v9.json"
PROMOTION_V10 = ROOT / "data/evals/diagnosis_promotion_policy_v10.json"
PROMOTION_V11 = ROOT / "data/evals/diagnosis_promotion_policy_v11.json"
QUALIFICATION_V9 = ROOT / "data/evals/reports/diagnosis_structured_safety_v9.json"
QUALIFICATION_V10 = ROOT / "data/evals/reports/diagnosis_structured_safety_v10.json"

V9_CONFIGURATION_ID = "diag-config-ba10b8e6820c3691"
V10_CONFIGURATION_ID = "diag-config-3f64de162dc937ee"
ROLLOUT_POLICY_V2 = "diagnosis-rollout-policy-v2-structured-authority"
ROLLOUT_POLICY_V3 = "diagnosis-rollout-policy-v3-structured-authority"

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


def main(*, budget_successor: bool = False, preagent_successor: bool = False) -> int:
    if preagent_successor:
        budget_successor = True
    checks: dict[str, bool] = {}
    go_tests = dict(GO_TESTS)
    if budget_successor:
        go_tests["promotion_registry_binds_v2"] = (
            "TestStructuredSafetyPromotionPolicyV9BindsBudgetSuccessor"
        )
        go_tests["provider_fresh_promotion_registry_binds_v2"] = (
            "TestStructuredSafetyPromotionPolicyV10BindsProviderFreshCohort"
        )
    if preagent_successor:
        go_tests.update(
            {
                "preagent_runtime_authority_evidence_extracted": (
                    "TestDiagnosisReplayAuthorityEvidenceRecognizesLegacyPreAgentSafetyFalsePositive"
                ),
                "preagent_evidence_fails_closed_without_exact_proof": (
                    "TestLegacyPreAgentSafetyEvidenceFailsClosedWithoutExactProof"
                ),
                "v2_preserves_preagent_rejection": (
                    "TestStructuredAuthorityV2RejectsLegacyPreAgentFalsePositiveRemoval"
                ),
                "v3_authorizes_proven_preagent_removal": (
                    "TestStructuredAuthorityRolloutAuthorizesProvenLegacyPreAgentFalsePositiveRemoval"
                ),
                "preagent_promotion_registry_binds_v3": (
                    "TestStructuredSafetyPromotionPolicyV11BindsPreAgentAuthorityRollout"
                ),
                "v3_cohort_isolated_from_v2_evidence": (
                    "TestStructuredAuthorityPolicyV3CohortDoesNotInheritV2Evidence"
                ),
            }
        )
    for check, test_name in go_tests.items():
        checks[check] = run_go_test(test_name)

    v7 = json.loads(PROMOTION_V7.read_text(encoding="utf-8"))
    v8 = json.loads(PROMOTION_V8.read_text(encoding="utf-8"))
    v9 = json.loads(PROMOTION_V9.read_text(encoding="utf-8"))
    v10 = json.loads(PROMOTION_V10.read_text(encoding="utf-8"))
    v11 = json.loads(PROMOTION_V11.read_text(encoding="utf-8"))
    current = v11 if preagent_successor else (v10 if budget_successor else v8)
    configuration_id = V10_CONFIGURATION_ID if budget_successor else V9_CONFIGURATION_ID
    rollout_policy = ROLLOUT_POLICY_V3 if preagent_successor else ROLLOUT_POLICY_V2
    qualification = json.loads(
        (QUALIFICATION_V10 if budget_successor else QUALIFICATION_V9).read_text(encoding="utf-8")
    )

    checks["v7_historical_policy_remains_implicit_v1"] = (
        "policy_revision" not in v7["rollout"] and v7["name"] == "diagnosis_promotion_v7"
    )
    checks["v8_explicitly_binds_structured_authority_policy"] = (
        v8["name"] == "diagnosis_promotion_v8"
        and v8["champion_configuration_id"] == "diag-config-5a4a13627e14b4cf"
        and v8["challenger_configuration_id"] == V9_CONFIGURATION_ID
        and v8["rollout"].get("policy_revision") == ROLLOUT_POLICY_V2
    )
    if budget_successor:
        checks["v9_explicitly_binds_budget_successor"] = (
            v9["name"] == "diagnosis_promotion_v9"
            and v9["champion_configuration_id"] == "diag-config-5a4a13627e14b4cf"
            and v9["challenger_configuration_id"] == V10_CONFIGURATION_ID
            and v9["rollout"].get("policy_revision") == ROLLOUT_POLICY_V2
        )
        checks["v10_explicitly_binds_provider_fresh_cohort"] = (
            v10["name"] == "diagnosis_promotion_v10"
            and v10["champion_configuration_id"] == "diag-config-5a4a13627e14b4cf"
            and v10["challenger_configuration_id"] == V10_CONFIGURATION_ID
            and v10["rollout"].get("policy_revision") == ROLLOUT_POLICY_V2
        )
    if preagent_successor:
        checks["v11_explicitly_binds_preagent_structured_authority_policy"] = (
            v11["name"] == "diagnosis_promotion_v11"
            and v11["champion_configuration_id"] == "diag-config-5a4a13627e14b4cf"
            and v11["challenger_configuration_id"] == V10_CONFIGURATION_ID
            and v11["rollout"].get("policy_revision") == ROLLOUT_POLICY_V3
        )
        checks["historical_v10_policy_remains_v2"] = (
            v10["rollout"].get("policy_revision") == ROLLOUT_POLICY_V2
        )
    checks["stop_rules_remain_fail_closed"] = current["rollout"].get("stop_rules") == {
        "unsafe_relaxations": 0,
        "forbidden_side_effects": 0,
        "configuration_mismatches": 0,
        "challenger_errors_before_pause": 1,
        "rate_gate_min_samples": 20,
        "max_hard_mismatch_rate": 0.1,
        "max_semantic_mismatch_rate": 0.25,
    }
    checks["qualified_dataset_identity_matches"] = (
        qualification.get("configuration_id") == configuration_id
        and qualification.get("dataset", {}).get("fingerprint")
        == "7ff22d4eaa9b1f6e8402f7df5647da9d77315b18da6a8a7809afb44d4e4b3876"
        and qualification.get("qualification", {}).get("qualified") is True
    )

    result = {
        "name": (
            "diagnosis-rollout-authority-policy-v3"
            if preagent_successor
            else (
                "diagnosis-rollout-authority-policy-v2"
                if budget_successor
                else "diagnosis-rollout-authority-policy-v1"
            )
        ),
        "configuration_id": configuration_id,
        "rollout_policy_revision": rollout_policy,
        "dataset_fingerprint": qualification.get("dataset", {}).get("fingerprint"),
        "passed": sum(checks.values()),
        "total": len(checks),
        "checks": checks,
        "go_tests": go_tests,
    }
    output = OUTPUT_V3 if preagent_successor else (OUTPUT_V2 if budget_successor else OUTPUT_V1)
    output.write_text(
        json.dumps(result, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(f"{result['passed']}/{result['total']} Diagnosis rollout authority checks passed")
    return 0 if result["passed"] == result["total"] else 1


if __name__ == "__main__":
    import sys

    raise SystemExit(
        main(
            budget_successor="--budget-successor" in sys.argv,
            preagent_successor="--preagent-successor" in sys.argv,
        )
    )
