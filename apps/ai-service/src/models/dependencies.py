"""Shared run-dependency protocols for typed Agents.

The protocol lives with execution contracts rather than concrete Agent tools so
models do not depend on the agent implementation package.
"""

from __future__ import annotations

from typing import Any, Protocol


class EvidenceSearcher(Protocol):
    async def search(self, query: str, *, top_k: int = 5) -> list[dict[str, Any]]: ...
