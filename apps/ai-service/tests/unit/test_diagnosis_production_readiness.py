from __future__ import annotations

from src.evals import diagnosis_production_readiness as readiness


def _policy() -> readiness.DiagnosisProductionPromotionPolicy:
    return readiness.DiagnosisProductionPromotionPolicy.model_validate(
        {
            "name": "diagnosis-v10-production-promotion-readiness-v1",
            "configuration_id": "diag-config-3f64de162dc937ee",
            "final_acceptance_report": "final.json",
            "historical_identity_report": "identity.json",
            "operational_audit_report": "ops.json",
            "production_provider_acceptance_report": "provider.json",
            "minimum_provider_samples": 20,
            "minimum_provider_success_rate": 1.0,
        }
    )


def _green_reports() -> dict[str, dict]:
    return {
        "final.json": {
            "accepted": True,
            "configuration_id": "diag-config-3f64de162dc937ee",
        },
        "identity.json": {"accepted": True},
        "ops.json": {"accepted": True},
        "provider.json": {
            "configuration_id": "diag-config-3f64de162dc937ee",
            "environment": "production-candidate",
            "physical_model": "openai/mimo-v2.5-pro",
            "summary": {
                "total": 20,
                "successes": 20,
                "errors": 0,
                "contract_failures": 0,
                "governance_rejections": 0,
                "configuration_mismatches": 0,
            },
        },
    }


def test_production_readiness_holds_when_provider_evidence_is_missing(monkeypatch) -> None:
    reports = _green_reports()
    reports.pop("provider.json")
    monkeypatch.setattr(
        readiness,
        "_read_optional_report",
        lambda path: reports.get(path),
    )
    monkeypatch.setattr(
        readiness,
        "_diagnosis_route",
        lambda path: {
            "model": (
                "openai/gemini-3.7-flash"
                if path == readiness.STAGING_LITELLM_CONFIG
                else "openai/mimo-v2.5-pro"
            )
        },
    )

    report = readiness.evaluate_production_promotion_readiness(_policy())

    assert report["decision"] == "hold"
    assert report["ready_for_production"] is False
    assert report["routes"]["physical_model_drift"] is True
    assert any("no production provider acceptance report" in reason for reason in report["reasons"])


def test_production_readiness_promotes_only_with_matching_production_evidence(
    monkeypatch,
) -> None:
    reports = _green_reports()
    monkeypatch.setattr(
        readiness,
        "_read_optional_report",
        lambda path: reports.get(path),
    )
    monkeypatch.setattr(
        readiness,
        "_diagnosis_route",
        lambda path: {
            "model": (
                "openai/gemini-3.7-flash"
                if path == readiness.STAGING_LITELLM_CONFIG
                else "openai/mimo-v2.5-pro"
            )
        },
    )

    report = readiness.evaluate_production_promotion_readiness(_policy())

    assert report["decision"] == "promote"
    assert report["ready_for_production"] is True
    assert report["reasons"] == []


def test_production_readiness_rejects_wrong_provider_identity(monkeypatch) -> None:
    reports = _green_reports()
    reports["provider.json"]["physical_model"] = "openai/gemini-3.7-flash"
    monkeypatch.setattr(
        readiness,
        "_read_optional_report",
        lambda path: reports.get(path),
    )
    monkeypatch.setattr(
        readiness,
        "_diagnosis_route",
        lambda path: {
            "model": (
                "openai/gemini-3.7-flash"
                if path == readiness.STAGING_LITELLM_CONFIG
                else "openai/mimo-v2.5-pro"
            )
        },
    )

    report = readiness.evaluate_production_promotion_readiness(_policy())

    assert report["decision"] == "hold"
    assert "provider report physical model does not match production route" in report["reasons"]
