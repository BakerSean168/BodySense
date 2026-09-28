from src.evals.diagnosis_information_gap_surface import information_gap_surface_summary


def test_information_gap_surface_qualification_is_identity_bound_and_complete():
    report = information_gap_surface_summary()
    assert report["name"] == "diagnosis-information-gap-surface-v7"
    assert report["configuration_id"] == "diag-config-0206f70742d8a7a1"
    assert report["governance_policy_revision"] == (
        "diagnosis-governance-v7-information-gap-surface"
    )
    assert report["detector_revision"] == "red-flag-detector-negation-bridge-v3"
    assert report["passed"] == report["total"] == 9
    assert report["failed"] == 0
