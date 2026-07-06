"""Consultation agent turn orchestration.

This module hides the multi-round tool loop behind a small interface so the
LangGraph node only adapts graph state to an agent turn.
"""

from __future__ import annotations

import asyncio
import re
from collections.abc import Callable
from typing import Any, Protocol, TypedDict

from ...ai import AiRequest
from ...ai.types import ChatMessage, ToolCall
from .tool_executor import ToolExecutor
from .tool_registry import ToolRegistry

MAX_TOOL_ROUNDS = 3


def _health_features_from_symptom(info: dict[str, Any]) -> dict[str, Any]:
    body_part = str(info.get("body_part", "") or "").strip()
    label = str(info.get("symptom_type", "") or "").strip() or body_part
    if not label:
        return {
            "posture_findings": [],
            "discomforts": [],
            "negative_findings": [],
            "movement_limitations": [],
            "red_flags": [],
            "user_answers": [],
        }

    item: dict[str, Any] = {
        "label": label,
        "source": "extracted_info",
    }
    if body_part:
        item["body_part"] = body_part
    severity = str(info.get("severity", "") or "").strip()
    if severity:
        item["value"] = severity
    details = [
        str(info.get(key, "") or "").strip()
        for key in ("duration", "trigger", "relief")
        if str(info.get(key, "") or "").strip()
    ]
    if details:
        item["details"] = "，".join(details)

    return {
        "posture_findings": [],
        "discomforts": [item],
        "negative_findings": [],
        "movement_limitations": [],
        "red_flags": [],
        "user_answers": [],
    }


class StreamWriter(Protocol):
    def __call__(self, event: dict[str, Any]) -> None: ...


class AIStreamer(Protocol):
    def generate_stream(self, req: AiRequest) -> Any: ...


class AgentTurnInput(TypedDict):
    user_message: str
    rag_results: list[dict[str, Any]]
    messages: list[ChatMessage]


class AgentTurnResult(TypedDict, total=False):
    accumulated_text: str
    extracted_symptoms: list[dict[str, Any]]
    llm_available: bool
    interrupted: bool


class AgentOrchestrator:
    """Runs one consultation agent turn, including tool calls and HITL interrupts."""

    def __init__(
        self,
        *,
        ai_provider: Callable[[], AIStreamer],
        registry: ToolRegistry,
        executor: ToolExecutor,
        max_tool_rounds: int = MAX_TOOL_ROUNDS,
    ) -> None:
        self._ai_provider = ai_provider
        self._registry = registry
        self._executor = executor
        self._max_tool_rounds = max_tool_rounds

    async def stream_turn(
        self,
        turn_input: AgentTurnInput,
        *,
        writer: StreamWriter,
    ) -> AgentTurnResult:
        accumulated_text = ""
        new_symptoms: list[dict[str, Any]] = []

        try:
            ai = self._ai_provider()
        except Exception:
            fallback_text = build_fallback_reply(
                turn_input["user_message"],
                turn_input.get("rag_results"),
            )
            for chunk in _chunk_text(fallback_text):
                accumulated_text += chunk
                writer({"type": "text_delta", "delta": chunk})
            return {
                "accumulated_text": accumulated_text,
                "llm_available": False,
                "extracted_symptoms": [],
            }

        provider_tools = self._registry.to_provider_tools()
        messages = list(turn_input["messages"])
        seen_queries: set[str] = set()
        seen_body_parts: set[str] = set()
        text_buffer = TextBuffer(writer)

        for _ in range(self._max_tool_rounds):
            completed_tool_calls: list[dict[str, Any]] = []

            async for event in ai.generate_stream(
                AiRequest(
                    use_case="consultation.reply",
                    messages=messages,
                    tools=provider_tools,
                    temperature=0.7,
                    max_tokens=2048,
                )
            ):
                if event.type == "text_delta" and event.text:
                    accumulated_text += event.text
                    text_buffer.append(event.text)
                    await asyncio.sleep(0)
                elif event.type == "tool_call_done" and event.tool_name:
                    text_buffer.flush()
                    tc_id = event.tool_call_id or ""
                    # Dedup: skip if this tool call ID already exists in the current round
                    if not any(tc["id"] == tc_id for tc in completed_tool_calls):
                        completed_tool_calls.append(
                            {
                                "id": tc_id,
                                "name": event.tool_name,
                                "arguments": event.tool_arguments or {},
                            }
                        )

            text_buffer.flush()
            if not completed_tool_calls:
                break

            tool_messages, has_search, interrupted = await self._handle_tool_calls(
                completed_tool_calls,
                writer=writer,
                seen_queries=seen_queries,
                seen_body_parts=seen_body_parts,
                new_symptoms=new_symptoms,
                text_buffer=text_buffer,
            )
            if interrupted:
                return {
                    "accumulated_text": accumulated_text,
                    "extracted_symptoms": new_symptoms,
                    "llm_available": True,
                    "interrupted": True,
                }

            if tool_messages:
                assistant_tool_calls = [
                    ToolCall(id=tc["id"], name=tc["name"], arguments=tc["arguments"])
                    for tc in completed_tool_calls
                ]
                assistant_msg = ChatMessage(role="assistant", content="")
                assistant_msg.tool_calls = assistant_tool_calls
                messages.append(assistant_msg)
                messages.extend(tool_messages)

            if has_search or not accumulated_text:
                continue
            break

        return {
            "accumulated_text": accumulated_text,
            "extracted_symptoms": new_symptoms,
            "llm_available": True,
        }

    async def _handle_tool_calls(
        self,
        completed_tool_calls: list[dict[str, Any]],
        *,
        writer: StreamWriter,
        seen_queries: set[str],
        seen_body_parts: set[str],
        new_symptoms: list[dict[str, Any]],
        text_buffer: "TextBuffer",
    ) -> tuple[list[ChatMessage], bool, bool]:
        tool_messages: list[ChatMessage] = []
        has_search = False
        seen_tc_ids: set[str] = set()

        for tc in completed_tool_calls:
            tc_id = tc["id"]
            tc_name = tc["name"]
            tc_args = tc["arguments"]

            # Dedup: skip tool calls already processed in this round
            if tc_id and tc_id in seen_tc_ids:
                continue
            if tc_id:
                seen_tc_ids.add(tc_id)

            writer({"type": "tool_call", "id": tc_id, "tool": tc_name, "args": tc_args})

            if tc_name == "extract_symptom_info":
                await self._handle_extract_symptom(
                    tc_id,
                    tc_name,
                    tc_args,
                    writer=writer,
                    seen_body_parts=seen_body_parts,
                    new_symptoms=new_symptoms,
                )
                tool_messages.append(
                    ChatMessage(role="tool", content="症状信息已提取。", tool_call_id=tc_id)
                )

            elif tc_name == "search_knowledge":
                has_search = True
                tool_messages.append(
                    await self._handle_search_knowledge(
                        tc_id,
                        tc_name,
                        tc_args,
                        writer=writer,
                        seen_queries=seen_queries,
                    )
                )

            elif tc_name == "ask_user":
                interrupted = await self._handle_ask_user(
                    tc_id,
                    tc_name,
                    tc_args,
                    writer=writer,
                    text_buffer=text_buffer,
                )
                if interrupted:
                    return tool_messages, has_search, True

        return tool_messages, has_search, False

    async def _handle_extract_symptom(
        self,
        tool_call_id: str,
        tool_name: str,
        arguments: dict[str, Any],
        *,
        writer: StreamWriter,
        seen_body_parts: set[str],
        new_symptoms: list[dict[str, Any]],
    ) -> None:
        body_part = arguments.get("body_part", "")
        if body_part and body_part not in seen_body_parts:
            seen_body_parts.add(body_part)
            result = await self._executor.execute(tool_call_id, tool_name, arguments)
            if result.status.value == "success":
                normalized = result.content or arguments
                new_symptoms.append(normalized)
                writer({"type": "extracted_info", "info": normalized})
                writer(
                    {
                        "type": "health_features",
                        "health_features": _health_features_from_symptom(normalized),
                    }
                )
            else:
                new_symptoms.append(arguments)
                writer({"type": "extracted_info", "info": arguments})
                writer(
                    {
                        "type": "health_features",
                        "health_features": _health_features_from_symptom(arguments),
                    }
                )

        writer(
            {
                "type": "tool_result",
                "id": tool_call_id,
                "tool": tool_name,
                "result": {"status": "ok"},
            }
        )

    async def _handle_search_knowledge(
        self,
        tool_call_id: str,
        tool_name: str,
        arguments: dict[str, Any],
        *,
        writer: StreamWriter,
        seen_queries: set[str],
    ) -> ChatMessage:
        query = arguments.get("query", "")
        if query in seen_queries:
            writer(
                {
                    "type": "tool_result",
                    "id": tool_call_id,
                    "tool": tool_name,
                    "result": {"has_results": False, "deduplicated": True},
                }
            )
            return ChatMessage(
                role="tool",
                content="之前已搜索过相同内容，请直接使用已有结果回答。",
                tool_call_id=tool_call_id,
            )
        seen_queries.add(query)

        result = await self._executor.execute(tool_call_id, tool_name, arguments)
        if result.status.value == "success":
            content = result.content or {}
            result_text = content.get("result_text", "")
            has_results = content.get("has_results", False)
            raw_results = content.get("raw_results", [])
            if has_results:
                emit_citation_events(raw_results, writer)
            else:
                writer(
                    {
                        "type": "knowledge_gap",
                        "query": query,
                        "message": "知识库中暂未找到相关专项资料，以下为通用建议。",
                    }
                )
        else:
            result_text = result.error or "搜索失败"
            has_results = False

        writer(
            {
                "type": "tool_result",
                "id": tool_call_id,
                "tool": tool_name,
                "result": {"has_results": has_results},
            }
        )
        return ChatMessage(role="tool", content=result_text, tool_call_id=tool_call_id)

    async def _handle_ask_user(
        self,
        tool_call_id: str,
        tool_name: str,
        arguments: dict[str, Any],
        *,
        writer: StreamWriter,
        text_buffer: "TextBuffer",
    ) -> bool:
        result = await self._executor.execute(tool_call_id, tool_name, arguments)
        text_buffer.flush()

        if result.status.value == "interrupted":
            question_data = result.content or {}
            writer(
                {
                    "type": "state.interaction.required",
                    "interaction_id": tool_call_id,
                    "tool_call_id": tool_call_id,
                    "question": question_data,
                }
            )
            return True

        writer(
            {
                "type": "tool_result",
                "id": tool_call_id,
                "tool": tool_name,
                "result": {"status": "error", "error": result.error},
            }
        )
        return False


class TextBuffer:
    def __init__(self, writer: StreamWriter, size: int = 20) -> None:
        self._writer = writer
        self._size = size
        self._value = ""

    def append(self, text: str) -> None:
        self._value += text
        if len(self._value) >= self._size:
            self.flush()

    def flush(self) -> None:
        if self._value:
            self._writer({"type": "text_delta", "delta": self._value})
            self._value = ""


def build_fallback_reply(
    user_message: str,
    rag_results: list[dict[str, Any]] | None = None,
) -> str:
    """Build a deterministic reply when no online LLM is configured."""
    if not rag_results:
        return (
            "我已经收到你的描述，但当前本地环境没有配置云端大模型，且知识库里暂时没有检索到足够匹配的条目。\n"
            "你可以继续补充具体部位、动作场景、是否双侧对称，以及持续多久，我会继续按本地知识库帮你缩小范围。"
        )

    top = rag_results[0]
    title = top.get("title", "相关体态问题")
    summary = top.get("summary", "").strip()
    content = top.get("body_markdown") or top.get("content") or summary or ""
    plain = _markdown_to_text(content)
    lines = [f'根据当前本地知识库，你提到的问题最接近"{title}"。']

    if summary:
        lines.append(f"核心判断：{summary}")
    if plain:
        lines.append(f"知识要点：{plain[:280]}")

    clips = top.get("clips") or []
    if clips:
        clip_titles = [c.get("title", "").strip() for c in clips[:2] if c.get("title")]
        if clip_titles:
            lines.append(f"可参考的动作演示：{'、'.join(clip_titles)}。")

    if len(rag_results) > 1:
        extra = [r.get("title", "").strip() for r in rag_results[1:3] if r.get("title")]
        if extra:
            lines.append(f"我同时参考了：{'、'.join(extra)}。")

    lines.append("当前回答来自本地 curated 知识库整理，不构成医疗诊断；")
    lines.append("如果你愿意，我可以继续根据你的具体症状帮你细化判断。")
    return "\n".join(lines)


def emit_citation_events(search_results: list[Any], writer: StreamWriter) -> None:
    """Emit NDJSON citation events for knowledge search results."""
    for result in search_results:
        body = getattr(result, "body_markdown", "") or ""
        writer(
            {
                "type": "citation",
                "citation": {
                    "title": getattr(result, "title", ""),
                    "summary": getattr(result, "summary", ""),
                    "body_markdown": body[:500] if len(body) > 500 else body,
                    "source_title": getattr(result, "source_title", ""),
                    "source_author": getattr(result, "source_author", ""),
                    "category": getattr(result, "category", ""),
                    "problem_slug": getattr(result, "problem_slug", ""),
                    "unit_type": getattr(result, "unit_type", ""),
                    "tags": getattr(result, "tags", []) or [],
                    "clips": getattr(result, "clips", []) or [],
                },
            }
        )


def _markdown_to_text(content: str) -> str:
    text = re.sub(r"^#+\s*", "", content, flags=re.MULTILINE)
    text = re.sub(r"^[*-]\s*", "", text, flags=re.MULTILINE)
    text = re.sub(r"\n{2,}", "\n", text)
    return text.strip()


def _chunk_text(text: str, chunk_size: int = 120) -> list[str]:
    return [text[i : i + chunk_size] for i in range(0, len(text), chunk_size)]
