"""Provider-neutral AI contracts backed by the internal LiteLLM gateway."""

from .errors import (
    AIError,
    GatewayError,
    GatewayProtocolError,
    GatewayRateLimitError,
    GatewayUnavailableError,
)
from .service import AIService
from .types import (
    AiDoneEvent,
    AiRequest,
    AiResponse,
    AiStreamEvent,
    AiTextDeltaEvent,
    AiToolCallDoneEvent,
    AiUsageEvent,
    TokenUsage,
)

__all__ = [
    "AIError",
    "AIService",
    "AiDoneEvent",
    "AiRequest",
    "AiResponse",
    "AiStreamEvent",
    "AiTextDeltaEvent",
    "AiToolCallDoneEvent",
    "AiUsageEvent",
    "GatewayError",
    "GatewayProtocolError",
    "GatewayRateLimitError",
    "GatewayUnavailableError",
    "TokenUsage",
]
