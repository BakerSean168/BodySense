import json
from pathlib import Path

from src.evals.diagnosis_promotion import evaluate_promotion_readiness, load_promotion_policy

ROOT = Path(__file__).resolve().parents[2]


def test_v8_negation_policy_report_is_green_and_identity_bound() -> None:
    report = json.loads(
        (ROOT / "data/evals/reports/diagnosis_negation_policy_v5.json").read_text(
            encoding="utf-8"
        )
    )
    assert report["passed"] == report["total"] == 15
    assert report["failed"] == 0
    assert report["configuration_id"] == "diag-config-d041da102ba90b81"
    assert report["governance_policy_revision"] == (
        "diagnosis-governance-v8-negation-list-local-claims"
    )
    assert report["detector_revision"] == "red-flag-detector-negation-list-local-v5"


def test_v8_promotion_readiness_is_ready_for_shadow() -> None:
    policy = load_promotion_policy(ROOT / "data/evals/diagnosis_promotion_policy_v6.json")
    report = evaluate_promotion_readiness(policy)
    assert policy.name == "diagnosis_promotion_v6"
    assert policy.champion_configuration_id == "diag-config-5a4a13627e14b4cf"
    assert policy.challenger_configuration_id == "diag-config-d041da102ba90b81"
    assert report["ready_for_shadow"] is True
    assert report["reasons"] == []
    assert report["required_policy_reports"][1]["passed"] == 15


def test_v8_challenger_compares_against_v7() -> None:
    report = json.loads(
        (ROOT / "data/evals/reports/diagnosis_negation_list_local_challenger.json").read_text(
            encoding="utf-8"
        )
    )
    assert report["comparison"]["candidate_configuration_id"] == (
        "diag-config-d041da102ba90b81"
    )
    assert report["comparison"]["champion_configuration_id"] == (
        "diag-config-4eb948f419994367"
    )


def test_v6_preserves_v5_chain_and_appends_v8() -> None:
    v5 = json.loads(
        (ROOT / "data/evals/diagnosis_promotion_policy_v5.json").read_text(encoding="utf-8")
    )
    v6 = json.loads(
        (ROOT / "data/evals/diagnosis_promotion_policy_v6.json").read_text(encoding="utf-8")
    )
    expected_ids = [
        "diag-config-5a4a13627e14b4cf",
        "diag-config-4a517fea19cb6c49",
        "diag-config-375187050b203078",
        "diag-config-4377355ba2012ce8",
        "diag-config-4eb948f419994367",
        "diag-config-d041da102ba90b81",
    ]
    assert v6["qualification_chain"][:5] == v5["qualification_chain"]
    assert [item["configuration_id"] for item in v6["qualification_chain"]] == expected_ids
    assert [item["predecessor_configuration_id"] for item in v6["qualification_chain"]] == [
        None,
        "diag-config-5a4a13627e14b4cf",
        "diag-config-4a517fea19cb6c49",
        "diag-config-375187050b203078",
        "diag-config-4377355ba2012ce8",
        "diag-config-4eb948f419994367",
    ]
    assert v6["qualification_chain"][-1]["report"] == (
        "data/evals/reports/diagnosis_negation_list_local_challenger.json"
    )
