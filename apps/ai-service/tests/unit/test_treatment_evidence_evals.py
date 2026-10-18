import json

from src.configuration.treatment_agent_config import CONFIG_ROOT, load_manifest
from src.evals.treatment_evidence_policy import (
    run_treatment_evidence_policy_qualification,
    treatment_evidence_policy_summary,
)
from src.evals.treatment_qualification import (
    DEFAULT_REPORT_PATH,
    compare_treatment_qualification_summaries,
    report_summary,
    run_treatment_qualification,
)


def test_treatment_evidence_gap_policy_is_nine_of_nine() -> None:
    summary = treatment_evidence_policy_summary(run_treatment_evidence_policy_qualification())
    assert summary["passed"] == 9
    assert summary["total"] == 9
    assert summary["failed"] == 0


def test_treatment_v2_qualifies_on_the_current_native_diagnosis_dataset() -> None:
    challenger = load_manifest(CONFIG_ROOT / "treatment-v2-evidence-gap.yaml")
    summary = report_summary(
        run_treatment_qualification(configuration_id=challenger.configuration_id)
    )

    assert challenger.configuration_id == "treat-config-f68eec9846664596"
    assert summary["passed"] == 4
    assert summary["qualification"]["qualified"] is True


def test_historical_v1_to_v2_promotion_evidence_stays_non_inferior() -> None:
    # The v1 Champion is archived and cannot run against the native Diagnosis
    # input, so the promotion is verified from its frozen paired reports.
    champion_summary = json.loads(DEFAULT_REPORT_PATH.read_text(encoding="utf-8"))
    challenger_summary = json.loads(
        (DEFAULT_REPORT_PATH.parent / "treatment_evidence_gap_challenger.json").read_text(
            encoding="utf-8"
        )
    )
    comparison = compare_treatment_qualification_summaries(
        champion_summary,
        challenger_summary,
    )

    assert challenger_summary["configuration_id"] == "treat-config-f68eec9846664596"
    assert comparison["regressions"] == []
    assert comparison["pass_rate_delta"] == 0.0
    assert comparison["non_inferior"] is True
    assert comparison["promotion_eligible"] is True
