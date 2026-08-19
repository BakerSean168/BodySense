"""CLI wrapper for the Diagnosis Pydantic Evals baseline."""


def _main() -> int:
    import argparse
    import sys
    from pathlib import Path

    service_root = Path(__file__).resolve().parents[1]
    if str(service_root) not in sys.path:
        sys.path.insert(0, str(service_root))

    from src.evals.diagnosis_baseline import (
        DEFAULT_DATASET_PATH,
        render_summary,
        report_summary,
        run_diagnosis_baseline,
        summary_json,
    )

    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dataset", type=Path, default=DEFAULT_DATASET_PATH)
    parser.add_argument("--json-output", type=Path)
    parser.add_argument("--stdout-only", action="store_true")
    args = parser.parse_args()

    report = run_diagnosis_baseline(args.dataset)
    print(render_summary(report), end="")
    summary = report_summary(report)

    if args.json_output and not args.stdout_only:
        args.json_output.parent.mkdir(parents=True, exist_ok=True)
        args.json_output.write_text(summary_json(report), encoding="utf-8")
        print(f"Wrote JSON report to {args.json_output}")

    return 1 if summary["failed"] else 0


if __name__ == "__main__":
    raise SystemExit(_main())
