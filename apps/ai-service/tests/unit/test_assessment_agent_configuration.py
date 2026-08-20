from pathlib import Path

import pytest
import yaml

from src.configuration.assessment_agent_config import (
    CONFIG_ROOT,
    AssessmentAgentManifest,
    get_assessment_configuration,
    get_default_assessment_configuration,
    load_manifest,
)


def test_default_assessment_configuration_is_repository_versioned_and_stable() -> None:
    config = get_default_assessment_configuration()
    assert config.role == "assessment"
    assert config.logical_model == "bodysense-structured"
    assert config.configuration_id.startswith("assess-config-")
    assert len(config.configuration_id) == len("assess-config-") + 16
    assert (CONFIG_ROOT / "assessment-v1.yaml").exists()
    assert get_assessment_configuration(config.configuration_id) == config
    # fingerprint identity is stable across runs
    assert get_default_assessment_configuration().configuration_id == config.configuration_id


def test_behavior_significant_revision_changes_assessment_configuration_id(
    tmp_path: Path,
) -> None:
    data = yaml.safe_load((CONFIG_ROOT / "assessment-v1.yaml").read_text(encoding="utf-8"))
    baseline = AssessmentAgentManifest.model_validate(data)
    data["prompt_revision"] = "assessment-prompt-v2"
    path = tmp_path / "changed.yaml"
    path.write_text(yaml.safe_dump(data, sort_keys=False), encoding="utf-8")
    assert load_manifest(path).configuration_id != baseline.configuration_id


def test_runtime_host_and_credentials_do_not_change_assessment_identity(monkeypatch) -> None:
    baseline = get_default_assessment_configuration().configuration_id
    monkeypatch.setenv("LITELLM_BASE_URL", "http://other.internal:4999/v1")
    monkeypatch.setenv("LITELLM_API_KEY", "different-secret")
    assert get_default_assessment_configuration().configuration_id == baseline


def test_unknown_assessment_configuration_is_rejected() -> None:
    with pytest.raises(ValueError, match="unknown Assessment configuration_id"):
        get_assessment_configuration("assess-config-does-not-exist")


def test_assessment_manifest_excludes_runtime_host_from_provenance() -> None:
    config = get_default_assessment_configuration()
    prov = config.provenance()
    assert prov["id"] == config.configuration_id
    assert prov["role"] == "assessment"
    assert prov["logical_model"] == "bodysense-structured"
    assert "temperature" not in prov  # generation settings live in the behavior fingerprint
