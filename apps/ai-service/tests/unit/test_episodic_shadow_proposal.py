"""S2-B transport and model-boundary tests use synthetic text exclusively."""

from __future__ import annotations

import json
from pathlib import Path
from uuid import NAMESPACE_URL, UUID, uuid5

import pytest
from pydantic import ValidationError
from pydantic_ai.messages import ModelMessage, ModelResponse
from pydantic_ai.models import ModelRequestParameters
from pydantic_ai.models.test import TestModel
from pydantic_ai.settings import ModelSettings

from src.api.episodic_shadow_proto_adapter import (
    parse_shadow_source_window,
    propose_shadow_quote_proto,
    serialize_shadow_selection,
)
from src.services.episodic_shadow_proposal import (
    SAFE_RESPONSE_PREFERENCES,
    InvalidShadowProposalError,
    ShadowGenerationDisabledError,
    ShadowQuoteSelection,
    ShadowSelectionOutput,
    propose_shadow_quote,
    validate_shadow_selection,
)

CONVERSATION_ID = "11111111-1111-4111-8111-111111111111"
USER_ID = "22222222-2222-4222-8222-222222222222"
MESSAGE_ID = "33333333-3333-4333-8333-333333333333"
ASSISTANT_ID = "44444444-4444-4444-8444-444444444444"
CONTEXT_USER_ID = "55555555-5555-4555-8555-555555555555"
CONTEXT_ASSISTANT_ID = "66666666-6666-4666-8666-666666666666"
POLICY_REVISION = "episodic-shadow-quote-v1"


def source_payload(text: str = "请简短回答。") -> dict[str, object]:
    return {
        "conversation_id": CONVERSATION_ID,
        "user_id": USER_ID,
        "from_seq": 1,
        "to_seq": 2,
        "policy_revision": POLICY_REVISION,
        "source_messages": [
            {"id": MESSAGE_ID, "seq": 1, "role": "user", "text": text},
            {"id": ASSISTANT_ID, "seq": 2, "role": "assistant", "text": "已收到。"},
        ],
    }


def contextual_payload(
    from_seq: int = 3, context_seqs: tuple[int, int] = (1, 2)
) -> dict[str, object]:
    return {
        "conversation_id": CONVERSATION_ID,
        "user_id": USER_ID,
        "from_seq": from_seq,
        "to_seq": from_seq + 1,
        "policy_revision": POLICY_REVISION,
        "source_messages": [
            {"id": MESSAGE_ID, "seq": from_seq, "role": "user", "text": "请简短回答。"},
            {"id": ASSISTANT_ID, "seq": from_seq + 1, "role": "assistant", "text": "已收到。"},
        ],
        "context_messages": [
            {
                "id": CONTEXT_USER_ID,
                "seq": context_seqs[0],
                "role": "user",
                "text": "上一轮合成问题。",
            },
            {
                "id": CONTEXT_ASSISTANT_ID,
                "seq": context_seqs[1],
                "role": "assistant",
                "text": "上一轮合成回答。",
            },
        ],
    }


def selected(
    *, quote: str = "请简短回答。", preference: str = "brief_answers", message_id: str = MESSAGE_ID
) -> ShadowSelectionOutput:
    return ShadowSelectionOutput(
        quotes=[
            ShadowQuoteSelection(
                source_message_id=UUID(message_id),
                quote=quote,
                preference=preference,
            )
        ]
    )


def test_typed_proposal_rejects_inconsistent_or_unknown_source_fields() -> None:
    assert parse_shadow_source_window(source_payload()).to_seq == 2
    for mutation in (
        lambda p: p.update(to_seq=3),
        lambda p: p.update(policy_revision="other"),
        lambda p: p["source_messages"][1].update(seq=3),
        lambda p: p["source_messages"][1].update(id=MESSAGE_ID),
        lambda p: p["source_messages"][0].update(text=" 请简短回答。"),
        lambda p: p["source_messages"][0].update(text="x" * 1201),
        lambda p: p["source_messages"][0].update(extra="injected"),
    ):
        payload = source_payload()
        mutation(payload)
        with pytest.raises(InvalidShadowProposalError):
            parse_shadow_source_window(payload)


def test_exact_whole_message_quote_and_versioned_proto_parity() -> None:
    window = parse_shadow_source_window(source_payload())
    actual = serialize_shadow_selection(window, selected())
    assert actual == {
        "policy_revision": POLICY_REVISION,
        "quotes": [
            {
                "source_message_id": MESSAGE_ID,
                "quote": "请简短回答。",
                "preference": "brief_answers",
            }
        ],
    }
    assert serialize_shadow_selection(window, ShadowSelectionOutput()) == {
        "policy_revision": POLICY_REVISION
    }
    for bad in (
        selected(quote="请分点回答。", preference="bullet_answers"),
        selected(quote="请简短回答。", preference="detailed_answers"),
        selected(message_id=ASSISTANT_ID),
        selected(message_id="55555555-5555-4555-8555-555555555555"),
    ):
        with pytest.raises(InvalidShadowProposalError):
            validate_shadow_selection(window, bad)

    with pytest.raises(ValidationError):
        ShadowSelectionOutput.model_validate({"quotes": [], "content_text": "伪造诊断"})


def test_context_is_bounded_untrusted_and_never_quotable() -> None:
    window = parse_shadow_source_window(contextual_payload())
    assert [item.seq for item in window.source_messages] == [3, 4]
    assert [item.seq for item in window.context_messages] == [1, 2]
    assert MESSAGE_ID in {item.id for item in window.source_messages}
    assert CONTEXT_USER_ID not in {item.id for item in window.source_messages}

    # Context can never be quoted, even when its text happens to match the
    # approved vocabulary.
    with pytest.raises(InvalidShadowProposalError):
        serialize_shadow_selection(
            window,
            selected(message_id=CONTEXT_USER_ID),
        )

    # Context must end immediately before from_seq.
    with pytest.raises(InvalidShadowProposalError):
        parse_shadow_source_window(contextual_payload(from_seq=4))
    # Context sequence must be < from_seq.
    with pytest.raises(InvalidShadowProposalError):
        parse_shadow_source_window(contextual_payload(from_seq=2))

    mutations = (
        # Source id cannot be reused as context.
        lambda p: p["context_messages"][0].update(id=MESSAGE_ID),
        # Padding changes the exact source text.
        lambda p: p["context_messages"][0].update(text=" 上一轮合成问题。 "),
    )
    for mutation in mutations:
        payload = contextual_payload()
        mutation(payload)
        with pytest.raises(InvalidShadowProposalError):
            parse_shadow_source_window(payload)

    too_many = contextual_payload(from_seq=4, context_seqs=(2, 3))
    too_many["context_messages"].insert(
        0,
        {
            "id": str(uuid5(NAMESPACE_URL, "third-context")),
            "seq": 1,
            "role": "user",
            "text": "更早的合成消息。",
        },
    )
    with pytest.raises(InvalidShadowProposalError):
        parse_shadow_source_window(too_many)


@pytest.mark.asyncio
async def test_testmodel_can_only_propose_valid_source_quote_when_opted_in() -> None:
    payload = source_payload()
    with pytest.raises(ShadowGenerationDisabledError):
        await propose_shadow_quote_proto(payload, model=TestModel(), enabled=False)
    with pytest.raises(ShadowGenerationDisabledError):
        await propose_shadow_quote_proto(payload, enabled=True)
    result = await propose_shadow_quote_proto(
        payload,
        model=TestModel(
            custom_output_args={
                "quotes": [
                    {
                        "source_message_id": MESSAGE_ID,
                        "quote": "请简短回答。",
                        "preference": "brief_answers",
                    }
                ]
            }
        ),
        enabled=True,
    )
    assert result["policy_revision"] == POLICY_REVISION
    assert result["quotes"][0]["quote"] == "请简短回答。"


def test_120_synthetic_claims_are_exact_with_no_clinical_vocabulary() -> None:
    """Structural source traceability, NOT a real-model clinical safety audit."""
    root = Path(__file__).resolve().parents[4]
    corpus = json.loads(
        (root / "contracts/internal/agent-runtime/fixtures/episodic-shadow-s2b.v1.json").read_text(
            encoding="utf-8"
        )
    )
    assert corpus["policy_revision"] == POLICY_REVISION
    assert len(corpus["accepted"]) == len(SAFE_RESPONSE_PREFERENCES)
    for entry in corpus["accepted"]:
        assert SAFE_RESPONSE_PREFERENCES[entry["quote"]] == (
            entry["preference"],
            entry["summary"],
        )
    accepted = 0
    for i in range(120):
        phrase = corpus["accepted"][i % len(corpus["accepted"])]
        payload = source_payload(phrase["quote"])
        mid = str(uuid5(NAMESPACE_URL, f"synthetic-memory-source-{i}"))
        payload["source_messages"][0]["id"] = mid
        window = parse_shadow_source_window(payload)
        result = serialize_shadow_selection(
            window, selected(quote=phrase["quote"], preference=phrase["preference"], message_id=mid)
        )
        assert len(result["quotes"]) == 1
        assert result["quotes"][0]["quote"] == window.source_messages[0].text
        accepted += 1
    assert accepted == 120
    for text in corpus["rejected"]:
        assert text not in SAFE_RESPONSE_PREFERENCES
        with pytest.raises(InvalidShadowProposalError):
            serialize_shadow_selection(
                parse_shadow_source_window(source_payload(text)),
                selected(quote=text, preference="brief_answers"),
            )


class CapturingTestModel(TestModel):
    """Records the settings the proposal actually hands to the model call."""

    def __init__(self, **kwargs: object) -> None:
        super().__init__(**kwargs)
        self.model_settings_seen: list[ModelSettings | None] = []

    async def request(
        self,
        messages: list[ModelMessage],
        model_settings: ModelSettings | None,
        model_request_parameters: ModelRequestParameters,
    ) -> ModelResponse:
        self.model_settings_seen.append(model_settings)
        return await super().request(messages, model_settings, model_request_parameters)


def proposal_model() -> CapturingTestModel:
    return CapturingTestModel(
        custom_output_args={
            "quotes": [
                {
                    "source_message_id": MESSAGE_ID,
                    "quote": "请简短回答。",
                    "preference": "brief_answers",
                }
            ]
        }
    )


async def captured_proposal_settings(
    monkeypatch: pytest.MonkeyPatch, value: str | None
) -> list[ModelSettings | None]:
    if value is None:
        monkeypatch.delenv("EPISODIC_PROPOSAL_MAX_TOKENS", raising=False)
    else:
        monkeypatch.setenv("EPISODIC_PROPOSAL_MAX_TOKENS", value)
    model = proposal_model()
    window = parse_shadow_source_window(source_payload())
    await propose_shadow_quote(window, model=model, enabled=True)
    assert model.model_settings_seen, "proposal never reached the model boundary"
    return model.model_settings_seen


@pytest.mark.asyncio
async def test_proposal_defaults_max_tokens_to_1024(monkeypatch: pytest.MonkeyPatch) -> None:
    captured = await captured_proposal_settings(monkeypatch, None)
    assert all(settings == {"temperature": 0.0, "max_tokens": 1024} for settings in captured)


@pytest.mark.asyncio
async def test_proposal_max_tokens_comes_from_env(monkeypatch: pytest.MonkeyPatch) -> None:
    captured = await captured_proposal_settings(monkeypatch, "777")
    assert all(settings == {"temperature": 0.0, "max_tokens": 777} for settings in captured)


@pytest.mark.asyncio
@pytest.mark.parametrize("invalid", ["", "   ", "not-a-number", "0", "-5"])
async def test_proposal_invalid_max_tokens_falls_back_to_default(
    monkeypatch: pytest.MonkeyPatch, invalid: str
) -> None:
    captured = await captured_proposal_settings(monkeypatch, invalid)
    assert all(settings == {"temperature": 0.0, "max_tokens": 1024} for settings in captured)
