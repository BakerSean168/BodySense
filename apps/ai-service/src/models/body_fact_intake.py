"""Bounded extraction proposals for the V2 BodyFact ledger, never commit receipts.

These are application-level model outputs, not alternate HTTP DTOs. The adapter
below maps only explicit user assertions to the existing canonical RecordBodyFacts
contract; Go independently verifies owner/run/message, regions and persistence.
"""

from __future__ import annotations

import re
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field, model_validator

BODY_FACT_INTAKE_POLICY = "body-fact-intake-v1"


class BodyFactDraft(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)
    kind: Literal["symptom", "health_history", "activity", "lifestyle", "training_feedback"]
    statement: str = Field(min_length=1, max_length=2000)
    subject: Literal["self"]
    time_expression: str = Field(default="", max_length=500)
    presence: Literal["present", "absent", "unknown"] = "unknown"


class BodyFactIntakeOutput(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)
    turn_kind: Literal["self_report", "general_question", "ambiguous", "correction", "other"]
    facts: list[BodyFactDraft] = Field(default_factory=list, max_length=20)
    clarification: str = Field(default="", max_length=500)

    @model_validator(mode="after")
    def validate_action(self) -> BodyFactIntakeOutput:
        if self.turn_kind != "self_report" and self.facts:
            raise ValueError("non-self-report turns cannot append facts")
        if self.turn_kind == "self_report" and not self.facts:
            raise ValueError("self_report requires at least one explicit assertion")
        return self


# This gate intentionally covers a conservative subset. Ambiguous subjects must
# be clarified, not guessed. It is not a medical classifier or a truth detector.
_SELF = re.compile(r"^(?:我(?!们)|本人|(?i:I\b|my\b))")
_OTHER_OR_HYPOTHETICAL = re.compile(
    r"我(?:的)?(?:朋友|同事|妈妈|爸爸|父亲|母亲|家人|孩子|儿子|女儿|妻子|丈夫|男友|女友)|"
    r"假如|假设|如果|是不是|会不会|是否|我想知道|我想了解|我听说|我怀疑|[？?]|吗[。.!！]?$|"
    r"(?i:\bif\b|\bwonder\b|\bsuspect\b|\bmy (?:friend|mother|father|child|wife|husband)\b)"
)
_NEGATION = re.compile(
    r"不(?:再)?(?:痛|疼|麻)|没有|无(?:疼痛|麻木)|(?i:\bno\b|\bnot\b|\bwithout\b)"
)


def validate_fact_proposal(output: BodyFactIntakeOutput, original: str) -> None:
    """Validate exact, complete source spans without stripping negation/subjects."""
    for fact in output.facts:
        statement = fact.statement
        if statement != statement.strip() or statement not in original:
            raise ValueError("statement must be a verbatim user-message excerpt")
        # Require a sentence/clause boundary: a quote or embedded relative clause
        # cannot be lifted out and relabelled as the user's own assertion.
        start = original.find(statement)
        prefix = original[:start].rstrip()
        if prefix and prefix[-1] not in "。！？!?\n；;.":
            raise ValueError("statement must preserve its complete subject context")
        if not _SELF.search(statement) or _OTHER_OR_HYPOTHETICAL.search(statement):
            raise ValueError("source is not an unambiguous first-person assertion")
        suffix = original[start + len(statement) :]
        if suffix and suffix[0] not in "。！？!?\n；;.":
            raise ValueError("statement must preserve its complete assertion context")
        if fact.presence == "present" and _NEGATION.search(statement):
            raise ValueError("a negative assertion cannot be labelled present")
        if fact.time_expression and fact.time_expression not in statement:
            raise ValueError("occurrence time must preserve explicit source wording")
        if fact.kind != "symptom" and fact.presence != "unknown":
            raise ValueError("presence applies only to symptom assertions")


def body_fact_proposal(
    output: BodyFactIntakeOutput, original: str, revision: int | float
) -> dict[str, object]:
    validate_fact_proposal(output, original)
    # google.protobuf.Struct numbers arrive as Python floats. Accept only a
    # losslessly integral JS/Struct-safe number; never round a fractional value.
    if (
        isinstance(revision, bool)
        or not isinstance(revision, (int, float))
        or revision < 0
        or revision > 2**53 - 1
        or not float(revision).is_integer()
    ):
        raise ValueError("a Go-owned lossless integer revision is required")
    revision = int(revision)
    facts: list[dict[str, object]] = []
    for fact in output.facts:
        details: dict[str, object] = {}
        if fact.kind == "symptom" and fact.presence != "unknown":
            details["presence"] = fact.presence
        temporal: dict[str, object] = {"precision": "unknown"}
        if fact.time_expression:
            temporal["original_expression"] = fact.time_expression
        facts.append(
            {
                "kind": fact.kind,
                "statement": fact.statement,
                "body_region_ids": [],
                "temporal": temporal,
                "details": details,
            }
        )
    return {"status": "proposed", "expected_revision": revision, "facts": facts}
