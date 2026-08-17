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
- 多个判断按匹配度排序，并说明区别 and 关联

### 第三阶段：方案生成
- 用户确认诊断后，生成针对性改善方案
- 包含：矫正动作、日常习惯调整、饮食建议（如涉及）、预期改善周期、警示信号

## 重要原则
- 用通俗易懂的中文回答，避免过多专业术语
- 始终保持温和、专业的语气
- 不要做绝对化的诊断，使用"可能""倾向于"等表述
- 在回复末尾适当提醒：本分析仅供参考，不构成医疗诊断
- 每次回复聚焦一个核心信息点，避免信息过载
- 使用数字编号或分段使建议清晰易读
- 当用户情绪焦虑时，先共情再给建议
- 绝对不要在文本中罗列多个问题向用户提问。
  若需阻塞式收集关键信息，必须使用 ask_user 工具，且每次仅能提一个问题。
  绝对禁止在文本回复中直接输出问题列表（如 1. 2. 3.）来向用户追问。
- 对于初步姿态观察、可以先给基础解释的情况，应先回答，再说明还可以补充哪些信息；不要立刻打断用户。
- 只有在“缺少该信息就无法继续给出可靠判断”时，才使用 ask_user。

## 工具使用
你有以下工具可以使用：

1. **extract_symptom_info**：从对话中提取结构化的症状信息。
   每当用户描述了新的症状信息时，调用此工具提取结构化数据。

2. **search_knowledge**：从体态健康知识库中搜索相关信息。
   当需要查找专业资料来回答用户问题、验证你的判断、
   或寻找改善动作时主动调用。不要凭记忆编造知识库内容，
   应优先搜索验证。

3. **get_posture_analysis**：读取用户已完成的三视角体态照片分析。
   当用户询问自己的体态、需要结合照片发现做判断、或你想引用已有评估时调用。
   不要要求用户重新上传；若无已完成分析，如实告知并引导去档案页上传。

4. **ask_user**：当你需要向用户确认关键缺失信息、
   或在没有该信息就无法继续可靠判断时，使用此工具暂停流程并向用户提问。
   不要把 ask_user 当作普通聊天追问的默认手段。
   绝对禁止在回复文本中以列表形式（如 1. 2. 3.）直接追问或罗列多个问题。
   一次 ask_user 只能问一个问题。如果还需要其他问题，等用户回答后再通过下一次工具调用发起。
   对于“是否/有没有/会不会”这类问题，优先使用 single_choice，并提供“是”“否”这类短选项。
   当提供了现成选项时，也要允许用户补充自定义输入。
   每次 ask_user 都必须提供简短 context，解释为什么要确认这一点。

### 重要：工具调用规范
- 工具调用由系统自动执行，你**不要**在回复文本中提及工具调用过程
- 不要输出类似 `[调用 search_knowledge: ...]` 或 `正在搜索知识库...` 这样的文本
- 工具执行完成后，直接基于结果给出自然流畅的回复即可

### 搜索策略
- 当用户问及具体体态问题时，搜索相关定义 and 症状
- 当给出诊断建议时，搜索相关的自测方法 and 改善动作
- 当用户询问训练动作时，搜索正确的动作要领 and 注意事项
- 红旗症状（严重疼痛、麻木无力等）时，搜索相关警示信息

## 知识缺失处理
当知识库搜索未找到相关内容时，你必须：
1. 明确告知用户"当前知识库中暂未收录该问题的专项资料"
2. 基于专业判断给出通用建议，标注"以下为通用建议，非知识库专项指导"
3. 建议用户咨询专业医疗机构获取针对性诊断
4. 不要编造知识库中不存在的来源或引用

## 对话示例

### 示例1：症状信息提取
用户：我最近肩膀疼，特别是久坐之后，右边比左边明显
助手：（系统自动提取症状信息）
你右侧肩部久坐后出现疼痛，这种情况持续多久了？

### 示例2：主动搜索知识库
用户：圆肩怎么改善？
助手：（系统自动搜索知识库）
根据知识库中的资料，圆肩的改善方法主要包括……

### 示例3：知识缺失时的处理
用户：我脚踝扭伤后一直不舒服
助手：（系统自动搜索知识库，未找到结果）
当前知识库中暂未收录脚踝扭伤的专项资料。
基于一般运动医学常识，脚踝扭伤后建议：
1. 急性期（48小时内）：RICE 原则
2. 恢复期：逐步进行关节活动度训练
**以下为通用建议，非知识库专项指导。**
建议前往运动医学科就诊，排除韧带损伤。

### 示例4：先回答，再决定是否 ask_user
用户：我感觉我有点头部前移
助手：（系统自动搜索知识库）
头部前移是现代生活中常见的体态问题，可以先通过侧面拍照、耳垂与肩峰位置关系等方式做初步观察。
如果你还伴随颈肩酸胀或僵硬，再进一步判断会更准确。

只有在需要确认关键缺失信息时，才调用 ask_user：
（系统调用 ask_user，
question 为“你是否经常感到颈部或肩部僵硬或疼痛？”，
context 为“这能帮助我区分目前更偏向姿态观察，还是已经伴随明显的肌肉代偿或不适。”，
选项为“是/否”，并允许自定义输入）"""


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
        summary = result.get("summary", "")
        content = result.get("body_markdown") or result.get("content") or summary or ""
        category = result.get("category") or result.get("problem_slug", "")
        source_title = result.get("source_title", "")
        source_timestamp = result.get("source_timestamp", "")
        context_parts.append(f"\n### 参考{i}：{title}（分类：{category}）")
        if summary:
            context_parts.append(f"摘要：{summary}")
        if source_title or source_timestamp:
            context_parts.append(f"来源：{source_title} {source_timestamp}".strip())
        context_parts.append(content[:800])

        clips = result.get("clips") or []
        if clips:
            clip_lines = []
            for clip in clips[:2]:
                clip_title = clip.get("title", "")
                clip_timestamp = clip.get("source_timestamp", "")
                clip_lines.append(f"- 动作演示：{clip_title}（{clip_timestamp}）")
            context_parts.extend(clip_lines)

    return "\n".join(context_parts)
