#!/usr/bin/env python3
"""Validate the repository-versioned Diagnosis promotion evidence chain."""

from __future__ import annotations


def _main() -> int:
    import json
    import sys
    from pathlib import Path

    service_root = Path(__file__).resolve().parents[1]
    if str(service_root) not in sys.path:
        sys.path.insert(0, str(service_root))

    from src.evals.diagnosis_promotion import (
        DEFAULT_POLICY_PATH,
        DEFAULT_REPORT_PATH,
        evaluate_promotion_readiness,
        load_promotion_policy,
    )

    policy = load_promotion_policy(DEFAULT_POLICY_PATH)
    report = evaluate_promotion_readiness(policy)
    DEFAULT_REPORT_PATH.write_text(
        json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print("# Diagnosis Promotion Readiness")
    print(f"- Champion: `{report['champion_configuration_id']}`")
    print(f"- Challenger: `{report['challenger_configuration_id']}`")
    print(f"- Dataset: `{report['dataset_fingerprint']}`")
    print(f"- Ready for shadow: {'YES' if report['ready_for_shadow'] else 'NO'}")
    if report["reasons"]:
        for reason in report["reasons"]:
            print(f"- BLOCK: {reason}")
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(_main())
