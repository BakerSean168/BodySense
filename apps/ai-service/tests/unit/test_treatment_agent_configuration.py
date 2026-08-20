from pathlib import Path

import pytest
import yaml

from src.configuration.treatment_agent_config import (
    CONFIG_ROOT,
    TreatmentAgentManifest,
    get_default_treatment_configuration,
    get_treatment_configuration,
    load_manifest,
)


def test_default_treatment_configuration_is_repository_versioned_and_stable() -> None:
    config = get_default_treatment_configuration()
    assert config.role == "treatment"
    assert config.logical_model == "bodysense-structured"
    assert config.configuration_id == "treat-config-85718f8e90ac9d80"
    assert (CONFIG_ROOT / "treatment-v1.yaml").exists()
    assert get_treatment_configuration(config.configuration_id) == config


def test_behavior_significant_revision_changes_treatment_configuration_id(tmp_path: Path) -> None:
    data = yaml.safe_load((CONFIG_ROOT / "treatment-v1.yaml").read_text(encoding="utf-8"))
    baseline = TreatmentAgentManifest.model_validate(data)
    data["prompt_revision"] = "treatment-prompt-v2"
    path = tmp_path / "changed.yaml"
    path.write_text(yaml.safe_dump(data, sort_keys=False), encoding="utf-8")
    assert load_manifest(path).configuration_id != baseline.configuration_id


def test_runtime_host_and_credentials_do_not_change_treatment_identity(monkeypatch) -> None:
    baseline = get_default_treatment_configuration().configuration_id
    monkeypatch.setenv("LITELLM_BASE_URL", "http://other.internal:4999/v1")
    monkeypatch.setenv("LITELLM_API_KEY", "different-secret")
    assert get_default_treatment_configuration().configuration_id == baseline


def test_unknown_treatment_configuration_is_rejected() -> None:
    with pytest.raises(ValueError, match="unknown Treatment configuration_id"):
        get_treatment_configuration("treat-config-does-not-exist")


def test_runtime_rejects_unimplemented_treatment_revisions() -> None:
    from src.agents.treatment_agent import create_treatment_agent

    with pytest.raises(ValueError, match="prompt revision"):
        create_treatment_agent(prompt_revision="treatment-prompt-does-not-exist")
    with pytest.raises(ValueError, match="output schema revision"):
        create_treatment_agent(output_schema_revision="treatment-output-does-not-exist")
    with pytest.raises(ValueError, match="tool policy revision"):
        create_treatment_agent(tool_policy_revision="treatment-tools-does-not-exist")
    with pytest.raises(ValueError, match="evidence policy revision"):
        create_treatment_agent(evidence_policy_revision="treatment-evidence-does-not-exist")


def test_go_control_plane_registers_repository_treatment_manifest() -> None:
    repo_root = Path(__file__).resolve().parents[4]
    go_policy = (
        repo_root / "apps/api/internal/service/agent_deployment_policy.go"
    ).read_text(encoding="utf-8")
    config = get_default_treatment_configuration()
    assert config.configuration_id in go_policy
