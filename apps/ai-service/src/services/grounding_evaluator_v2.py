"""Eval-only Treatment grounding evaluator v2.

This module deliberately does not replace production faithfulness governance.
It evaluates material intervention claims against explicitly cited, admissible
retrieved evidence and returns explainable reason codes. Deterministic
provenance checks always run before semantic support; an optional judge may
only resolve genuinely uncertain semantic cases.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from typing import Any, Callable, Literal, Mapping, Sequence

from .faithfulness_checker import EXERCISE_ALIASES

GroundingSupport = Literal[
    "supported",
    "partial",
    "unsupported",
    "contraindicated",
    "uncertain",
]
GroundingJudge = Callable[["InterventionClaim", Sequence["AdmissibleEvidence"]], GroundingSupport]


@dataclass(frozen=True, slots=True)
class InterventionClaim:
    """Material semantic unit evaluated against Treatment evidence."""

    kind: str
    title: str
    intended_goal: str
    dosage: Mapping[str, Any] = field(default_factory=dict)
    progression: str | list[str] | None = None
    stop_conditions: tuple[str, ...] = ()
    supporting_evidence_ids: tuple[str, ...] = ()


@dataclass(frozen=True, slots=True)
class AdmissibleEvidence:
    evidence_id: str
    title: str
    text: str
    admissible: bool = True
    source_type: str = "knowledge_unit"


@dataclass(frozen=True, slots=True)
class ClaimGroundingResult:
    claim: InterventionClaim
    support: GroundingSupport
    evidence_ids: tuple[str, ...]
    reasons: tuple[str, ...]
    judge_used: bool = False


@dataclass(frozen=True, slots=True)
class GroundingEvaluationV2:
    verdict: Literal["supported", "degraded", "rejected"]
    claims: tuple[ClaimGroundingResult, ...]
    evaluator_revision: str = "treatment-grounding-v2"

    def to_dict(self) -> dict[str, Any]:
        return {
            "evaluator_revision": self.evaluator_revision,
            "verdict": self.verdict,
            "claims": [
                {
                    "intervention": result.claim.title,
                    "kind": result.claim.kind,
                    "support": result.support,
                    "evidence_ids": list(result.evidence_ids),
                    "reasons": list(result.reasons),
                    "judge_used": result.judge_used,
                }
                for result in self.claims
            ],
        }


_MATERIAL_DOSAGE_KEYS = ("sets", "reps", "duration", "frequency")
_CONTRAINDICATION_MARKERS = (
    "禁忌",
    "不建议进行",
    "不应进行",
    "避免进行",
    "不要做",
    "禁止进行",
    "contraindicated",
)
_NON_SUPPORT_MARKERS = (
    "不支持将",
    "不能据此推荐",
    "并不支持",
    "not support",
    "does not support",
)
_PROGRESSION_MARKERS = ("逐步", "逐渐", "循序渐进", "进阶", "增加", "progress")
_STOP_MARKERS = ("停止", "停下", "加重", "疼痛", "麻木", "不适", "stop")


def _normalize(value: Any) -> str:
    return re.sub(r"\s+", "", str(value or "").strip().lower())


def _evidence_text(raw: Mapping[str, Any]) -> str:
    return "\n".join(
        str(raw.get(key) or "")
        for key in ("title", "summary", "body_markdown", "content", "excerpt")
    )


def _evidence_record(raw: Mapping[str, Any]) -> AdmissibleEvidence:
    return AdmissibleEvidence(
        evidence_id=str(raw.get("evidence_id") or ""),
        title=str(raw.get("title") or ""),
        text=_evidence_text(raw),
        admissible=raw.get("admissible", True) is not False,
        source_type=str(raw.get("source_type") or "knowledge_unit"),
    )


def _claim_terms(title: str) -> set[str]:
    normalized = _normalize(title)
    if not normalized:
        return set()
    terms = {normalized}
    for canonical, aliases in EXERCISE_ALIASES.items():
        family = {_normalize(canonical), *(_normalize(alias) for alias in aliases)}
        if normalized in family:
            terms.update(term for term in family if len(term) >= 2)
    return {term for term in terms if len(term) >= 2}


def _title_supports(claim: InterventionClaim, evidence: AdmissibleEvidence) -> bool:
    text = _normalize(evidence.text)
    return any(term in text for term in _claim_terms(claim.title))


def _value_supported(key: str, value: Any, text: str) -> bool:
    """Require material prescription values, not only the exercise name."""
    normalized_text = _normalize(text)
    if value in (None, "", [], {}):
        return True
    if isinstance(value, (list, tuple)):
        return all(_value_supported(key, item, text) for item in value)

    normalized_value = _normalize(value)
    if normalized_value and normalized_value in normalized_text:
        return True

    # Numeric Treatment fields often serialize as integers while evidence says
    # "2组" / "8次". Tie each number to its dosage unit to avoid accepting an
    # unrelated number elsewhere in the evidence.
    if isinstance(value, (int, float)) or normalized_value.replace(".", "", 1).isdigit():
        units = {
            "sets": ("组", "set", "sets"),
            "reps": ("次", "rep", "reps"),
            "duration": ("秒", "分钟", "min", "minute", "second"),
        }.get(key, ())
        return any(f"{normalized_value}{_normalize(unit)}" in normalized_text for unit in units)
    return False


def _material_phrase_supported(
    value: str | list[str] | None, text: str, markers: Sequence[str]
) -> bool:
    if value in (None, "", []):
        return True
    values = value if isinstance(value, list) else [value]
    normalized_text = _normalize(text)
    for item in values:
        normalized_item = _normalize(item)
        if normalized_item and normalized_item in normalized_text:
            continue
        # For safety/progression prose, require at least two meaningful
        # concepts to be present. This is deterministic and intentionally
        # conservative: semantic uncertainty is left for the optional judge.
        hits = sum(
            1
            for marker in markers
            if _normalize(marker) in normalized_text and _normalize(marker) in normalized_item
        )
        if hits < 2:
            return False
    return True


def _build_claims(treatment: Mapping[str, Any]) -> list[InterventionClaim]:
    default_evidence_ids = tuple(
        str(value) for value in treatment.get("evidence_ids", []) if str(value).strip()
    )
    claims: list[InterventionClaim] = []
    for raw in treatment.get("interventions", []):
        if not isinstance(raw, Mapping):
            continue
        prescription = raw.get("prescription")
        if not isinstance(prescription, Mapping):
            prescription = {}
        evidence_ids = raw.get("evidence_ids", default_evidence_ids)
        if not isinstance(evidence_ids, list | tuple):
            evidence_ids = default_evidence_ids
        stop_conditions = prescription.get("stop_conditions", [])
        if isinstance(stop_conditions, str):
            stop_conditions = [stop_conditions]
        elif not isinstance(stop_conditions, list):
            stop_conditions = []
        dosage = {key: prescription[key] for key in _MATERIAL_DOSAGE_KEYS if key in prescription}
        claims.append(
            InterventionClaim(
                kind=str(raw.get("kind") or ""),
                title=str(raw.get("title") or "").strip(),
                intended_goal=str(raw.get("goal") or treatment.get("goal") or "").strip(),
                dosage=dosage,
                progression=prescription.get("progression"),
                stop_conditions=tuple(str(item) for item in stop_conditions if str(item).strip()),
                supporting_evidence_ids=tuple(
                    str(item) for item in evidence_ids if str(item).strip()
                ),
            )
        )
    return claims


class GroundingEvaluatorV2:
    """Layered eval-only evaluator with deterministic policy first."""

    def __init__(self, *, judge: GroundingJudge | None = None):
        self._judge = judge

    def evaluate(
        self,
        treatment: Mapping[str, Any],
        retrieved_evidence: Sequence[Mapping[str, Any]],
    ) -> GroundingEvaluationV2:
        evidence = [_evidence_record(item) for item in retrieved_evidence]
        evidence_by_id = {item.evidence_id: item for item in evidence if item.evidence_id}
        results = tuple(
            self._evaluate_claim(claim, evidence, evidence_by_id)
            for claim in _build_claims(treatment)
        )

        if any(result.support in {"unsupported", "contraindicated"} for result in results):
            verdict: Literal["supported", "degraded", "rejected"] = "degraded"
        elif any(result.support in {"partial", "uncertain"} for result in results):
            verdict = "degraded"
        else:
            verdict = "supported"
        return GroundingEvaluationV2(verdict=verdict, claims=results)

    def _evaluate_claim(
        self,
        claim: InterventionClaim,
        all_evidence: Sequence[AdmissibleEvidence],
        evidence_by_id: Mapping[str, AdmissibleEvidence],
    ) -> ClaimGroundingResult:
        # Layer 1: cited provenance/admissibility is an invariant. A semantic
        # judge is never allowed to override these failures.
        if not claim.supporting_evidence_ids:
            return ClaimGroundingResult(claim, "unsupported", (), ("no_supporting_evidence_ids",))

        missing_ids = tuple(
            evidence_id
            for evidence_id in claim.supporting_evidence_ids
            if evidence_id not in evidence_by_id
        )
        if missing_ids:
            return ClaimGroundingResult(
                claim,
                "unsupported",
                claim.supporting_evidence_ids,
                ("cited_evidence_id_not_retrieved",),
            )

        cited = tuple(evidence_by_id[evidence_id] for evidence_id in claim.supporting_evidence_ids)
        if any(not item.admissible for item in cited):
            return ClaimGroundingResult(
                claim,
                "unsupported",
                claim.supporting_evidence_ids,
                ("cited_evidence_not_admissible",),
            )

        # External RAG may support intervention knowledge, never a user fact.
        if claim.kind == "user_fact" or any(item.source_type == "user_fact" for item in cited):
            return ClaimGroundingResult(
                claim,
                "unsupported",
                claim.supporting_evidence_ids,
                ("user_fact_cannot_be_authorized_by_external_rag",),
            )

        # Layer 2: structured semantic support. Only cited evidence participates.
        supporting = tuple(item for item in cited if _title_supports(claim, item))
        if not supporting:
            return self._uncertain_or_unsupported(claim, cited)

        combined = "\n".join(item.text for item in supporting)
        normalized_combined = _normalize(combined)
        if any(
            marker in normalized_combined for marker in map(_normalize, _CONTRAINDICATION_MARKERS)
        ):
            return ClaimGroundingResult(
                claim,
                "contraindicated",
                tuple(item.evidence_id for item in supporting),
                ("intervention_contraindicated_by_cited_evidence",),
            )
        if any(marker in normalized_combined for marker in map(_normalize, _NON_SUPPORT_MARKERS)):
            return ClaimGroundingResult(
                claim,
                "unsupported",
                tuple(item.evidence_id for item in supporting),
                ("lexical_overlap_without_claim_support",),
            )

        reasons: list[str] = []
        for key, value in claim.dosage.items():
            if not _value_supported(key, value, combined):
                reasons.append(f"{key}_not_supported_by_cited_evidence")
        if not _material_phrase_supported(claim.progression, combined, _PROGRESSION_MARKERS):
            reasons.append("progression_not_supported_by_cited_evidence")
        if not _material_phrase_supported(list(claim.stop_conditions), combined, _STOP_MARKERS):
            reasons.append("stop_conditions_not_supported_by_cited_evidence")

        if reasons:
            return ClaimGroundingResult(
                claim,
                "partial",
                tuple(item.evidence_id for item in supporting),
                tuple(reasons),
            )

        reason = "intervention_exact_or_alias_supported"
        return ClaimGroundingResult(
            claim,
            "supported",
            tuple(item.evidence_id for item in supporting),
            (reason,),
        )

    def _uncertain_or_unsupported(
        self,
        claim: InterventionClaim,
        cited: Sequence[AdmissibleEvidence],
    ) -> ClaimGroundingResult:
        # Same-body-part lexical overlap is not evidence of the intervention.
        # Very high character overlap without an exact/alias match is the only
        # path eligible for an optional semantic judge.
        title = _normalize(claim.title)
        evidence_text = _normalize("\n".join(item.text for item in cited))
        chars = {char for char in title if char.strip()}
        overlap = (sum(1 for char in chars if char in evidence_text) / len(chars)) if chars else 0.0
        if len(title) >= 2 and overlap >= 0.75 and self._judge is not None:
            judged = self._judge(claim, cited)
            if judged == "supported":
                return ClaimGroundingResult(
                    claim,
                    "supported",
                    tuple(item.evidence_id for item in cited),
                    ("semantic_judge_supported",),
                    judge_used=True,
                )
            if judged in {"partial", "contraindicated", "unsupported"}:
                return ClaimGroundingResult(
                    claim,
                    judged,
                    tuple(item.evidence_id for item in cited),
                    (f"semantic_judge_{judged}",),
                    judge_used=True,
                )
            return ClaimGroundingResult(
                claim,
                "uncertain",
                tuple(item.evidence_id for item in cited),
                ("semantic_support_uncertain",),
                judge_used=True,
            )

        reason = "same_body_part_not_intervention" if overlap > 0 else "intervention_not_supported"
        return ClaimGroundingResult(
            claim,
            "unsupported",
            tuple(item.evidence_id for item in cited),
            (reason,),
        )
