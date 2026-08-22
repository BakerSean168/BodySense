from pathlib import Path

import pytest
import yaml

from src.configuration.title_agent_config import (
    CONFIG_ROOT,
    TitleAgentManifest,
    get_default_title_configuration,
    get_title_configuration,
    load_manifest,
)


def test_default_title_configuration_is_repository_versioned_and_stable() -> None:
    config = get_default_title_configuration()
    assert config.role == "title"
    assert config.logical_model == "bodysense-text"
    assert config.configuration_id.startswith("title-config-")
    assert len(config.configuration_id) == len("title-config-") + 16
    assert (CONFIG_ROOT / "title-v1.yaml").exists()
    assert get_title_configuration(config.configuration_id) == config
    assert get_default_title_configuration().configuration_id == config.configuration_id


def test_behavior_significant_revision_changes_title_configuration_id(
    tmp_path: Path,
) -> None:
    data = yaml.safe_load((CONFIG_ROOT / "title-v1.yaml").read_text(encoding="utf-8"))
    baseline = TitleAgentManifest.model_validate(data)
    data["prompt_revision"] = "title-prompt-v2"
    path = tmp_path / "changed.yaml"
    path.write_text(yaml.safe_dump(data, sort_keys=False), encoding="utf-8")
    assert load_manifest(path).configuration_id != baseline.configuration_id


def test_runtime_host_and_credentials_do_not_change_title_identity(monkeypatch) -> None:
    baseline = get_default_title_configuration().configuration_id
    monkeypatch.setenv("LITELLM_BASE_URL", "http://other.internal:4999/v1")
    monkeypatch.setenv("LITELLM_API_KEY", "different-secret")
    assert get_default_title_configuration().configuration_id == baseline


def test_unknown_title_configuration_is_rejected() -> None:
    with pytest.raises(ValueError, match="unknown Title configuration_id"):
        get_title_configuration("title-config-does-not-exist")


def test_title_manifest_excludes_runtime_host_from_provenance() -> None:
    config = get_default_title_configuration()
    prov = config.provenance()
    assert prov["id"] == config.configuration_id
    assert prov["role"] == "title"
    assert prov["logical_model"] == "bodysense-text"
    assert "temperature" not in prov
