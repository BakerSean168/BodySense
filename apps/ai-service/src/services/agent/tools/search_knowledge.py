"""search_knowledge tool — queries the knowledge base for posture health info."""

from __future__ import annotations

from typing import Any

from ....rag.knowledge_library import get_knowledge_library
from ..tool_types import RuntimeToolDefinition, ToolCategory

# Tool schema — same as the existing KNOWLEDGE_SEARCH_TOOL in consultation_graph.py
SEARCH_KNOWLEDGE_SCHEMA: dict[str, Any] = {
    "name": "search_knowledge",
    "description": (
        "从体态健康知识库中搜索相关信息。当需要查找症状定义、自我检测方法、"
        "改善动作、风险提示、肌肉失衡原因等专业知识时调用此工具。"
        "搜索结果包含标题、摘要、详细内容和相关动作演示。"
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "query": {
                "type": "string",
                "description": "搜索查询文本，如：'圆肩 自测' 或 '腰痛 改善动作'",
            },
            "top_k": {
                "type": "integer",
                "description": "返回结果数量，默认3",
                "default": 3,
            },
        },
        "required": ["query"],
    },
}


async def handle_search_knowledge(arguments: dict[str, Any]) -> dict[str, Any]:
    """Execute a knowledge search.

    Returns a dict with:
        - result_text: formatted text for the LLM
        - has_results: whether any results were found
        - raw_results: list of SearchResult objects for citation emission
    """
    library = get_knowledge_library()
    results = await library.search(
        query=arguments.get("query", ""),
        top_k=arguments.get("top_k", 3),
    )

    if not results:
        return {
            "result_text": (
                "【知识库无匹配】未找到与该查询相关的知识库条目。"
                "请基于你的专业判断回答，并明确标注信息来源为个人建议而非知识库专项指导。"
                "如用户问题较专业，建议引导用户咨询专业医疗机构。"
            ),
            "has_results": False,
            "raw_results": [],
        }

    parts = []
    for i, r in enumerate(results[:3], 1):
        title = r.title or "未知标题"
        summary = r.summary or ""
        content = (r.body_markdown or "")[:600]
        source = r.source_title or ""
        category = r.category or ""

        part = f"[{i}] {title}"
        if category:
            part += f"（分类：{category}）"
        if summary:
            part += f"\n摘要：{summary}"
        if source:
            part += f"\n来源：{source}"
        if content:
            part += f"\n内容：{content}"
        parts.append(part)

    return {
        "result_text": "\n\n".join(parts),
        "has_results": True,
        "raw_results": results,
    }


def make_search_knowledge_tool() -> RuntimeToolDefinition:
    """Create a RuntimeToolDefinition for search_knowledge."""
    return RuntimeToolDefinition(
        name=SEARCH_KNOWLEDGE_SCHEMA["name"],
        description=SEARCH_KNOWLEDGE_SCHEMA["description"],
        parameters=SEARCH_KNOWLEDGE_SCHEMA["parameters"],
        category=ToolCategory.QUERY,
        handler=handle_search_knowledge,
        required_params=["query"],
    )
