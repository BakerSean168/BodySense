"""Project a governed Treatment output onto the Go PlanProposal(create) contract.

The ``training-plan-agent-proposal-v1`` block is the only part of a Treatment
response that Go turns into a pending ``PlanProposal(kind=create)``. It carries
Agent judgment only: no template, entry, plan, revision or owner IDs, no
decision and no status. Go assigns every identity, validates the block strictly
and rejects the whole output on any defect, so this projection never repairs,
guesses or drops Agent content to make it pass. A rejected or plan-less output
projects as ``unavailable`` and Go persists nothing.
"""

from __future__ import annotations

from typing import Any

PLAN_PROPOSAL_SCHEMA_REVISION = "training-plan-agent-proposal-v1"
_EMITTABLE_VERDICTS = frozenset({"accepted", "degraded"})
_TARGET_FIELDS = ("min", "max", "seconds", "meters")
_OPTIONAL_EXERCISE_FIELDS = (
    ("rest_seconds", "restSeconds"),
    ("side", "side"),
    ("load_or_intensity", "loadOrIntensity"),
    ("instructions", "instructions"),
)


def _unavailable(reason: str) -> dict[str, Any]:
    return {
        "schemaRevision": PLAN_PROPOSAL_SCHEMA_REVISION,
        "outcome": "unavailable",
        "reason": reason,
    }


def _target(raw: dict[str, Any]) -> dict[str, Any]:
    target: dict[str, Any] = {"kind": raw.get("kind")}
    for key in _TARGET_FIELDS:
        if raw.get(key) is not None:
            target[key] = raw[key]
    return target


def _exercise(raw: dict[str, Any]) -> dict[str, Any]:
    exercise: dict[str, Any] = {
        "exerciseId": raw.get("exercise_key"),
        "nameSnapshot": raw.get("name"),
        "sets": raw.get("sets"),
        "target": _target(raw.get("target") or {}),
    }
    for source, destination in _OPTIONAL_EXERCISE_FIELDS:
        if raw.get(source) is not None:
            exercise[destination] = raw[source]
    return exercise


def _session(raw: dict[str, Any]) -> dict[str, Any]:
    return {
        "title": raw.get("title"),
        "stages": [
            {
                "phase": stage.get("phase"),
                "exercises": [_exercise(item) for item in stage.get("exercises") or []],
            }
            for stage in raw.get("stages") or []
        ],
    }


def project_plan_proposal(emitted: dict[str, Any]) -> dict[str, Any]:
    """Return the strict Go ingestion block for one governed Treatment output."""

    verdict = (emitted.get("governance") or {}).get("verdict")
    if verdict not in _EMITTABLE_VERDICTS:
        return _unavailable("governance_rejected")
    plan = emitted.get("plan")
    if not isinstance(plan, dict):
        return _unavailable("no_plan")

    frequency: dict[str, Any] = {}
    if plan.get("sessions_per_week") is not None:
        frequency["sessionsPerWeek"] = plan["sessions_per_week"]
    if plan.get("frequency_description"):
        frequency["description"] = plan["frequency_description"]

    return {
        "schemaRevision": PLAN_PROPOSAL_SCHEMA_REVISION,
        "outcome": "proposed",
        "title": plan.get("title"),
        "goal": emitted.get("goal"),
        "summary": emitted.get("summary"),
        "content": {
            "sessions": [_session(item) for item in plan.get("sessions") or []],
            "frequency": frequency,
            "constraints": list(plan.get("constraints") or []),
        },
        "evidenceIds": list(emitted.get("evidence_ids") or []),
    }
