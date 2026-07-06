"""OpenAI-compatible provider implementation."""
from __future__ import annotations

import json
import logging
from collections.abc import AsyncIterator
from typing import Any

import openai

from ..errors import ProviderError, RateLimitError
from ..types import AiRequest, AiResponse, AiStreamEvent, TokenUsage, ToolCall

logger = logging.getLogger(__name__)


class OpenAICompatibleProvider:
    def __init__(
        self,
        provider_id: str,
        model_id: str,
        base_url: str,
        api_key: str,
        capabilities: set[str],
    ):
        self._provider_id = provider_id
        self._model_id = model_id
        self._capabilities = capabilities
        self._client = openai.AsyncOpenAI(base_url=base_url, api_key=api_key)

    @property
    def id(self) -> str:
        return self._provider_id

    @property
    def capabilities(self) -> set[str]:
        return self._capabilities

    @property
    def model_id(self) -> str:
        return self._model_id

    async def generate(self, req: AiRequest) -> AiResponse:
        messages = self._convert_messages(req.messages)
        tools = self._convert_tools(req.tools) if req.tools else None

        kwargs: dict[str, Any] = {
            "model": self._model_id,
            "messages": messages,
        }
        if tools:
            kwargs["tools"] = tools
        if req.temperature is not None:
            kwargs["temperature"] = req.temperature
        if req.max_tokens is not None:
            kwargs["max_tokens"] = req.max_tokens
        if req.response_format == "json_object":
            kwargs["response_format"] = {"type": "json_object"}

        try:
            response = await self._client.chat.completions.create(**kwargs)
        except openai.RateLimitError as e:
            raise RateLimitError(str(e)) from e
        except openai.APIError as e:
            raise ProviderError(str(e), status_code=e.status_code) from e

        choice = response.choices[0]
        tool_calls = None
        if choice.message.tool_calls:
            tool_calls = self._parse_tool_calls(choice.message.tool_calls)

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
            provider=self._provider_id,
            usage=usage,
            finish_reason=choice.finish_reason,
            tool_calls=tool_calls,
            raw=response,
        )

    async def generate_stream(self, req: AiRequest) -> AsyncIterator[AiStreamEvent]:
        messages = self._convert_messages(req.messages)
        tools = self._convert_tools(req.tools) if req.tools else None

        kwargs: dict[str, Any] = {
            "model": self._model_id,
            "messages": messages,
            "stream": True,
        }
        if tools:
            kwargs["tools"] = tools
        if req.temperature is not None:
            kwargs["temperature"] = req.temperature
        if req.max_tokens is not None:
            kwargs["max_tokens"] = req.max_tokens
        if req.response_format == "json_object":
            kwargs["response_format"] = {"type": "json_object"}

        try:
            stream = await self._client.chat.completions.create(**kwargs)
        except openai.RateLimitError as e:
            raise RateLimitError(str(e)) from e
        except openai.APIError as e:
            raise ProviderError(str(e), status_code=e.status_code) from e

        # Accumulate tool call state
        tool_call_accumulators: dict[int, dict] = {}
        emitted_tool_call_ids: set[str] = set()
        finish_reason_emitted = False

        try:
            async for chunk in stream:
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
                        idx = tc_delta.index
                        if idx not in tool_call_accumulators:
                            tool_call_accumulators[idx] = {
                                "id": tc_delta.id or "",
                                "name": "",
                                "arguments": "",
                            }
                        acc = tool_call_accumulators[idx]
                        if tc_delta.id:
                            acc["id"] = tc_delta.id
                        if tc_delta.function:
                            if tc_delta.function.name:
                                acc["name"] = tc_delta.function.name
                            if tc_delta.function.arguments:
                                acc["arguments"] += tc_delta.function.arguments

                if choice.finish_reason and not finish_reason_emitted:
                    finish_reason_emitted = True
                    # Emit completed tool calls, skipping IDs already emitted
                    for idx in sorted(tool_call_accumulators.keys()):
                        acc = tool_call_accumulators[idx]
                        tc_id = acc["id"]
                        if tc_id and tc_id in emitted_tool_call_ids:
                            continue
                        if tc_id:
                            emitted_tool_call_ids.add(tc_id)
                        args = {}
                        try:
                            args = json.loads(acc["arguments"]) if acc["arguments"] else {}
                        except json.JSONDecodeError as e:
                            logger.warning(
                                "Failed to parse tool call arguments for %s: %s",
                                acc["name"],
                                e,
                            )
                        yield AiStreamEvent(
                            type="tool_call_done",
                            tool_call_id=tc_id,
                            tool_name=acc["name"],
                            tool_arguments=args,
                        )

                    yield AiStreamEvent(
                        type="done",
                        finish_reason=choice.finish_reason,
                    )
        except openai.RateLimitError as e:
            raise RateLimitError(str(e)) from e
        except openai.APIError as e:
            raise ProviderError(str(e), status_code=e.status_code) from e

    async def health_check(self) -> bool:
        try:
            await self._client.models.list()
            return True
        except Exception:
            return False

    def _convert_messages(self, messages: list) -> list[dict]:
        result = []
        for msg in messages:
            m: dict[str, Any] = {"role": msg.role, "content": msg.content}
            if msg.tool_calls:
                m["tool_calls"] = [
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
                m["tool_call_id"] = msg.tool_call_id
            result.append(m)
        return result

    def _convert_tools(self, tools: list) -> list[dict]:
        return [
            {
                "type": "function",
                "function": {
                    "name": t.name,
                    "description": t.description,
                    "parameters": t.parameters,
                },
            }
            for t in tools
        ]

    def _parse_tool_calls(self, tool_calls) -> list[ToolCall]:
        result = []
        for tc in tool_calls:
            args = {}
            if tc.function.arguments:
                try:
                    args = json.loads(tc.function.arguments)
                except json.JSONDecodeError as e:
                    logger.warning(
                        "Failed to parse tool call arguments for %s: %s",
                        tc.function.name,
                        e,
                    )
            result.append(ToolCall(
                id=tc.id,
                name=tc.function.name,
                arguments=args,
            ))
        return result
