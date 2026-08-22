"""Qualification diagnostic for Treatment Grounding Eval v2.

It intentionally compares v2 with the current production v1 checker without
changing production governance. The disagreement slice is a review artifact,
not an automatic promotion decision.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from src.services.faithfulness_checker import FaithfulnessChecker
from src.services.grounding_evaluator_v2 import GroundingEvaluatorV2

DEFAULT_DATASET = (
    Path(__file__).resolve().parents[2] / "tests" / "fixtures" / "treatment_grounding_v2_cases.json"
)


def evaluate_grounding_v2_dataset(path: Path = DEFAULT_DATASET) -> dict[str, Any]:
    cases = json.loads(path.read_text(encoding="utf-8"))
    v1 = FaithfulnessChecker()
    v2 = GroundingEvaluatorV2()
    results: list[dict[str, Any]] = []

    for case in cases:
        old = v1.check_treatment_faithfulness(case["treatment"], case["evidence"])
        new = v2.evaluate(case["treatment"], case["evidence"])
        primary_support = new.claims[0].support if new.claims else "supported"
        results.append(
            {
                "id": case["id"],
                "slice": case["slice"],
                "v1_faithful": old.faithful,
                "v2_support": primary_support,
                "v2_verdict": new.verdict,
                "v2_reasons": list(new.claims[0].reasons) if new.claims else [],
                "disagrees": old.faithful != (primary_support == "supported"),
            }
        )

    return {
        "evaluator_revision": "treatment-grounding-v2",
        "production_gate_changed": False,
        "case_count": len(results),
        "disagreement_count": sum(1 for item in results if item["disagrees"]),
        "cases": results,
    }
