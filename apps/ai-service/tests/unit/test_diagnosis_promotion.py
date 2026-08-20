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
