"""Check the Treatment EvidenceGap v2 configuration and its frozen v1 -> v2 promotion.

The v1 Champion is archived and cannot run on the native Diagnosis input, so the
paired promotion is verified from its frozen reports. The v2 configuration must
still qualify on the current dataset and pass the EvidenceGap policy.
"""


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

    reports = service_root / "data" / "evals" / "reports"
    challenger = load_manifest(CONFIG_ROOT / "treatment-v2-evidence-gap.yaml")
    current_summary = report_summary(
        run_treatment_qualification(configuration_id=challenger.configuration_id)
    )
    champion_summary = json.loads(DEFAULT_REPORT_PATH.read_text(encoding="utf-8"))
    frozen_challenger = json.loads(
        (reports / "treatment_evidence_gap_challenger.json").read_text(encoding="utf-8")
    )
    comparison = compare_treatment_qualification_summaries(champion_summary, frozen_challenger)
    evidence_summary = treatment_evidence_policy_summary(
        run_treatment_evidence_policy_qualification()
    )
    ready = bool(
        evidence_summary["passed"] == evidence_summary["total"]
        and evidence_summary["total"] > 0
        and current_summary["qualification"]["qualified"]
        and frozen_challenger["configuration_id"] == challenger.configuration_id
        and comparison["promotion_eligible"]
    )

    print("# Treatment EvidenceGap Configuration Check")
    print(f"- Champion (frozen): `{champion_summary['configuration_id']}`")
    print(f"- Challenger: `{challenger.configuration_id}`")
    print(
        f"- Current qualification: {current_summary['passed']}/{current_summary['total']}"
        f" on `{current_summary['dataset']['fingerprint']}`"
    )
    print(f"- EvidenceGap policy: {evidence_summary['passed']}/{evidence_summary['total']}")
    print(f"- Frozen promotion regressions: {len(comparison['regressions'])}")
    print(f"- Frozen promotion non-inferior: {'YES' if comparison['non_inferior'] else 'NO'}")
    print(f"- Ready: {'YES' if ready else 'NO'}")

    if not args.stdout_only:
        # The paired challenger/readiness reports are frozen promotion evidence.
        (reports / "treatment_evidence_policy_v2.json").write_text(
            json.dumps(evidence_summary, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
    return 0 if ready else 1


if __name__ == "__main__":
    raise SystemExit(_main())
