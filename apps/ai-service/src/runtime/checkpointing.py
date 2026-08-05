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
from psycopg.rows import dict_row
from psycopg_pool import AsyncConnectionPool

logger = logging.getLogger(__name__)

# Opt-in switch for running without a durable checkpointer.
#
# The checkpointer holds the Agent Thread runtime truth (ADR 0002): message
# state, pending interrupts, and resume identity. An in-memory saver loses all
# of it on restart, which silently breaks interrupt/resume across requests.
# Degrading to it must therefore be an explicit local/CI decision, never an
# accident in a deployed environment — so initialization fails loudly by
# default and only falls back when this flag is set.
EPHEMERAL_CHECKPOINTER_ENV = "BODYSENSE_ALLOW_EPHEMERAL_CHECKPOINTER"

# Upper bound on the initial connection wait. Without it an unreachable database
# makes the pool retry indefinitely and startup hangs instead of reporting.
CONNECT_TIMEOUT_SECONDS = 5.0

_init_lock = asyncio.Lock()
_checkpointer: Any | None = None
_checkpointer_context: AsyncIterator[Any] | None = None
_checkpointer_pool: AsyncConnectionPool | None = None


class CheckpointerUnavailableError(RuntimeError):
    """Raised when no durable checkpointer can be initialized."""


def _ephemeral_fallback_allowed() -> bool:
    return os.getenv(EPHEMERAL_CHECKPOINTER_ENV, "").strip().lower() in {
        "1",
        "true",
        "yes",
    }


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
    global _checkpointer, _checkpointer_context, _checkpointer_pool
    if _checkpointer is not None:
        return _checkpointer

    async with _init_lock:
        if _checkpointer is not None:
            return _checkpointer

        database_url = _build_database_url()
        try:
            pool = AsyncConnectionPool(
                database_url,
                kwargs={"autocommit": True, "prepare_threshold": 0, "row_factory": dict_row},
                open=False,
            )
            # Bound the wait so an unreachable database surfaces as a startup
            # failure instead of retrying connections indefinitely.
            await pool.open(wait=True, timeout=CONNECT_TIMEOUT_SECONDS)
            checkpointer = AsyncPostgresSaver(pool)
            await checkpointer.setup()
            _checkpointer_context = None
            _checkpointer_pool = pool
            _checkpointer = checkpointer
            logger.info("Initialized LangGraph Postgres checkpointer")
        except Exception as exc:
            _checkpointer_context = None
            if _checkpointer_pool is not None:
                await _checkpointer_pool.close()
            _checkpointer_pool = None

            if not _ephemeral_fallback_allowed():
                logger.exception(
                    "Failed to initialize Postgres checkpointer. The Agent Thread "
                    "runtime truth (interrupt/resume state) requires durable "
                    "storage, so startup is aborted. Fix the database connection, "
                    "or set %s=true to explicitly accept an ephemeral in-memory "
                    "checkpointer (local development and CI only).",
                    EPHEMERAL_CHECKPOINTER_ENV,
                )
                raise CheckpointerUnavailableError(
                    "Postgres checkpointer unavailable and ephemeral fallback is not "
                    f"enabled; set {EPHEMERAL_CHECKPOINTER_ENV}=true to allow an "
                    "in-memory checkpointer."
                ) from exc

            logger.warning(
                "Postgres checkpointer unavailable (%s). Falling back to an "
                "in-memory checkpointer because %s is enabled. Agent Thread state "
                "is NOT durable: pending interrupts and resume identity are lost on "
                "restart. Never use this in a deployed environment.",
                exc,
                EPHEMERAL_CHECKPOINTER_ENV,
            )
            _checkpointer = InMemorySaver()

    return _checkpointer


async def get_runtime_checkpointer() -> Any:
    return await initialize_runtime_checkpointer()


async def shutdown_runtime_checkpointer() -> None:
    global _checkpointer, _checkpointer_context, _checkpointer_pool
    if _checkpointer_context is None and _checkpointer_pool is None:
        _checkpointer = None
        return

    if _checkpointer_context is not None:
        await _checkpointer_context.__aexit__(None, None, None)
    if _checkpointer_pool is not None:
        await _checkpointer_pool.close()
    _checkpointer_context = None
    _checkpointer_pool = None
    _checkpointer = None


@asynccontextmanager
async def runtime_checkpointer_lifespan():
    await initialize_runtime_checkpointer()
    try:
        yield
    finally:
        await shutdown_runtime_checkpointer()
