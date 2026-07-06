"""Model router with circuit breaker."""
from __future__ import annotations

import logging
import time

from .config import RouterConfig
from .errors import NoAvailableProviderError
from .providers.base import AiProvider

logger = logging.getLogger(__name__)

_COOLDOWN_BASE = 60.0  # seconds
_COOLDOWN_MAX = 480.0  # max backoff cap


class ModelRouter:
    def __init__(self, config: RouterConfig, providers: dict[str, AiProvider]):
        self._config = config
        self._providers = providers  # "provider_id/model_id" -> provider instance
        # key -> (open_until: float, failure_count: int)
        self._circuit_breaker: dict[str, tuple[float, int]] = {}

    def _cleanup_expired(self) -> None:
        """Remove expired circuit breaker entries."""
        now = time.monotonic()
        expired = [k for k, (until, _) in self._circuit_breaker.items() if until <= now]
        for k in expired:
            del self._circuit_breaker[k]

    def is_open(self, provider_id: str, model_id: str) -> bool:
        """Check if the circuit breaker is open for a provider/model."""
        key = f"{provider_id}/{model_id}"
        entry = self._circuit_breaker.get(key)
        if entry is None:
            return False
        open_until, _ = entry
        if open_until <= time.monotonic():
            del self._circuit_breaker[key]
            return False
        return True

    def select(
        self, use_case: str, required_capabilities: set[str] | None = None
    ) -> AiProvider:
        """Select a provider for the given use_case."""
        route = self._config.routes.get(use_case)
        if not route:
            raise NoAvailableProviderError(f"No route configured for use_case: {use_case}")

        self._cleanup_expired()
        now = time.monotonic()

        for candidate in route.candidates:
            key = f"{candidate.provider}/{candidate.model}"

            # Check circuit breaker
            entry = self._circuit_breaker.get(key)
            if entry and entry[0] > now:
                logger.debug(f"Circuit breaker active for {key}, skipping")
                continue

            # Check capabilities
            provider = self._providers.get(key)
            if not provider:
                logger.warning(f"Provider {key} not found")
                continue

            if required_capabilities and not required_capabilities.issubset(provider.capabilities):
                logger.debug(f"Provider {key} missing capabilities {required_capabilities}")
                continue

            return provider

        raise NoAvailableProviderError(f"All candidates for {use_case} are unavailable")

    def get_provider(self, provider_id: str, model_id: str) -> AiProvider:
        key = f"{provider_id}/{model_id}"
        provider = self._providers.get(key)
        if not provider:
            raise NoAvailableProviderError(f"Provider {key} not found")
        return provider

    def trip_breaker(self, provider_id: str, model_id: str) -> None:
        key = f"{provider_id}/{model_id}"
        _, prev_count = self._circuit_breaker.get(key, (0.0, 0))
        new_count = prev_count + 1
        # Exponential backoff: 60s, 120s, 240s, capped at 480s
        cooldown = min(_COOLDOWN_BASE * (2 ** prev_count), _COOLDOWN_MAX)
        self._circuit_breaker[key] = (time.monotonic() + cooldown, new_count)
        logger.warning(
            "Circuit breaker tripped for %s (failure #%d, cooldown=%.0fs)",
            key,
            new_count,
            cooldown,
        )
