from scripts.evaluate_health_document_candidate import evaluate


def _thresholds() -> dict:
    return {
        "schema_version": "health-document-benchmark-thresholds-v2",
        "source_accuracy": {
            "critical_numeric_error_rate_max": 0.005,
            "safety_critical_numeric_error_rate_max": 0.0,
            "name_recall_min": 0.99,
            "numeric_exact_match_min": 0.99,
            "unit_exact_match_min": 0.99,
            "row_bundle_exact_match_min": 0.975,
        },
        "resource": {
            "failed_fixture_count_max": 0,
            "cgroup_memory_events_max_max": 0,
            "cgroup_memory_events_oom_max": 0,
            "cgroup_memory_events_oom_kill_max": 0,
            "cgroup_swap_peak_mb_max": 0.0,
        },
    }


def _summary() -> dict:
    return {
        "candidate_id": "candidate",
        "configuration_id": "hdex-config-test",
        "corpus_manifest_sha256": "a" * 64,
        "harness_revision": "health-document-benchmark-v4",
        "execution_topology_revision": "per-document-subprocess-v1",
        "source_accuracy": {
            "critical_numeric_error_rate": 0.0,
            "name_recall": 1.0,
            "numeric_exact_match": 1.0,
            "unit_exact_match": 1.0,
            "reference_range_exact_match": 1.0,
            "row_bundle_exact_match": 1.0,
        },
        "fixture_execution": {"total": 40, "succeeded": 40, "failed": 0},
        "runtime": {
            "cgroup_memory_events_max": 0,
            "cgroup_memory_events_oom": 0,
            "cgroup_memory_events_oom_kill": 0,
            "cgroup_swap_peak_mb": 0.0,
        },
        "champion_selection_ready": False,
        "champion_selection_blocker": "deidentified_double_reviewed_real-layout_subset_missing",
    }


def test_mechanics_can_qualify_while_champion_selection_remains_blocked() -> None:
    result = evaluate(_summary(), _thresholds())
    assert result["mechanics_qualified"] is True
    assert result["mechanics_blocking_reasons"] == []
    assert result["champion_selection_ready"] is False
    assert result["champion_selection_blockers"] == [
        "deidentified_double_reviewed_real-layout_subset_missing"
    ]


def test_source_numeric_error_and_memory_pressure_fail_mechanics() -> None:
    summary = _summary()
    summary["source_accuracy"]["critical_numeric_error_rate"] = 0.01
    summary["runtime"]["cgroup_memory_events_max"] = 3
    summary["runtime"]["cgroup_swap_peak_mb"] = 8.0
    result = evaluate(summary, _thresholds())
    assert result["mechanics_qualified"] is False
    assert any(
        "source.critical_numeric_error_rate" in reason
        for reason in result["mechanics_blocking_reasons"]
    )
    assert any(
        "resource.memory_events_max" in reason for reason in result["mechanics_blocking_reasons"]
    )
    assert any("resource.swap_peak_mb" in reason for reason in result["mechanics_blocking_reasons"])


def test_any_failed_fixture_fails_mechanics_even_if_accuracy_is_perfect() -> None:
    summary = _summary()
    summary["fixture_execution"] = {"total": 40, "succeeded": 39, "failed": 1}
    result = evaluate(summary, _thresholds())
    assert result["mechanics_qualified"] is False
    assert any(
        "resource.failed_fixture_count" in reason for reason in result["mechanics_blocking_reasons"]
    )


def test_verified_authority_scope_blocks_false_admission_and_low_coverage() -> None:
    thresholds = {
        "schema_version": "health-document-benchmark-thresholds-v3",
        "selection_scope": "verified-evidence-authority",
        "evidence_authority": {
            "critical_false_admission_rate_max": 0.0,
            "auto_admission_exact_rate_min": 1.0,
            "auto_admission_coverage_min": 0.9,
        },
        "resource": {
            "failed_fixture_count_max": 0,
            "cgroup_memory_events_max_max": 0,
            "cgroup_memory_events_oom_max": 0,
            "cgroup_memory_events_oom_kill_max": 0,
            "cgroup_swap_peak_mb_max": 0.0,
        },
    }
    summary = _summary()
    summary["evidence_authority"] = {
        "critical_false_admission_rate": 0.01,
        "auto_admission_exact_rate": 0.99,
        "auto_admission_coverage": 0.89,
    }
    result = evaluate(summary, thresholds)
    assert result["mechanics_qualified"] is False
    assert any(
        "authority.critical_false_admission_rate" in reason
        for reason in result["mechanics_blocking_reasons"]
    )
    assert any(
        "authority.auto_admission_coverage" in reason
        for reason in result["mechanics_blocking_reasons"]
    )


def test_verified_authority_scope_can_pass_mechanics_while_real_layout_stays_blocked() -> None:
    thresholds = {
        "schema_version": "health-document-benchmark-thresholds-v3",
        "selection_scope": "verified-evidence-authority",
        "evidence_authority": {
            "critical_false_admission_rate_max": 0.0,
            "auto_admission_exact_rate_min": 1.0,
            "auto_admission_coverage_min": 0.9,
        },
        "resource": {
            "failed_fixture_count_max": 0,
            "cgroup_memory_events_max_max": 0,
            "cgroup_memory_events_oom_max": 0,
            "cgroup_memory_events_oom_kill_max": 0,
            "cgroup_swap_peak_mb_max": 0.0,
        },
    }
    summary = _summary()
    summary["evidence_authority"] = {
        "critical_false_admission_rate": 0.0,
        "auto_admission_exact_rate": 1.0,
        "auto_admission_coverage": 0.95,
    }
    result = evaluate(summary, thresholds)
    assert result["mechanics_qualified"] is True
    assert result["champion_selection_ready"] is False
    assert result["champion_selection_blockers"] == [
        "deidentified_double_reviewed_real-layout_subset_missing"
    ]
