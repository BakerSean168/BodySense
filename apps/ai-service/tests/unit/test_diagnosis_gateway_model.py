from pathlib import Path

import pytest

from src.ai.diagnosis_gateway_model import (
    DIAGNOSIS_LOGICAL_MODEL,
    diagnosis_model_settings,
    get_diagnosis_gateway_model,
    get_diagnosis_runtime_model,
)
from src.configuration.diagnosis_agent_config import get_default_diagnosis_configuration

CONFIG = get_default_diagnosis_configuration()
ROOT = Path(__file__).resolve().parents[4]


def test_diagnosis_model_is_logical_gateway_model(monkeypatch) -> None:
    monkeypatch.setenv("LITELLM_BASE_URL", "http://gateway.test:4000/v1")
    monkeypatch.setenv("LITELLM_API_KEY", "sk-internal-test")
    get_diagnosis_gateway_model.cache_clear()

    model = get_diagnosis_runtime_model(CONFIG)

    assert DIAGNOSIS_LOGICAL_MODEL == "bodysense-diagnosis"
    assert model.model_name == DIAGNOSIS_LOGICAL_MODEL
    assert str(model.provider.base_url) == "http://gateway.test:4000/v1/"
    assert diagnosis_model_settings(CONFIG) == {"temperature": 0.3, "max_tokens": 2048}


def test_legacy_diagnosis_backend_switch_is_ignored_after_retirement(monkeypatch) -> None:
    monkeypatch.setenv("DIAGNOSIS_MODEL_BACKEND", "legacy")
    monkeypatch.setenv("LITELLM_BASE_URL", "http://gateway.test:4000/v1")
    monkeypatch.setenv("LITELLM_API_KEY", "sk-internal-test")
    get_diagnosis_gateway_model.cache_clear()

    model = get_diagnosis_runtime_model(CONFIG)

    assert model.model_name == DIAGNOSIS_LOGICAL_MODEL
    assert str(model.provider.base_url) == "http://gateway.test:4000/v1/"


def test_diagnosis_rejects_unimplemented_model_group_revision() -> None:
    changed = CONFIG.model_copy(update={"model_group_revision": "diagnosis-model-group-future"})
    with pytest.raises(ValueError, match="unsupported Diagnosis model group revision"):
        get_diagnosis_runtime_model(changed)


def test_diagnosis_source_cannot_reintroduce_legacy_provider_routing() -> None:
    files = [
        ROOT / "apps/ai-service/src/services/diagnosis_service.py",
        ROOT / "apps/ai-service/src/ai/diagnosis_gateway_model.py",
        ROOT / "apps/ai-service/src/agents/diagnosis_agent.py",
    ]
    source = "\n".join(path.read_text(encoding="utf-8") for path in files)
    for forbidden in (
        "DIAGNOSIS_MODEL_BACKEND",
        "pydantic_model",
        "ModelRouter",
        "AIService",
        "llm.json",
        "MIMO_API_KEY",
        "OPENROUTER_API_KEY",
    ):
        assert forbidden not in source

    for compose in (
        ROOT / "docker/docker-compose.yml",
        ROOT / "docker/docker-compose.prod.yml",
        ROOT / "docker/docker-compose.prod.do.yml",
    ):
        assert "DIAGNOSIS_MODEL_BACKEND" not in compose.read_text(encoding="utf-8")
