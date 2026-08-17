"""Unit tests for the forced governance seam on diagnosis/treatment."""

from __future__ import annotations

from src.runtime.governance import guard_structured_output


def test_guard_diagnosis_accepted():
    payload = {
        "candidates": [
            {
                "name": "圆肩倾向",
                "confidence": "中",
                "severity": "轻度",
                "basis": "久坐含胸",
                "typical_symptoms": "肩颈酸胀",
            }
        ]
    }
    guarded = guard_structured_output("diagnosis", payload)

    assert guarded.verdict == "accepted"
    assert guarded.payload is not None
    assert guarded.payload["candidates"][0]["name"] == "圆肩倾向"
    emitted = guarded.to_emit_dict()
    assert emitted["governance"]["verdict"] == "accepted"
    assert "candidates" in emitted
    assert "safety_fallback" not in emitted


def test_guard_diagnosis_rejected_missing_schema_blocks_raw_payload():
    payload = {"notes": "model forgot diagnoses key"}
    guarded = guard_structured_output("diagnosis", payload)

    assert guarded.verdict == "rejected"
    assert guarded.payload is None
    emitted = guarded.to_emit_dict()
    assert emitted["governance"]["verdict"] == "rejected"
    assert "candidates" not in emitted
    assert "notes" not in emitted
    assert "safety_fallback" in emitted
    assert "医疗" in emitted["safety_fallback"] or "专业" in emitted["safety_fallback"]
    # Raw unsafe / incomplete payload must not leak through any key.
    assert "model forgot" not in str(emitted)


def test_guard_diagnosis_rejected_on_clinical_red_flag_claim():
    """Red-flag keywords in clinical claim fields hard-block emit."""
    payload = {
        "candidates": [
            {
                "name": "普通酸胀",
                "confidence": "高",
                "severity": "轻度",
                "basis": "用户描述剧烈疼痛且麻木无力，但仍建议自行拉伸即可。",
                "typical_symptoms": "颈肩不适",
            }
        ]
    }
    guarded = guard_structured_output("diagnosis", payload)
    assert guarded.verdict == "rejected"
    assert guarded.payload is None
    emitted = guarded.to_emit_dict()
    # Clinical claim body is blocked; only governance + safety fallback remain.
    assert "candidates" not in emitted
    assert "safety_fallback" in emitted
    # Reasons may mention the matched keyword for audit, but the raw model
    # diagnoses list must never be delivered.
    assert "basis" not in emitted


def test_guard_ignores_red_flag_keywords_in_warning_signs():
    """Intentional caution text must not false-reject a valid plan."""
    payload = {
        "status": "proposed",
        "goal": "改善圆肩",
        "duration_weeks": 4,
        "interventions": [
            {
                "kind": "exercise",
                "title": "开胸拉伸",
                "description": "温和开胸",
                "prescription": {"sets": 2, "reps": "30秒"},
            }
        ],
        "warning_signs": ["出现剧烈疼痛或麻木无力时及时就医"],
    }
    guarded = guard_structured_output("treatment", payload)
    assert guarded.verdict in ("accepted", "degraded")
    assert guarded.payload is not None
    assert "interventions" in guarded.to_emit_dict()


def test_guard_treatment_degraded_on_ungrounded_faithfulness():
    """Faithfulness is degrade-only: ungrounded exercises still emit, annotated."""
    payload = {
        "status": "proposed",
        "goal": "改善圆肩",
        "duration_weeks": 4,
        "interventions": [
            {
                "kind": "exercise",
                "title": "完全虚构的动作XYZ",
                "description": "x",
                "prescription": {"sets": 3, "reps": 10},
            }
        ],
        "warning_signs": [],
    }
    rag_results = [
        {
            "title": "胸椎伸展",
            "body_markdown": "胸椎伸展有助于改善圆肩。",
            "clips": [],
        }
    ]
    guarded = guard_structured_output(
        "treatment",
        payload,
        rag_results=rag_results,
    )

    assert guarded.verdict == "degraded"
    assert guarded.payload is not None
    emitted = guarded.to_emit_dict()
    assert emitted["governance"]["verdict"] == "degraded"
    assert emitted["interventions"][0]["title"] == "完全虚构的动作XYZ"
    assert "safety_note" in emitted


def test_guard_treatment_rejected_missing_plan_blocks_raw():
    payload = {"other": "no plan here"}
    guarded = guard_structured_output("treatment", payload)

    assert guarded.verdict == "rejected"
    assert guarded.payload is None
    emitted = guarded.to_emit_dict()
    assert "treatment_plan" not in emitted
    assert "other" not in emitted
    assert emitted["safety_fallback"]


def test_guard_diagnosis_rejected_empty_diagnoses_list_still_has_field():
    """Empty list satisfies 'field present'; missing field is the hard reject."""
    # diagnoses key present but empty — schema only checks key presence.
    payload = {"candidates": []}
    guarded = guard_structured_output("diagnosis", payload)
    # Not a schema miss; may accept or degrade depending on other policies.
    assert guarded.verdict in ("accepted", "degraded")
    assert guarded.payload is not None


def test_guard_treatment_accepted_when_grounded():
    payload = {
        "status": "proposed",
        "goal": "激活臀部",
        "duration_weeks": 4,
        "interventions": [
            {
                "kind": "exercise",
                "title": "臀桥",
                "description": "仰卧抬臀",
                "prescription": {"sets": 3, "reps": 12},
            }
        ],
        "daily_habits": ["每小时起身"],
        "expected_timeline": "4周",
        "warning_signs": ["急性剧痛"],
    }
    rag_results = [
        {
            "title": "臀桥训练",
            "body_markdown": "臀桥是一种常见的臀部激活训练，有助于骨盆稳定。",
            "clips": [],
        }
    ]
    guarded = guard_structured_output(
        "treatment",
        payload,
        rag_results=rag_results,
    )

    assert guarded.verdict in ("accepted", "degraded")
    assert guarded.payload is not None
    emitted = guarded.to_emit_dict()
    assert emitted["interventions"][0]["title"] == "臀桥"


def test_to_safety_events_always_emits_reviewed():
    payload = {
        "candidates": [
            {
                "name": "圆肩倾向",
                "confidence": "中",
                "severity": "轻度",
                "basis": "久坐含胸",
                "typical_symptoms": "肩颈酸胀",
            }
        ]
    }
    guarded = guard_structured_output("diagnosis", payload)
    events = guarded.to_safety_events()
    assert len(events) == 1
    assert events[0]["type"] == "safety.output_reviewed"
    assert events[0]["verdict"] == "accepted"
    assert events[0]["kind"] == "diagnosis"


def test_to_safety_events_rejected_emits_pair():
    guarded = guard_structured_output("treatment", {"other": "no plan"})
    events = guarded.to_safety_events()
    assert [e["type"] for e in events] == [
        "safety.output_reviewed",
        "safety.output_rejected",
    ]
    assert events[0]["verdict"] == "rejected"
    assert events[1]["verdict"] == "rejected"
    assert events[1]["safety_fallback"]
