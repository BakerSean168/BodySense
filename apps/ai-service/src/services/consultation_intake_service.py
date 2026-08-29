"""Application service for typed Consultation state acquisition."""

from __future__ import annotations

import logging
import re
from collections.abc import Callable

from pydantic_ai.models import Model

from ..agents.consultation_intake_agent import create_consultation_intake_agent
from ..ai.consultation_intake_gateway_model import (
    consultation_intake_model_settings,
    get_consultation_intake_runtime_model,
)
from ..configuration.consultation_agent_config import ConsultationAgentManifest
from ..models.consultation_intake import (
    ConsultationIntakeDependencies,
    ConsultationIntakeOutput,
    ConsultationSymptomDraft,
)

logger = logging.getLogger(__name__)

ModelResolver = Callable[[ConsultationAgentManifest], Model]

_BODY_PART_PATTERNS: tuple[tuple[str, str], ...] = (
    (r"右侧?臀|右臀", "右臀"),
    (r"左侧?臀|左臀", "左臀"),
    (r"臀部|屁股", "臀部"),
    (r"下背|腰部|腰", "腰部"),
    (r"颈部|脖子", "颈部"),
    (r"肩部|肩膀|肩", "肩部"),
    (r"大腿", "大腿"),
    (r"小腿", "小腿"),
    (r"膝盖|膝", "膝部"),
    (r"脚踝|踝", "踝部"),
    (r"手臂|上臂|前臂", "手臂"),
    (r"手腕|腕", "腕部"),
    (r"头部|头", "头部"),
)
_SYMPTOM_PATTERNS: tuple[tuple[str, str], ...] = (
    (r"疼痛|痛|疼", "疼痛"),
    (r"麻木|发麻|麻", "麻木"),
    (r"刺痛|针刺", "刺痛"),
    (r"酸胀|酸痛|发酸", "酸痛/酸胀"),
    (r"僵硬|僵", "僵硬"),
    (r"无力|乏力", "无力"),
    (r"头晕|眩晕", "头晕"),
)
_DURATION_RE = re.compile(
    r"(?:持续|已经|有|大概|差不多)?\s*(\d+(?:\.\d+)?\s*(?:天|周|个月|月|年|小时))"
)
_SEVERITY_RE = re.compile(r"(?:疼痛|严重|程度)?\s*(\d{1,2})\s*(?:/|分之)\s*10")
_FIRST_PERSON_RE = re.compile(r"我|我的|本人|最近|现在|这几天|一直|感觉|出现|开始")
_GENERAL_QUESTION_RE = re.compile(r"是什么|什么意思|为什么|怎么回事|如何判断|会不会|能不能|是否会")


class ConsultationIntakeService:
    def __init__(self, *, model_resolver: ModelResolver | None = None) -> None:
        self._model_resolver = model_resolver or get_consultation_intake_runtime_model

    async def assess(
        self,
        *,
        latest_user_message: str,
        profile: dict[str, object],
        body_state: dict[str, object],
        config: ConsultationAgentManifest,
    ) -> ConsultationIntakeOutput:
        intake = config.intake
        if intake is None or not latest_user_message.strip():
            return ConsultationIntakeOutput(turn_kind="other")

        agent = create_consultation_intake_agent(
            prompt_revision=intake.prompt_revision,
            output_schema_revision=intake.output_schema_revision,
            policy_revision=intake.policy_revision,
        )
        deps = ConsultationIntakeDependencies(
            latest_user_message=latest_user_message.strip(),
            profile=dict(profile),
            body_state=dict(body_state),
        )
        try:
            result = await agent.run(
                latest_user_message.strip(),
                deps=deps,
                model=self._model_resolver(config),
                model_settings=consultation_intake_model_settings(config),
            )
            return result.output
        except Exception:
            logger.exception("Consultation intake model failed; using conservative fallback")
            # State acquisition must degrade conservatively. The fallback only
            # recognizes explicit first-person symptom language and never turns
            # a generic health question into durable user state.
            return deterministic_intake_fallback(latest_user_message)


def deterministic_intake_fallback(message: str) -> ConsultationIntakeOutput:
    text = message.strip()
    if not text:
        return ConsultationIntakeOutput(turn_kind="other")
    if _GENERAL_QUESTION_RE.search(text) and not _FIRST_PERSON_RE.search(text):
        return ConsultationIntakeOutput(turn_kind="general_question")
    if not _FIRST_PERSON_RE.search(text):
        return ConsultationIntakeOutput(turn_kind="other")

    body_part = ""
    for pattern, normalized in _BODY_PART_PATTERNS:
        if re.search(pattern, text):
            body_part = normalized
            break
    symptom_type = ""
    for pattern, normalized in _SYMPTOM_PATTERNS:
        if re.search(pattern, text):
            symptom_type = normalized
            break
    if not body_part or not symptom_type:
        return ConsultationIntakeOutput(turn_kind="other")

    duration_match = _DURATION_RE.search(text)
    severity_match = _SEVERITY_RE.search(text)
    radiation = ""
    radiation_match = re.search(r"(?:放射|串|延伸|牵扯)到?([^，。；！？]{1,20})", text)
    if radiation_match:
        radiation = radiation_match.group(1).strip()

    symptom = ConsultationSymptomDraft(
        body_part=body_part,
        symptom_type=symptom_type,
        duration=duration_match.group(1).replace(" ", "") if duration_match else "",
        severity=f"{severity_match.group(1)}/10" if severity_match else "",
        radiation=radiation,
        trigger="久坐" if "久坐" in text else "",
        neurological_signs=(
            "无力" if "无力" in text else "麻木" if re.search(r"麻木|发麻", text) else ""
        ),
    )
    return ConsultationIntakeOutput(
        turn_kind="symptom_report",
        symptoms=[symptom],
        rationale="deterministic explicit first-person fallback",
    )
