"""Explicit agent workflow for consultation sessions."""

import re
from dataclasses import dataclass
from enum import Enum

from ..models.consultation import ChatContext, ExtractedInfo


class ConsultationIntent(str, Enum):
    """Classified user intent."""

    SUPPLEMENT_SYMPTOM = "supplement_symptom"
    REQUEST_ANALYSIS = "request_analysis"
    CONFIRM_DIAGNOSIS = "confirm_diagnosis"
    REQUEST_TREATMENT = "request_treatment"
    GENERAL_QUESTION = "general_question"
    CLARIFICATION = "clarification"


class AgentAction(str, Enum):
    """Actions the agent can take."""

    ASK_FOLLOW_UP = "ask_follow_up"
    GENERATE_DIAGNOSIS = "generate_diagnosis"
    GENERATE_TREATMENT = "generate_treatment"
    PROVIDE_INFO = "provide_info"
    ACKNOWLEDGE = "acknowledge"


@dataclass
class WorkflowDecision:
    """Decision from the workflow engine."""

    intent: ConsultationIntent
    action: AgentAction
    confidence: float
    reasoning: str


# Intent classification patterns
INTENT_PATTERNS: list[tuple[str, ConsultationIntent, float]] = [
    # Request analysis
    (r"(分析|诊断|判断|评估|看看|怎么回事)", ConsultationIntent.REQUEST_ANALYSIS, 0.8),
    (r"(帮我看看|什么问题|什么原因|什么情况)", ConsultationIntent.REQUEST_ANALYSIS, 0.7),
    (r"(检查|测试|自测|自查)", ConsultationIntent.REQUEST_ANALYSIS, 0.7),
    # Confirm diagnosis
    (r"(确认|同意|是的|对|没错|就是这个)", ConsultationIntent.CONFIRM_DIAGNOSIS, 0.8),
    (r"(第[一二三]个|选[一二三])", ConsultationIntent.CONFIRM_DIAGNOSIS, 0.7),
    # Request treatment
    (r"(方案|改善|治疗|训练|怎么处理|怎么办)", ConsultationIntent.REQUEST_TREATMENT, 0.8),
    (r"(缓解|纠正|矫正|康复)", ConsultationIntent.REQUEST_TREATMENT, 0.7),
    # Clarification
    (r"(什么意思|不太明白|能解释|详细说)", ConsultationIntent.CLARIFICATION, 0.7),
]

# Keywords indicating symptom information
SYMPTOM_INDICATORS = [
    "疼", "痛", "酸", "胀", "麻", "木", "僵", "紧",
    "不适", "难受", "不舒服", "别扭",
    "久坐", "运动", "晨起", "夜间",
]


class ConsultationAgentWorkflow:
    """Explicit agent workflow for consultation sessions."""

    def classify_intent(
        self,
        user_message: str,
        context: ChatContext,
    ) -> ConsultationIntent:
        """
        Classify user intent from message and context.

        Args:
            user_message: The user's message text.
            context: Current chat context with history and extracted info.

        Returns:
            Classified ConsultationIntent.
        """
        message = user_message.strip()

        # Check if message contains symptom information
        has_symptom_info = any(kw in message for kw in SYMPTOM_INDICATORS)

        # Check patterns
        best_intent = ConsultationIntent.GENERAL_QUESTION
        best_confidence = 0.3

        for pattern, intent, confidence in INTENT_PATTERNS:
            if re.search(pattern, message):
                if confidence > best_confidence:
                    best_confidence = confidence
                    best_intent = intent

        # If has symptom info and no strong pattern match, treat as supplement
        if has_symptom_info and best_confidence < 0.7:
            return ConsultationIntent.SUPPLEMENT_SYMPTOM

        # Context-based refinement
        phase = getattr(context, "phase", "collecting")
        if phase == "ready_for_analysis" and best_intent == ConsultationIntent.GENERAL_QUESTION:
            # After diagnosis is ready, general questions might be confirmation
            if any(kw in message for kw in ["好", "行", "可以", "嗯"]):
                return ConsultationIntent.CONFIRM_DIAGNOSIS

        return best_intent

    def merge_extracted_info(
        self,
        existing: ExtractedInfo,
        new_info: dict,
    ) -> bool:
        """
        Merge new symptom info with existing, avoiding duplicates.

        Delegates to ExtractedInfo.add_symptom() for the actual merge logic.

        Args:
            existing: Current extracted info.
            new_info: New symptom info dict.

        Returns:
            True if info was added/updated, False if no change.
        """
        body_part = new_info.get("body_part", "")
        if not body_part:
            return False

        # Snapshot existing state for this body part
        existing_symptom = None
        for symptom in existing.symptoms:
            if symptom.body_part == body_part:
                existing_symptom = {k: v for k, v in vars(symptom).items() if v is not None}
                break

        # Delegate to ExtractedInfo.add_symptom()
        existing.add_symptom(new_info)

        # Check if this is a new symptom
        if existing_symptom is None:
            return True

        # Check if any field was updated
        for symptom in existing.symptoms:
            if symptom.body_part == body_part:
                current = {k: v for k, v in vars(symptom).items() if v is not None}
                return current != existing_symptom
        return False

    def should_analyze(self, extracted_info: ExtractedInfo) -> bool:
        """
        Determine if enough info has been collected for diagnosis.

        Args:
            extracted_info: Current extracted info.

        Returns:
            True if ready for analysis.
        """
        if not extracted_info.symptoms:
            return False

        # At least one symptom with body_part and one detail
        for symptom in extracted_info.symptoms:
            if not symptom.body_part:
                continue
            has_detail = any(
                [
                    symptom.symptom_type,
                    symptom.duration,
                    symptom.trigger,
                    symptom.severity,
                ]
            )
            if has_detail:
                return True

        return False

    def should_generate_treatment(
        self,
        phase: str,
        diagnosis_confirmed: bool,
    ) -> bool:
        """
        Determine if treatment generation should proceed.

        Args:
            phase: Current consultation phase.
            diagnosis_confirmed: Whether user has confirmed a diagnosis.

        Returns:
            True if treatment generation should proceed.
        """
        return phase == "diagnosis_confirmed" and diagnosis_confirmed

    def decide_next_action(
        self,
        intent: ConsultationIntent,
        context: ChatContext,
    ) -> WorkflowDecision:
        """
        Decide the next action based on intent and context.

        Args:
            intent: Classified user intent.
            context: Current chat context.

        Returns:
            WorkflowDecision with action and reasoning.
        """
        phase = "collecting"
        if hasattr(context, "phase"):
            phase = context.phase

        if intent == ConsultationIntent.SUPPLEMENT_SYMPTOM:
            return WorkflowDecision(
                intent=intent,
                action=AgentAction.ASK_FOLLOW_UP,
                confidence=0.9,
                reasoning="User provided symptom info, continue collecting details.",
            )

        if intent == ConsultationIntent.REQUEST_ANALYSIS:
            if self.should_analyze(context.extracted_info):
                return WorkflowDecision(
                    intent=intent,
                    action=AgentAction.GENERATE_DIAGNOSIS,
                    confidence=0.8,
                    reasoning="Enough symptom info collected, ready for diagnosis.",
                )
            return WorkflowDecision(
                intent=intent,
                action=AgentAction.ASK_FOLLOW_UP,
                confidence=0.7,
                reasoning="Not enough symptom info yet, need more details.",
            )

        if intent == ConsultationIntent.CONFIRM_DIAGNOSIS:
            return WorkflowDecision(
                intent=intent,
                action=AgentAction.GENERATE_TREATMENT,
                confidence=0.8,
                reasoning="User confirmed diagnosis, proceed to treatment.",
            )

        if intent == ConsultationIntent.REQUEST_TREATMENT:
            if phase in ("diagnosis_confirmed", "ready_for_analysis"):
                return WorkflowDecision(
                    intent=intent,
                    action=AgentAction.GENERATE_TREATMENT,
                    confidence=0.8,
                    reasoning="Diagnosis confirmed, ready for treatment generation.",
                )
            return WorkflowDecision(
                intent=intent,
                action=AgentAction.ASK_FOLLOW_UP,
                confidence=0.6,
                reasoning="Need diagnosis before treatment.",
            )

        if intent == ConsultationIntent.CLARIFICATION:
            return WorkflowDecision(
                intent=intent,
                action=AgentAction.PROVIDE_INFO,
                confidence=0.8,
                reasoning="User requested clarification.",
            )

        return WorkflowDecision(
            intent=intent,
            action=AgentAction.ASK_FOLLOW_UP,
            confidence=0.5,
            reasoning="General question, continue conversation.",
        )


# Singleton
_workflow: ConsultationAgentWorkflow | None = None


def get_agent_workflow() -> ConsultationAgentWorkflow:
    """Get or create the default agent workflow."""
    global _workflow
    if _workflow is None:
        _workflow = ConsultationAgentWorkflow()
    return _workflow
