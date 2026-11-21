"""S4 hard isolation: clinical input builders must not read episodic recall."""

from pathlib import Path

SRC = Path(__file__).resolve().parents[2] / "src"

# Each file assembles a Diagnosis, Safety or TrainingPlan input.
CLINICAL_INPUT_MODULES = [
    "agents/diagnosis_agent.py",
    "services/diagnosis_service.py",
    "services/red_flag_detector.py",
    "services/governance/output_guard.py",
    "services/treatment_agent_service.py",
    "services/treatment_plan_proposal.py",
]


def test_clinical_inputs_never_reference_episodic_recall() -> None:
    for relative in CLINICAL_INPUT_MODULES:
        source = (SRC / relative).read_text(encoding="utf-8")
        assert "episodic_notes" not in source, relative


def _is_runtime_boundary(relative: Path) -> bool:
    """The runtime API boundary forwards the notes; only the thread engine reads them."""
    return relative.parts[:2] == ("api", "routes") and relative.name.startswith("runtime")


def test_episodic_recall_only_reaches_the_consultation_system_prompt() -> None:
    holders = []
    for path in SRC.rglob("*.py"):
        if _is_runtime_boundary(path.relative_to(SRC)) or path.name.startswith(
            "consultation_thread"
        ):
            continue
        if "episodic_notes" in path.read_text(encoding="utf-8"):
            holders.append(str(path.relative_to(SRC)))
    assert holders == [], holders
