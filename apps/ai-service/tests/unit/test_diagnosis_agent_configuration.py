from pathlib import Path

import pytest
import yaml

from src.configuration.diagnosis_agent_config import (
    CONFIG_ROOT,
    DiagnosisAgentManifest,
    get_default_diagnosis_configuration,
    get_diagnosis_configuration,
    load_manifest,
)


def test_default_diagnosis_configuration_is_repository_versioned_and_stable() -> None:
    config = get_default_diagnosis_configuration()
    assert config.role == "diagnosis"
    assert config.logical_model == "bodysense-diagnosis"
    assert config.configuration_id.startswith("diag-config-")
    assert (CONFIG_ROOT / "diagnosis-v3-decision-authority.yaml").exists()
    assert get_diagnosis_configuration(config.configuration_id) == config


def test_behavior_significant_revision_changes_configuration_id(tmp_path: Path) -> None:
    data = yaml.safe_load((CONFIG_ROOT / "diagnosis-v1.yaml").read_text(encoding="utf-8"))
    baseline = DiagnosisAgentManifest.model_validate(data)
    data["prompt_revision"] = "diagnosis-prompt-v4"
    path = tmp_path / "changed.yaml"
    path.write_text(yaml.safe_dump(data, sort_keys=False), encoding="utf-8")
    assert load_manifest(path).configuration_id != baseline.configuration_id


def test_runtime_host_and_credentials_are_not_part_of_configuration(monkeypatch) -> None:
    baseline = get_default_diagnosis_configuration().configuration_id
    monkeypatch.setenv("LITELLM_BASE_URL", "http://different-runtime.internal:4999/v1")
    monkeypatch.setenv("LITELLM_API_KEY", "different-secret")
    assert get_default_diagnosis_configuration().configuration_id == baseline


def test_unknown_configuration_is_rejected() -> None:
    with pytest.raises(ValueError, match="unknown Diagnosis configuration_id"):
        get_diagnosis_configuration("diag-config-does-not-exist")


def test_go_control_plane_registers_every_repository_diagnosis_manifest() -> None:
    repo_root = Path(__file__).resolve().parents[4]
    go_policy = (repo_root / "apps/api/internal/service/agent_deployment_policy.go").read_text(
        encoding="utf-8"
    ) + (repo_root / "apps/api/internal/service/diagnosis_decision_policy.go").read_text(
        encoding="utf-8"
    )
    manifests = [load_manifest(path) for path in sorted(CONFIG_ROOT.glob("diagnosis-*.yaml"))]
    assert get_default_diagnosis_configuration() in manifests
    for config in manifests:
        assert config.configuration_id in go_policy
        assert config.decision_policy_revision in go_policy


def test_manifest_format_revision_does_not_change_behavior_identity() -> None:
    data = yaml.safe_load((CONFIG_ROOT / "diagnosis-v1.yaml").read_text(encoding="utf-8"))
    baseline = DiagnosisAgentManifest.model_validate(data)
    data["manifest_revision"] = "diagnosis-agent-manifest-v2"
    changed = DiagnosisAgentManifest.model_validate(data)
    assert changed.configuration_id == baseline.configuration_id


def test_runtime_rejects_manifest_revision_that_is_not_implemented() -> None:
    from src.agents.diagnosis_agent import create_diagnosis_agent

    config = get_default_diagnosis_configuration()
    with pytest.raises(ValueError, match="prompt revision"):
        create_diagnosis_agent(prompt_revision="diagnosis-prompt-does-not-exist")
    with pytest.raises(ValueError, match="tool policy revision"):
        create_diagnosis_agent(tool_policy_revision="diagnosis-tools-does-not-exist")
    with pytest.raises(ValueError, match="evidence policy revision"):
        create_diagnosis_agent(evidence_policy_revision="diagnosis-evidence-does-not-exist")
    assert config.configuration_id == "diag-config-5a4a13627e14b4cf"
    assert config.prompt_revision == "diagnosis-prompt-v4-evidence-gap"
