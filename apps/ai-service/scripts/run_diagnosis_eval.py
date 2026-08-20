"""CLI wrapper for Diagnosis Agent configuration qualification."""


def _main() -> int:
    import argparse
    import json
    import sys
    from pathlib import Path

    service_root = Path(__file__).resolve().parents[1]
    if str(service_root) not in sys.path:
        sys.path.insert(0, str(service_root))

    from src.evals.diagnosis_qualification import (
        DEFAULT_DATASET_PATH,
        compare_qualification_summaries,
        render_summary,
        report_summary,
        run_diagnosis_qualification,
        summary_json,
    )

    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dataset", type=Path, default=DEFAULT_DATASET_PATH)
    parser.add_argument("--configuration-id")
    parser.add_argument("--json-output", type=Path)
    parser.add_argument("--compare-to", type=Path)
    parser.add_argument("--stdout-only", action="store_true")
    args = parser.parse_args()

    run = run_diagnosis_qualification(args.dataset, configuration_id=args.configuration_id)
    summary = report_summary(run)
    comparison = None
    if args.compare_to is not None:
        champion = json.loads(args.compare_to.read_text(encoding="utf-8"))
        comparison = compare_qualification_summaries(champion, summary)

    print(render_summary(run, comparison=comparison), end="")
    if args.json_output and not args.stdout_only:
        args.json_output.parent.mkdir(parents=True, exist_ok=True)
        args.json_output.write_text(summary_json(run, comparison=comparison), encoding="utf-8")
        print(f"Wrote JSON report to {args.json_output}")

    qualified = bool(summary["qualification"]["qualified"])
    non_inferior = comparison is None or bool(comparison["non_inferior"])
    return 0 if qualified and non_inferior else 1


if __name__ == "__main__":
    raise SystemExit(_main())
