"""Configuration -> red-flag detector revision matrix (R2-11).

The detector ships four revisions side by side in
``src/services/red_flag_detector.py``. Which revision actually runs is
decided three layers up: an agent configuration file names a governance
policy revision, ``guard_structured_output`` validates and forwards it,
and ``governance_issues`` resolves it through
``DIAGNOSIS_RED_FLAG_DETECTOR_REVISION_BY_POLICY`` (diagnosis only).
These tests pin that whole chain exactly as it stands today. They are a
contract snapshot, not a behaviour change: the default revision stays
the literal v1, and no detection semantics are altered.
"""

from __future__ import annotations

import inspect

import pytest

from src.configuration.diagnosis_agent_config import (
    CONFIG_ROOT,
    get_default_diagnosis_configuration,
    load_manifest,
)
from src.runtime import governance_issues
from src.runtime.governance import guard_structured_output
from src.runtime.governance_policy import (
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V3,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V5,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V6,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V7,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V8,
    DIAGNOSIS_RED_FLAG_DETECTOR_REVISION_BY_POLICY,
)
from src.services.red_flag_detector import (
    DEFAULT_RED_FLAG_DETECTOR_REVISION,
    RED_FLAG_DETECTOR_REVISION_LITERAL_V1,
    RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2,
    RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3,
    RED_FLAG_DETECTOR_REVISION_NEGATION_LIST_V4,
    RedFlagDetector,
)

# The exact policy -> detector resolution that exists today. Diagnosis
# governance v8 is deliberately absent: it takes the structured-safety
# path in governance_issues and never reaches the keyword detector.
EXPECTED_POLICY_TO_DETECTOR = {
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V3: RED_FLAG_DETECTOR_REVISION_LITERAL_V1,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4: RED_FLAG_DETECTOR_REVISION_LITERAL_V1,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V5: RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V6: RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V7: RED_FLAG_DETECTOR_REVISION_NEGATION_LIST_V4,
}

# Every shipped diagnosis agent configuration, the governance policy
# revision it names, and the detector revision that ultimately runs.
EXPECTED_CONFIGURATION_MATRIX = {
    "diagnosis-v3-decision-authority.yaml": (
        DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V3,
        RED_FLAG_DETECTOR_REVISION_LITERAL_V1,
    ),
    "diagnosis-v4-claim-surface.yaml": (
        DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4,
        RED_FLAG_DETECTOR_REVISION_LITERAL_V1,
    ),
    "diagnosis-v5-negation-aware.yaml": (
        DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V5,
        RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2,
    ),
    "diagnosis-v6-negation-bridge.yaml": (
        DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V6,
        RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3,
    ),
    "diagnosis-v7-negation-list.yaml": (
        DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V7,
        RED_FLAG_DETECTOR_REVISION_NEGATION_LIST_V4,
    ),
    "diagnosis-v8-structured-safety.yaml": (
        DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V8,
        None,  # structured-safety path; keyword detector not consulted
    ),
    "diagnosis-v9-structured-safety-context.yaml": (
        DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V8,
        None,
    ),
    "diagnosis-v10-structured-safety-budget.yaml": (
        DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V8,
        None,
    ),
}

_MINIMAL_PAYLOADS = {
    "diagnosis": {"candidates": []},
    "treatment": {"goal": "matrix", "interventions": []},
    "posture": {
        "view": "front",
        "findings": [],
        "summary_markdown": "matrix",
        "disclaimer": "matrix",
    },
    "assessment": {"observations": []},
}


@pytest.fixture
def captured_detector_revisions(monkeypatch: pytest.MonkeyPatch) -> list[str]:
    """Record the detector revision governance_issues resolves to."""
    captured: list[str] = []

    def fake_check_red_flags(*_args: object, **kwargs: object) -> list[object]:
        captured.append(str(kwargs["detector_revision"]))
        return []

    monkeypatch.setattr(governance_issues, "check_red_flags", fake_check_red_flags)
    return captured


def _guard(kind: str, policy_revision: str | None) -> None:
    payload = dict(_MINIMAL_PAYLOADS[kind])
    kwargs: dict[str, object] = {"policy_revision": policy_revision}
    if kind == "diagnosis" and policy_revision == DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V8:
        payload["safety_findings"] = []
        kwargs["body_state"] = {"facts": [], "observations": []}
    if kind == "assessment":
        kwargs["assessment_evidence_catalog"] = {}
    guard_structured_output(kind, payload, **kwargs)  # type: ignore[arg-type]


def test_detector_revision_constants_are_pinned() -> None:
    assert RED_FLAG_DETECTOR_REVISION_LITERAL_V1 == "red-flag-detector-literal-v1"
    assert RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2 == "red-flag-detector-negation-aware-v2"
    assert RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3 == "red-flag-detector-negation-bridge-v3"
    assert RED_FLAG_DETECTOR_REVISION_NEGATION_LIST_V4 == "red-flag-detector-negation-list-v4"
    # The default revision remains the literal v1 (R2-11: do not move it).
    assert DEFAULT_RED_FLAG_DETECTOR_REVISION == RED_FLAG_DETECTOR_REVISION_LITERAL_V1
    detect_default = inspect.signature(RedFlagDetector.detect).parameters["revision"].default
    assert detect_default == RED_FLAG_DETECTOR_REVISION_LITERAL_V1


def test_policy_to_detector_map_is_exact() -> None:
    assert DIAGNOSIS_RED_FLAG_DETECTOR_REVISION_BY_POLICY == EXPECTED_POLICY_TO_DETECTOR
    assert DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V8 not in (
        DIAGNOSIS_RED_FLAG_DETECTOR_REVISION_BY_POLICY
    )


@pytest.mark.parametrize(
    ("policy_revision", "expected_detector"),
    list(EXPECTED_POLICY_TO_DETECTOR.items()),
    ids=["v3", "v4", "v5", "v6", "v7"],
)
def test_diagnosis_policy_revision_resolves_to_detector(
    captured_detector_revisions: list[str],
    policy_revision: str,
    expected_detector: str,
) -> None:
    _guard("diagnosis", policy_revision)
    assert captured_detector_revisions == [expected_detector]


def test_diagnosis_without_policy_revision_uses_current_default(
    captured_detector_revisions: list[str],
) -> None:
    # guard_structured_output substitutes DIAGNOSIS_GOVERNANCE_POLICY_REVISION,
    # which today is the v4 policy and therefore the literal v1 detector.
    assert DIAGNOSIS_GOVERNANCE_POLICY_REVISION == DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4
    _guard("diagnosis", None)
    assert captured_detector_revisions == [RED_FLAG_DETECTOR_REVISION_LITERAL_V1]


def test_diagnosis_v8_never_reaches_keyword_detector(
    captured_detector_revisions: list[str],
) -> None:
    _guard("diagnosis", DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V8)
    assert captured_detector_revisions == []


@pytest.mark.parametrize("kind", ["treatment", "posture", "assessment"])
def test_non_diagnosis_kinds_resolve_to_default_detector(
    captured_detector_revisions: list[str], kind: str
) -> None:
    _guard(kind, None)
    assert captured_detector_revisions == [RED_FLAG_DETECTOR_REVISION_LITERAL_V1]


@pytest.mark.parametrize(
    ("filename", "expected"),
    list(EXPECTED_CONFIGURATION_MATRIX.items()),
    ids=list(EXPECTED_CONFIGURATION_MATRIX),
)
def test_agent_configuration_files_resolve_to_expected_detector(
    captured_detector_revisions: list[str],
    filename: str,
    expected: tuple[str, str | None],
) -> None:
    expected_policy, expected_detector = expected
    manifest = load_manifest(CONFIG_ROOT / filename)
    assert manifest.governance_policy_revision == expected_policy
    _guard("diagnosis", manifest.governance_policy_revision)
    if expected_detector is None:
        assert captured_detector_revisions == []
    else:
        assert captured_detector_revisions == [expected_detector]


def test_configuration_matrix_covers_every_diagnosis_config_file() -> None:
    on_disk = {path.name for path in CONFIG_ROOT.glob("diagnosis-*.yaml")}
    assert on_disk == set(EXPECTED_CONFIGURATION_MATRIX)


def test_default_diagnosis_configuration_is_structured_safety() -> None:
    manifest = get_default_diagnosis_configuration()
    assert manifest.governance_policy_revision == DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V8
