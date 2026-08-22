from pathlib import Path

import pytest
import yaml

from src.configuration.posture_agent_config import (
    CONFIG_ROOT,
    PostureAgentManifest,
    get_default_posture_configuration,
    get_posture_configuration,
    load_manifest,
)


def test_default_posture_configuration_is_repository_versioned_and_stable() -> None:
    config = get_default_posture_configuration()
    assert config.role == "posture"
    assert config.logical_model == "bodysense-posture"
    assert config.configuration_id.startswith("posture-config-")
    assert len(config.configuration_id) == len("posture-config-") + 16
    assert (CONFIG_ROOT / "posture-v1.yaml").exists()
    assert get_posture_configuration(config.configuration_id) == config
    assert get_default_posture_configuration().configuration_id == config.configuration_id


def test_behavior_significant_revision_changes_posture_configuration_id(
    tmp_path: Path,
) -> None:
    data = yaml.safe_load((CONFIG_ROOT / "posture-v1.yaml").read_text(encoding="utf-8"))
    baseline = PostureAgentManifest.model_validate(data)
    data["prompt_revision"] = "posture-prompt-v2"
    path = tmp_path / "changed.yaml"
    path.write_text(yaml.safe_dump(data, sort_keys=False), encoding="utf-8")
    assert load_manifest(path).configuration_id != baseline.configuration_id


def test_runtime_host_and_credentials_do_not_change_posture_identity(monkeypatch) -> None:
    baseline = get_default_posture_configuration().configuration_id
    monkeypatch.setenv("LITELLM_BASE_URL", "http://other.internal:4999/v1")
    monkeypatch.setenv("LITELLM_API_KEY", "different-secret")
    assert get_default_posture_configuration().configuration_id == baseline


def test_unknown_posture_configuration_is_rejected() -> None:
    with pytest.raises(ValueError, match="unknown Posture configuration_id"):
        get_posture_configuration("posture-config-does-not-exist")


def test_posture_manifest_excludes_runtime_host_from_provenance() -> None:
    config = get_default_posture_configuration()
    prov = config.provenance()
    assert prov["id"] == config.configuration_id
    assert prov["role"] == "posture"
    assert prov["logical_model"] == "bodysense-posture"
    assert "temperature" not in prov
