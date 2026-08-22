"""Grounding Eval v2 required dataset slices and deterministic invariants."""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from src.evals.treatment_grounding_v2 import evaluate_grounding_v2_dataset
from src.services.grounding_evaluator_v2 import GroundingEvaluatorV2

DATASET = Path(__file__).resolve().parents[1] / "fixtures" / "treatment_grounding_v2_cases.json"


@pytest.mark.parametrize(
    "case", json.loads(DATASET.read_text(encoding="utf-8")), ids=lambda case: case["id"]
)
def test_required_grounding_v2_dataset_slice(case: dict) -> None:
    result = GroundingEvaluatorV2().evaluate(case["treatment"], case["evidence"])
    assert result.claims
    claim = result.claims[0]
    assert claim.support == case["expected_support"]
    assert case["expected_reason"] in claim.reasons


def test_missing_cited_evidence_id_fails_before_semantic_judge() -> None:
    judge_called = False

    def judge(*_args):
        nonlocal judge_called
        judge_called = True
        return "supported"

    treatment = {
        "goal": "test",
        "evidence_ids": ["missing"],
        "interventions": [{"kind": "exercise", "title": "臀桥", "prescription": {}}],
    }
    result = GroundingEvaluatorV2(judge=judge).evaluate(treatment, [])
    assert result.claims[0].support == "unsupported"
    assert result.claims[0].reasons == ("cited_evidence_id_not_retrieved",)
    assert judge_called is False


def test_inadmissible_evidence_cannot_be_overridden_by_judge() -> None:
    judge_called = False

    def judge(*_args):
        nonlocal judge_called
        judge_called = True
        return "supported"

    treatment = {
        "goal": "test",
        "evidence_ids": ["E1"],
        "interventions": [{"kind": "exercise", "title": "臀桥", "prescription": {}}],
    }
    evidence = [
        {"evidence_id": "E1", "title": "臀桥", "body_markdown": "臀桥", "admissible": False}
    ]
    result = GroundingEvaluatorV2(judge=judge).evaluate(treatment, evidence)
    assert result.claims[0].reasons == ("cited_evidence_not_admissible",)
    assert judge_called is False


def test_optional_judge_only_resolves_uncertain_semantic_case() -> None:
    calls = 0

    def judge(_claim, _evidence):
        nonlocal calls
        calls += 1
        return "supported"

    treatment = {
        "goal": "test",
        "evidence_ids": ["E1"],
        "interventions": [{"kind": "exercise", "title": "颈椎回缩训练", "prescription": {}}],
    }
    evidence = [
        {
            "evidence_id": "E1",
            "title": "颈椎回收训练说明",
            "body_markdown": "颈椎回收训练用于姿势练习。",
        }
    ]
    result = GroundingEvaluatorV2(judge=judge).evaluate(treatment, evidence)
    assert calls == 1
    assert result.claims[0].support == "supported"
    assert result.claims[0].judge_used is True


def test_v2_report_records_v1_disagreement_without_changing_production_gate() -> None:
    report = evaluate_grounding_v2_dataset(DATASET)
    assert report["case_count"] == 10
    assert report["disagreement_count"] >= 3
    assert report["production_gate_changed"] is False
    by_id = {case["id"]: case for case in report["cases"]}
    assert by_id["dose-unsupported"]["v1_faithful"] is True
    assert by_id["dose-unsupported"]["v2_support"] == "partial"
    assert by_id["contraindicated"]["v1_faithful"] is True
    assert by_id["contraindicated"]["v2_support"] == "contraindicated"
    assert by_id["misleading-lexical-overlap"]["v1_faithful"] is True
    assert by_id["misleading-lexical-overlap"]["v2_support"] == "unsupported"
