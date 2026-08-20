"""Run deterministic qualification for the Diagnosis EvidenceGap policy runtime."""


def _main() -> int:
    import sys
    from pathlib import Path

    service_root = Path(__file__).resolve().parents[1]
    if str(service_root) not in sys.path:
        sys.path.insert(0, str(service_root))

    from src.evals.diagnosis_evidence_policy import (
        evidence_policy_summary,
        run_evidence_policy_qualification,
    )

    summary = evidence_policy_summary(run_evidence_policy_qualification())
    print("# Diagnosis EvidenceGap Policy Qualification")
    print()
    print(f"- Result: {summary['passed']}/{summary['total']} passed")
    for case in summary["cases"]:
        mark = "PASS" if case["passed"] else "FAIL"
        print(f"- `{mark}` `{case['name']}`")
    return 1 if summary["failed"] else 0


if __name__ == "__main__":
    raise SystemExit(_main())
