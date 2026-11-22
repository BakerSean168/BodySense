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
    "asr_api_key",
    "asr_omni_base_url",
    "asr_omni_model",
    "asr_provider",
    "asr_transcription_base_url",
    "asr_transcription_model",
    "asr_whisper_model",
    "build_database_url",
    "embedding_api_key",
    "embedding_base_url",
    "embedding_dimensions",
    "embedding_model",
    "embedding_provider",
    "embedding_revision",
    "cors_origins",
    "deterministic_ai_enabled",
    "e2e_stub_ai_enabled",
    "ephemeral_checkpointer_allowed",
    "episodic_proposal_max_tokens",
    "gateway_api_key",
    "gateway_base_url",
    "health_document_model_root",
    "knowledge_data_root",
    "knowledge_pool_max_size",
    "knowledge_pool_min_size",
    "local_embedding_max_concurrency",
    "pose_model_path_override",
    "splitter_provider",
    "tessdata_prefix",
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


def embedding_dimensions(default: int = 1536, *, env: dict[str, str] | None = None) -> int:
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


def asr_provider(*, env: dict[str, str] | None = None) -> str:
    """ASR provider selector ("whisper.cpp" by default)."""
    lookup = os.environ if env is None else env
    return lookup.get("ASR_PROVIDER", "whisper.cpp")


def asr_whisper_model(*, env: dict[str, str] | None = None) -> str:
    """whisper.cpp model file name ("ggml-base.bin" by default)."""
    lookup = os.environ if env is None else env
    return lookup.get("ASR_WHISPER_MODEL", "ggml-base.bin")


def asr_api_key(*, env: dict[str, str] | None = None) -> str | None:
    """API key for the remote ASR endpoints.

    ASR_API_KEY wins; OPENAI_API_KEY is the legacy fallback.
    """
    lookup = os.environ if env is None else env
    return lookup.get("ASR_API_KEY") or lookup.get("OPENAI_API_KEY")


def asr_transcription_base_url(*, env: dict[str, str] | None = None) -> str | None:
    """Base URL of the OpenAI-compatible audio transcription endpoint."""
    lookup = os.environ if env is None else env
    return lookup.get("ASR_API_BASE_URL")


def asr_transcription_model(*, env: dict[str, str] | None = None) -> str:
    """Model name for the OpenAI-compatible transcription endpoint."""
    lookup = os.environ if env is None else env
    return lookup.get("ASR_API_MODEL", "mimo-v2.5-asr")


def asr_omni_base_url(*, env: dict[str, str] | None = None) -> str:
    """Base URL of the MiMo omni chat-completions ASR endpoint."""
    lookup = os.environ if env is None else env
    return lookup.get("ASR_API_BASE_URL", "https://token-plan-cn.xiaomimimo.com/v1")


def asr_omni_model(*, env: dict[str, str] | None = None) -> str:
    """Model name for the MiMo omni chat-completions ASR endpoint."""
    lookup = os.environ if env is None else env
    return lookup.get("ASR_API_MODEL", "mimo-v2.5")


def gateway_base_url(*, env: dict[str, str] | None = None) -> str:
    """Base URL of the internal LiteLLM gateway transport."""
    lookup = os.environ if env is None else env
    return lookup.get("LITELLM_BASE_URL", "http://localhost:4000/v1").strip()


def gateway_api_key(*, env: dict[str, str] | None = None) -> str:
    """Client credential for the internal LiteLLM gateway transport."""
    lookup = os.environ if env is None else env
    return lookup.get("LITELLM_API_KEY", "").strip()


def cors_origins(*, env: dict[str, str] | None = None) -> list[str]:
    """Allowed CORS origins for the HTTP API (comma-separated in the env)."""
    lookup = os.environ if env is None else env
    return lookup.get("CORS_ORIGINS", "http://localhost:5173").split(",")


def deterministic_ai_enabled(*, env: dict[str, str] | None = None) -> bool:
    """Whether deterministic test models replace the real providers."""
    lookup = os.environ if env is None else env
    return lookup.get("BODYSENSE_DETERMINISTIC_AI", "").strip().lower() in {
        "1",
        "true",
        "yes",
        "on",
    }


def e2e_stub_ai_enabled(*, env: dict[str, str] | None = None) -> bool:
    """Whether the E2E stub AI is active (explicit opt-in, non-production)."""
    lookup = os.environ if env is None else env
    environment = lookup.get("ENVIRONMENT", "development").lower()
    return lookup.get("BODYSENSE_E2E_STUB_AI") == "1" and environment in {
        "development",
        "test",
        "e2e",
    }


def ephemeral_checkpointer_allowed(*, env: dict[str, str] | None = None) -> bool:
    """Whether the in-memory checkpointer fallback is explicitly allowed."""
    lookup = os.environ if env is None else env
    return lookup.get("BODYSENSE_ALLOW_EPHEMERAL_CHECKPOINTER", "").strip().lower() in {
        "1",
        "true",
        "yes",
    }


def episodic_proposal_max_tokens(*, env: dict[str, str] | None = None) -> int:
    """Output cap for one episodic shadow proposal (default 1024).

    Unset, non-numeric, zero or negative values keep the default.
    """
    lookup = os.environ if env is None else env
    raw = lookup.get("EPISODIC_PROPOSAL_MAX_TOKENS", "").strip()
    if raw == "":
        return 1024
    try:
        max_tokens = int(raw)
    except ValueError:
        return 1024
    if max_tokens <= 0:
        return 1024
    return max_tokens


def health_document_model_root(default: str, *, env: dict[str, str] | None = None) -> str:
    """Root directory of the health-document OCR models.

    The env var (when set) wins over the caller-provided ``default``, matching
    the historical ``os.getenv(MODEL_ROOT_ENV, str(DEFAULT_MODEL_ROOT))`` read.
    """
    lookup = os.environ if env is None else env
    return lookup.get("BODYSENSE_HEALTH_DOCUMENT_MODEL_ROOT", default)


def knowledge_data_root(default: str, *, env: dict[str, str] | None = None) -> str:
    """Allowed data root for knowledge video ingestion.

    The env var (when set) wins over the caller-provided ``default``, matching
    the historical ``os.getenv("KNOWLEDGE_DATA_ROOT", ...)`` read.
    """
    lookup = os.environ if env is None else env
    return lookup.get("KNOWLEDGE_DATA_ROOT", default)


def knowledge_pool_max_size(*, env: dict[str, str] | None = None) -> int:
    """Upper bound of the knowledge Postgres connection pool."""
    lookup = os.environ if env is None else env
    return int(lookup.get("KNOWLEDGE_POOL_MAX_SIZE", "8"))


def knowledge_pool_min_size(*, env: dict[str, str] | None = None) -> int:
    """Lower bound of the knowledge Postgres connection pool."""
    lookup = os.environ if env is None else env
    return int(lookup.get("KNOWLEDGE_POOL_MIN_SIZE", "1"))


def pose_model_path_override(*, env: dict[str, str] | None = None) -> str:
    """Explicit pose landmarker model path override (empty when unset)."""
    lookup = os.environ if env is None else env
    return lookup.get("BODYSENSE_POSE_MODEL_PATH", "").strip()


def splitter_provider(*, env: dict[str, str] | None = None) -> str:
    """Knowledge-pack splitter backend selector ("heuristic" by default)."""
    lookup = os.environ if env is None else env
    return lookup.get("SPLITTER_PROVIDER", "heuristic")


def tessdata_prefix(*, env: dict[str, str] | None = None) -> str:
    """Tesseract data directory override (empty when unset)."""
    lookup = os.environ if env is None else env
    return lookup.get("TESSDATA_PREFIX", "").strip()
