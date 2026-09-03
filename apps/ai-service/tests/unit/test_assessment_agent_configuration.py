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
    assert (CONFIG_ROOT / "assessment-v4.yaml").exists()
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


def test_assessment_historical_and_current_configs_are_repository_known_and_distinct() -> None:
    from src.configuration.assessment_agent_config import (
        get_assessment_configuration,
        known_assessment_configuration_ids,
    )

    ids = known_assessment_configuration_ids()
    assert len(ids) == 5
    v5 = get_default_assessment_configuration()
    v1 = get_assessment_configuration("assess-config-fbff8155337b388d")
    v2 = get_assessment_configuration("assess-config-cae55474253e1601")
    v3 = get_assessment_configuration("assess-config-c6cfff22aa362fff")
    v4 = get_assessment_configuration("assess-config-e579030c2b8b540c")
    assert (
        len(
            {
                v1.configuration_id,
                v2.configuration_id,
                v3.configuration_id,
                v4.configuration_id,
                v5.configuration_id,
            }
        )
        == 5
    )
    assert v1.output_schema_revision == "assessment-output-v1"
    assert v2.output_schema_revision == "assessment-output-v1"
    assert v3.output_schema_revision == "assessment-output-v2"
    assert v3.evidence_policy_revision == "assessment-evidence-contract-v2"
    # Assessment v4 keeps its immutable evidence-contract-v3 identity for replay.
    assert v4.output_schema_revision == "assessment-output-v2"
    assert v4.prompt_revision == "assessment-prompt-v3-evidence-contract"
    assert v4.evidence_policy_revision == "assessment-evidence-contract-v3"
    assert v4.governance_policy_revision == "assessment-governance-v2"
    assert v4.decision_policy_revision == "assessment-go-generation-v2"
    # Assessment v5 is the serving configuration that admits reviewed evidence.
    assert v5.output_schema_revision == "assessment-output-v2"
    assert v5.prompt_revision == "assessment-prompt-v3-evidence-contract"
    assert v5.evidence_policy_revision == "assessment-evidence-contract-v4"
    assert v5.governance_policy_revision == "assessment-governance-v2"
    assert v5.decision_policy_revision == "assessment-go-generation-v2"


def test_assessment_v2_agent_accepts_v2_prompt_revision() -> None:
    from src.agents.assessment_agent import create_assessment_agent
    from src.prompts.assessment import (
        ASSESSMENT_PROMPT_REVISION_V2,
        get_assessment_system_prompt,
    )

    assert "v2" in get_assessment_system_prompt(ASSESSMENT_PROMPT_REVISION_V2)
    agent = create_assessment_agent(prompt_revision=ASSESSMENT_PROMPT_REVISION_V2)
    assert agent.name == "bodysense_assessment"
    with pytest.raises(ValueError, match="unsupported Assessment prompt revision"):
        get_assessment_system_prompt("assessment-prompt-does-not-exist")
