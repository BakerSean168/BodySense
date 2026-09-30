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
            "production_provider_preflight_report": "preflight.json",
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
        "preflight.json": {
            "environment": "production",
            "configured_primary_model": "openai/gemini-3.7-flash",
            "gateway_healthy": True,
            "primary_credential_present": True,
            "fallback_credential_present": True,
            "ready_for_primary_qualification": True,
            "logical_probe": {
                "http_status": 200,
                "actual_model": "openai/gemini-3.7-flash",
                "attempted_fallbacks": 0,
            },
        },
        "provider.json": {
            "configuration_id": "diag-config-3f64de162dc937ee",
            "environment": "production-candidate",
            "physical_model": "openai/gemini-3.7-flash",
            "route_attestation": {
                "before": {
                    "http_status": 200,
                    "actual_model": "openai/gemini-3.7-flash",
                    "attempted_fallbacks": 0,
                },
                "after": {
                    "http_status": 200,
                    "actual_model": "openai/gemini-3.7-flash",
                    "attempted_fallbacks": 0,
                },
            },
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
                else "openai/gemini-3.7-flash"
            )
        },
    )

    report = readiness.evaluate_production_promotion_readiness(_policy())

    assert report["decision"] == "hold"
    assert report["ready_for_production"] is False
    assert report["routes"]["physical_model_drift"] is False
    assert "production provider acceptance report is missing" in report["reasons"]


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
                else "openai/gemini-3.7-flash"
            )
        },
    )

    report = readiness.evaluate_production_promotion_readiness(_policy())

    assert report["decision"] == "promote"
    assert report["ready_for_production"] is True
    assert report["reasons"] == []


def test_production_readiness_rejects_wrong_provider_identity(monkeypatch) -> None:
    reports = _green_reports()
    reports["provider.json"]["physical_model"] = "openrouter/deepseek/deepseek-chat"
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
                else "openai/gemini-3.7-flash"
            )
        },
    )

    report = readiness.evaluate_production_promotion_readiness(_policy())

    assert report["decision"] == "hold"
    assert "provider report physical model does not match production route" in report["reasons"]


def test_production_readiness_holds_when_primary_credential_is_missing(monkeypatch) -> None:
    reports = _green_reports()
    reports["preflight.json"].update(
        {
            "primary_credential_present": False,
            "ready_for_primary_qualification": False,
            "reasons": ["primary_credential_missing", "fallback_credential_expired"],
            "logical_probe": {
                "http_status": 500,
                "actual_model": None,
                "attempted_fallbacks": 0,
            },
        }
    )
    monkeypatch.setattr(readiness, "_read_optional_report", lambda path: reports.get(path))
    monkeypatch.setattr(
        readiness,
        "_diagnosis_route",
        lambda path: {
            "model": (
                "openai/gemini-3.7-flash"
                if path == readiness.STAGING_LITELLM_CONFIG
                else "openai/gemini-3.7-flash"
            )
        },
    )

    report = readiness.evaluate_production_promotion_readiness(_policy())

    assert report["decision"] == "hold"
    assert "production primary provider credential is missing" in report["reasons"]
    assert "production Diagnosis fallback credential is expired" in report["reasons"]
    assert "production Diagnosis logical route probe failed" in report["reasons"]
    assert report["evidence"]["production_provider_preflight_ready"] is False


def test_production_readiness_rejects_attested_fallback(monkeypatch) -> None:
    reports = _green_reports()
    reports["provider.json"]["route_attestation"]["after"] = {
        "http_status": 200,
        "actual_model": "openrouter/deepseek/deepseek-chat",
        "attempted_fallbacks": 1,
    }
    monkeypatch.setattr(readiness, "_read_optional_report", lambda path: reports.get(path))
    monkeypatch.setattr(
        readiness,
        "_diagnosis_route",
        lambda path: {
            "model": (
                "openai/gemini-3.7-flash"
                if path == readiness.STAGING_LITELLM_CONFIG
                else "openai/gemini-3.7-flash"
            )
        },
    )

    report = readiness.evaluate_production_promotion_readiness(_policy())

    assert report["decision"] == "hold"
    assert "provider route attestation after did not use the production model" in report["reasons"]
    assert "provider route attestation after used a fallback" in report["reasons"]
