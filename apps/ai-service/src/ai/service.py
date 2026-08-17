"""AIService - unified entry point for all LLM calls."""

from __future__ import annotations

import logging
import os
from collections.abc import AsyncIterator
from pathlib import Path
from typing import cast

from ..testing_support.deterministic_ai import (
    deterministic_ai_enabled,
    deterministic_text_for,
    deterministic_usage,
)
from .config import RouteDefaults, load_config
from .errors import NoAvailableProviderError, ProviderError, RateLimitError
from .providers.base import AiProvider
from .providers.openai_compatible import OpenAICompatibleProvider
from .router import ModelRouter
from .types import AiRequest, AiResponse, AiStreamEvent, TokenUsage

logger = logging.getLogger(__name__)

_DEFAULT_CONFIG_PATH = Path(__file__).parent.parent / "config" / "models.yaml"


class AIService:
    def __init__(self, config_path: str | Path | None = None):
        if config_path is None:
            config_path = os.environ.get("AI_CONFIG_PATH", _DEFAULT_CONFIG_PATH)
        self._config = load_config(config_path)
        self._providers: dict[str, AiProvider] = {}
        self._init_providers()
        self._router = ModelRouter(self._config, self._providers)

    def _init_providers(self) -> None:
        for provider_id, provider_cfg in self._config.providers.items():
            for model_cfg in provider_cfg.models:
                key = f"{provider_id}/{model_cfg.id}"
                if provider_cfg.type == "openai-compatible":
                    self._providers[key] = OpenAICompatibleProvider(
                        provider_id=provider_id,
                        model_id=model_cfg.id,
                        base_url=provider_cfg.base_url,
                        api_key=provider_cfg.api_key,
                        capabilities=model_cfg.capabilities,
                    )

    async def generate(self, req: AiRequest) -> AiResponse:
        """Non-streaming call with auto fallback."""
        if deterministic_ai_enabled():
            usage = TokenUsage(**deterministic_usage())
            return AiResponse(
                text=deterministic_text_for(req.use_case),
                model="deterministic-local",
                provider="bodysense-test",
                usage=usage,
                finish_reason="stop",
            )
        route = self._config.routes.get(req.use_case)
        if not route:
            raise NoAvailableProviderError(f"No route for {req.use_case}")

        merged = self._apply_defaults(req, route.defaults)
        last_error = None

        for candidate in route.candidates:
            # Skip candidates whose circuit breaker is open
            if self._router.is_open(candidate.provider, candidate.model):
                continue
            try:
                provider = self._router.get_provider(candidate.provider, candidate.model)
                return await provider.generate(merged)
            except RateLimitError as e:
                self._router.trip_breaker(candidate.provider, candidate.model)
                last_error = e
                continue
            except (ProviderError, NoAvailableProviderError) as e:
                # Only trip breaker for rate limit; config errors (400/401) are not retryable
                last_error = e
                continue

        raise NoAvailableProviderError(f"All candidates for {req.use_case} failed") from last_error

    async def generate_stream(self, req: AiRequest) -> AsyncIterator[AiStreamEvent]:
        """Streaming call with auto fallback (only before first chunk)."""
        if deterministic_ai_enabled():
            text = deterministic_text_for(req.use_case)
            midpoint = max(1, len(text) // 2)
            for chunk in (text[:midpoint], text[midpoint:]):
                if chunk:
                    yield AiStreamEvent(type="text_delta", text=chunk)
            yield AiStreamEvent(type="usage", usage=TokenUsage(**deterministic_usage()))
            yield AiStreamEvent(type="done", finish_reason="stop")
            return
        route = self._config.routes.get(req.use_case)
        if not route:
            raise NoAvailableProviderError(f"No route for {req.use_case}")

        merged = self._apply_defaults(req, route.defaults)
        merged.stream = True
        last_error = None

        for candidate in route.candidates:
            # Skip candidates whose circuit breaker is open
            if self._router.is_open(candidate.provider, candidate.model):
                continue
            try:
                provider = self._router.get_provider(candidate.provider, candidate.model)
                async for event in cast(
                    "AsyncIterator[AiStreamEvent]", provider.generate_stream(merged)
                ):
                    yield event
                return
            except RateLimitError as e:
                self._router.trip_breaker(candidate.provider, candidate.model)
                last_error = e
                continue
            except (ProviderError, NoAvailableProviderError) as e:
                # Only trip breaker for rate limit; config errors (400/401) are not retryable
                last_error = e
                continue

        raise NoAvailableProviderError(f"All candidates for {req.use_case} failed") from last_error

    def _apply_defaults(self, req: AiRequest, defaults: RouteDefaults) -> AiRequest:
        return AiRequest(
            use_case=req.use_case,
            messages=req.messages,
            tools=req.tools,
            stream=req.stream,
            response_format=req.response_format or defaults.response_format,
            temperature=req.temperature if req.temperature is not None else defaults.temperature,
            max_tokens=req.max_tokens if req.max_tokens is not None else defaults.max_tokens,
            metadata=req.metadata,
        )
