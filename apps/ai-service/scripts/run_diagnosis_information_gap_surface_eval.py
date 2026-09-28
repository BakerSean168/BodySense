#!/usr/bin/env python3
"""Run and write the immutable v7 information-gap claim-surface qualification."""

from __future__ import annotations


def _main() -> int:
    import argparse
    import json
    import sys
    from pathlib import Path

    service_root = Path(__file__).resolve().parents[1]
    if str(service_root) not in sys.path:
        sys.path.insert(0, str(service_root))

    from src.evals.diagnosis_information_gap_surface import information_gap_surface_summary

    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--json-output", type=Path, required=True)
    args = parser.parse_args()
    summary = information_gap_surface_summary()
    args.json_output.parent.mkdir(parents=True, exist_ok=True)
    args.json_output.write_text(
        json.dumps(summary, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(
        f"# Diagnosis Information-Gap Surface Qualification: "
        f"{summary['passed']}/{summary['total']} passed"
    )
    return 1 if summary["failed"] else 0


if __name__ == "__main__":
    raise SystemExit(_main())
