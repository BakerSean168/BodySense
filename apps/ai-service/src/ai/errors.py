"""Custom exceptions for the AI provider system."""


class AIError(Exception):
    """Base exception for AI provider errors."""


class RateLimitError(AIError):
    """Raised when a provider returns 429."""


class ProviderError(AIError):
    """Raised for non-rate-limit provider errors."""

    def __init__(self, message: str, status_code: int | None = None):
        super().__init__(message)
        self.status_code = status_code


class NoAvailableProviderError(AIError):
    """Raised when all candidates for a use_case are unavailable."""


class ConfigError(AIError):
    """Raised for configuration errors."""
