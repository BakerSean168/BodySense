from pathlib import Path

import pytest
import yaml

from src.configuration.consultation_agent_config import (
    CONFIG_ROOT,
    ConsultationAgentManifest,
    get_consultation_configuration,
    get_default_consultation_configuration,
    load_manifest,
)


def test_default_consultation_configuration_is_repository_versioned_and_stable() -> None:
    config = get_default_consultation_configuration()
    assert config.role == "consultation"
    assert config.logical_model == "bodysense-consultation"
    assert config.configuration_id.startswith("consult-config-")
    assert len(config.configuration_id) == len("consult-config-") + 16
    assert (CONFIG_ROOT / "consultation-v1.yaml").exists()
    assert get_consultation_configuration(config.configuration_id) == config
    assert get_default_consultation_configuration().configuration_id == config.configuration_id


def test_behavior_significant_revision_changes_consultation_configuration_id(
    tmp_path: Path,
) -> None:
    data = yaml.safe_load((CONFIG_ROOT / "consultation-v1.yaml").read_text(encoding="utf-8"))
    baseline = ConsultationAgentManifest.model_validate(data)
    data["prompt_revision"] = "consultation-prompt-v2"
    path = tmp_path / "changed.yaml"
    path.write_text(yaml.safe_dump(data, sort_keys=False), encoding="utf-8")
    assert load_manifest(path).configuration_id != baseline.configuration_id


def test_runtime_host_and_credentials_do_not_change_consultation_identity(monkeypatch) -> None:
    baseline = get_default_consultation_configuration().configuration_id
    monkeypatch.setenv("LITELLM_BASE_URL", "http://other.internal:4999/v1")
    monkeypatch.setenv("LITELLM_API_KEY", "different-secret")
    assert get_default_consultation_configuration().configuration_id == baseline


def test_unknown_consultation_configuration_is_rejected() -> None:
    with pytest.raises(ValueError, match="unknown Consultation configuration_id"):
        get_consultation_configuration("consult-config-does-not-exist")


def test_consultation_manifest_excludes_runtime_host_from_provenance() -> None:
    config = get_default_consultation_configuration()
    prov = config.provenance()
    assert prov["id"] == config.configuration_id
    assert prov["role"] == "consultation"
    assert prov["logical_model"] == "bodysense-consultation"
    assert "temperature" not in prov
