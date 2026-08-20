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


def test_treatment_evidence_gap_policy_is_five_of_five() -> None:
    summary = treatment_evidence_policy_summary(run_treatment_evidence_policy_qualification())
    assert summary["passed"] == 5
    assert summary["total"] == 5
    assert summary["failed"] == 0


def test_treatment_v2_is_non_inferior_on_same_qualification_dataset() -> None:
    challenger = load_manifest(CONFIG_ROOT / "treatment-v2-evidence-gap.yaml")
    challenger_summary = report_summary(
        run_treatment_qualification(configuration_id=challenger.configuration_id)
    )
    champion_summary = json.loads(DEFAULT_REPORT_PATH.read_text(encoding="utf-8"))
    comparison = compare_treatment_qualification_summaries(
        champion_summary,
        challenger_summary,
    )

    assert challenger.configuration_id == "treat-config-f68eec9846664596"
    assert challenger_summary["passed"] == 4
    assert comparison["regressions"] == []
    assert comparison["pass_rate_delta"] == 0.0
    assert comparison["non_inferior"] is True
    assert comparison["promotion_eligible"] is True
