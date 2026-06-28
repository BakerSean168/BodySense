"""Reassessment service for training plan adjustments."""

import json
from typing import Any

from ..ai import AiRequest, AIService
from ..ai.types import ChatMessage

REASSESSMENT_PROMPT = """你是一位专业的体态健康训练顾问。根据用户的训练反馈和复评结果，
分析训练效果并给出下一阶段的调整建议。

## 输出要求
以 JSON 格式输出调整建议：
{
  "analysis": "训练效果分析总结",
  "adjustments": {
    "difficulty": "增加/保持/降低",
    "duration": "延长/保持/缩短",
    "exercise_changes": [
      {
        "action": "替换/增加/移除",
        "exercise": "动作名称",
        "reason": "原因"
      }
    ]
  },
  "next_phase_plan": {
    "focus": "下一阶段重点",
    "exercises": [
      {
        "name": "动作名称",
        "description": "描述",
        "sets": "组数",
        "reps": "次数",
        "notes": "注意事项"
      }
    ]
  },
  "motivation": "鼓励和建议"
}"""


class ReassessmentService:
    """Service for generating reassessment analysis."""

    def __init__(self) -> None:
        self._ai = AIService()

    async def analyze_feedback(
        self,
        feedback: dict[str, Any],
        training_logs: list[dict[str, Any]],
        current_plan: dict[str, Any],
    ) -> dict[str, Any]:
        """
        Analyze training feedback and generate adjustment suggestions.

        Args:
            feedback: User's reassessment feedback (symptom changes, feelings, difficulties).
            training_logs: Recent training logs.
            current_plan: Current training plan details.

        Returns:
            Adjustment suggestions.
        """
        # Build context
        context_parts = ["## 用户复评反馈"]
        if feedback.get("symptom_changes"):
            context_parts.append(f"- 症状变化：{feedback['symptom_changes']}")
        if feedback.get("training_feeling"):
            context_parts.append(f"- 训练感受：{feedback['training_feeling']}")
        if feedback.get("difficulties"):
            context_parts.append(f"- 遇到困难：{feedback['difficulties']}")

        context_parts.append("\n## 训练日志摘要")
        check_in_count = sum(1 for log in training_logs if log.get("is_checked_in"))
        context_parts.append(f"- 打卡次数：{check_in_count}")
        if training_logs:
            recent_notes = [log.get("notes", "") for log in training_logs[:5] if log.get("notes")]
            if recent_notes:
                context_parts.append(f"- 近期笔记：{'; '.join(recent_notes)}")

        context_parts.append("\n## 当前计划")
        context_parts.append(f"- 目标：{current_plan.get('goal', '未知')}")
        context_parts.append(f"- 周期：{current_plan.get('duration_weeks', 0)} 周")
        context_parts.append(f"- 当前：第 {current_plan.get('current_week', 1)} 周")

        context = "\n".join(context_parts)

        messages = [
            ChatMessage(role="system", content=REASSESSMENT_PROMPT),
            ChatMessage(role="user", content=context),
        ]

        # Call LLM via AIService (json_mode guarantees valid JSON)
        response = await self._ai.generate(AiRequest(
            use_case="llm.json",
            messages=messages,
            response_format="json_object",
            temperature=0.3,
            max_tokens=2048,
        ))

        return json.loads(response.text)


_reassessment_service: ReassessmentService | None = None


def get_reassessment_service() -> ReassessmentService:
    global _reassessment_service
    if _reassessment_service is None:
        _reassessment_service = ReassessmentService()
    return _reassessment_service
