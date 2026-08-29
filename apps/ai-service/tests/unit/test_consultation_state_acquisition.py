from src.models.consultation_intake import ConsultationIntakeOutput, ConsultationSymptomDraft
from src.services.consultation_intake_service import deterministic_intake_fallback
from src.services.consultation_state_acquisition import (
    apply_structured_intake_answer,
    build_symptom_intake_question,
    intake_state_candidates,
)


def test_general_knowledge_question_does_not_create_user_state() -> None:
    output = deterministic_intake_fallback("坐骨神经痛是什么？")
    assert output.turn_kind == "general_question"
    assert output.symptoms == []
    assert output.lifestyle == []


def test_explicit_first_person_symptom_creates_review_candidate() -> None:
    output = deterministic_intake_fallback("我右臀疼了两周，久坐时会更明显，还会放射到小腿")
    assert output.turn_kind == "symptom_report"
    assert len(output.symptoms) == 1
    symptom = output.symptoms[0]
    assert symptom.body_part == "右臀"
    assert symptom.symptom_type == "疼痛"
    assert symptom.duration == "两周" or symptom.duration == ""
    assert symptom.trigger == "久坐"


def test_intake_candidate_has_stable_capture_id_and_is_unconfirmed() -> None:
    output = ConsultationIntakeOutput(
        turn_kind="symptom_report",
        symptoms=[
            ConsultationSymptomDraft(
                body_part="右臀",
                symptom_type="疼痛",
                radiation="小腿",
            )
        ],
    )
    first, _ = intake_state_candidates(
        output,
        run_id="run-1",
        latest_user_message="我右臀疼，会放射到小腿",
    )
    second, _ = intake_state_candidates(
        output,
        run_id="run-1",
        latest_user_message="我右臀疼，会放射到小腿",
    )
    assert first[0]["capture_id"] == second[0]["capture_id"]
    assert len(first[0]["capture_id"]) == 24
    assert first[0]["confirmed"] is False


def test_gap_policy_builds_at_most_three_typed_fields_and_preserves_binding() -> None:
    symptom = {
        "capture_id": "0123456789abcdef01234567",
        "body_part": "右臀",
        "symptom_type": "疼痛",
        "radiation": "小腿",
    }
    question = build_symptom_intake_question(symptom)
    assert question is not None
    assert question["purpose"] == "symptom_intake"
    assert 1 <= len(question["fields"]) <= 3
    assert question["fields"][0]["key"] == "neurological_signs"
    assert question["state_binding"]["capture_id"] == symptom["capture_id"]


def test_structured_answer_merges_into_same_symptom_capture() -> None:
    symptom = {
        "capture_id": "0123456789abcdef01234567",
        "body_part": "右臀",
        "symptom_type": "疼痛",
        "radiation": "小腿",
    }
    question = build_symptom_intake_question(symptom)
    assert question is not None
    completed = apply_structured_intake_answer(
        question,
        {
            "fields": {
                "neurological_signs": "没有",
                "duration": "1–4周",
                "severity": "中度（5–6/10）",
            }
        },
    )
    assert completed is not None
    assert completed["capture_id"] == symptom["capture_id"]
    assert completed["body_part"] == "右臀"
    assert completed["neurological_signs"] == "没有"
    assert completed["duration"] == "1–4周"
    assert completed["confirmed"] is True
