from src.models.consultation_intake import ConsultationIntakeOutput, ConsultationSymptomDraft
from src.runtime.consultation_thread import _get_conversation_text, _guard_final_assistant_text
from src.services.consultation_intake_service import deterministic_intake_fallback
from src.services.consultation_state_acquisition import (
    apply_structured_intake_answer,
    build_symptom_intake_question,
    intake_state_candidates,
)


def test_safety_text_ignores_assistant_and_tool_red_flag_examples() -> None:
    state = {
        "runtime_messages": [
            {"role": "user", "content": "我只是想了解坐骨神经痛是什么"},
            {"role": "assistant", "content": "如果出现放射痛或会阴麻木需要警惕。"},
            {"role": "tool", "content": "选项：大小便控制异常或会阴麻木"},
        ]
    }
    assert _get_conversation_text(state) == "我只是想了解坐骨神经痛是什么"


def test_runtime_guard_drops_optional_manual_followup_question() -> None:
    text = "目前先按上面的方式观察。\n\n需要我帮你搜索一些缓解动作吗？"
    guarded, question = _guard_final_assistant_text(text)
    assert guarded == "目前先按上面的方式观察。"
    assert question is None


def test_runtime_guard_drops_entire_multi_offer_question_tail() -> None:
    text = (
        "目前先减少连续久坐。\n\n是否需要我为你搜索一些缓解动作？\n或者你有其他问题想进一步讨论？"
    )
    guarded, question = _guard_final_assistant_text(text)
    assert guarded == "目前先减少连续久坐。"
    assert question is None


def test_runtime_guard_routes_real_trailing_question_to_hitl() -> None:
    text = "这个信息会影响下一步安全判断。\n\n你目前有没有出现明显无力？"
    guarded, question = _guard_final_assistant_text(text)
    assert guarded == "这个信息会影响下一步安全判断。"
    assert question == "你目前有没有出现明显无力？"


def test_runtime_guard_removes_embedded_manual_question_block() -> None:
    text = (
        "可以先减少连续久坐。\n\n"
        "3. **需要进一步确认的信息**：\n"
        "- 疼痛是否伴随其他症状（如麻木、刺痛、放射痛）？\n"
        "- 是否有特定动作会加重或缓解疼痛？\n"
        "为了更精准地分析，我需要您补充以下信息："
    )
    guarded, question = _guard_final_assistant_text(text)
    assert guarded == "可以先减少连续久坐。"
    assert question == "疼痛是否伴随其他症状（如麻木、刺痛、放射痛）？"
    assert "是否有特定动作" not in guarded


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


def test_gluteal_pain_without_radiation_still_requests_neurological_screen() -> None:
    symptom = {
        "capture_id": "0123456789abcdef01234567",
        "body_part": "右臀",
        "symptom_type": "疼痛",
        "duration": "两周",
        "trigger": "久坐",
    }
    question = build_symptom_intake_question(symptom)
    assert question is not None
    assert question["fields"][0]["key"] == "neurological_signs"


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
