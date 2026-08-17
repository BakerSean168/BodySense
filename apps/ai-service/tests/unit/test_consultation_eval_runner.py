"""Tests for the structured consultation eval runner."""

from src.evals.consultation_eval_runner import DEFAULT_CASES_PATH, run_consultation_evals


def test_structured_consultation_eval_cases_pass():
    report = run_consultation_evals(DEFAULT_CASES_PATH)

    assert report["overall"]["total_cases"] == 6
    assert report["overall"]["failed_cases"] == 0
    assert report["overall"]["passed_cases"] == 6
    assert [suite["suite"] for suite in report["suites"]] == ["red_flags", "faithfulness"]
