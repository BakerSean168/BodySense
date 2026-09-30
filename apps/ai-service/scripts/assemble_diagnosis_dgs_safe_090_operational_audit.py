"""Assemble non-sensitive DGS-SAFE-090 operational evidence from runtime reports."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_OUTPUT = ROOT / "data/evals/reports/diagnosis_dgs_safe_090_operational_audit.json"
DEFAULT_IDENTITY = ROOT / "data/evals/reports/diagnosis_historical_identity_audit.json"
V10 = "diag-config-3f64de162dc937ee"
V3 = "diag-config-5a4a13627e14b4cf"


def read_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"expected JSON object: {path}")
    return value


def smoke_summary(path: Path, expected_configuration_id: str) -> tuple[dict[str, Any], list[str]]:
    raw = read_json(path)
    smoke = raw.get("smoke") or {}
    rollout = smoke.get("rollout") or {}
    reasons: list[str] = []
    if raw.get("failure_reason") is not None:
        reasons.append(f"{path.name}: smoke failure")
    if smoke.get("status") != "completed":
        reasons.append(f"{path.name}: status_not_completed")
    if smoke.get("governance_verdict") != "accepted":
        reasons.append(f"{path.name}: governance_not_accepted")
    if smoke.get("agent_configuration_id") != expected_configuration_id:
        reasons.append(f"{path.name}: configuration_mismatch")
    if rollout.get("stage") != "champion":
        reasons.append(f"{path.name}: stage_not_champion")
    if rollout.get("served_configuration_id") != expected_configuration_id:
        reasons.append(f"{path.name}: served_configuration_mismatch")
    if rollout.get("champion_configuration_id") != expected_configuration_id:
        reasons.append(f"{path.name}: champion_configuration_mismatch")
    return {
        "status": smoke.get("status"),
        "governance_verdict": smoke.get("governance_verdict"),
        "candidate_count": smoke.get("candidate_count"),
        "agent_configuration_id": smoke.get("agent_configuration_id"),
        "rollout": {
            "stage": rollout.get("stage"),
            "served_configuration_id": rollout.get("served_configuration_id"),
            "champion_configuration_id": rollout.get("champion_configuration_id"),
            "challenger_configuration_id": rollout.get("challenger_configuration_id"),
            "promotion_record": rollout.get("promotion_record"),
        },
    }, reasons


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--identity-report", type=Path, default=DEFAULT_IDENTITY)
    parser.add_argument("--historical-replay-report", type=Path, required=True)
    parser.add_argument("--rollback-verification-report", type=Path, required=True)
    parser.add_argument("--restore-verification-report", type=Path, required=True)
    parser.add_argument("--smoke-v10-before", type=Path, required=True)
    parser.add_argument("--smoke-v3", type=Path, required=True)
    parser.add_argument("--smoke-v10-after", type=Path, required=True)
    parser.add_argument("--runtime-revision", required=True)
    parser.add_argument("--final-champion", default=V10)
    parser.add_argument("--json-output", type=Path, default=DEFAULT_OUTPUT)
    args = parser.parse_args()

    reasons: list[str] = []
    identity = read_json(args.identity_report)
    replay = read_json(args.historical_replay_report)
    rollback_verify = read_json(args.rollback_verification_report)
    restore_verify = read_json(args.restore_verification_report)

    if identity.get("accepted") is not True:
        reasons.append("historical identity audit is not green")
    if replay.get("accepted") is not True or int(replay.get("failed") or 0) != 0:
        reasons.append("historical database replay audit is not green")
    if rollback_verify.get("unchanged") is not True:
        reasons.append("history changed during v3 rollback")
    if restore_verify.get("unchanged") is not True:
        reasons.append("history changed during v10 restore")
    if rollback_verify.get("before_root_sha256") != rollback_verify.get("protected_root_sha256"):
        reasons.append("protected root changed during v3 rollback")
    if restore_verify.get("before_root_sha256") != restore_verify.get("protected_root_sha256"):
        reasons.append("protected root changed during v10 restore")
    if args.final_champion != V10:
        reasons.append("final Champion is not immutable Diagnosis v10")

    smoke_before, smoke_before_reasons = smoke_summary(args.smoke_v10_before, V10)
    smoke_v3, smoke_v3_reasons = smoke_summary(args.smoke_v3, V3)
    smoke_after, smoke_after_reasons = smoke_summary(args.smoke_v10_after, V10)
    reasons.extend(smoke_before_reasons)
    reasons.extend(smoke_v3_reasons)
    reasons.extend(smoke_after_reasons)

    report = {
        "name": "diagnosis-dgs-safe-090-operational-audit-v1",
        "accepted": not reasons,
        "reasons": reasons,
        "runtime_revision": args.runtime_revision,
        "final_champion_configuration_id": args.final_champion,
        "historical_identity": {
            "accepted": identity.get("accepted"),
            "passed": identity.get("passed"),
            "total": identity.get("total"),
        },
        "historical_replay": {
            "accepted": replay.get("accepted"),
            "total_samples": replay.get("total_samples"),
            "passed": replay.get("passed"),
            "failed": replay.get("failed"),
            "configurations": replay.get("configurations"),
        },
        "immutability": {
            "rollback": rollback_verify,
            "restore": restore_verify,
        },
        "public_diagnosis_smoke": {
            "initial_v10": smoke_before,
            "rollback_v3": smoke_v3,
            "restored_v10": smoke_after,
        },
    }
    args.json_output.parent.mkdir(parents=True, exist_ok=True)
    args.json_output.write_text(
        json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(
        f"DGS-SAFE-090 operational audit: {'ACCEPTED' if report['accepted'] else 'BLOCKED'}"
    )
    if reasons:
        for reason in reasons:
            print(f"- {reason}")
    print(f"Wrote JSON report to {args.json_output}")
    return 0 if report["accepted"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
