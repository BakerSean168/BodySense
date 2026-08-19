from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[4]
CONFIG = ROOT / "docker" / "litellm" / "config.yaml"
SMOKE_CONFIG = ROOT / "docker" / "litellm" / "config.smoke.yaml"


def _load(path: Path) -> dict:
    with path.open(encoding="utf-8") as handle:
        return yaml.safe_load(handle)


def _groups(config: dict) -> list[str]:
    return [item["model_name"] for item in config["model_list"]]


def test_gateway_exposes_one_public_diagnosis_group_with_ordered_fallback() -> None:
    config = _load(CONFIG)
    assert _groups(config) == ["bodysense-diagnosis", "bodysense-diagnosis-fallback"]
    assert config["router_settings"]["fallbacks"] == [
        {"bodysense-diagnosis": ["bodysense-diagnosis-fallback"]}
    ]


def test_gateway_owns_physical_provider_credentials_by_environment_reference() -> None:
    config = _load(CONFIG)
    primary, fallback = config["model_list"]
    assert primary["litellm_params"]["model"] == "openai/mimo-v2.5-pro"
    assert primary["litellm_params"]["api_base"] == "os.environ/MIMO_BASE_URL"
    assert primary["litellm_params"]["api_key"] == "os.environ/MIMO_API_KEY"
    assert fallback["litellm_params"]["model"] == "openrouter/deepseek/deepseek-chat"
    assert fallback["litellm_params"]["api_key"] == "os.environ/OPENROUTER_API_KEY"
    assert config["general_settings"]["master_key"] == "os.environ/LITELLM_MASTER_KEY"


def test_smoke_config_preserves_production_routing_graph() -> None:
    production = _load(CONFIG)
    smoke = _load(SMOKE_CONFIG)
    assert _groups(smoke) == _groups(production)
    assert smoke["router_settings"]["fallbacks"] == production["router_settings"]["fallbacks"]
    assert smoke["model_list"][0]["litellm_params"]["mock_response"] == (
        "litellm.InternalServerError"
    )
    assert smoke["model_list"][1]["litellm_params"]["mock_response"] == (
        "bodysense-gateway-fallback-ok"
    )
