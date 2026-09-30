"""Production-promotion readiness decision for Diagnosis v10."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import yaml
from pydantic import BaseModel, ConfigDict, Field

from src.configuration.diagnosis_agent_config import SERVICE_ROOT

REPO_ROOT = SERVICE_ROOT.parents[1]
DEFAULT_POLICY_PATH = SERVICE_ROOT / "data/evals/diagnosis_production_promotion_policy.json"
DEFAULT_REPORT_PATH = (
    SERVICE_ROOT / "data/evals/reports/diagnosis_production_promotion_readiness.json"
)
STAGING_LITELLM_CONFIG = REPO_ROOT / "docker/litellm/config.staging.yaml"
PRODUCTION_LITELLM_CONFIG = REPO_ROOT / "docker/litellm/config.yaml"


class DiagnosisProductionPromotionPolicy(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    name: str
    configuration_id: str
    final_acceptance_report: str
    historical_identity_report: str
    operational_audit_report: str
    production_provider_acceptance_report: str
    minimum_provider_samples: int = Field(gt=0)
    minimum_provider_success_rate: float = Field(ge=0.0, le=1.0)


def load_policy(
    path: Path = DEFAULT_POLICY_PATH,
) -> DiagnosisProductionPromotionPolicy:
    return DiagnosisProductionPromotionPolicy.model_validate_json(
        path.read_text(encoding="utf-8")
    )


def _read_optional_report(relative: str) -> dict[str, Any] | None:
    path = SERVICE_ROOT / relative
    if not path.is_file():
        return None
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"report must be a JSON object: {relative}")
    return value


def _diagnosis_route(path: Path) -> dict[str, Any]:
    raw = yaml.safe_load(path.read_text(encoding="utf-8"))
    for item in raw.get("model_list", []):
        if item.get("model_name") == "bodysense-diagnosis":
            params = item.get("litellm_params")
            if isinstance(params, dict):
                return params
    raise ValueError(f"bodysense-diagnosis route missing: {path}")


def evaluate_production_promotion_readiness(
    policy: DiagnosisProductionPromotionPolicy,
) -> dict[str, Any]:
    reasons: list[str] = []
    acceptance = _read_optional_report(policy.final_acceptance_report)
    identity = _read_optional_report(policy.historical_identity_report)
    operational = _read_optional_report(policy.operational_audit_report)
    provider = _read_optional_report(policy.production_provider_acceptance_report)

    if acceptance is None or acceptance.get("accepted") is not True:
        reasons.append("Diagnosis v10 final acceptance is not green")
    elif acceptance.get("configuration_id") != policy.configuration_id:
        reasons.append("final acceptance configuration mismatch")

    if identity is None or identity.get("accepted") is not True:
        reasons.append("historical v3-v7 identity audit is not green")

    if operational is None or operational.get("accepted") is not True:
        reasons.append("DGS-SAFE-090 operational audit is not green")

    staging_route = _diagnosis_route(STAGING_LITELLM_CONFIG)
    production_route = _diagnosis_route(PRODUCTION_LITELLM_CONFIG)
    staging_model = str(staging_route.get("model") or "")
    production_model = str(production_route.get("model") or "")

    route_drift = staging_model != production_model
    if provider is None:
        if route_drift:
            reasons.append(
                "production Diagnosis provider differs from the staging-accepted provider "
                "and has no production provider acceptance report"
            )
        else:
            reasons.append("production provider acceptance report is missing")
        provider_summary: dict[str, Any] = {}
    else:
        provider_summary = provider.get("summary") or {}
        if provider.get("configuration_id") != policy.configuration_id:
            reasons.append("production provider configuration mismatch")
        if provider.get("environment") != "production-candidate":
            reasons.append("provider report is not production-candidate evidence")
        if provider.get("physical_model") != production_model:
            reasons.append("provider report physical model does not match production route")
        total = int(provider_summary.get("total") or 0)
        successes = int(provider_summary.get("successes") or 0)
        success_rate = successes / total if total else 0.0
        if total < policy.minimum_provider_samples:
            reasons.append("production provider sample count below minimum")
        if success_rate < policy.minimum_provider_success_rate:
            reasons.append("production provider success rate below minimum")
        if int(provider_summary.get("errors") or 0) != 0:
            reasons.append("production provider report contains errors")
        if int(provider_summary.get("contract_failures") or 0) != 0:
            reasons.append("production provider report contains contract failures")
        if int(provider_summary.get("governance_rejections") or 0) != 0:
            reasons.append("production provider report contains governance rejections")
        if int(provider_summary.get("configuration_mismatches") or 0) != 0:
            reasons.append("production provider report contains configuration mismatches")

    return {
        "name": policy.name,
        "configuration_id": policy.configuration_id,
        "decision": "promote" if not reasons else "hold",
        "ready_for_production": not reasons,
        "reasons": reasons,
        "evidence": {
            "final_acceptance": acceptance is not None and acceptance.get("accepted") is True,
            "historical_identity": identity is not None and identity.get("accepted") is True,
            "operational_audit": operational is not None and operational.get("accepted") is True,
            "production_provider_acceptance_present": provider is not None,
        },
        "routes": {
            "staging_model": staging_model,
            "production_model": production_model,
            "physical_model_drift": route_drift,
        },
        "production_provider_summary": provider_summary,
    }


def write_report(report: dict[str, Any], path: Path = DEFAULT_REPORT_PATH) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )


def render_report(report: dict[str, Any]) -> str:
    lines = [
        f"Diagnosis production promotion: {report['decision'].upper()}",
        f"staging model: {report['routes']['staging_model']}",
        f"production model: {report['routes']['production_model']}",
    ]
    if report["reasons"]:
        lines.append("reasons:")
        lines.extend(f"- {reason}" for reason in report["reasons"])
    return "\n".join(lines) + "\n"
