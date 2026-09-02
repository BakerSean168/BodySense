"""Combine synthetic, real-layout, and supply-chain evidence for Champion readiness."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any


def evaluate(
    synthetic: dict[str, Any],
    real_layout: dict[str, Any],
    supply_chain: dict[str, Any],
) -> dict[str, Any]:
    blockers: list[str] = []
    configuration_ids = {
        synthetic.get("configuration_id"),
        real_layout.get("configuration_id"),
        supply_chain.get("configuration_id"),
    }
    configuration_ids.discard(None)
    if len(configuration_ids) != 1:
        blockers.append("configuration_identity_mismatch")
    configuration_id = next(iter(configuration_ids), None)

    if not synthetic.get("mechanics_qualified", False):
        blockers.append("synthetic_mechanics_not_qualified")
    if not real_layout.get("mechanics_qualified", False):
        blockers.append("real_layout_mechanics_not_qualified")
    if not real_layout.get("champion_selection_ready", False):
        blockers.append("real_layout_selection_not_ready")
    if not supply_chain.get("machine_evidence_complete", False):
        blockers.append("supply_chain_machine_evidence_incomplete")
    if not supply_chain.get("production_promotion_ready", False):
        blockers.append("supply_chain_production_review_not_approved")

    return {
        "schema_version": "health-document-promotion-readiness-v1",
        "configuration_id": configuration_id,
        "ready_for_champion_selection": not blockers,
        "blockers": blockers,
        "synthetic_corpus_manifest_sha256": synthetic.get("corpus_manifest_sha256"),
        "real_layout_corpus_manifest_sha256": real_layout.get("corpus_manifest_sha256"),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--synthetic-qualification", type=Path, required=True)
    parser.add_argument("--real-layout-qualification", type=Path, required=True)
    parser.add_argument("--supply-chain-evidence", type=Path, required=True)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--fail-if-blocked", action="store_true")
    args = parser.parse_args()
    result = evaluate(
        json.loads(args.synthetic_qualification.read_text(encoding="utf-8")),
        json.loads(args.real_layout_qualification.read_text(encoding="utf-8")),
        json.loads(args.supply_chain_evidence.read_text(encoding="utf-8")),
    )
    rendered = json.dumps(result, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered, encoding="utf-8")
    print(rendered, end="")
    return 1 if args.fail_if_blocked and result["blockers"] else 0


if __name__ == "__main__":
    raise SystemExit(main())
