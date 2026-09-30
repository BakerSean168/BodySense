from __future__ import annotations

from copy import deepcopy

from src.evals.diagnosis_historical_identity_audit import (
    HistoricalIdentityPolicy,
    evaluate_historical_identity,
    load_policy,
)


def test_historical_v3_v7_identity_audit_is_green() -> None:
    policy = load_policy()
    report = evaluate_historical_identity(policy)

    assert report["accepted"] is True
    assert report["passed"] == 5
    assert report["total"] == 5
    assert report["reasons"] == []
    assert [item["version"] for item in report["configurations"]] == [
        "v3",
        "v4",
        "v5",
        "v6",
        "v7",
    ]


def test_historical_identity_audit_fails_closed_on_identity_drift() -> None:
    raw = load_policy().model_dump(mode="json")
    mutated = deepcopy(raw)
    mutated["configurations"][2]["configuration_id"] = "diag-config-drifted"
    policy = HistoricalIdentityPolicy.model_validate(mutated)

    report = evaluate_historical_identity(policy)

    assert report["accepted"] is False
    assert any("v5: configuration_id_mismatch" == reason for reason in report["reasons"])
