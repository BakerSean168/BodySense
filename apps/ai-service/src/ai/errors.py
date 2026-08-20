"""Errors exposed by the internal LiteLLM gateway transport."""


class AIError(Exception):
    """Base error for model-gateway operations."""


class GatewayRateLimitError(AIError):
    """The internal gateway returned a rate-limit response."""


class GatewayError(AIError):
    """The internal gateway returned an API error."""

    def __init__(self, message: str, status_code: int | None = None):
        super().__init__(message)
        self.status_code = status_code


class GatewayUnavailableError(AIError):
    """The internal gateway or requested logical route is unavailable."""
