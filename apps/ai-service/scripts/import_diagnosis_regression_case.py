#!/usr/bin/env python3
"""Append a reviewed historical Diagnosis replay export to qualification YAML."""

from __future__ import annotations

import argparse
from pathlib import Path

from src.evals.diagnosis_qualification import DEFAULT_DATASET_PATH
from src.evals.diagnosis_regression_import import append_regression_export


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--input",
        required=True,
        type=Path,
        help="JSON exported by regression-export",
    )
    parser.add_argument("--dataset", type=Path, default=DEFAULT_DATASET_PATH)
    args = parser.parse_args()

    updated = append_regression_export(args.input, args.dataset)
    print(f"REGRESSION_CASE_IMPORTED=PASS cases={len(updated.cases)} dataset={args.dataset}")


if __name__ == "__main__":
    main()
