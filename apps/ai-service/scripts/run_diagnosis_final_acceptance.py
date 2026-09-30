"""Evaluate the standalone Diagnosis v10 final-acceptance contract."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from src.evals.diagnosis_final_acceptance import (  # noqa: E402
    DEFAULT_POLICY_PATH,
    DEFAULT_REPORT_PATH,
    evaluate_final_acceptance,
    load_policy,
    render_report,
)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--policy", type=Path, default=DEFAULT_POLICY_PATH)
    parser.add_argument("--json-output", type=Path, default=DEFAULT_REPORT_PATH)
    args = parser.parse_args()

    policy = load_policy(args.policy)
    report = evaluate_final_acceptance(policy)
    print(render_report(report), end="")
    args.json_output.parent.mkdir(parents=True, exist_ok=True)
    args.json_output.write_text(
        json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(f"Wrote JSON report to {args.json_output}")
    return 0 if report["accepted"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
