"""LangGraph checkpointer lifecycle for consultation threads."""

from __future__ import annotations

import asyncio
import logging
import os
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from typing import Any

from langgraph.checkpoint.memory import InMemorySaver
from langgraph.checkpoint.postgres.aio import AsyncPostgresSaver

logger = logging.getLogger(__name__)

_init_lock = asyncio.Lock()
_checkpointer: Any | None = None
_checkpointer_context: AsyncIterator[Any] | None = None


def _build_database_url() -> str:
    env_url = os.getenv("LANGGRAPH_CHECKPOINTER_URL") or os.getenv("DATABASE_URL")
    if env_url:
        return env_url.replace("+asyncpg", "").replace("+psycopg", "")

    host = os.getenv("DB_HOST", "127.0.0.1")
    port = os.getenv("DB_PORT", "5432")
    name = os.getenv("DB_NAME", "bodysense")
    user = os.getenv("DB_USER", "bodysense")
    password = os.getenv("DB_PASSWORD", "bodysense123")
    return f"postgresql://{user}:{password}@{host}:{port}/{name}"


async def initialize_runtime_checkpointer() -> Any:
    global _checkpointer, _checkpointer_context
    if _checkpointer is not None:
        return _checkpointer

    async with _init_lock:
        if _checkpointer is not None:
            return _checkpointer

        database_url = _build_database_url()
        try:
            context = AsyncPostgresSaver.from_conn_string(database_url)
            checkpointer = await context.__aenter__()
            await checkpointer.setup()
            _checkpointer_context = context
            _checkpointer = checkpointer
            logger.info("Initialized LangGraph Postgres checkpointer")
        except Exception:
            logger.exception(
                "Failed to initialize Postgres checkpointer; falling back to in-memory saver"
            )
            _checkpointer_context = None
            _checkpointer = InMemorySaver()

    return _checkpointer


async def get_runtime_checkpointer() -> Any:
    return await initialize_runtime_checkpointer()


async def shutdown_runtime_checkpointer() -> None:
    global _checkpointer, _checkpointer_context
    if _checkpointer_context is None:
        _checkpointer = None
        return

    await _checkpointer_context.__aexit__(None, None, None)
    _checkpointer_context = None
    _checkpointer = None


@asynccontextmanager
async def runtime_checkpointer_lifespan():
    await initialize_runtime_checkpointer()
    try:
        yield
    finally:
        await shutdown_runtime_checkpointer()
