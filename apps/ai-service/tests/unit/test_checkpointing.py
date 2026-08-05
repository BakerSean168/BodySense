"""Checkpointer fail-fast / explicit ephemeral opt-in tests."""

from __future__ import annotations

import pytest

from src.runtime import checkpointing
from src.runtime.checkpointing import (
    EPHEMERAL_CHECKPOINTER_ENV,
    CheckpointerUnavailableError,
    initialize_runtime_checkpointer,
    shutdown_runtime_checkpointer,
)


@pytest.fixture(autouse=True)
async def _reset_checkpointer_globals():
    """Ensure each test starts from a clean module-level checkpointer state."""
    await shutdown_runtime_checkpointer()
    checkpointing._checkpointer = None
    checkpointing._checkpointer_context = None
    checkpointing._checkpointer_pool = None
    yield
    await shutdown_runtime_checkpointer()
    checkpointing._checkpointer = None
    checkpointing._checkpointer_context = None
    checkpointing._checkpointer_pool = None


@pytest.mark.asyncio
async def test_initialize_fails_loudly_without_ephemeral_opt_in(monkeypatch):
    """Production-like default: unreachable DB aborts startup, no silent memory."""
    monkeypatch.delenv(EPHEMERAL_CHECKPOINTER_ENV, raising=False)
    monkeypatch.setenv("LANGGRAPH_CHECKPOINTER_URL", "postgresql://no-such-host:1/none")

    class _BoomPool:
        def __init__(self, *args, **kwargs):
            pass

        async def open(self, *args, **kwargs):
            raise OSError("connection refused")

        async def close(self):
            return None

    monkeypatch.setattr(checkpointing, "AsyncConnectionPool", _BoomPool)

    with pytest.raises(CheckpointerUnavailableError) as exc_info:
        await initialize_runtime_checkpointer()

    assert EPHEMERAL_CHECKPOINTER_ENV in str(exc_info.value)
    assert checkpointing._checkpointer is None


@pytest.mark.asyncio
async def test_initialize_ephemeral_only_with_explicit_opt_in(monkeypatch):
    """Local/CI may opt into InMemorySaver, but only via the explicit env flag."""
    monkeypatch.setenv(EPHEMERAL_CHECKPOINTER_ENV, "true")
    monkeypatch.setenv("LANGGRAPH_CHECKPOINTER_URL", "postgresql://no-such-host:1/none")

    class _BoomPool:
        def __init__(self, *args, **kwargs):
            pass

        async def open(self, *args, **kwargs):
            raise OSError("connection refused")

        async def close(self):
            return None

    monkeypatch.setattr(checkpointing, "AsyncConnectionPool", _BoomPool)

    saver = await initialize_runtime_checkpointer()
    assert saver is not None
    # InMemorySaver is the only non-Postgres fallback allowed under the flag.
    assert type(saver).__name__ == "InMemorySaver"


@pytest.mark.asyncio
async def test_ephemeral_flag_falsey_values_still_fail_fast(monkeypatch):
    monkeypatch.setenv(EPHEMERAL_CHECKPOINTER_ENV, "0")
    monkeypatch.setenv("LANGGRAPH_CHECKPOINTER_URL", "postgresql://no-such-host:1/none")

    class _BoomPool:
        def __init__(self, *args, **kwargs):
            pass

        async def open(self, *args, **kwargs):
            raise OSError("connection refused")

        async def close(self):
            return None

    monkeypatch.setattr(checkpointing, "AsyncConnectionPool", _BoomPool)

    with pytest.raises(CheckpointerUnavailableError):
        await initialize_runtime_checkpointer()
