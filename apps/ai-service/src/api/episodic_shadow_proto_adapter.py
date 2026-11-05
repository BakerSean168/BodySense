"""Typed Proto adapter for S2-B proposals; transport never enters PydanticAI.

Private cross-language data is validated here, handed to an isolated
application service as ordinary dataclasses, and revalidated on return.
No HTTP route, auth bypass, runtime hookup or model-provider fallback.
"""

from __future__ import annotations

from google.protobuf import json_format
from protovalidate import ValidationError, Validator
from pydantic_ai.models import Model

from ..generated.runtimeproto.bodysense.runtime.v1 import runtime_pb2
from ..services.episodic_shadow_proposal import (
    CONTEXT_MAX_CHARS,
    POLICY_REVISION,
    SOURCE_MAX_CHARS,
    InvalidShadowProposalError,
    ShadowGenerationDisabledError,
    ShadowSelectionOutput,
    ShadowSourceMessage,
    ShadowSourceWindow,
    propose_shadow_quote,
    validate_shadow_selection,
)


def parse_shadow_source_window(payload: dict[str, object]) -> ShadowSourceWindow:
    command = runtime_pb2.EpisodicShadowGenerationCommand()
    try:
        json_format.ParseDict(payload, command, ignore_unknown_fields=False)
        Validator().validate(command)
    except (json_format.ParseError, ValidationError, ValueError, TypeError) as exc:
        raise InvalidShadowProposalError("invalid typed shadow generation command") from exc

    sources = command.source_messages
    if (
        command.policy_revision != POLICY_REVISION
        or command.to_seq < command.from_seq
        or command.to_seq - command.from_seq + 1 != len(sources)
        or sum(len(item.text) for item in sources) > SOURCE_MAX_CHARS
        or len({item.id for item in sources}) != len(sources)
    ):
        raise InvalidShadowProposalError("source window is not bounded and contiguous")
    for index, source in enumerate(sources):
        if source.seq != command.from_seq + index or source.text != source.text.strip():
            raise InvalidShadowProposalError("source window changed ordering or padding")

    context = command.context_messages
    source_ids = {item.id for item in sources}
    if (
        len(context) > 2
        or sum(len(item.text) for item in context) > CONTEXT_MAX_CHARS
        or any(item.seq >= command.from_seq for item in context)
        or (context and context[-1].seq != command.from_seq - 1)
        or any(
            context[index].seq + 1 != context[index + 1].seq for index in range(len(context) - 1)
        )
        or len({item.id for item in context}) != len(context)
        or any(item.id in source_ids for item in context)
        or any(item.text != item.text.strip() for item in context)
    ):
        raise InvalidShadowProposalError("context window is not bounded and contiguous")

    return ShadowSourceWindow(
        conversation_id=command.conversation_id,
        user_id=command.user_id,
        from_seq=command.from_seq,
        to_seq=command.to_seq,
        policy_revision=command.policy_revision,
        source_messages=tuple(
            ShadowSourceMessage(id=s.id, seq=s.seq, role=s.role, text=s.text) for s in sources
        ),
        context_messages=tuple(
            ShadowSourceMessage(id=s.id, seq=s.seq, role=s.role, text=s.text) for s in context
        ),
    )


def serialize_shadow_selection(
    window: ShadowSourceWindow,
    selection: ShadowSelectionOutput,
) -> dict[str, object]:
    validated = validate_shadow_selection(window, selection)
    wire = runtime_pb2.EpisodicShadowDraftProposal(policy_revision=POLICY_REVISION)
    for claim in validated.quotes:
        wire.quotes.add(
            source_message_id=str(claim.source_message_id),
            quote=claim.quote,
            preference=claim.preference,
        )
    try:
        Validator().validate(wire)
    except ValidationError as exc:
        raise InvalidShadowProposalError("invalid typed shadow quote proposal") from exc
    return json_format.MessageToDict(wire, preserving_proto_field_name=True)


async def propose_shadow_quote_proto(
    payload: dict[str, object],
    *,
    model: Model | None = None,
    enabled: bool = False,
) -> dict[str, object]:
    if not enabled:
        raise ShadowGenerationDisabledError("S2-B shadow model proposal is not enabled")
    if model is None:
        raise ShadowGenerationDisabledError("S2-B requires an explicitly injected model")
    window = parse_shadow_source_window(payload)
    selection = await propose_shadow_quote(window, model=model, enabled=enabled)
    return serialize_shadow_selection(window, selection)
