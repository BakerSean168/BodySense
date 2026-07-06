"""LangGraph-based consultation agent workflow.

Converts the linear ChatService flow into an explicit state graph with:
- safety_check: red flag detection + citation emission
- classify_intent: rule-based intent classification
- generate_response: LLM streaming with tool calling (or fallback), including symptom extraction
- decide_phase: phase determination and NDJSON event emission
- generate_diagnosis / generate_treatment: optional downstream actions
"""

import logging
from collections.abc import AsyncIterator
from typing import Annotated, Any, TypedDict

from langgraph.graph import END, START, StateGraph
from langgraph.types import StreamWriter

from ..ai import AIService
from ..ai.types import ChatMessage
from ..models.consultation import ChatContext, ExtractedInfo
from ..prompts.consultation import format_profile_context, get_system_prompt
from .agent.consultation_tools import get_consultation_executor, get_consultation_registry
from .agent.orchestrator import (
    AgentOrchestrator,
    emit_citation_events,
)
from .agent.orchestrator import (
    build_fallback_reply as build_fallback_reply,
)
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

    # --- Interruption state ---
    interrupted: bool


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


_emit_citation_events = emit_citation_events


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
    decision = workflow.decide_next_action(intent, context, state["user_message"])

    return {
        "intent": intent.value,
        "workflow_action": decision.action.value,
    }


async def generate_response(
    state: ConsultationState, *, writer: StreamWriter
) -> dict[str, Any]:
    """Stream LLM response through the agent orchestration Module."""
    orchestrator = AgentOrchestrator(
        ai_provider=_get_ai_service,
        registry=get_consultation_registry(),
        executor=get_consultation_executor(),
    )
    return await orchestrator.stream_turn(
        {
            "user_message": state["user_message"],
            "rag_results": state.get("rag_results", []),
            "messages": build_messages(state),
        },
        writer=writer,
    )



async def decide_phase(state: ConsultationState, *, writer: StreamWriter) -> dict[str, Any]:
    """Determine the current phase and emit phase_change event."""
    if state.get("interrupted"):
        return {}

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
    if state.get("interrupted"):
        # Don't emit __done__ for interrupted runs — the stream ends here
        return {}

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
