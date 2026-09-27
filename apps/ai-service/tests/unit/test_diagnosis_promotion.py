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
