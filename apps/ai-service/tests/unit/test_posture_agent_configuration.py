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
    assert (CONFIG_ROOT / "posture-v2.yaml").exists()
    assert config.configuration_id == "posture-config-efa3a84622818772"
    assert config.geometry_mechanism is not None
    assert config.geometry_mechanism.mechanism_revision == "posture-geometry-v1"
    assert "/latest/" not in config.geometry_mechanism.model_uri
    assert get_posture_configuration(config.configuration_id) == config
    assert get_default_posture_configuration().configuration_id == config.configuration_id


def test_historical_v1_identity_remains_stable() -> None:
    legacy = load_manifest(CONFIG_ROOT / "posture-v1.yaml")
    assert legacy.configuration_id == "posture-config-3a774008db422a31"
    assert legacy.geometry_mechanism is None


def test_behavior_significant_revision_changes_posture_configuration_id(
    tmp_path: Path,
) -> None:
    data = yaml.safe_load((CONFIG_ROOT / "posture-v2.yaml").read_text(encoding="utf-8"))
    baseline = PostureAgentManifest.model_validate(data)
    data["prompt_revision"] = "posture-prompt-v2"
    path = tmp_path / "changed.yaml"
    path.write_text(yaml.safe_dump(data, sort_keys=False), encoding="utf-8")
    assert load_manifest(path).configuration_id != baseline.configuration_id


def test_geometry_revision_and_model_hash_change_posture_configuration_id() -> None:
    baseline = get_default_posture_configuration()
    assert baseline.geometry_mechanism is not None
    changed_threshold = baseline.model_copy(
        update={
            "geometry_mechanism": baseline.geometry_mechanism.model_copy(
                update={"threshold_revision": "posture-geometry-thresholds-v2"}
            )
        }
    )
    changed_model = baseline.model_copy(
        update={
            "geometry_mechanism": baseline.geometry_mechanism.model_copy(
                update={"model_sha256": "0" * 64}
            )
        }
    )
    assert changed_threshold.configuration_id != baseline.configuration_id
    assert changed_model.configuration_id != baseline.configuration_id


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
    assert prov["geometry_mechanism_revision"] == "posture-geometry-v1"
    assert "temperature" not in prov
