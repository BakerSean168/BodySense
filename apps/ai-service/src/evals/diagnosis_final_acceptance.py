"""Final acceptance gate for structured-safety Diagnosis v10.

This gate deliberately evaluates v10 on its own safety/runtime contract. Historical
v3-v10 rollout comparisons remain advisory evidence and cannot veto acceptance solely
because legacy v3 produced different prose-detector behavior.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any, Literal

import yaml
from pydantic import BaseModel, ConfigDict, Field

SERVICE_ROOT = Path(__file__).resolve().parents[2]
REPO_ROOT = SERVICE_ROOT.parents[1]
DEFAULT_POLICY_PATH = SERVICE_ROOT / "data/evals/diagnosis_v10_final_acceptance_policy.json"
DEFAULT_REPORT_PATH = SERVICE_ROOT / "data/evals/reports/diagnosis_v10_final_acceptance.json"
STAGING_LITELLM_CONFIG = REPO_ROOT / "docker/litellm/config.staging.yaml"


class RequiredPolicyReport(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    report: str
    expected_name: str
    minimum_pass_rate: float = Field(ge=0.0, le=1.0)


class AcceptanceRequirements(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    minimum_deterministic_checks: int = Field(gt=0)
    minimum_provider_samples: int = Field(gt=0)
    minimum_provider_success_rate: float = Field(ge=0.0, le=1.0)
    maximum_provider_errors: int = Field(ge=0)
    maximum_provider_contract_failures: int = Field(ge=0)
    maximum_provider_governance_rejections: int = Field(ge=0)
    maximum_provider_configuration_mismatches: int = Field(ge=0)


class StagingRouteExpectation(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    logical_model: str
    expected_model: str
    expected_api_base: str
    expected_api_key: str


class LegacyRolloutPolicy(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    mode: Literal["advisory_only"]
    frozen_policy_revision: str
    last_cohort: str
    reason: str = Field(min_length=1)


class DiagnosisFinalAcceptancePolicy(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    name: str
    configuration_id: str
    qualification_report: str
    required_policy_reports: list[RequiredPolicyReport] = Field(min_length=1)
    provider_report: str
    requirements: AcceptanceRequirements
    staging_route: StagingRouteExpectation
    legacy_rollout: LegacyRolloutPolicy


def load_policy(path: Path = DEFAULT_POLICY_PATH) -> DiagnosisFinalAcceptancePolicy:
    return DiagnosisFinalAcceptancePolicy.model_validate_json(path.read_text(encoding="utf-8"))


def _read_report(relative: str) -> dict[str, Any]:
    path = SERVICE_ROOT / relative
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"acceptance evidence must be an object: {relative}")
    return value


def _staging_route(logical_model: str) -> dict[str, Any] | None:
    raw = yaml.safe_load(STAGING_LITELLM_CONFIG.read_text(encoding="utf-8"))
    for item in raw.get("model_list", []):
        if item.get("model_name") == logical_model:
            params = item.get("litellm_params")
            return params if isinstance(params, dict) else None
    return None


def evaluate_final_acceptance(
    policy: DiagnosisFinalAcceptancePolicy,
) -> dict[str, Any]:
    reasons: list[str] = []

    qualification = _read_report(policy.qualification_report)
    qualification_total = int(qualification.get("total") or 0)
    qualification_passed = int(qualification.get("passed") or 0)
    qualification_state = qualification.get("qualification") or {}
    if qualification.get("configuration_id") != policy.configuration_id:
        reasons.append("qualification configuration mismatch")
    if qualification_state.get("qualified") is not True:
        reasons.append("qualification is not qualified")
    if qualification_passed != qualification_total or qualification_total <= 0:
        reasons.append("qualification cases are not 100% green")

    policy_reports: list[dict[str, Any]] = []
    policy_total = 0
    policy_passed = 0
    for required in policy.required_policy_reports:
        report = _read_report(required.report)
        total = int(report.get("total") or 0)
        passed = int(report.get("passed") or 0)
        rate = passed / total if total else 0.0
        policy_total += total
        policy_passed += passed
        if report.get("name") != required.expected_name:
            reasons.append(f"policy report identity mismatch: {required.report}")
        if rate < required.minimum_pass_rate:
            reasons.append(f"policy report failed: {required.report}")
        policy_reports.append(
            {
                "report": required.report,
                "name": report.get("name"),
                "passed": passed,
                "total": total,
                "pass_rate": rate,
                "minimum_pass_rate": required.minimum_pass_rate,
            }
        )

    deterministic_total = qualification_total + policy_total
    deterministic_passed = qualification_passed + policy_passed
    if deterministic_total < policy.requirements.minimum_deterministic_checks:
        reasons.append("deterministic check count below acceptance minimum")
    if deterministic_passed != deterministic_total:
        reasons.append("deterministic acceptance checks are not 100% green")

    provider = _read_report(policy.provider_report)
    provider_dataset = provider.get("dataset") or {}
    if provider_dataset.get("scope") != "structured_capture_only":
        reasons.append("provider report scope is not structured_capture_only")

    provider_summary = provider.get("summary") or {}
    provider_total = int(provider_summary.get("total") or 0)
    provider_successes = int(provider_summary.get("successes") or 0)
    provider_errors = int(provider_summary.get("errors") or 0)
    provider_contract_failures = int(provider_summary.get("contract_failures") or 0)
    provider_governance_rejections = int(provider_summary.get("governance_rejections") or 0)
    provider_configuration_mismatches = int(
        provider_summary.get("configuration_mismatches") or 0
    )
    provider_rate = provider_successes / provider_total if provider_total else 0.0

    if provider.get("configuration_id") != policy.configuration_id:
        reasons.append("provider report configuration mismatch")
    if provider_total < policy.requirements.minimum_provider_samples:
        reasons.append("provider sample count below acceptance minimum")
    if provider_rate < policy.requirements.minimum_provider_success_rate:
        reasons.append("provider success rate below acceptance minimum")
    if provider_errors > policy.requirements.maximum_provider_errors:
        reasons.append("provider error budget exceeded")
    if provider_contract_failures > policy.requirements.maximum_provider_contract_failures:
        reasons.append("provider contract failure budget exceeded")
    if provider_governance_rejections > policy.requirements.maximum_provider_governance_rejections:
        reasons.append("provider governance rejection budget exceeded")
    if (
        provider_configuration_mismatches
        > policy.requirements.maximum_provider_configuration_mismatches
    ):
        reasons.append("provider configuration mismatch budget exceeded")

    route = _staging_route(policy.staging_route.logical_model)
    route_matches = (
        route is not None
        and route.get("model") == policy.staging_route.expected_model
        and route.get("api_base") == policy.staging_route.expected_api_base
        and route.get("api_key") == policy.staging_route.expected_api_key
    )
    if not route_matches:
        reasons.append("staging Diagnosis physical route does not match acceptance policy")

    return {
        "name": policy.name,
        "configuration_id": policy.configuration_id,
        "accepted": not reasons,
        "reasons": reasons,
        "deterministic": {
            "passed": deterministic_passed,
            "total": deterministic_total,
            "minimum_required": policy.requirements.minimum_deterministic_checks,
            "qualification": {
                "report": policy.qualification_report,
                "passed": qualification_passed,
                "total": qualification_total,
                "qualified": qualification_state.get("qualified") is True,
            },
            "policy_reports": policy_reports,
        },
        "provider": {
            "report": policy.provider_report,
            "scope": provider_dataset.get("scope"),
            "successes": provider_successes,
            "total": provider_total,
            "success_rate": provider_rate,
            "errors": provider_errors,
            "contract_failures": provider_contract_failures,
            "governance_rejections": provider_governance_rejections,
            "configuration_mismatches": provider_configuration_mismatches,
        },
        "staging_route": {
            "logical_model": policy.staging_route.logical_model,
            "expected_model": policy.staging_route.expected_model,
            "matches": route_matches,
        },
        "legacy_rollout": policy.legacy_rollout.model_dump(mode="json"),
    }


def render_report(report: dict[str, Any]) -> str:
    deterministic = report["deterministic"]
    provider = report["provider"]
    status = "ACCEPTED" if report["accepted"] else "BLOCKED"
    lines = [
        f"Diagnosis v10 final acceptance: {status}",
        (
            "deterministic: "
            f"{deterministic['passed']}/{deterministic['total']} "
            f"(minimum {deterministic['minimum_required']})"
        ),
        (
            "provider: "
            f"{provider['successes']}/{provider['total']} "
            f"success, errors={provider['errors']}, "
            f"contract_failures={provider['contract_failures']}, "
            f"governance_rejections={provider['governance_rejections']}"
        ),
        f"staging route pinned: {report['staging_route']['matches']}",
        "legacy rollout evidence: advisory_only",
    ]
    if report["reasons"]:
        lines.append("reasons:")
        lines.extend(f"- {reason}" for reason in report["reasons"])
    return "\n".join(lines) + "\n"
