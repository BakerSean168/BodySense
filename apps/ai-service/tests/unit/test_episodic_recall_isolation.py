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


def test_episodic_recall_only_reaches_the_consultation_system_prompt() -> None:
    holders = []
    for path in SRC.rglob("*.py"):
        if path.name in {"runtime.py", "consultation_thread.py"}:
            continue
        if "episodic_notes" in path.read_text(encoding="utf-8"):
            holders.append(str(path.relative_to(SRC)))
    assert holders == [], holders
