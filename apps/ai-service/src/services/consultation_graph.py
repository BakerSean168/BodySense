"""LangGraph-based consultation agent workflow.

Converts the linear ChatService flow into an explicit state graph with:
- safety_check: red flag detection + citation emission
- classify_intent: rule-based intent classification
- generate_response: LLM streaming with tool calling (or fallback), including symptom extraction
- decide_phase: phase determination and NDJSON event emission
- generate_diagnosis / generate_treatment: optional downstream actions
"""

import asyncio
import logging
import re
from collections.abc import AsyncIterator
from typing import Annotated, Any, TypedDict

from langgraph.graph import END, START, StateGraph
from langgraph.types import StreamWriter

from ..ai import AiRequest, AIService
from ..ai.types import ChatMessage, ToolCall
from ..models.consultation import ChatContext, ExtractedInfo
from ..prompts.consultation import format_profile_context, get_system_prompt
from .agent.consultation_tools import get_consultation_executor, get_consultation_registry
from .agent_workflow import get_agent_workflow
from .red_flag_detector import get_red_flag_detector

logger = logging.getLogger(__name__)

MAX_CONTEXT_TURNS = 10

# Module-level AIService singleton to avoid re-parsing config on every request.
_ai_service_instance: AIService | None = None


def _get_ai_service() -> AIService:
    """Get or create the module-level AIService singleton."""
    global _ai_service_instance
    if _ai_service_instance is None:
        _ai_service_instance = AIService()
    return _ai_service_instance
MAX_TOOL_ROUNDS = 3


def _merge_symptoms(
    existing: list[dict[str, Any]],
    new: list[dict[str, Any]],
) -> list[dict[str, Any]]:
    """Merge symptom lists, updating by body_part to avoid duplicates.

    New values overwrite existing values for the same body_part, including
    empty/None values (allowing field clearing by the LLM when it corrects
    a previous extraction). If this becomes a problem, filter None values
    in the update dict before merging.
    """
    by_part: dict[str, dict[str, Any]] = {}
    for s in existing:
        part = s.get("body_part", "")
        if part:
            by_part[part] = dict(s)
    for s in new:
        part = s.get("body_part", "")
        if not part:
            continue
        if part in by_part:
            by_part[part].update(s)
        else:
            by_part[part] = dict(s)
    return list(by_part.values())


# ---------------------------------------------------------------------------
# State definition
# ---------------------------------------------------------------------------

class ConsultationState(TypedDict, total=False):
    """State for the consultation agent graph."""

    # --- Inputs (set once before graph invocation) ---
    session_id: str
    user_id: str
    user_message: str
    profile: dict[str, Any]
    conversation_history: list[dict[str, Any]]
    rag_results: list[dict[str, Any]]

    # --- Accumulators (custom reducer merges by body_part) ---
    extracted_symptoms: Annotated[list[dict[str, Any]], _merge_symptoms]

    # --- Per-step outputs ---
    red_flag_result: dict[str, Any] | None
    intent: str
    workflow_action: str
    accumulated_text: str
    phase: str
    llm_available: bool
    diagnosis_result: dict[str, Any] | None
    treatment_result: dict[str, Any] | None


# ---------------------------------------------------------------------------
# Helper functions (extracted from ChatService for reuse by graph nodes)
# ---------------------------------------------------------------------------

def build_messages(state: ConsultationState) -> list[ChatMessage]:
    """Build the messages list for the LLM from graph state.

    RAG context is no longer pre-injected into the system prompt.
    The agent uses the search_knowledge tool to retrieve information
    on demand, resulting in more targeted searches.
    """
    messages: list[ChatMessage] = []

    profile_context = format_profile_context(state.get("profile", {}))
    system_content = get_system_prompt(profile_context)

    extracted = state.get("extracted_symptoms", [])
    if extracted:
        info_lines = ["## 已提取的症状信息"]
        for s in extracted:
            body_part = s.get("body_part", "")
            if not body_part:
                continue
            line = f"- {body_part}：{s.get('symptom_type', '待补充')}"
            if s.get("duration"):
                line += f"，持续{s['duration']}"
            if s.get("trigger"):
                line += f"，{s['trigger']}时出现"
            info_lines.append(line)
        if len(info_lines) > 1:
            system_content += "\n\n" + "\n".join(info_lines)

    messages.append(ChatMessage(role="system", content=system_content))

    history = state.get("conversation_history", [])
    history = history[-MAX_CONTEXT_TURNS * 2:]
    for msg in history:
        role = msg.get("role", "user")
        content = msg.get("content", "")
        messages.append(ChatMessage(role=role, content=content))

    messages.append(ChatMessage(role="user", content=state["user_message"]))
    return messages


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


def _markdown_to_text(content: str) -> str:
    """Flatten markdown into readable plain text."""
    text = re.sub(r"^#+\s*", "", content, flags=re.MULTILINE)
    text = re.sub(r"^[*-]\s*", "", text, flags=re.MULTILINE)
    text = re.sub(r"\n{2,}", "\n", text)
    return text.strip()


def _chunk_text(text: str, chunk_size: int = 120) -> list[str]:
    """Split a reply into stream-friendly chunks."""
    return [text[i : i + chunk_size] for i in range(0, len(text), chunk_size)]


def _search_results_to_dicts(search_results: list[Any]) -> list[dict[str, Any]]:
    """Convert SearchResult objects to serializable dicts for RAG context."""
    return [
        {
            "title": r.title,
            "summary": r.summary,
            "body_markdown": r.body_markdown,
            "category": r.category,
            "source_title": r.source_title,
            "source_author": r.source_author,
            "problem_slug": r.problem_slug,
            "unit_type": r.unit_type,
            "tags": r.tags,
            "clips": [
                {
                    "title": c.get("title", ""),
                    "source_timestamp": c.get("source_timestamp", ""),
                }
                for c in (r.clips or [])
            ],
        }
        for r in search_results
    ]


def _emit_citation_events(
    search_results: list[Any],
    writer: StreamWriter,
) -> None:
    """Emit NDJSON citation events for knowledge search results."""
    for result in search_results:
        body = getattr(result, "body_markdown", "") or ""
        writer({
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
        })


def _get_conversation_text(history: list[dict[str, Any]], user_message: str) -> str:
    """Extract full conversation text for red flag scanning."""
    texts = []
    for msg in history[-MAX_CONTEXT_TURNS * 2 :]:
        content = msg.get("content", "")
        if content:
            texts.append(str(content))
    texts.append(user_message)
    return " ".join(texts)


def _determine_phase(extracted_symptoms: list[dict[str, Any]]) -> str:
    """Determine the next workflow phase from extracted symptom completeness.

    Requires at least one symptom with body_part AND at least one detail
    (symptom_type, duration, trigger, or severity) to advance.
    """
    for symptom in extracted_symptoms:
        body_part = symptom.get("body_part", "")
        if not body_part:
            continue
        has_detail = any(
            symptom.get(k) for k in ("symptom_type", "duration", "trigger", "severity")
        )
        if has_detail:
            return "ready_for_analysis"
    return "collecting"


# ---------------------------------------------------------------------------
# Graph nodes
# ---------------------------------------------------------------------------

async def safety_check(state: ConsultationState, *, writer: StreamWriter) -> dict[str, Any]:
    """Check for red flags. Citations are now emitted by the agent's search_knowledge tool calls."""
    # Note: citation events are no longer emitted here. They are emitted
    # dynamically when the LLM calls search_knowledge in generate_response.
    # Pre-fetched rag_results (from Go handler) are kept as fallback context
    # for the system prompt but do not generate citation events.

    # Check red flags
    detector = get_red_flag_detector()
    conversation_text = _get_conversation_text(
        state.get("conversation_history", []), state["user_message"]
    )
    extracted_symptoms = state.get("extracted_symptoms", [])
    red_flag_result = detector.detect(extracted_symptoms, conversation_text)

    if red_flag_result.has_red_flags:
        writer({"type": "red_flag", **red_flag_result.to_dict()})

    return {
        "red_flag_result": red_flag_result.to_dict() if red_flag_result.has_red_flags else None,
    }


async def classify_intent(state: ConsultationState) -> dict[str, Any]:
    """Classify user intent and decide the next action."""
    workflow = get_agent_workflow()

    context = ChatContext(
        session_id=state.get("session_id", ""),
        user_id=state.get("user_id", ""),
        profile=state.get("profile", {}),
        extracted_info=ExtractedInfo.from_dict(state.get("extracted_symptoms", [])),
        messages=state.get("conversation_history", []),
        phase=state.get("phase", "collecting"),
    )

    intent = workflow.classify_intent(state["user_message"], context)
    decision = workflow.decide_next_action(intent, context)

    return {
        "intent": intent.value,
        "workflow_action": decision.action.value,
    }


async def generate_response(
    state: ConsultationState, *, writer: StreamWriter
) -> dict[str, Any]:
    """Stream LLM response with tool calling, or fallback to local knowledge.

    Supports a multi-round tool loop: the LLM can call search_knowledge to
    retrieve information from the knowledge base, then continue generating
    with the results. Up to MAX_TOOL_ROUNDS iterations are allowed.
    """
    accumulated_text = ""
    new_symptoms: list[dict[str, Any]] = []

    try:
        ai = _get_ai_service()
    except Exception:
        # Fallback path: no LLM configured — search locally and build reply
        fallback_text = build_fallback_reply(state["user_message"], state.get("rag_results"))
        for chunk in _chunk_text(fallback_text):
            accumulated_text += chunk
            writer({"type": "text_delta", "delta": chunk})
        return {
            "accumulated_text": accumulated_text,
            "llm_available": False,
            "extracted_symptoms": [],
        }

    # Build tools from registry
    registry = get_consultation_registry()
    executor = get_consultation_executor()
    provider_tools = registry.to_provider_tools()

    # Multi-round tool loop
    messages = build_messages(state)
    seen_queries: set[str] = set()
    seen_body_parts: set[str] = set()  # for deduplicating extract_symptom_info

    # Text buffering to avoid per-token NDJSON events (each CJK token is 1-3 chars)
    _text_buf = ""
    _text_buf_size = 20  # flush ~20 chars at a time

    def _flush_text_buf() -> None:
        nonlocal _text_buf
        if _text_buf:
            writer({"type": "text_delta", "delta": _text_buf})
            _text_buf = ""

    for _round in range(MAX_TOOL_ROUNDS):
        # Collect completed tool calls from AiStreamEvent "tool_call_done" events
        completed_tool_calls: list[dict[str, Any]] = []

        async for event in ai.generate_stream(AiRequest(
            use_case="consultation.reply",
            messages=messages,
            tools=provider_tools,
            temperature=0.7,
            max_tokens=2048,
        )):
            if event.type == "text_delta" and event.text:
                accumulated_text += event.text
                _text_buf += event.text
                if len(_text_buf) >= _text_buf_size:
                    _flush_text_buf()
                await asyncio.sleep(0)

            elif event.type == "tool_call_done" and event.tool_name:
                _flush_text_buf()
                completed_tool_calls.append({
                    "id": event.tool_call_id or "",
                    "name": event.tool_name,
                    "arguments": event.tool_arguments or {},
                })

        # Flush remaining text at end of each round
        _flush_text_buf()

        # If no tool calls, we're done
        if not completed_tool_calls:
            break

        # Process tool calls and build tool results for next round
        tool_messages: list[ChatMessage] = []
        has_search = False

        for tc in completed_tool_calls:
            tc_id = tc["id"]
            tc_name = tc["name"]
            tc_args = tc["arguments"]

            # Emit generic tool_call event for protocol compliance
            writer({
                "type": "tool_call",
                "id": tc_id,
                "tool": tc_name,
                "args": tc_args,
            })

            if tc_name == "extract_symptom_info":
                body_part = tc_args.get("body_part", "")
                # Deduplicate: same body_part in this response only emits once
                if body_part and body_part not in seen_body_parts:
                    seen_body_parts.add(body_part)
                    # Execute via handler for validation/normalization
                    tool_result = await executor.execute(tc_id, tc_name, tc_args)
                    if tool_result.status.value == "success":
                        normalized = tool_result.content or tc_args
                        new_symptoms.append(normalized)
                        writer({"type": "extracted_info", "info": normalized})
                    else:
                        new_symptoms.append(tc_args)
                        writer({"type": "extracted_info", "info": tc_args})
                writer({
                    "type": "tool_result",
                    "id": tc_id,
                    "tool": tc_name,
                    "result": {"status": "ok"},
                })
                tool_messages.append(ChatMessage(
                    role="tool",
                    content="症状信息已提取。",
                    tool_call_id=tc_id,
                ))

            elif tc_name == "search_knowledge":
                query = tc_args.get("query", "")
                # Deduplicate identical queries
                if query in seen_queries:
                    tool_messages.append(ChatMessage(
                        role="tool",
                        content="之前已搜索过相同内容，请直接使用已有结果回答。",
                        tool_call_id=tc_id,
                    ))
                    continue
                seen_queries.add(query)

                # Execute via handler
                tool_result = await executor.execute(tc_id, tc_name, tc_args)
                has_search = True

                if tool_result.status.value == "success":
                    content = tool_result.content or {}
                    result_text = content.get("result_text", "")
                    has_results = content.get("has_results", False)
                    raw_results = content.get("raw_results", [])

                    if has_results:
                        _emit_citation_events(raw_results, writer)
                    else:
                        writer({
                            "type": "knowledge_gap",
                            "query": query,
                            "message": "知识库中暂未找到相关专项资料，以下为通用建议。",
                        })
                else:
                    result_text = tool_result.error or "搜索失败"
                    has_results = False

                writer({
                    "type": "tool_result",
                    "id": tc_id,
                    "tool": tc_name,
                    "result": {"has_results": has_results},
                })
                tool_messages.append(ChatMessage(
                    role="tool",
                    content=result_text,
                    tool_call_id=tc_id,
                ))

        # If we had tool calls, append assistant message with tool calls
        # and tool results to messages for the next round
        if tool_messages:
            assistant_tool_calls = [
                ToolCall(id=tc["id"], name=tc["name"], arguments=tc["arguments"])
                for tc in completed_tool_calls
            ]
            assistant_msg = ChatMessage(role="assistant", content="")
            assistant_msg.tool_calls = assistant_tool_calls
            messages.append(assistant_msg)
            messages.extend(tool_messages)

        # Continue the loop if:
        # - search_knowledge was called (LLM needs to use the RAG results), OR
        # - no text was generated yet (LLM needs another round to produce a reply)
        if has_search:
            continue
        if not accumulated_text:
            # LLM only called tools (e.g. extract_symptom_info) without generating
            # any text. Give it another round to produce a user-facing reply.
            continue
        break

    return {
        "accumulated_text": accumulated_text,
        "extracted_symptoms": new_symptoms,
        "llm_available": True,
    }



async def decide_phase(state: ConsultationState, *, writer: StreamWriter) -> dict[str, Any]:
    """Determine the current phase and emit phase_change event."""
    extracted_symptoms = state.get("extracted_symptoms", [])
    new_phase = _determine_phase(extracted_symptoms)
    current_phase = state.get("phase", "collecting")

    if new_phase != current_phase:
        reason = (
            "symptom details collected"
            if new_phase == "ready_for_analysis"
            else "phase updated"
        )
        writer({"type": "phase_change", "phase": new_phase, "reason": reason})

    return {"phase": new_phase}


async def emit_done(state: ConsultationState, *, writer: StreamWriter) -> dict[str, Any]:
    """Terminal node: emit the __done__ event with final state snapshot."""
    writer({
        "type": "__done__",
        "session_id": state.get("session_id", ""),
        "full_text": state.get("accumulated_text", ""),
        "extracted_info": state.get("extracted_symptoms", []),
        "phase": state.get("phase", "collecting"),
    })
    return {}


async def generate_diagnosis(state: ConsultationState, *, writer: StreamWriter) -> dict[str, Any]:
    """Generate diagnosis candidates when intent requests analysis.

    Searches the knowledge base for relevant information before generating
    the diagnosis, ensuring citations and faithfulness checks work correctly.
    """
    from .diagnosis_service import get_diagnosis_service

    service = get_diagnosis_service()
    extracted_info = state.get("extracted_symptoms", [])
    profile = state.get("profile", {})
    history = state.get("conversation_history", [])
    conversation_summary = " ".join(
        m.get("content", "") for m in history[-6:] if m.get("content")
    )

    # Search knowledge base for diagnosis-relevant information
    rag_results: list[dict[str, Any]] = []
    if extracted_info:
        # Build search query from extracted symptoms
        parts = [s.get("body_part", "") for s in extracted_info if s.get("body_part")]
        symptoms = [s.get("symptom_type", "") for s in extracted_info if s.get("symptom_type")]
        query = " ".join(parts + symptoms + ["诊断", "自测"])
        try:
            from ..rag.knowledge_library import get_knowledge_library
            library = get_knowledge_library()
            search_results = await library.search(query=query, top_k=5)
            _emit_citation_events(search_results, writer)
            rag_results = _search_results_to_dicts(search_results)
        except Exception:
            logger.warning("RAG search failed during diagnosis generation", exc_info=True)

    result = await service.generate_diagnosis(
        extracted_info=extracted_info,
        profile=profile,
        conversation_summary=conversation_summary,
        rag_results=rag_results,
    )

    writer({"type": "diagnosis_result", "diagnoses": result.get("diagnoses", [])})
    writer({"type": "phase_change", "phase": "analysis_ready", "reason": "diagnosis generated"})

    return {"diagnosis_result": result, "phase": "analysis_ready"}


async def generate_treatment(state: ConsultationState, *, writer: StreamWriter) -> dict[str, Any]:
    """Generate treatment plan when diagnosis is confirmed.

    Searches the knowledge base for exercise and treatment information
    before generating the plan, ensuring faithfulness checks work.
    """
    from .diagnosis_service import get_diagnosis_service

    service = get_diagnosis_service()
    extracted_info = state.get("extracted_symptoms", [])
    profile = state.get("profile", {})
    diagnosis_result = state.get("diagnosis_result", {})

    # Search knowledge base for treatment-relevant information
    rag_results: list[dict[str, Any]] = []
    diagnosis_name = ""
    if isinstance(diagnosis_result, dict):
        diagnoses = diagnosis_result.get("diagnoses", [])
        if diagnoses:
            diagnosis_name = diagnoses[0].get("name", "") if isinstance(diagnoses[0], dict) else ""

    query_parts = [p for p in [diagnosis_name] if p]
    parts = [s.get("body_part", "") for s in extracted_info if s.get("body_part")]
    query_parts.extend(parts)
    query_parts.append("改善 训练 动作")

    if query_parts:
        query = " ".join(query_parts)
        try:
            from ..rag.knowledge_library import get_knowledge_library
            library = get_knowledge_library()
            search_results = await library.search(query=query, top_k=5)
            _emit_citation_events(search_results, writer)
            rag_results = _search_results_to_dicts(search_results)
        except Exception:
            logger.warning("RAG search failed during treatment generation", exc_info=True)

    result = await service.generate_treatment(
        confirmed_diagnosis=diagnosis_result,
        extracted_info=extracted_info,
        profile=profile,
        rag_results=rag_results,
    )

    writer({"type": "treatment_result", "treatment_plan": result.get("treatment_plan", {})})
    writer({"type": "phase_change", "phase": "plan_ready", "reason": "treatment plan generated"})

    return {"treatment_result": result, "phase": "plan_ready"}


# ---------------------------------------------------------------------------
# Routing
# ---------------------------------------------------------------------------

def route_on_action(state: ConsultationState) -> str:
    """Route to the next node based on the workflow action."""
    action = state.get("workflow_action", "")
    if action == "generate_diagnosis":
        return "generate_diagnosis"
    if action == "generate_treatment":
        return "generate_treatment"
    return "emit_done"


# ---------------------------------------------------------------------------
# Graph builder
# ---------------------------------------------------------------------------

def build_consultation_graph() -> Any:
    """Build and compile the consultation agent state graph."""
    graph = StateGraph(ConsultationState)

    # Add nodes
    graph.add_node("safety_check", safety_check)
    graph.add_node("classify_intent", classify_intent)
    graph.add_node("generate_response", generate_response)
    graph.add_node("decide_phase", decide_phase)
    graph.add_node("generate_diagnosis", generate_diagnosis)
    graph.add_node("generate_treatment", generate_treatment)
    graph.add_node("emit_done", emit_done)

    # Linear flow
    graph.add_edge(START, "safety_check")
    graph.add_edge("safety_check", "classify_intent")
    graph.add_edge("classify_intent", "generate_response")
    graph.add_edge("generate_response", "decide_phase")

    # Conditional routing after phase decision
    graph.add_conditional_edges("decide_phase", route_on_action)

    # All paths converge to emit_done, then END
    graph.add_edge("generate_diagnosis", "emit_done")
    graph.add_edge("generate_treatment", "emit_done")
    graph.add_edge("emit_done", END)

    return graph.compile()


# ---------------------------------------------------------------------------
# Singleton
# ---------------------------------------------------------------------------

_compiled_graph = None


def get_consultation_graph():
    """Get or create the compiled consultation graph."""
    global _compiled_graph
    if _compiled_graph is None:
        _compiled_graph = build_consultation_graph()
    return _compiled_graph


async def stream_consultation(
    state: ConsultationState,
) -> AsyncIterator[dict[str, Any]]:
    """Stream consultation events from the graph.

    Yields dicts that represent NDJSON event payloads.
    The caller serializes each as a single JSON line.
    """
    # Validate required fields
    for field in ("session_id", "user_id", "user_message"):
        if not state.get(field):
            raise ValueError(f"Missing required field in consultation state: {field}")

    graph = get_consultation_graph()

    async for chunk in graph.astream(state, stream_mode="custom"):
        yield chunk
