"""Prompt templates for the consultation chat flow."""


# System prompt for the consultation chat
SYSTEM_PROMPT = """你是一位专业的体态健康顾问，名叫"体态助手"。
你的职责是通过对话帮助用户分析体态问题，提供专业的健康建议。

## 你的工作方式

### 第一阶段：问题描述与引导
- 基于用户的身体档案信息进行初步分析
- 主动引导用户补充关键细节：症状部位、性质、持续时间、触发场景、缓解方式等
- 当用户描述模糊时，提供选项引导细化（例如："你说的'肩膀疼'，是以下哪种感觉？"）
- 积极询问容易被忽略但诊断价值高的信息：症状时间规律、是否与特定动作相关、两侧是否对称等

### 第二阶段：可能性分析
- 综合所有收集到的信息，给出一个或多个可能的体态问题判断
- 每个判断包含：问题名称（通俗易懂）、匹配依据、典型表现、严重程度、置信度
- 多个判断按匹配度排序，并说明区别和关联

### 第三阶段：方案生成
- 用户确认诊断后，生成针对性改善方案
- 包含：矫正动作、日常习惯调整、饮食建议（如涉及）、预期改善周期、警示信号

## 重要原则
- 用通俗易懂的中文回答，避免过多专业术语
- 始终保持温和、专业的语气
- 不要做绝对化的诊断，使用"可能""倾向于"等表述
- 在回复末尾适当提醒：本分析仅供参考，不构成医疗诊断

## 工具使用
你有一个 symptom_extraction 工具，用于从对话中实时提取结构化的症状信息。
每当用户描述了新的症状信息时，调用此工具提取结构化数据。"""


def get_system_prompt(profile_context: str = "") -> str:
    """Get the system prompt with optional user profile context."""
    prompt = SYSTEM_PROMPT
    if profile_context:
        prompt += f"\n\n## 用户身体档案\n{profile_context}"
    return prompt


def format_profile_context(profile: dict) -> str:
    """Format user profile into context string."""
    lines = []
    if profile.get("gender"):
        lines.append(f"- 性别：{profile['gender']}")
    if profile.get("age"):
        lines.append(f"- 年龄：{profile['age']}岁")
    if profile.get("height_cm") and profile.get("weight_kg"):
        lines.append(f"- 身高/体重：{profile['height_cm']}cm / {profile['weight_kg']}kg")
    if profile.get("bmi"):
        lines.append(f"- BMI：{profile['bmi']}")
    if profile.get("occupation"):
        lines.append(f"- 职业/日常活动：{profile['occupation']}")
    if profile.get("sleep_time") and profile.get("wake_time"):
        lines.append(f"- 作息：{profile['sleep_time']} 入睡，{profile['wake_time']} 起床")
    if profile.get("exercise_type"):
        lines.append(f"- 运动类型：{profile['exercise_type']}")
    if profile.get("exercise_frequency"):
        lines.append(f"- 运动频率：{profile['exercise_frequency']}")
    if profile.get("injury_history"):
        lines.append(f"- 既往伤病史：{profile['injury_history']}")
    if profile.get("self_description"):
        lines.append(f"- 自我描述：{profile['self_description']}")
    return "\n".join(lines) if lines else "（用户尚未填写身体档案）"


# Tool definition for symptom extraction
SYMPTOM_EXTRACTION_TOOL = {
    "name": "extract_symptom_info",
    "description": "从对话中提取结构化的症状信息。当用户描述了新的症状相关数据时调用此工具。",
    "parameters": {
        "type": "object",
        "properties": {
            "body_part": {
                "type": "string",
                "description": "涉及的身体部位，如：肩部、腰部、颈椎、膝盖等",
            },
            "symptom_type": {
                "type": "string",
                "description": "症状类型，如：酸胀、刺痛、钝痛、麻木、僵硬等",
            },
            "duration": {
                "type": "string",
                "description": "症状持续时间，如：3天、2周、半年",
            },
            "trigger": {
                "type": "string",
                "description": "触发场景，如：久坐后、运动后、晨起时",
            },
            "relief": {
                "type": "string",
                "description": "缓解方式，如：按压后缓解、活动后减轻",
            },
            "severity": {
                "type": "string",
                "description": "严重程度：轻度/中度/重度",
            },
            "additional_notes": {
                "type": "string",
                "description": "其他补充信息",
            },
        },
        "required": ["body_part"],
    },
}


def build_rag_context(search_results: list[dict]) -> str:
    """Build RAG context from knowledge base search results."""
    if not search_results:
        return ""

    context_parts = ["## 相关知识库参考"]
    for i, result in enumerate(search_results[:3], 1):
        title = result.get("title", "")
        content = result.get("content", "")
        category = result.get("category", "")
        context_parts.append(f"\n### 参考{i}：{title}（分类：{category}）")
        context_parts.append(content[:500])

    return "\n".join(context_parts)
