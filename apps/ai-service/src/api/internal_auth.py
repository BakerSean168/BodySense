"""Inbound shared-token authentication for the internal HTTP APIs.

Both FastAPI apps (the main AI service in ``src.main`` and the
health-document runtime in ``src.document_main``) mount
``InternalTokenMiddleware`` so every business route requires the caller to
present the shared internal token in the ``X-Internal-Token`` header. The
Go API is the only legitimate caller and sends the header on every
ai-service request (see apps/api/internal/service/ai_client.go).

Backward compatibility: the token comes from ``AI_SERVICE_INTERNAL_TOKEN``
via ``src.settings``. While the variable is unset the middleware lets
every request through, so an existing deployment keeps working until the
same token has been configured on both ends; ``log_auth_configuration``
makes that state loud at startup instead of silent.
"""

from __future__ import annotations

import hmac
import logging

from starlette.requests import Request
from starlette.responses import JSONResponse
from starlette.types import ASGIApp, Receive, Scope, Send

from ..settings import internal_token

logger = logging.getLogger(__name__)

INTERNAL_TOKEN_HEADER = "x-internal-token"

# Paths reachable without the token, on both apps. /health is the
# deployment probe surface; everything else is a business route and is
# covered whenever a token is configured.
EXEMPT_PATHS = frozenset({"/health"})


def log_auth_configuration(service: str) -> None:
    """Log whether inbound internal authentication is active."""
    if internal_token() is None:
        logger.warning(
            "内部鉴权未启用 (internal token auth disabled) for %s: "
            "AI_SERVICE_INTERNAL_TOKEN is not set, so all routes are "
            "reachable without authentication. Configure the same token "
            "on this service and the Go API to enable it.",
            service,
        )
    else:
        logger.info("Internal token authentication enabled for %s.", service)


class InternalTokenMiddleware:
    """Pure ASGI middleware enforcing the shared internal token.

    The token is read per request so configuration changes and tests do
    not need a process restart or module reload. The comparison is
    constant time, and streaming responses pass through untouched.
    """

    def __init__(self, app: ASGIApp) -> None:
        self.app = app

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return
        expected = internal_token()
        if expected is not None and scope.get("path") not in EXEMPT_PATHS:
            provided = Request(scope).headers.get(INTERNAL_TOKEN_HEADER, "")
            if not hmac.compare_digest(provided, expected):
                response = JSONResponse(status_code=401, content={"detail": "unauthorized"})
                await response(scope, receive, send)
                return
        await self.app(scope, receive, send)
