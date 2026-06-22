"""Prompt templates for health assessment report generation."""

ASSESSMENT_SYSTEM_PROMPT = """你是一位专业的体态健康评估专家。
根据用户的身体档案信息，生成一份全面的健康评估报告。

## 评估维度
你需要从以下维度进行评分（每项 0-100 分）：
1. **体态**：基于身高体重 BMI、职业久坐情况、运动习惯等
2. **运动能力**：基于运动类型、频率、伤病史等
3. **生活习惯**：基于作息时间、日常活动类型等
4. **伤病风险**：基于既往伤病史、运动习惯、年龄等

## 健康等级
根据综合评分给出健康等级：
- **A**（90-100）：优秀，保持良好习惯
- **B**（75-89）：良好，有改善空间
- **C**（60-74）：一般，建议关注并改善
- **D**（<60）：需要改善，建议咨询专业人士

## 输出要求
你必须以 JSON 格式输出评估结果，结构如下：
{
  "health_grade": "A/B/C/D",
  "dimension_scores": {
    "posture": <0-100>,
    "exercise": <0-100>,
    "lifestyle": <0-100>,
    "injury_risk": <0-100>,
    "overall": <0-100>
  },
  "identified_issues": [
    {
      "issue": "问题名称",
      "severity": "轻度/中度/重度",
      "description": "问题描述",
      "priority": 1
    }
  ],
  "improvement_summary": {
    "exercise": "运动建议概要",
    "lifestyle": "生活习惯建议概要",
    "nutrition": "饮食建议概要（如适用）",
    "general": "总体改善方向"
  }
}

## 重要原则
- 评分要客观，不要过于乐观或悲观
- 问题识别要基于数据，不要臆测
- 改善建议要具体可执行
- 必须在报告末尾附注医疗免责声明"""

ASSESSMENT_DISCLAIMER = (
    "本报告仅供参考，不构成医疗诊断。"
    "如存在持续疼痛或严重不适，建议前往专业医疗机构就诊。"
)


def get_assessment_prompt(profile: dict, rag_context: str = "") -> str:
    """Build the assessment prompt from user profile."""
    parts = ["请根据以下用户信息生成健康评估报告：\n"]

    # Profile info
    parts.append("## 用户身体档案")
    if profile.get("gender"):
        parts.append(f"- 性别：{profile['gender']}")
    if profile.get("age"):
        parts.append(f"- 年龄：{profile['age']}岁")
    if profile.get("height_cm") and profile.get("weight_kg"):
        parts.append(f"- 身高/体重：{profile['height_cm']}cm / {profile['weight_kg']}kg")
    if profile.get("bmi"):
        parts.append(f"- BMI：{profile['bmi']}")
    if profile.get("occupation"):
        parts.append(f"- 职业/日常活动：{profile['occupation']}")
    if profile.get("sleep_time") and profile.get("wake_time"):
        parts.append(f"- 作息：{profile['sleep_time']} 入睡，{profile['wake_time']} 起床")
    if profile.get("exercise_type"):
        parts.append(f"- 运动类型：{profile['exercise_type']}")
    if profile.get("exercise_frequency"):
        parts.append(f"- 运动频率：{profile['exercise_frequency']}")
    if profile.get("injury_history"):
        parts.append(f"- 既往伤病史：{profile['injury_history']}")
    if profile.get("self_description"):
        parts.append(f"- 自我描述：{profile['self_description']}")

    # RAG context
    if rag_context:
        parts.append(f"\n{rag_context}")

    parts.append(f"\n\n{ASSESSMENT_DISCLAIMER}")

    return "\n".join(parts)
