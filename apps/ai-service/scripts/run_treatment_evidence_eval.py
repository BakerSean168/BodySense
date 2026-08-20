"""Qualify the Treatment EvidenceGap v2 Challenger against the v1 Champion."""


def _main() -> int:
    import argparse
    import json
    import sys
    from pathlib import Path

    service_root = Path(__file__).resolve().parents[1]
    if str(service_root) not in sys.path:
        sys.path.insert(0, str(service_root))

    from src.configuration.treatment_agent_config import CONFIG_ROOT, load_manifest
    from src.evals.treatment_evidence_policy import (
        run_treatment_evidence_policy_qualification,
        treatment_evidence_policy_summary,
    )
    from src.evals.treatment_qualification import (
        DEFAULT_REPORT_PATH,
        compare_treatment_qualification_summaries,
        report_summary,
        run_treatment_qualification,
    )

    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--stdout-only", action="store_true")
    args = parser.parse_args()

    challenger = load_manifest(CONFIG_ROOT / "treatment-v2-evidence-gap.yaml")
    challenger_run = run_treatment_qualification(configuration_id=challenger.configuration_id)
    challenger_summary = report_summary(challenger_run)
    champion_summary = json.loads(DEFAULT_REPORT_PATH.read_text(encoding="utf-8"))
    comparison = compare_treatment_qualification_summaries(
        champion_summary,
        challenger_summary,
    )
    evidence_report = run_treatment_evidence_policy_qualification()
    evidence_summary = treatment_evidence_policy_summary(evidence_report)
    ready = bool(
        evidence_summary["passed"] == evidence_summary["total"]
        and evidence_summary["total"] > 0
        and comparison["promotion_eligible"]
    )
    result = {
        "name": "treatment_evidence_gap_v2_readiness",
        "champion_configuration_id": champion_summary["configuration_id"],
        "challenger_configuration_id": challenger.configuration_id,
        "dataset_fingerprint": comparison["dataset_fingerprint"],
        "evidence_policy": evidence_summary,
        "qualification": challenger_summary,
        "comparison": comparison,
        "ready_for_later_rollout": ready,
    }

    print("# Treatment EvidenceGap Challenger Readiness")
    print(f"- Champion: `{champion_summary['configuration_id']}`")
    print(f"- Challenger: `{challenger.configuration_id}`")
    print(f"- General qualification: {challenger_summary['passed']}/{challenger_summary['total']}")
    print(f"- EvidenceGap policy: {evidence_summary['passed']}/{evidence_summary['total']}")
    print(f"- Deterministic regressions: {len(comparison['regressions'])}")
    print(f"- Non-inferior: {'YES' if comparison['non_inferior'] else 'NO'}")
    print(f"- Ready for later rollout work: {'YES' if ready else 'NO'}")

    if not args.stdout_only:
        reports = service_root / "data" / "evals" / "reports"
        reports.mkdir(parents=True, exist_ok=True)
        (reports / "treatment_evidence_gap_challenger.json").write_text(
            json.dumps(challenger_summary, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
        (reports / "treatment_evidence_policy_v2.json").write_text(
            json.dumps(evidence_summary, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
        (reports / "treatment_evidence_gap_readiness.json").write_text(
            json.dumps(result, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
    return 0 if ready else 1


if __name__ == "__main__":
    raise SystemExit(_main())
