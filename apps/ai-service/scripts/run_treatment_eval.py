"""CLI wrapper for Treatment Agent configuration qualification."""


def _main() -> int:
    import argparse
    import sys
    from pathlib import Path

    service_root = Path(__file__).resolve().parents[1]
    if str(service_root) not in sys.path:
        sys.path.insert(0, str(service_root))

    from src.evals.treatment_qualification import (
        DEFAULT_DATASET_PATH,
        DEFAULT_REPORT_PATH,
        render_summary,
        report_summary,
        run_treatment_qualification,
        summary_json,
    )

    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dataset", type=Path, default=DEFAULT_DATASET_PATH)
    parser.add_argument("--configuration-id")
    parser.add_argument("--json-output", type=Path, default=DEFAULT_REPORT_PATH)
    parser.add_argument("--stdout-only", action="store_true")
    args = parser.parse_args()

    run = run_treatment_qualification(args.dataset, configuration_id=args.configuration_id)
    summary = report_summary(run)
    print(render_summary(run), end="")
    if not args.stdout_only:
        args.json_output.parent.mkdir(parents=True, exist_ok=True)
        args.json_output.write_text(summary_json(run), encoding="utf-8")
        print(f"Wrote JSON report to {args.json_output}")
    return 0 if summary["qualification"]["qualified"] else 1


if __name__ == "__main__":
    raise SystemExit(_main())
