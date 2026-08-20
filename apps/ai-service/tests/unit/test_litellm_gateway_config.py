from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[4]
CONFIG = ROOT / "docker" / "litellm" / "config.yaml"
SMOKE_CONFIG = ROOT / "docker" / "litellm" / "config.smoke.yaml"

PUBLIC_GROUPS = {
    "bodysense-diagnosis",
    "bodysense-consultation",
    "bodysense-structured",
    "bodysense-text",
    "bodysense-posture",
}
FALLBACK_GROUPS = {
    "bodysense-diagnosis-fallback",
    "bodysense-general-fallback",
    "bodysense-posture-fallback",
}
EXPECTED_FALLBACKS = [
    {"bodysense-diagnosis": ["bodysense-diagnosis-fallback"]},
    {"bodysense-consultation": ["bodysense-general-fallback"]},
    {"bodysense-structured": ["bodysense-general-fallback"]},
    {"bodysense-text": ["bodysense-general-fallback"]},
    {"bodysense-posture": ["bodysense-posture-fallback"]},
]


def _load(path: Path) -> dict:
    with path.open(encoding="utf-8") as handle:
        return yaml.safe_load(handle)


def _groups(config: dict) -> list[str]:
    return [item["model_name"] for item in config["model_list"]]


def test_gateway_exposes_all_business_logical_groups_with_central_fallbacks() -> None:
    config = _load(CONFIG)
    assert set(_groups(config)) == PUBLIC_GROUPS | FALLBACK_GROUPS
    assert config["router_settings"]["fallbacks"] == EXPECTED_FALLBACKS


def test_gateway_owns_llm_physical_provider_credentials() -> None:
    config = _load(CONFIG)
    groups = {item["model_name"]: item["litellm_params"] for item in config["model_list"]}
    assert groups["bodysense-diagnosis"]["model"] == "openai/mimo-v2.5-pro"
    assert groups["bodysense-diagnosis"]["api_base"] == "os.environ/MIMO_BASE_URL"
    assert groups["bodysense-diagnosis"]["api_key"] == "os.environ/MIMO_API_KEY"
    assert groups["bodysense-general-fallback"]["model"] == (
        "openrouter/deepseek/deepseek-chat"
    )
    assert groups["bodysense-general-fallback"]["api_key"] == "os.environ/OPENROUTER_API_KEY"
    assert groups["bodysense-posture"]["model"] == (
        "openrouter/qwen/qwen2.5-vl-72b-instruct"
    )
    assert config["general_settings"]["master_key"] == "os.environ/LITELLM_MASTER_KEY"


def test_smoke_config_preserves_complete_gateway_routing_graph() -> None:
    production = _load(CONFIG)
    smoke = _load(SMOKE_CONFIG)
    assert set(_groups(smoke)) == set(_groups(production))
    assert smoke["router_settings"]["fallbacks"] == production["router_settings"]["fallbacks"]
    groups = {item["model_name"]: item["litellm_params"] for item in smoke["model_list"]}
    assert groups["bodysense-diagnosis"]["mock_response"] == "litellm.InternalServerError"
    assert groups["bodysense-diagnosis-fallback"]["mock_response"] == (
        "bodysense-gateway-fallback-ok"
    )
