"""Run the DGS-SAFE-090 historical Diagnosis identity audit."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from src.evals.diagnosis_historical_identity_audit import (  # noqa: E402
    DEFAULT_POLICY_PATH,
    DEFAULT_REPORT_PATH,
    evaluate_historical_identity,
    load_policy,
    render_report,
    write_report,
)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--policy", type=Path, default=DEFAULT_POLICY_PATH)
    parser.add_argument("--json-output", type=Path, default=DEFAULT_REPORT_PATH)
    args = parser.parse_args()

    report = evaluate_historical_identity(load_policy(args.policy))
    print(render_report(report), end="")
    write_report(report, args.json_output)
    print(f"Wrote JSON report to {args.json_output}")
    return 0 if report["accepted"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
