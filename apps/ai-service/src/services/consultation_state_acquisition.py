"""Deterministic policy around typed Consultation intake output."""

from __future__ import annotations

import hashlib
from typing import Any

from ..models.consultation_intake import ConsultationIntakeOutput

CAPTURE_ID_LENGTH = 24


def symptom_capture_id(
    *,
    run_id: str,
    latest_user_message: str,
    index: int,
    body_part: str,
    symptom_type: str,
) -> str:
    material = "\x00".join(
        (
            run_id.strip(),
            str(index),
            body_part.strip(),
            symptom_type.strip(),
            latest_user_message.strip(),
        )
    )
    return hashlib.sha256(material.encode("utf-8")).hexdigest()[:CAPTURE_ID_LENGTH]


def intake_state_candidates(
    output: ConsultationIntakeOutput,
    *,
    run_id: str,
    latest_user_message: str,
) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    symptoms: list[dict[str, Any]] = []
    for index, draft in enumerate(output.symptoms):
        item = draft.model_dump(mode="json")
        item["capture_id"] = symptom_capture_id(
            run_id=run_id,
            latest_user_message=latest_user_message,
            index=index,
            body_part=draft.body_part,
            symptom_type=draft.symptom_type,
        )
        item["confirmed"] = False
        symptoms.append(item)
    lifestyle = [item.model_dump(mode="json") for item in output.lifestyle]
    return symptoms, lifestyle


def build_symptom_intake_question(symptom: dict[str, Any]) -> dict[str, Any] | None:
    """Build one decision-focused form with at most three missing fields."""
    capture_id = str(symptom.get("capture_id") or "").strip().lower()
    body_part = str(symptom.get("body_part") or "身体不适部位").strip()
    symptom_type = str(symptom.get("symptom_type") or "不适").strip()
    if len(capture_id) != CAPTURE_ID_LENGTH:
        return None

    fields: list[dict[str, Any]] = []
    field_map: dict[str, str] = {}

    def add_field(
        key: str,
        target: str,
        label: str,
        answer_type: str,
        options: list[str],
    ) -> None:
        if len(fields) >= 3:
            return
        fields.append(
            {
                "key": key,
                "label": label,
                "answer_type": answer_type,
                "options": options,
                "required": True,
            }
        )
        field_map[key] = target

    symptom_text = symptom_type + str(symptom.get("additional_notes") or "")
    has_neurological_context = bool(str(symptom.get("neurological_signs") or "").strip())
    has_radiation = bool(str(symptom.get("radiation") or "").strip())
    neurological_symptom = any(token in symptom_text for token in ("麻", "无力", "刺痛", "电击"))
    axial_or_gluteal_pain = any(
        token in body_part for token in ("腰", "背", "臀", "颈", "脖")
    ) and any(token in symptom_text for token in ("痛", "疼", "酸", "胀"))
    if (
        has_radiation or neurological_symptom or axial_or_gluteal_pain
    ) and not has_neurological_context:
        add_field(
            "neurological_signs",
            "neurological_signs",
            "有没有同时出现这些神经相关信号？",
            "single_choice",
            [
                "没有",
                "有麻木或针刺感",
                "有明显无力",
                "有大小便控制异常或会阴麻木",
            ],
        )

    if not str(symptom.get("duration") or "").strip():
        add_field(
            "duration",
            "duration",
            "大约持续多久了？",
            "single_choice",
            ["今天或24小时内", "2–7天", "1–4周", "超过1个月"],
        )

    if not str(symptom.get("severity") or "").strip():
        add_field(
            "severity",
            "severity",
            "最明显时大约有多严重？",
            "single_choice",
            ["轻微（0–2/10）", "轻中度（3–4/10）", "中度（5–6/10）", "较重（7–10/10）"],
        )

    if not str(symptom.get("trigger") or "").strip():
        add_field(
            "trigger",
            "trigger",
            "通常在什么情况下更明显？",
            "multi_choice",
            ["久坐或久站", "弯腰或转身", "走路或跑步", "训练或负重"],
        )

    if not str(symptom.get("functional_impact") or "").strip():
        add_field(
            "functional_impact",
            "functional_impact",
            "对日常活动影响多大？",
            "single_choice",
            ["基本不影响", "有些受限", "明显影响日常活动", "无法正常活动"],
        )

    if not fields:
        return None

    seed_info = {
        key: value
        for key, value in symptom.items()
        if key
        in {
            "capture_id",
            "body_part",
            "symptom_type",
            "duration",
            "trigger",
            "relief",
            "severity",
            "radiation",
            "functional_impact",
            "neurological_signs",
            "onset",
            "additional_notes",
        }
        and value not in (None, "")
    }
    return {
        "question": f"我已暂存你提到的“{body_part}{symptom_type}”。请补全下面信息：",
        "reason": "complete_symptom_intake",
        "context": "这些信息会直接影响安全分层和下一步判断；提交后会合并到同一条身体记录中。",
        "answer_type": "text",
        "required": True,
        "fields": fields,
        "purpose": "symptom_intake",
        "state_binding": {
            "capture_id": capture_id,
            "seed_info": seed_info,
            "field_map": field_map,
        },
    }


def apply_structured_intake_answer(
    question: dict[str, Any],
    answer: Any,
) -> dict[str, Any] | None:
    binding = question.get("state_binding")
    if not isinstance(binding, dict) or question.get("purpose") != "symptom_intake":
        return None
    seed = binding.get("seed_info")
    field_map = binding.get("field_map")
    if not isinstance(seed, dict) or not isinstance(field_map, dict):
        return None
    completed = dict(seed)
    fields = answer.get("fields") if isinstance(answer, dict) else None
    if isinstance(fields, dict):
        for answer_key, value in fields.items():
            target = field_map.get(answer_key)
            if isinstance(target, str) and target and value not in (None, ""):
                completed[target] = str(value).strip()
    completed["confirmed"] = True
    return completed
