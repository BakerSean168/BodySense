"""Deterministic Assessment evidence-catalog/rendering contract tests."""

from src.services.assessment_evidence import (
    ASSESSMENT_EVIDENCE_POLICY_REVISION_V2,
    ASSESSMENT_EVIDENCE_POLICY_REVISION_V3,
    ASSESSMENT_EVIDENCE_POLICY_REVISION_V4,
    assessment_evidence_issues,
    build_assessment_evidence_catalog,
    build_assessment_evidence_coverage,
    render_assessment_observations,
)


def _catalog_fixture():
    fact_id = "11111111-1111-1111-1111-111111111111"
    report_upload_id = "22222222-2222-2222-2222-222222222222"
    posture_upload_id = "33333333-3333-3333-3333-333333333333"
    catalog = build_assessment_evidence_catalog(
        profile={"gender": "male", "age_years": 22},
        body_state={
            "facts": [
                {
                    "id": fact_id,
                    "kind": "lifestyle.exercise",
                    "value": "健身；频率：1-2",
                    "review_state": "confirmed",
                    "lifecycle_state": "active",
                }
            ],
            "observations": [
                {
                    "kind": "anthropometry.height",
                    "value": {"value": 172, "unit": "cm"},
                    "review_state": "confirmed",
                    "lifecycle_state": "active",
                }
            ],
        },
        report_indicators=[
            {
                "upload_id": report_upload_id,
                "indicator_index": 0,
                "value": {
                    "name": "25-OH-D",
                    "value": "12.5",
                    "unit": "ng/mL",
                    "evidence_admissibility": {
                        "status": "admissible",
                        "policy_revision": "ocr-indicator-admissibility-v1",
                        "reason_codes": ["high_confidence_ocr_and_indicator"],
                    },
                },
            }
        ],
        posture_analysis={
            "has_analysis": True,
            "views": [
                {
                    "upload_id": posture_upload_id,
                    "view": "front",
                    "analysis": {
                        "findings": [
                            {
                                "key": "uneven_shoulders",
                                "label": "肩部对称性待复核",
                                "evidence": "右侧肩峰位置略高",
                            }
                        ],
                        "summary_markdown": "正面肩部对称性值得复核。",
                    },
                }
            ],
            "summaries": ["正面肩部对称性值得复核。"],
        },
    )
    return catalog, fact_id, report_upload_id, posture_upload_id


def test_catalog_uses_durable_refs_excludes_profile_and_separates_coverage_domains() -> None:
    catalog, fact_id, report_upload_id, posture_upload_id = _catalog_fixture()

    assert not any(ref.startswith("profile:") for ref in catalog)
    assert f"body_state:fact:{fact_id}" in catalog
    assert f"report:upload:{report_upload_id}:indicator:0" in catalog
    assert f"posture:upload:{posture_upload_id}:finding:0" in catalog
    assert "posture:summary:0" not in catalog

    domains = build_assessment_evidence_coverage(catalog)["domains"]
    assert domains["posture"]["status"] == "available"
    assert domains["exercise"]["status"] == "available"
    assert domains["anthropometry"]["status"] == "available"
    assert domains["health_report"]["status"] == "available"
    # A lab report is not automatically injury/symptom evidence.
    assert domains["injury_symptoms"]["status"] == "missing"


def test_catalog_excludes_unverified_or_reasoning_excluded_body_state() -> None:
    catalog = build_assessment_evidence_catalog(
        profile={},
        body_state={
            "facts": [
                {
                    "id": "44444444-4444-4444-4444-444444444444",
                    "kind": "lifestyle.exercise",
                    "value": "健身；频率：1-2",
                    "review_state": "unverified",
                    "lifecycle_state": "active",
                    "excluded_from_reasoning": True,
                }
            ]
        },
        report_indicators=[],
        posture_analysis={},
    )
    assert catalog == {}
    assert build_assessment_evidence_coverage(catalog)["domains"]["exercise"]["status"] == "missing"


def test_evidence_gate_requires_one_unique_compatible_ref() -> None:
    catalog, fact_id, _, _ = _catalog_fixture()
    ref = f"body_state:fact:{fact_id}"

    assert (
        assessment_evidence_issues(
            {"observations": [{"kind": "exercise_pattern", "evidence_refs": [ref]}]}, catalog
        )
        == []
    )

    duplicate = assessment_evidence_issues(
        {
            "observations": [
                {"kind": "exercise_pattern", "evidence_refs": [ref]},
                {"kind": "exercise_pattern", "evidence_refs": [ref]},
            ]
        },
        catalog,
    )
    assert any("more than once" in issue.message for issue in duplicate)

    wrong_source = assessment_evidence_issues(
        {"observations": [{"kind": "posture_alignment", "evidence_refs": [ref]}]}, catalog
    )
    assert any("cannot use body_state evidence" in issue.message for issue in wrong_source)


def test_renderer_uses_source_snapshot_not_model_prose() -> None:
    catalog, fact_id, report_upload_id, posture_upload_id = _catalog_fixture()
    selections = [
        {"kind": "exercise_pattern", "evidence_refs": [f"body_state:fact:{fact_id}"]},
        {
            "kind": "posture_asymmetry",
            "evidence_refs": [f"posture:upload:{posture_upload_id}:finding:0"],
        },
        {
            "kind": "report_indicator",
            "evidence_refs": [f"report:upload:{report_upload_id}:indicator:0"],
        },
    ]
    rendered = render_assessment_observations(selections, catalog)

    assert rendered[0]["description"] == "来源记录：健身；频率：1-2。"
    assert rendered[1]["description"] == "体态分析记录：右侧肩峰位置略高。"
    assert rendered[2]["description"] == "报告记录：25-OH-D=12.5 ng/mL。"


def test_report_indicator_v3_requires_explicit_admissibility_provenance() -> None:
    report_upload_id = "55555555-5555-5555-5555-555555555555"
    base = {
        "upload_id": report_upload_id,
        "indicator_index": 0,
        "value": {"name": "Vitamin D", "value": "25.3", "unit": "ng/mL"},
    }

    legacy = build_assessment_evidence_catalog(
        profile={},
        body_state={},
        report_indicators=[base],
        posture_analysis={},
        evidence_policy_revision=ASSESSMENT_EVIDENCE_POLICY_REVISION_V2,
    )
    ref = f"report:upload:{report_upload_id}:indicator:0"
    assert ref in legacy

    current = build_assessment_evidence_catalog(
        profile={},
        body_state={},
        report_indicators=[base],
        posture_analysis={},
        evidence_policy_revision=ASSESSMENT_EVIDENCE_POLICY_REVISION_V3,
    )
    assert ref not in current


def test_report_indicator_v3_excludes_review_required_and_accepts_admissible() -> None:
    report_upload_id = "66666666-6666-6666-6666-666666666666"
    ref0 = f"report:upload:{report_upload_id}:indicator:0"
    ref1 = f"report:upload:{report_upload_id}:indicator:1"
    catalog = build_assessment_evidence_catalog(
        profile={},
        body_state={},
        report_indicators=[
            {
                "upload_id": report_upload_id,
                "indicator_index": 0,
                "value": {
                    "name": "Vitamin D",
                    "value": "25.3",
                    "unit": "ng/mL",
                    "evidence_admissibility": {
                        "status": "needs_review",
                        "policy_revision": "ocr-indicator-admissibility-v1",
                        "reason_codes": ["indicator_confidence_medium"],
                    },
                },
            },
            {
                "upload_id": report_upload_id,
                "indicator_index": 1,
                "value": {
                    "name": "Ferritin",
                    "value": "50",
                    "unit": "ng/mL",
                    "evidence_admissibility": {
                        "status": "admissible",
                        "policy_revision": "ocr-indicator-admissibility-v1",
                        "reason_codes": ["high_confidence_ocr_and_indicator"],
                    },
                },
            },
        ],
        posture_analysis={},
        evidence_policy_revision=ASSESSMENT_EVIDENCE_POLICY_REVISION_V3,
    )
    assert ref0 not in catalog
    assert ref1 in catalog
    coverage = build_assessment_evidence_coverage(catalog)
    assert coverage["domains"]["health_report"]["status"] == "available"


def _reviewed_entry(
    *, action: str = "confirm", index: int = 0, omit_extraction_run: bool = False, value=None
) -> dict:
    entry = {
        "upload_id": "22222222-2222-2222-2222-222222222222",
        "indicator_index": index,
        "indicator_id": "ferritin" if index else "25-oh-d",
        "review_id": "44444444-4444-4444-4444-444444444444",
        "reviewer_user_id": "77777777-7777-7777-7777-777777777777",
        "action": action,
        "reviewed": True,
        "source_refs": ["src:a"],
        "page_ref": {"src:a": {"page_number": 1, "bbox": [0, 0, 1, 1]}},
        "value": value
        or {
            "indicator_id": "ferritin" if index else "25-oh-d",
            "name": "Ferritin" if index else "25-OH-D",
            "value": "50.0" if index else "12.5",
            "unit": "ng/mL",
        },
    }
    if omit_extraction_run:
        return entry
    entry["extraction_run_id"] = "33333333-3333-3333-3333-333333333333"
    return entry


def test_v4_reviewed_lane_admits_confirmed_and_corrected_but_fails_closed() -> None:
    confirmed = _reviewed_entry(action="confirm", index=0)
    corrected = _reviewed_entry(
        action="correct",
        index=1,
        value={"indicator_id": "ferritin", "name": "Ferritin", "value": "50.0", "unit": "ng/mL"},
    )
    catalog = build_assessment_evidence_catalog(
        profile={},
        body_state={},
        report_indicators=[],
        reviewed_report_evidence=[confirmed, corrected],
        posture_analysis={},
        evidence_policy_revision=ASSESSMENT_EVIDENCE_POLICY_REVISION_V4,
    )
    assert "report:upload:22222222-2222-2222-2222-222222222222:indicator:0" in catalog
    assert "report:upload:22222222-2222-2222-2222-222222222222:indicator:1" in catalog
    corrected_value = catalog[
        "report:upload:22222222-2222-2222-2222-222222222222:indicator:1"
    ].value
    assert corrected_value["reviewed"] is True
    provenance = corrected_value["review_provenance"]
    assert provenance["action"] == "correct"
    assert provenance["review_id"] == corrected["review_id"]
    assert provenance["upload_id"] == corrected["upload_id"]
    assert provenance["extraction_run_id"] == corrected["extraction_run_id"]
    assert provenance["source_refs"] == ["src:a"]
    assert provenance["page_ref"]["src:a"]["page_number"] == 1

    # Missing provenance fails closed: the reviewed lane never manufactures
    # authority without exact upload/extraction-run/review provenance.
    incomplete = build_assessment_evidence_catalog(
        profile={},
        body_state={},
        report_indicators=[],
        reviewed_report_evidence=[_reviewed_entry(omit_extraction_run=True)],
        posture_analysis={},
        evidence_policy_revision=ASSESSMENT_EVIDENCE_POLICY_REVISION_V4,
    )
    assert incomplete == {}

    missing_source = _reviewed_entry(action="confirm", index=0)
    missing_source["source_refs"] = []
    assert (
        build_assessment_evidence_catalog(
            profile={},
            body_state={},
            report_indicators=[],
            reviewed_report_evidence=[missing_source],
            posture_analysis={},
            evidence_policy_revision=ASSESSMENT_EVIDENCE_POLICY_REVISION_V4,
        )
        == {}
    )

    mismatched_candidate = _reviewed_entry(action="correct", index=1)
    mismatched_candidate["value"] = dict(mismatched_candidate["value"])
    mismatched_candidate["value"]["indicator_id"] = "another-indicator"
    assert (
        build_assessment_evidence_catalog(
            profile={},
            body_state={},
            report_indicators=[],
            reviewed_report_evidence=[mismatched_candidate],
            posture_analysis={},
            evidence_policy_revision=ASSESSMENT_EVIDENCE_POLICY_REVISION_V4,
        )
        == {}
    )

    # A rejected review action is never admitted even with provenance.
    rejected = build_assessment_evidence_catalog(
        profile={},
        body_state={},
        report_indicators=[],
        reviewed_report_evidence=[_reviewed_entry(action="reject", index=0)],
        posture_analysis={},
        evidence_policy_revision=ASSESSMENT_EVIDENCE_POLICY_REVISION_V4,
    )
    assert rejected == {}


def test_v4_machine_admissible_lane_keeps_working_and_replay_is_preserved() -> None:
    report_upload_id = "22222222-2222-2222-2222-222222222222"
    admissible = {
        "upload_id": report_upload_id,
        "indicator_index": 0,
        "value": {
            "name": "Vitamin D",
            "value": "25.3",
            "unit": "ng/mL",
            "evidence_admissibility": {
                "status": "admissible",
                "policy_revision": "ocr-indicator-admissibility-v1",
                "reason_codes": ["high_confidence_ocr_and_indicator"],
            },
        },
    }
    catalog = build_assessment_evidence_catalog(
        profile={},
        body_state={},
        report_indicators=[admissible],
        posture_analysis={},
        evidence_policy_revision=ASSESSMENT_EVIDENCE_POLICY_REVISION_V4,
    )
    assert f"report:upload:{report_upload_id}:indicator:0" in catalog

    # The legacy evidence-contract-v3 catalog ignores the reviewed lane entirely,
    # so historical replay keeps its original machine-only semantics.
    replay = build_assessment_evidence_catalog(
        profile={},
        body_state={},
        report_indicators=[admissible],
        reviewed_report_evidence=[_reviewed_entry(action="confirm", index=0)],
        posture_analysis={},
        evidence_policy_revision=ASSESSMENT_EVIDENCE_POLICY_REVISION_V3,
    )
    assert set(replay) == {f"report:upload:{report_upload_id}:indicator:0"}
