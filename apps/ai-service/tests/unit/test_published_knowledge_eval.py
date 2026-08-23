from __future__ import annotations

import importlib.util
import sys
from pathlib import Path

from src.rag.knowledge_library import SearchResult

SCRIPT = Path(__file__).resolve().parents[2] / "scripts" / "run_published_knowledge_eval.py"
spec = importlib.util.spec_from_file_location("published_knowledge_eval", SCRIPT)
assert spec and spec.loader
module = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = module
spec.loader.exec_module(module)

PublishedEvalCase = module.PublishedEvalCase
evaluate_case = module.evaluate_case
load_cases = module.load_cases


def _published_result() -> SearchResult:
    return SearchResult(
        id=1,
        unit_key="tfu-pain",
        problem_slug="pain-science",
        category="pain-science",
        unit_type="reference",
        title="疼痛定义",
        summary="不愉快感觉与情绪体验",
        body_markdown="二者不是同一现象。",
        similarity=0.2,
        source_key="thought-forest:z/pain.md",
        source_type="thought_forest_note",
        source_title="Pain",
        source_author="Thought Forest",
        source_start_sec=0.0,
        source_end_sec=0.0,
        lifecycle_status="published",
        review_status="reviewed",
        quality_score=0.95,
        publication_id="pub-1",
        published_version=3,
        publication_key="pain-v3",
        publication_batch_key="pain-batch",
        unit_metadata={
            "source_locator": {
                "locator_type": "markdown_lines",
                "git_commit": "abc123",
                "path": "z/pain.md",
                "line_start": 20,
                "line_end": 23,
            },
            "claim_candidate": {"claim_id": "claim-pain", "claim_kind": "definition"},
            "claim_review": {"review_id": "review-pain", "decision": "approved"},
        },
    )


def test_positive_case_requires_exact_publication_citation_and_grounding() -> None:
    case = PublishedEvalCase(
        case_id="pain",
        query="什么是疼痛",
        expect="hit",
        expected_unit_key="tfu-pain",
        expected_claim_id="claim-pain",
        expected_evidence_terms=("不愉快感觉", "不是同一现象"),
    )
    result = evaluate_case(case, [_published_result()], expected_publication_key="pain-v3")
    assert result.passed is True
    assert result.citation_status == "valid"
    assert result.grounding_status == "supported"
    assert result.identity_status == "match"
    assert result.provenance_status == "valid"


def test_negative_case_fails_when_any_published_result_is_returned() -> None:
    case = PublishedEvalCase(case_id="negative", query="PostgreSQL", expect="no_result")
    result = evaluate_case(case, [_published_result()], expected_publication_key="pain-v3")
    assert result.passed is False
    assert result.retrieval_status == "unexpected_result"


def test_negative_case_passes_on_empty_retrieval() -> None:
    case = PublishedEvalCase(case_id="negative", query="PostgreSQL", expect="no_result")
    result = evaluate_case(case, [], expected_publication_key="pain-v3")
    assert result.passed is True
    assert result.retrieval_status == "expected_empty"


def test_committed_published_eval_dataset_is_well_formed() -> None:
    repo_root = Path(__file__).resolve().parents[4]
    cases = load_cases(repo_root / "docs/knowledges/eval/published-knowledge-pilot.jsonl")
    assert len(cases) == 6
    assert sum(case.expect == "hit" for case in cases) == 3
    assert sum(case.expect == "no_result" for case in cases) == 3
