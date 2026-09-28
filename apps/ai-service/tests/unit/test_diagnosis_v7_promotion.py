from pathlib import Path

from src.evals.diagnosis_promotion import evaluate_promotion_readiness, load_promotion_policy


def test_v7_negation_list_promotion_is_ready_for_shadow() -> None:
    root = Path(__file__).resolve().parents[2]
    policy = load_promotion_policy(root / "data/evals/diagnosis_promotion_policy_v5.json")
    report = evaluate_promotion_readiness(policy)
    assert policy.name == "diagnosis_promotion_v5"
    assert policy.challenger_configuration_id == "diag-config-4eb948f419994367"
    assert report["ready_for_shadow"] is True
    assert report["reasons"] == []
