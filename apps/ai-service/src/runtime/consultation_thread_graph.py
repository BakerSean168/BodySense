"""Routing, phase transition, and terminal nodes for the consultation graph."""

from __future__ import annotations

from typing import Any, Literal

from langgraph.graph import END, START, StateGraph
from langgraph.types import StreamWriter

from .consultation_thread_intake import (
    acquire_turn_state,
    confirm_suggestion,
    enforce_state_acquisition,
    propose_suggestion,
)
from .consultation_thread_safety import safety_check
from .consultation_thread_state import ConsultationThreadState, prepare_turn
from .consultation_thread_tools import execute_tool


def _determine_phase(extracted_symptoms: list[dict[str, Any]]) -> str:
    for symptom in extracted_symptoms:
        body_part = symptom.get("body_part", "")
        if not body_part:
            continue
        if not symptom.get("symptom_type"):
            continue
        detail_count = sum(
            bool(symptom.get(key))
            for key in (
                "duration",
                "trigger",
                "severity",
                "radiation",
                "functional_impact",
                "neurological_signs",
            )
        )
        if detail_count >= 2:
            return "ready_for_analysis"
    return "collecting"


def route_after_model(state: ConsultationThreadState) -> Literal["execute_tool", "decide_phase"]:
    if state.get("pending_tool_calls"):
        return "execute_tool"
    return "decide_phase"


def route_after_tool(state: ConsultationThreadState) -> Literal["execute_tool", "llm_turn"]:
    if state.get("pending_tool_calls"):
        return "execute_tool"
    return "llm_turn"


async def decide_phase(state: ConsultationThreadState, *, writer: StreamWriter) -> dict[str, Any]:
    extracted_symptoms = state.get("extracted_symptoms", [])
    new_phase = _determine_phase(extracted_symptoms)
    current_phase = state.get("phase", "collecting")
    if new_phase != current_phase:
        reason = (
            "symptom details collected" if new_phase == "ready_for_analysis" else "phase updated"
        )
        writer({"type": "phase_change", "phase": new_phase, "reason": reason})
    return {"phase": new_phase}


async def emit_done(state: ConsultationThreadState, *, writer: StreamWriter) -> dict[str, Any]:
    writer(
        {
            "type": "__done__",
            "session_id": state.get("session_id", ""),
            "full_text": state.get("accumulated_text", ""),
            "extracted_info": state.get("extracted_symptoms", []),
            "phase": state.get("phase", "collecting"),
        }
    )
    return {}


def _build_graph(checkpointer: Any):
    # Imported here: the engine module owns the model node and imports this one.
    from .consultation_thread import llm_turn

    graph = StateGraph(ConsultationThreadState)
    graph.add_node("prepare_turn", prepare_turn)
    graph.add_node("acquire_turn_state", acquire_turn_state)
    graph.add_node("safety_check", safety_check)
    graph.add_node("enforce_state_acquisition", enforce_state_acquisition)
    graph.add_node("propose_suggestion", propose_suggestion)
    graph.add_node("confirm_suggestion", confirm_suggestion)
    graph.add_node("llm_turn", llm_turn)
    graph.add_node("execute_tool", execute_tool)
    graph.add_node("decide_phase", decide_phase)
    graph.add_node("emit_done", emit_done)

    graph.add_edge(START, "prepare_turn")
    graph.add_edge("prepare_turn", "acquire_turn_state")
    graph.add_edge("acquire_turn_state", "safety_check")
    graph.add_edge("safety_check", "enforce_state_acquisition")
    graph.add_edge("enforce_state_acquisition", "propose_suggestion")
    graph.add_edge("propose_suggestion", "confirm_suggestion")
    graph.add_edge("confirm_suggestion", "llm_turn")
    graph.add_conditional_edges("llm_turn", route_after_model)
    graph.add_conditional_edges("execute_tool", route_after_tool)
    graph.add_edge("decide_phase", "emit_done")
    graph.add_edge("emit_done", END)

    return graph.compile(checkpointer=checkpointer)
