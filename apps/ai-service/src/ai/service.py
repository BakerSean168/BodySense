"""Provider-neutral LLM service backed exclusively by the internal LiteLLM gateway."""

from __future__ import annotations

import json
import logging
from collections.abc import AsyncIterator
from typing import Any, cast

import openai

from ..testing_support.deterministic_ai import (
    deterministic_ai_enabled,
    deterministic_text_for,
    deterministic_usage,
)
from .errors import GatewayError, GatewayRateLimitError, GatewayUnavailableError
from .gateway import gateway_credentials, gateway_route
from .types import AiRequest, AiResponse, AiStreamEvent, TokenUsage, ToolCall

logger = logging.getLogger(__name__)


class AIService:
    """Preserve the business-facing request contract while centralizing routing in LiteLLM."""

    def __init__(self) -> None:
        self._client: openai.AsyncOpenAI | None = None
        if not deterministic_ai_enabled():
            base_url, api_key = gateway_credentials()
            self._client = openai.AsyncOpenAI(base_url=base_url, api_key=api_key)

    async def generate(self, req: AiRequest) -> AiResponse:
        if deterministic_ai_enabled():
            usage = TokenUsage(**deterministic_usage())
            return AiResponse(
                text=deterministic_text_for(req.use_case),
                model="deterministic-local",
                provider="bodysense-test",
                usage=usage,
                finish_reason="stop",
            )

        route = gateway_route(req.use_case)
        client = self._require_client()
        merged = self._apply_defaults(req)
        kwargs = self._request_kwargs(merged, route.logical_model)
        try:
            response = await client.chat.completions.create(**kwargs)
        except openai.RateLimitError as exc:
            raise GatewayRateLimitError(str(exc)) from exc
        except openai.APIError as exc:
            raise GatewayError(str(exc), status_code=getattr(exc, "status_code", None)) from exc

        choice = response.choices[0]
        usage = None
        if response.usage:
            usage = TokenUsage(
                input_tokens=response.usage.prompt_tokens,
                output_tokens=response.usage.completion_tokens,
                total_tokens=response.usage.total_tokens,
            )
        return AiResponse(
            text=choice.message.content or "",
            model=response.model,
            provider="litellm-gateway",
            usage=usage,
            finish_reason=choice.finish_reason,
            tool_calls=self._parse_tool_calls(choice.message.tool_calls),
            raw=response,
        )

    async def generate_stream(self, req: AiRequest) -> AsyncIterator[AiStreamEvent]:
        if deterministic_ai_enabled():
            text = deterministic_text_for(req.use_case)
            midpoint = max(1, len(text) // 2)
            for chunk in (text[:midpoint], text[midpoint:]):
                if chunk:
                    yield AiStreamEvent(type="text_delta", text=chunk)
            yield AiStreamEvent(type="usage", usage=TokenUsage(**deterministic_usage()))
            yield AiStreamEvent(type="done", finish_reason="stop")
            return

        route = gateway_route(req.use_case)
        client = self._require_client()
        merged = self._apply_defaults(req)
        merged.stream = True
        kwargs = self._request_kwargs(merged, route.logical_model)
        kwargs["stream"] = True
        kwargs["stream_options"] = {"include_usage": True}
        try:
            stream = await client.chat.completions.create(**kwargs)
        except openai.RateLimitError as exc:
            raise GatewayRateLimitError(str(exc)) from exc
        except openai.APIError as exc:
            raise GatewayError(str(exc), status_code=getattr(exc, "status_code", None)) from exc

        tool_call_accumulators: dict[int, dict[str, str]] = {}
        finish_reason_emitted = False
        try:
            async for chunk in cast(Any, stream):
                if not chunk.choices:
                    if chunk.usage:
                        yield AiStreamEvent(
                            type="usage",
                            usage=TokenUsage(
                                input_tokens=chunk.usage.prompt_tokens,
                                output_tokens=chunk.usage.completion_tokens,
                                total_tokens=chunk.usage.total_tokens,
                            ),
                        )
                    continue
                choice = chunk.choices[0]
                delta = choice.delta
                if delta.content:
                    yield AiStreamEvent(type="text_delta", text=delta.content)
                if delta.tool_calls:
                    for tc_delta in delta.tool_calls:
                        accumulator = tool_call_accumulators.setdefault(
                            tc_delta.index,
                            {"id": "", "name": "", "arguments": ""},
                        )
                        if tc_delta.id:
                            accumulator["id"] = tc_delta.id
                        if tc_delta.function:
                            if tc_delta.function.name:
                                accumulator["name"] = tc_delta.function.name
                            if tc_delta.function.arguments:
                                accumulator["arguments"] += tc_delta.function.arguments
                if choice.finish_reason and not finish_reason_emitted:
                    finish_reason_emitted = True
                    for index in sorted(tool_call_accumulators):
                        accumulator = tool_call_accumulators[index]
                        arguments: dict[str, Any] = {}
                        if accumulator["arguments"]:
                            try:
                                arguments = json.loads(accumulator["arguments"])
                            except json.JSONDecodeError:
                                logger.warning(
                                    "Failed to parse tool call arguments for %s",
                                    accumulator["name"],
                                )
                        yield AiStreamEvent(
                            type="tool_call_done",
                            tool_call_id=accumulator["id"],
                            tool_name=accumulator["name"],
                            tool_arguments=arguments,
                        )
                    yield AiStreamEvent(type="done", finish_reason=choice.finish_reason)
        except openai.RateLimitError as exc:
            raise GatewayRateLimitError(str(exc)) from exc
        except openai.APIError as exc:
            raise GatewayError(str(exc), status_code=getattr(exc, "status_code", None)) from exc

    def _require_client(self) -> openai.AsyncOpenAI:
        if self._client is None:
            raise GatewayUnavailableError("LiteLLM gateway client is not configured")
        return self._client

    def _apply_defaults(self, req: AiRequest) -> AiRequest:
        route = gateway_route(req.use_case)
        return AiRequest(
            use_case=req.use_case,
            messages=req.messages,
            tools=req.tools,
            stream=req.stream,
            response_format=req.response_format or route.response_format,
            temperature=req.temperature if req.temperature is not None else route.temperature,
            max_tokens=req.max_tokens if req.max_tokens is not None else route.max_tokens,
            metadata=req.metadata,
        )

    def _request_kwargs(self, req: AiRequest, logical_model: str) -> dict[str, Any]:
        kwargs: dict[str, Any] = {
            "model": logical_model,
            "messages": self._convert_messages(req.messages),
        }
        if req.tools:
            kwargs["tools"] = self._convert_tools(req.tools)
        if req.temperature is not None:
            kwargs["temperature"] = req.temperature
        if req.max_tokens is not None:
            kwargs["max_tokens"] = req.max_tokens
        if req.response_format == "json_object":
            kwargs["response_format"] = {"type": "json_object"}
        return kwargs

    @staticmethod
    def _convert_messages(messages: list[Any]) -> list[dict[str, Any]]:
        result: list[dict[str, Any]] = []
        for msg in messages:
            item: dict[str, Any] = {"role": msg.role, "content": msg.content}
            if msg.tool_calls:
                item["tool_calls"] = [
                    {
                        "id": tc.id,
                        "type": "function",
                        "function": {
                            "name": tc.name,
                            "arguments": json.dumps(tc.arguments, ensure_ascii=False),
                        },
                    }
                    for tc in msg.tool_calls
                ]
            if msg.tool_call_id:
                item["tool_call_id"] = msg.tool_call_id
            result.append(item)
        return result

    @staticmethod
    def _convert_tools(tools: list[Any]) -> list[dict[str, Any]]:
        return [
            {
                "type": "function",
                "function": {
                    "name": tool.name,
                    "description": tool.description,
                    "parameters": tool.parameters,
                },
            }
            for tool in tools
        ]

    @staticmethod
    def _parse_tool_calls(tool_calls: Any) -> list[ToolCall] | None:
        if not tool_calls:
            return None
        result: list[ToolCall] = []
        for tool_call in tool_calls:
            arguments: dict[str, Any] = {}
            if tool_call.function.arguments:
                try:
                    arguments = json.loads(tool_call.function.arguments)
                except json.JSONDecodeError:
                    logger.warning(
                        "Failed to parse tool call arguments for %s",
                        tool_call.function.name,
                    )
            result.append(
                ToolCall(
                    id=tool_call.id,
                    name=tool_call.function.name,
                    arguments=arguments,
                )
            )
        return result
