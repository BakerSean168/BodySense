"""Run deterministic qualification for the current Assessment evidence contract."""


def _main() -> int:
    import json
    import sys
    from pathlib import Path

    service_root = Path(__file__).resolve().parents[1]
    if str(service_root) not in sys.path:
        sys.path.insert(0, str(service_root))

    from src.evals.assessment_evidence_policy import (
        assessment_evidence_policy_summary,
        run_assessment_evidence_policy_qualification,
    )

    summary = assessment_evidence_policy_summary(run_assessment_evidence_policy_qualification())
    print("# Assessment Evidence Contract Qualification")
    print()
    print(f"- Result: {summary['passed']}/{summary['total']} passed")
    for case in summary["cases"]:
        mark = "PASS" if case["passed"] else "FAIL"
        print(f"- `{mark}` `{case['name']}`")

    reports = service_root / "data" / "evals" / "reports"
    reports.mkdir(parents=True, exist_ok=True)
    (reports / "assessment_evidence_contract_v4.json").write_text(
        json.dumps(summary, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    return 1 if summary["failed"] else 0


if __name__ == "__main__":
    raise SystemExit(_main())
