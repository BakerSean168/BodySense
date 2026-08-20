"""Provider-neutral AI contracts backed by the internal LiteLLM gateway."""

from .errors import AIError, GatewayError, GatewayRateLimitError, GatewayUnavailableError
from .service import AIService
from .types import AiRequest, AiResponse, AiStreamEvent, TokenUsage

__all__ = [
    "AIError",
    "AIService",
    "AiRequest",
    "AiResponse",
    "AiStreamEvent",
    "GatewayError",
    "GatewayRateLimitError",
    "GatewayUnavailableError",
    "TokenUsage",
]
