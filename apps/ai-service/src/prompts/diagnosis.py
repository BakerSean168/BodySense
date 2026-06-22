"""Prompt templates for diagnosis analysis and treatment plan generation."""

DIAGNOSIS_SYSTEM_PROMPT = """你是一位专业的体态健康顾问。根据对话中收集到的症状信息和用户档案，
给出可能性分析判断。

## 输出要求
你必须以 JSON 格式输出可能性判断列表，结构如下：
{
  "diagnoses": [
    {
      "name": "问题名称（通俗易懂）",
      "confidence": "高/中/低",
      "severity": "轻度/中度/重度",
      "basis": "匹配依据（基于用户描述的哪些症状）",
      "typical_symptoms": "该问题的典型表现描述",
      "differential": "与其他可能判断的区别说明"
    }
  ]
}

## 重要原则
- 根据信息充分程度给出 1-3 个可能性判断
- 按匹配度从高到低排序
- 使用通俗易懂的中文，避免过多专业术语
- 不要做绝对化诊断，使用"可能""倾向于"等表述
- 说明各判断之间的区别和关联"""


TREATMENT_SYSTEM_PROMPT = """你是一位专业的体态健康改善顾问。根据确认的诊断结果，
生成个性化的改善方案。

## 输出要求
你必须以 JSON 格式输出改善方案，结构如下：
{
  "treatment_plan": {
    "goal": "训练目标描述",
    "duration_weeks": 4,
    "correction_exercises": [
      {
        "name": "动作名称",
        "description": "动作描述",
        "sets": "组数",
        "reps": "次数/时长",
        "notes": "注意事项"
      }
    ],
    "daily_habits": ["习惯调整建议1", "习惯调整建议2"],
    "nutrition_advice": "饮食建议（如适用）",
    "expected_timeline": "预期改善周期描述",
    "warning_signs": ["警示信号1", "警示信号2"]
  }
}

## 重要原则
- 方案要具体可执行，不要笼统建议
- 矫正动作要描述清楚，包含组数、次数、注意事项
- 结合知识库中的改善方法（如有）
- 明确列出警示信号：出现哪些情况应停止并就医
- 使用通俗易懂的中文"""


def get_diagnosis_prompt(
    extracted_info: list[dict],
    profile: dict,
    conversation_summary: str,
    rag_context: str = "",
) -> str:
    """Build the diagnosis prompt."""
    parts = ["请根据以下信息给出可能性分析判断：\n"]

    # Profile info
    parts.append("## 用户档案")
    if profile.get("age"):
        parts.append(f"- 年龄：{profile['age']}岁")
    if profile.get("occupation"):
        parts.append(f"- 职业：{profile['occupation']}")
    if profile.get("exercise_frequency"):
        parts.append(f"- 运动频率：{profile['exercise_frequency']}")

    # Extracted symptoms
    parts.append("\n## 已提取的症状信息")
    for info in extracted_info:
        line = f"- {info.get('body_part', '未知部位')}"
        if info.get("symptom_type"):
            line += f"：{info['symptom_type']}"
        if info.get("duration"):
            line += f"，持续{info['duration']}"
        if info.get("trigger"):
            line += f"，{info['trigger']}时出现"
        if info.get("severity"):
            line += f"（{info['severity']}）"
        parts.append(line)

    # Conversation summary
    if conversation_summary:
        parts.append(f"\n## 对话摘要\n{conversation_summary}")

    # RAG context
    if rag_context:
        parts.append(f"\n{rag_context}")

    return "\n".join(parts)


def get_treatment_prompt(
    confirmed_diagnosis: dict,
    extracted_info: list[dict],
    profile: dict,
    rag_context: str = "",
) -> str:
    """Build the treatment plan prompt."""
    parts = ["请根据以下确认的诊断生成个性化改善方案：\n"]

    # Confirmed diagnosis
    parts.append("## 确认的诊断")
    parts.append(f"- 问题名称：{confirmed_diagnosis.get('name', '未知')}")
    parts.append(f"- 严重程度：{confirmed_diagnosis.get('severity', '未知')}")
    if confirmed_diagnosis.get("basis"):
        parts.append(f"- 匹配依据：{confirmed_diagnosis['basis']}")

    # Profile info
    parts.append("\n## 用户档案")
    if profile.get("age"):
        parts.append(f"- 年龄：{profile['age']}岁")
    if profile.get("occupation"):
        parts.append(f"- 职业：{profile['occupation']}")
    if profile.get("exercise_type"):
        parts.append(f"- 运动类型：{profile['exercise_type']}")
    if profile.get("exercise_frequency"):
        parts.append(f"- 运动频率：{profile['exercise_frequency']}")
    if profile.get("injury_history"):
        parts.append(f"- 伤病史：{profile['injury_history']}")

    # Extracted symptoms
    parts.append("\n## 症状信息")
    for info in extracted_info:
        line = f"- {info.get('body_part', '未知部位')}"
        if info.get("symptom_type"):
            line += f"：{info['symptom_type']}"
        parts.append(line)

    # RAG context
    if rag_context:
        parts.append(f"\n{rag_context}")

    return "\n".join(parts)
