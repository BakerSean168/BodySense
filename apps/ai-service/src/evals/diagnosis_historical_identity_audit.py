"""Historical Diagnosis configuration identity audit for DGS-SAFE-090."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from pydantic import BaseModel, ConfigDict, Field

from src.configuration.diagnosis_agent_config import (
    SERVICE_ROOT,
    get_diagnosis_configuration,
    load_manifest,
)

DEFAULT_POLICY_PATH = (
    SERVICE_ROOT / "data/evals/diagnosis_historical_identity_policy.json"
)
DEFAULT_REPORT_PATH = (
    SERVICE_ROOT / "data/evals/reports/diagnosis_historical_identity_audit.json"
)


class HistoricalConfigurationExpectation(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    version: str = Field(pattern=r"^v[3-7]$")
    manifest: str
    configuration_id: str
    decision_policy_revision: str
    governance_policy_revision: str


class SharedContractExpectation(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    logical_model: str
    model_group_revision: str
    prompt_revision: str
    output_schema_revision: str
    tool_policy_revision: str
    evidence_policy_revision: str


class HistoricalIdentityPolicy(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    name: str
    configurations: list[HistoricalConfigurationExpectation] = Field(
        min_length=5, max_length=5
    )
    shared_contract: SharedContractExpectation


def load_policy(path: Path = DEFAULT_POLICY_PATH) -> HistoricalIdentityPolicy:
    return HistoricalIdentityPolicy.model_validate_json(path.read_text(encoding="utf-8"))


def evaluate_historical_identity(
    policy: HistoricalIdentityPolicy,
) -> dict[str, Any]:
    expected_versions = {"v3", "v4", "v5", "v6", "v7"}
    seen_versions: set[str] = set()
    seen_ids: set[str] = set()
    reasons: list[str] = []
    checks: list[dict[str, Any]] = []

    for item in policy.configurations:
        manifest_path = SERVICE_ROOT / item.manifest
        row_reasons: list[str] = []
        if not manifest_path.is_file():
            row_reasons.append("manifest_missing")
            checks.append(
                {
                    "version": item.version,
                    "manifest": item.manifest,
                    "configuration_id": item.configuration_id,
                    "passed": False,
                    "reasons": row_reasons,
                }
            )
            reasons.append(f"{item.version}: manifest_missing")
            continue

        manifest = load_manifest(manifest_path)
        try:
            resolved = get_diagnosis_configuration(item.configuration_id)
        except ValueError:
            resolved = None
            row_reasons.append("resolver_identity_missing")
        expected_shared = policy.shared_contract.model_dump(mode="json")

        if manifest.configuration_id != item.configuration_id:
            row_reasons.append("configuration_id_mismatch")
        if resolved is not None and resolved.configuration_id != item.configuration_id:
            row_reasons.append("resolver_identity_mismatch")
        if (
            resolved is not None
            and manifest.canonical_behavior_json() != resolved.canonical_behavior_json()
        ):
            row_reasons.append("resolver_behavior_mismatch")
        if manifest.decision_policy_revision != item.decision_policy_revision:
            row_reasons.append("decision_policy_revision_mismatch")
        if manifest.governance_policy_revision != item.governance_policy_revision:
            row_reasons.append("governance_policy_revision_mismatch")
        for field_name, expected in expected_shared.items():
            if getattr(manifest, field_name) != expected:
                row_reasons.append(f"{field_name}_mismatch")

        if item.version in seen_versions:
            row_reasons.append("duplicate_version")
        if item.configuration_id in seen_ids:
            row_reasons.append("duplicate_configuration_id")
        seen_versions.add(item.version)
        seen_ids.add(item.configuration_id)

        checks.append(
            {
                "version": item.version,
                "manifest": item.manifest,
                "configuration_id": item.configuration_id,
                "fingerprint": manifest.fingerprint,
                "decision_policy_revision": manifest.decision_policy_revision,
                "governance_policy_revision": manifest.governance_policy_revision,
                "passed": not row_reasons,
                "reasons": row_reasons,
            }
        )
        reasons.extend(f"{item.version}: {reason}" for reason in row_reasons)

    if seen_versions != expected_versions:
        missing = sorted(expected_versions - seen_versions)
        extra = sorted(seen_versions - expected_versions)
        reasons.append(f"historical version set mismatch missing={missing} extra={extra}")

    passed = sum(1 for check in checks if check["passed"])
    return {
        "name": policy.name,
        "accepted": not reasons and passed == 5,
        "passed": passed,
        "total": 5,
        "reasons": reasons,
        "configurations": checks,
        "shared_contract": policy.shared_contract.model_dump(mode="json"),
    }


def render_report(report: dict[str, Any]) -> str:
    status = "ACCEPTED" if report["accepted"] else "BLOCKED"
    lines = [
        f"Diagnosis historical identity audit: {status}",
        f"identities: {report['passed']}/{report['total']}",
    ]
    for item in report["configurations"]:
        marker = "PASS" if item["passed"] else "FAIL"
        lines.append(
            f"- {item['version']} {item['configuration_id']}: {marker}"
        )
    if report["reasons"]:
        lines.append("reasons:")
        lines.extend(f"- {reason}" for reason in report["reasons"])
    return "\n".join(lines) + "\n"


def write_report(report: dict[str, Any], path: Path = DEFAULT_REPORT_PATH) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
