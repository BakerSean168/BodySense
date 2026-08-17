"""AiProvider protocol definition."""

from __future__ import annotations

from collections.abc import AsyncIterator
from typing import Protocol

from ..types import AiRequest, AiResponse, AiStreamEvent


class AiProvider(Protocol):
    @property
    def id(self) -> str: ...

    @property
    def capabilities(self) -> set[str]: ...

    @property
    def model_id(self) -> str: ...

    async def generate(self, req: AiRequest) -> AiResponse: ...
    def generate_stream(self, req: AiRequest) -> AsyncIterator[AiStreamEvent]: ...
    async def health_check(self) -> bool: ...
