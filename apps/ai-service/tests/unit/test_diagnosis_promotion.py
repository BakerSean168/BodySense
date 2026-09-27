from pathlib import Path

from src.evals.diagnosis_promotion import evaluate_promotion_readiness, load_promotion_policy


def test_repository_promotion_evidence_is_ready_for_shadow() -> None:
    policy = load_promotion_policy()
    report = evaluate_promotion_readiness(policy)
    assert report["ready_for_shadow"] is True
    assert report["reasons"] == []
    assert report["qualification_chain"][0]["configuration_id"] == policy.champion_configuration_id
    assert (
        report["qualification_chain"][-1]["configuration_id"]
        == policy.challenger_configuration_id
    )
    assert report["interaction_experiment"]["required"] is False
    assert report["rollout"]["canary_steps_bps"] == [500, 2500, 5000]


def test_claim_surface_successor_promotion_evidence_is_ready_for_shadow() -> None:
    policy_path = (
        Path(__file__).resolve().parents[2]
        / "data/evals/diagnosis_promotion_policy_v2.json"
    )
    policy = load_promotion_policy(policy_path)
    report = evaluate_promotion_readiness(policy)

    assert policy.name == "diagnosis_promotion_v2"
    assert policy.champion_configuration_id == "diag-config-5a4a13627e14b4cf"
    assert policy.challenger_configuration_id == "diag-config-4a517fea19cb6c49"
    assert report["ready_for_shadow"] is True
    assert report["reasons"] == []


def test_negation_aware_successor_promotion_evidence_is_ready_for_shadow() -> None:
    policy_path = (
        Path(__file__).resolve().parents[2]
        / "data/evals/diagnosis_promotion_policy_v3.json"
    )
    policy = load_promotion_policy(policy_path)
    report = evaluate_promotion_readiness(policy)

    assert policy.name == "diagnosis_promotion_v3"
    assert policy.champion_configuration_id == "diag-config-5a4a13627e14b4cf"
    assert policy.challenger_configuration_id == "diag-config-375187050b203078"
    assert report["ready_for_shadow"] is True
    assert report["reasons"] == []
    assert report["qualification_chain"][1]["configuration_id"] == "diag-config-4a517fea19cb6c49"
    assert (
        report["qualification_chain"][2]["predecessor_configuration_id"]
        == "diag-config-4a517fea19cb6c49"
    )
    assert [item["report"] for item in report["required_policy_reports"]] == [
        "data/evals/reports/diagnosis_evidence_policy_v2.json",
        "data/evals/reports/diagnosis_negation_policy_v2.json",
    ]
    negation_report = report["required_policy_reports"][1]
    assert negation_report["passed"] == negation_report["total"] == 9
    assert negation_report["identity"] == {
        "name": "diagnosis-negation-policy-v2",
        "configuration_id": "diag-config-375187050b203078",
        "governance_policy_revision": "diagnosis-governance-v5-negation-aware-claims",
        "detector_revision": "red-flag-detector-negation-aware-v2",
    }


def test_partial_negation_policy_report_blocks_readiness(monkeypatch) -> None:
    policy_path = (
        Path(__file__).resolve().parents[2]
        / "data/evals/diagnosis_promotion_policy_v3.json"
    )
    policy = load_promotion_policy(policy_path)
    from src.evals import diagnosis_promotion

    original_read_report = diagnosis_promotion._read_report

    def read_report(path: str):
        report = original_read_report(path)
        if path.endswith("diagnosis_negation_policy_v2.json"):
            return {**report, "passed": report["passed"] - 1}
        return report

    monkeypatch.setattr(diagnosis_promotion, "_read_report", read_report)
    result = evaluate_promotion_readiness(policy)
    assert result["ready_for_shadow"] is False
    assert (
        "required policy report failed: data/evals/reports/diagnosis_negation_policy_v2.json"
        in result["reasons"]
    )
