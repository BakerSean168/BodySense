"""Deterministic model fixtures for local deploy and browser E2E validation.

The switch is deliberately explicit. Production and ordinary development keep
using configured providers; only a caller that sets ``BODYSENSE_DETERMINISTIC_AI``
gets these stable outputs. The typed Agents still execute their normal PydanticAI
structured-output and governance paths.
"""

from __future__ import annotations

import os
from typing import Any, Literal

from pydantic_ai.models import Model
from pydantic_ai.models.test import TestModel

_TRUE_VALUES = {"1", "true", "yes", "on"}


def deterministic_ai_enabled() -> bool:
    return os.getenv("BODYSENSE_DETERMINISTIC_AI", "").strip().lower() in _TRUE_VALUES


def deterministic_diagnosis_model(*, call_tools: list[str] | Literal["all"] = "all") -> Model:
    return TestModel(
        call_tools=call_tools,
        custom_output_args={
            "status": "completed",
            "scope": "full_body",
            "summary": "当前资料支持一个与久坐负荷相关的颈肩可能性候选。",
            "candidates": [
                {
                    "concern_key": "region:颈肩",
                    "name": "久坐相关颈肩负荷模式",
                    "confidence": "中",
                    "severity": "轻度",
                    "evidence_strength": "中",
                    "impact": "久坐后出现颈肩酸胀，可能影响舒适度与活动耐受。",
                    "basis": "当前 BodyState 记录了久坐后颈肩酸胀。",
                    "typical_symptoms": "久坐后颈肩酸胀或僵硬。",
                    "differential": "仍需结合活动度、自测和时间变化继续复核。",
                    "basis_fact_ids": [],
                    "basis_observation_ids": [],
                    "supporting_evidence_ids": [],
                    "counterevidence_ids": [],
                    "reasoning_summary": "候选仅表达当前资料支持的可能性，不构成医学诊断。",
                    "missing_information": [],
                    "safety_notes": [],
                }
            ],
            "cross_concern_patterns": [],
            "information_gaps": [],
            "safety_summary": {},
        },
    )


def deterministic_treatment_model(*, call_tools: list[str] | Literal["all"] = "all") -> Model:
    return TestModel(
        call_tools=call_tools,
        custom_output_args={
            "status": "proposed",
            "summary": "以温和活动与久坐负荷管理为主，并持续记录变化。",
            "goal": "降低久坐后的颈肩酸胀",
            "duration_weeks": 4,
            "interventions": [
                {
                    "kind": "exercise",
                    "title": "下巴微收",
                    "description": "保持自然呼吸，在无明显不适的范围内轻柔完成。",
                    "prescription": {
                        "sets": 2,
                        "reps": 8,
                        "frequency": "每日一次",
                        "notes": "动作轻柔，不追求大幅度。",
                        "stop_conditions": ["疼痛或麻木明显加重"],
                    },
                }
            ],
            "daily_habits": ["每 45 分钟起身短暂活动"],
            "expected_timeline": "2 至 4 周观察趋势",
            "warning_signs": ["出现进行性无力或麻木时停止并寻求专业评估"],
            "review_triggers": ["症状持续加重或出现新不适"],
            "safety_notes": [],
            "evidence_ids": [],
        },
    )


def deterministic_assessment_model() -> Model:
    return TestModel(
        custom_output_args={
            "status": "completed",
            "health_grade": "B",
            "dimension_scores": {
                "posture": 72,
                "exercise": 68,
                "lifestyle": 70,
                "injury_risk": 75,
                "overall": 71,
            },
            "observations": [
                {
                    "kind": "posture_alignment",
                    "body_region": "肩部",
                    "label": "肩部对称性待复核",
                    "description": "当前资料提示肩部对称性值得进一步观察。",
                    "severity": "轻度",
                    "confidence": "中",
                    "method": "assessment",
                    "condition": {"source": "deterministic_validation"},
                }
            ],
            "summary": "当前资料支持一项待用户审核的观察。",
            "information_gaps": [],
            "safety_notes": [],
        }
    )


def deterministic_text_for(use_case: str) -> str:
    if use_case == "consultation.reply":
        return "我已经记录了你的描述。你可以在右侧 BodyState 中补充、确认或纠正当前事实。"
    if use_case == "conversation.title":
        return "颈肩状态记录"
    return "{}"


def deterministic_usage() -> dict[str, Any]:
    return {"input_tokens": 8, "output_tokens": 16, "total_tokens": 24}
