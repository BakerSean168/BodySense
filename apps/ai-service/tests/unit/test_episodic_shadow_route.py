"""Internal S2-B shadow proposal route tests use synthetic text exclusively."""

from __future__ import annotations

import copy
import json
from pathlib import Path

import pytest
from pydantic_ai.models.test import TestModel

from src.ai.errors import GatewayUnavailableError

ROOT = Path(__file__).resolve().parents[4]
WIRE = json.loads(
    (ROOT / "contracts/internal/agent-runtime/fixtures/episodic-shadow-wire.v1.json").read_text(
        encoding="utf-8"
    )
)
PATH = "/runtime/episodic-shadow/proposals"
MESSAGE_ID = "33333333-3333-4333-8333-333333333333"


def _model_with(quote: str, preference: str = "brief_answers", message_id: str = MESSAGE_ID):
    return TestModel(
        custom_output_args={
            "quotes": [
                {
                    "source_message_id": message_id,
                    "quote": quote,
                    "preference": preference,
                }
            ]
        }
    )


def test_valid_command_returns_the_shared_golden_proposal(client, monkeypatch) -> None:
    monkeypatch.setattr(
        "src.api.routes.episodic_shadow._shadow_quote_model",
        lambda: _model_with(WIRE["proposal"]["quotes"][0]["quote"]),
    )
    response = client.post(PATH, json=WIRE["command"])
    assert response.status_code == 200
    # One shared wire fixture: the quote must be an entire source message.
    assert response.json() == WIRE["proposal"]
    source_texts = {item["text"] for item in WIRE["command"]["source_messages"]}
    assert response.json()["quotes"][0]["quote"] in source_texts


@pytest.mark.parametrize(
    "mutation",
    (
        lambda command: command.update(to_seq=3),
        lambda command: command.update(policy_revision="other"),
        lambda command: command["source_messages"][0].update(text=" 请简短回答。"),
        lambda command: command["source_messages"][0].update(extra="injected"),
    ),
)
def test_invalid_command_is_rejected_before_any_model(client, monkeypatch, mutation) -> None:
    def never_resolve():
        raise AssertionError("invalid command reached model resolution")

    monkeypatch.setattr("src.api.routes.episodic_shadow._shadow_quote_model", never_resolve)
    command = copy.deepcopy(WIRE["command"])
    mutation(command)
    response = client.post(PATH, json=command)
    assert response.status_code == 422
    assert response.json()["detail"] == "invalid private shadow proposal command"


def test_model_quoting_outside_the_source_window_is_never_returned(client, monkeypatch) -> None:
    monkeypatch.setattr(
        "src.api.routes.episodic_shadow._shadow_quote_model",
        lambda: _model_with("请分点回答。"),
    )
    response = client.post(PATH, json=WIRE["command"])
    assert response.status_code == 500
    assert "quotes" not in response.text


def test_unavailable_gateway_model_is_reported_as_unavailable(client, monkeypatch) -> None:
    def unavailable():
        raise GatewayUnavailableError("LITELLM_API_KEY is required")

    monkeypatch.setattr("src.api.routes.episodic_shadow._shadow_quote_model", unavailable)
    response = client.post(PATH, json=WIRE["command"])
    assert response.status_code == 503
    assert response.json()["detail"] == "shadow proposal generation unavailable"
