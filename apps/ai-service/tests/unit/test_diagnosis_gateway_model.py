from src.ai.diagnosis_gateway_model import (
    DIAGNOSIS_LOGICAL_MODEL,
    diagnosis_model_settings,
    get_diagnosis_gateway_model,
)
from src.ai.diagnosis_model_boundary import get_diagnosis_runtime_model


def test_diagnosis_model_is_logical_gateway_model(monkeypatch) -> None:
    monkeypatch.setenv("LITELLM_BASE_URL", "http://gateway.test:4000/v1")
    monkeypatch.setenv("LITELLM_API_KEY", "sk-internal-test")
    get_diagnosis_gateway_model.cache_clear()

    model = get_diagnosis_gateway_model()

    assert DIAGNOSIS_LOGICAL_MODEL == "bodysense-diagnosis"
    assert model.model_name == DIAGNOSIS_LOGICAL_MODEL
    assert str(model.provider.base_url) == "http://gateway.test:4000/v1/"
    assert diagnosis_model_settings() == {"temperature": 0.3, "max_tokens": 2048}


def test_runtime_model_uses_gateway_when_new_runtime_contract_is_enabled(monkeypatch) -> None:
    monkeypatch.setenv("DIAGNOSIS_MODEL_BACKEND", "litellm")
    monkeypatch.setenv("LITELLM_BASE_URL", "http://gateway.test:4000/v1")
    monkeypatch.setenv("LITELLM_API_KEY", "sk-internal-test")
    get_diagnosis_gateway_model.cache_clear()

    model = get_diagnosis_runtime_model()

    assert model.model_name == DIAGNOSIS_LOGICAL_MODEL


def test_runtime_model_defaults_to_legacy_only_for_unsynchronized_runtime(monkeypatch) -> None:
    from pydantic_ai.models.test import TestModel

    calls: list[str] = []
    expected = TestModel()

    def fake_legacy_model(use_case: str):
        calls.append(use_case)
        return expected

    monkeypatch.delenv("DIAGNOSIS_MODEL_BACKEND", raising=False)
    monkeypatch.setattr("src.ai.pydantic_model.get_pydantic_model", fake_legacy_model)

    assert get_diagnosis_runtime_model() is expected
    assert calls == ["llm.json"]



def test_runtime_model_rejects_unknown_backend(monkeypatch) -> None:
    import pytest

    from src.ai.errors import NoAvailableProviderError

    monkeypatch.setenv("DIAGNOSIS_MODEL_BACKEND", "surprise")
    with pytest.raises(NoAvailableProviderError, match="DIAGNOSIS_MODEL_BACKEND"):
        get_diagnosis_runtime_model()
