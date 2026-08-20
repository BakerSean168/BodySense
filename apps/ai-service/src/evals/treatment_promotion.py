"""Machine-verifiable Treatment Champion/Challenger promotion readiness."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from pydantic import BaseModel, ConfigDict, Field

from src.configuration.treatment_agent_config import get_treatment_configuration

SERVICE_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_POLICY_PATH = SERVICE_ROOT / "data/evals/treatment_promotion_policy.json"
DEFAULT_REPORT_PATH = SERVICE_ROOT / "data/evals/reports/treatment_promotion_readiness.json"


class QualificationLink(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    report: str
    configuration_id: str
    predecessor_configuration_id: str | None = None


class RequiredPolicyReport(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    report: str
    minimum_pass_rate: float = Field(ge=0.0, le=1.0)


class InteractionExperimentPolicy(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    required: bool
    reason: str = Field(min_length=1)


class StopRules(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    unsafe_relaxations: int = Field(ge=0)
    forbidden_side_effects: int = Field(ge=0)
    configuration_mismatches: int = Field(ge=0)
    challenger_errors_before_pause: int = Field(ge=1)
    rate_gate_min_samples: int = Field(gt=0)
    max_hard_mismatch_rate: float = Field(ge=0.0, le=1.0)
    max_semantic_mismatch_rate: float = Field(ge=0.0, le=1.0)


class RolloutPolicy(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    shadow_min_samples: int = Field(gt=0)
    canary_steps_bps: list[int] = Field(min_length=1)
    promotion_bps: int
    stable_assignment: str = Field(min_length=1)
    stop_rules: StopRules


class TreatmentPromotionPolicy(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    name: str = Field(min_length=1)
    champion_configuration_id: str
    challenger_configuration_id: str
    qualification_chain: list[QualificationLink] = Field(min_length=2)
    required_policy_reports: list[RequiredPolicyReport] = Field(default_factory=list)
    interaction_experiment: InteractionExperimentPolicy
    rollout: RolloutPolicy


def load_promotion_policy(path: Path = DEFAULT_POLICY_PATH) -> TreatmentPromotionPolicy:
    policy = TreatmentPromotionPolicy.model_validate(json.loads(path.read_text(encoding="utf-8")))
    steps = policy.rollout.canary_steps_bps
    if steps != sorted(set(steps)) or any(step <= 0 or step >= 10000 for step in steps):
        raise ValueError("canary_steps_bps must be unique ascending values between 1 and 9999")
    if policy.rollout.promotion_bps != 10000:
        raise ValueError("promotion_bps must be exactly 10000")
    return policy


def _read_report(relative: str) -> dict[str, Any]:
    raw = json.loads((SERVICE_ROOT / relative).read_text(encoding="utf-8"))
    if not isinstance(raw, dict):
        raise ValueError(f"promotion evidence must be a JSON object: {relative}")
    return raw


def evaluate_promotion_readiness(policy: TreatmentPromotionPolicy) -> dict[str, Any]:
    reasons: list[str] = []
    dataset_fingerprint = ""
    links: list[dict[str, Any]] = []

    for index, link in enumerate(policy.qualification_chain):
        get_treatment_configuration(link.configuration_id)
        report = _read_report(link.report)
        report_config = str(report.get("configuration_id") or "")
        qualification = report.get("qualification") or {}
        dataset = report.get("dataset") or {}
        fingerprint = str(dataset.get("fingerprint") or "")

        if report_config != link.configuration_id:
            reasons.append(f"configuration mismatch in {link.report}")
        if qualification.get("qualified") is not True:
            reasons.append(f"configuration is not qualified: {link.configuration_id}")
        if not fingerprint:
            reasons.append(f"dataset fingerprint missing: {link.report}")
        elif not dataset_fingerprint:
            dataset_fingerprint = fingerprint
        elif fingerprint != dataset_fingerprint:
            reasons.append(f"dataset fingerprint drift: {link.report}")

        comparison: dict[str, Any] | None = None
        if index == 0:
            if link.predecessor_configuration_id is not None:
                reasons.append("first qualification link cannot have a predecessor")
        else:
            readiness = _read_report("data/evals/reports/treatment_evidence_gap_readiness.json")
            raw_comparison = readiness.get("comparison")
            comparison = raw_comparison if isinstance(raw_comparison, dict) else None
            if comparison is None:
                reasons.append("paired Treatment comparison is missing")
            else:
                if comparison.get("challenger_configuration_id") != link.configuration_id:
                    reasons.append("challenger identity mismatch")
                if comparison.get("champion_configuration_id") != link.predecessor_configuration_id:
                    reasons.append("predecessor identity mismatch")
                if comparison.get("non_inferior") is not True:
                    reasons.append("Treatment non-inferiority failed")
                if comparison.get("promotion_eligible") is not True:
                    reasons.append("Treatment promotion eligibility failed")
                if comparison.get("regressions"):
                    reasons.append("Treatment deterministic regressions are present")

        links.append(
            {
                "configuration_id": link.configuration_id,
                "predecessor_configuration_id": link.predecessor_configuration_id,
                "qualified": qualification.get("qualified") is True,
                "passed": report.get("passed"),
                "total": report.get("total"),
                "comparison": comparison,
                "report": link.report,
            }
        )

    if policy.qualification_chain[0].configuration_id != policy.champion_configuration_id:
        reasons.append("qualification chain does not start at declared champion")
    if policy.qualification_chain[-1].configuration_id != policy.challenger_configuration_id:
        reasons.append("qualification chain does not end at declared challenger")

    policy_reports: list[dict[str, Any]] = []
    for required in policy.required_policy_reports:
        report = _read_report(required.report)
        total = int(report.get("total") or 0)
        passed = int(report.get("passed") or 0)
        pass_rate = passed / total if total else 0.0
        if pass_rate < required.minimum_pass_rate:
            reasons.append(f"required policy report failed: {required.report}")
        policy_reports.append(
            {
                "report": required.report,
                "passed": passed,
                "total": total,
                "pass_rate": pass_rate,
                "minimum_pass_rate": required.minimum_pass_rate,
            }
        )

    return {
        "name": policy.name,
        "champion_configuration_id": policy.champion_configuration_id,
        "challenger_configuration_id": policy.challenger_configuration_id,
        "dataset_fingerprint": dataset_fingerprint,
        "qualification_chain": links,
        "required_policy_reports": policy_reports,
        "interaction_experiment": policy.interaction_experiment.model_dump(mode="json"),
        "rollout": policy.rollout.model_dump(mode="json"),
        "ready_for_shadow": not reasons,
        "reasons": reasons,
    }
