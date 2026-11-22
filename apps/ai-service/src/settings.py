"""Unified environment settings for the AI service.

Single assembly point for database configuration. There is intentionally no
default password: a missing DB_PASSWORD fails fast instead of silently
connecting (or failing to connect) with hard-coded credentials.
"""

from __future__ import annotations

import os
from urllib.parse import quote_plus

__all__ = [
    "DatabaseSettingsError",
    "build_database_url",
    "embedding_api_key",
    "embedding_base_url",
    "embedding_dimensions",
    "embedding_model",
    "embedding_provider",
    "embedding_revision",
    "local_embedding_max_concurrency",
]


class DatabaseSettingsError(RuntimeError):
    """Raised when database configuration is incomplete or invalid."""


def _strip_driver_suffix(url: str) -> str:
    """Normalize SQLAlchemy-style driver suffixes (psycopg/asyncpg) away."""
    return url.replace("+asyncpg", "").replace("+psycopg", "")


def build_database_url(
    *, env: dict[str, str] | None = None, url_overrides: tuple[str, ...] = ()
) -> str:
    """Build the Postgres connection URL from environment configuration.

    ``env`` defaults to ``os.environ``; tests pass an explicit dict.

    ``url_overrides`` lists environment variables that provide a complete URL,
    in priority order — the first one that is set wins. Callers pass their
    own precedence chain (e.g. ``("LANGGRAPH_CHECKPOINTER_URL", "DATABASE_URL")``).

    Without a complete URL, the connection is assembled from DB_HOST/DB_PORT/
    DB_NAME/DB_USER/DB_PASSWORD. DB_PASSWORD has no default: when missing the
    assembly fails fast with an error naming DB_PASSWORD. The password (and
    every other component) is URL-escaped so values with special characters
    round-trip unchanged.
    """
    lookup = os.environ if env is None else env

    for var in url_overrides:
        env_url = lookup.get(var)
        if env_url:
            return _strip_driver_suffix(env_url)

    host = lookup.get("DB_HOST", "127.0.0.1")
    port = lookup.get("DB_PORT", "5432")
    name = lookup.get("DB_NAME", "bodysense")
    user = lookup.get("DB_USER", "bodysense")
    password = lookup.get("DB_PASSWORD")
    if not password:
        raise DatabaseSettingsError(
            "DB_PASSWORD is not set: refusing to build a database URL without "
            "credentials. Set DB_PASSWORD (or DATABASE_URL) in the environment."
        )
    return (
        f"postgresql://{quote_plus(user)}:{quote_plus(password)}"
        f"@{quote_plus(host)}:{quote_plus(port)}/{quote_plus(name)}"
    )


def embedding_provider(*, env: dict[str, str] | None = None) -> str:
    """Embedding backend selector ("hashing" by default)."""
    lookup = os.environ if env is None else env
    return lookup.get("EMBEDDING_PROVIDER", "hashing")


def embedding_model(*, env: dict[str, str] | None = None) -> str:
    """Model name for semantic embeddings."""
    lookup = os.environ if env is None else env
    return lookup.get("EMBEDDING_MODEL", "paraphrase-multilingual-MiniLM-L12-v2")


def embedding_dimensions(
    default: int = 1536, *, env: dict[str, str] | None = None
) -> int:
    """Vector dimension used to size and validate embeddings.

    The env var (when set) wins over the caller-provided ``default``, matching
    the historical ``os.getenv("EMBEDDING_DIMENSIONS", str(dimension))`` read.
    """
    lookup = os.environ if env is None else env
    return int(lookup.get("EMBEDDING_DIMENSIONS", str(default)))


def embedding_api_key(*, env: dict[str, str] | None = None) -> str | None:
    """API key for the OpenAI-compatible embedding endpoint.

    EMBEDDING_API_KEY wins; OPENAI_API_KEY is the legacy fallback.
    """
    lookup = os.environ if env is None else env
    return lookup.get("EMBEDDING_API_KEY") or lookup.get("OPENAI_API_KEY")


def embedding_base_url(*, env: dict[str, str] | None = None) -> str | None:
    """Base URL of the OpenAI-compatible embedding endpoint."""
    lookup = os.environ if env is None else env
    return lookup.get("EMBEDDING_BASE_URL")


def embedding_revision(*, env: dict[str, str] | None = None) -> str:
    """Deployed revision of the semantic embedding model (no default)."""
    lookup = os.environ if env is None else env
    return lookup.get("EMBEDDING_REVISION", "").strip()


def local_embedding_max_concurrency(*, env: dict[str, str] | None = None) -> int:
    """Worker-thread cap for local transformer inference (at least 1)."""
    lookup = os.environ if env is None else env
    return max(1, int(lookup.get("LOCAL_EMBEDDING_MAX_CONCURRENCY", "2")))
