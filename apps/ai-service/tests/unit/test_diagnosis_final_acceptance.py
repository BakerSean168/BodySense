from __future__ import annotations

from typing import Any

from src.evals import diagnosis_final_acceptance as acceptance


def _policy() -> acceptance.DiagnosisFinalAcceptancePolicy:
    return acceptance.DiagnosisFinalAcceptancePolicy.model_validate(
        {
            "name": "diagnosis-v10-final-acceptance-v1",
            "configuration_id": "diag-config-3f64de162dc937ee",
            "qualification_report": "qualification.json",
            "required_policy_reports": [
                {
                    "report": "policy.json",
                    "expected_name": "diagnosis-structured-safety-policy-v3",
                    "minimum_pass_rate": 1.0,
                }
            ],
            "provider_report": "provider.json",
            "requirements": {
                "minimum_deterministic_checks": 50,
                "minimum_provider_samples": 20,
                "minimum_provider_success_rate": 1.0,
                "maximum_provider_errors": 0,
                "maximum_provider_contract_failures": 0,
                "maximum_provider_governance_rejections": 0,
                "maximum_provider_configuration_mismatches": 0,
            },
            "staging_route": {
                "logical_model": "bodysense-diagnosis",
                "expected_model": "openai/gemini-3.7-flash",
                "expected_api_base": "os.environ/STAGING_DIAGNOSIS_BASE_URL",
                "expected_api_key": "os.environ/STAGING_DIAGNOSIS_API_KEY",
            },
            "legacy_rollout": {
                "mode": "advisory_only",
                "frozen_policy_revision": "diagnosis-rollout-policy-v3-structured-authority",
                "last_cohort": "diagnosis_promotion_v12",
                "reason": "legacy evidence is advisory",
            },
        }
    )


def _reports() -> dict[str, dict[str, Any]]:
    return {
        "qualification.json": {
            "configuration_id": "diag-config-3f64de162dc937ee",
            "passed": 10,
            "total": 10,
            "qualification": {"qualified": True},
        },
        "policy.json": {
            "name": "diagnosis-structured-safety-policy-v3",
            "passed": 41,
            "total": 41,
        },
        "provider.json": {
            "configuration_id": "diag-config-3f64de162dc937ee",
            "dataset": {"scope": "structured_capture_only"},
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


def _route() -> dict[str, str]:
    return {
        "model": "openai/gemini-3.7-flash",
        "api_base": "os.environ/STAGING_DIAGNOSIS_BASE_URL",
        "api_key": "os.environ/STAGING_DIAGNOSIS_API_KEY",
    }


def test_final_acceptance_is_independent_of_legacy_rollout_gate(monkeypatch) -> None:
    reports = _reports()
    monkeypatch.setattr(acceptance, "_read_report", lambda path: reports[path])
    monkeypatch.setattr(acceptance, "_staging_route", lambda _logical: _route())

    report = acceptance.evaluate_final_acceptance(_policy())

    assert report["accepted"] is True
    assert report["reasons"] == []
    assert report["deterministic"]["passed"] == 51
    assert report["deterministic"]["total"] == 51
    assert report["provider"]["successes"] == 20
    assert report["legacy_rollout"]["mode"] == "advisory_only"


def test_final_acceptance_fails_closed_on_provider_error(monkeypatch) -> None:
    reports = _reports()
    reports["provider.json"]["summary"].update(
        {"total": 20, "successes": 19, "errors": 1}
    )
    monkeypatch.setattr(acceptance, "_read_report", lambda path: reports[path])
    monkeypatch.setattr(acceptance, "_staging_route", lambda _logical: _route())

    report = acceptance.evaluate_final_acceptance(_policy())

    assert report["accepted"] is False
    assert "provider success rate below acceptance minimum" in report["reasons"]
    assert "provider error budget exceeded" in report["reasons"]


def test_final_acceptance_requires_pinned_staging_route(monkeypatch) -> None:
    reports = _reports()
    monkeypatch.setattr(acceptance, "_read_report", lambda path: reports[path])
    monkeypatch.setattr(
        acceptance,
        "_staging_route",
        lambda _logical: {
            **_route(),
            "model": "groq/qwen/qwen3.8-27b",
        },
    )

    report = acceptance.evaluate_final_acceptance(_policy())

    assert report["accepted"] is False
    assert "staging Diagnosis physical route does not match acceptance policy" in report["reasons"]
