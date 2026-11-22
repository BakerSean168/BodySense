"""Fail-fast database configuration tests for the unified settings module.

Guards issue #624 (R2-01): a missing DB_PASSWORD must never silently fall
back to a hard-coded default credential when assembling database URLs.
"""

from __future__ import annotations

import re
from pathlib import Path
from urllib.parse import unquote, urlsplit

import pytest

from src.rag import knowledge_library as knowledge_library_module
from src.runtime import checkpointing as checkpointing_module
from src.settings import DatabaseSettingsError, build_database_url

DB_ENV_VARS = (
    "DATABASE_URL",
    "LANGGRAPH_CHECKPOINTER_URL",
    "DB_HOST",
    "DB_PORT",
    "DB_NAME",
    "DB_USER",
    "DB_PASSWORD",
)


@pytest.fixture(autouse=True)
def clean_db_env(monkeypatch: pytest.MonkeyPatch):
    """Start every test from an environment without any database config."""
    for var in DB_ENV_VARS:
        monkeypatch.delenv(var, raising=False)


def _knowledge_library_url() -> str:
    # Build without __init__ so the test does not depend on embedding setup.
    instance = object.__new__(knowledge_library_module.KnowledgeLibrary)
    return knowledge_library_module.KnowledgeLibrary._build_database_url(instance)


def _checkpointing_url() -> str:
    return checkpointing_module._build_database_url()


class TestSettingsBuilder:
    def test_missing_password_raises_naming_db_password(self) -> None:
        env = {"DB_HOST": "db.example.com", "DB_USER": "someone"}
        with pytest.raises(DatabaseSettingsError) as exc_info:
            build_database_url(env=env)
        assert "DB_PASSWORD" in str(exc_info.value)

    def test_empty_password_is_rejected(self) -> None:
        with pytest.raises(DatabaseSettingsError) as exc_info:
            build_database_url(env={"DB_PASSWORD": ""})
        assert "DB_PASSWORD" in str(exc_info.value)

    def test_assembles_from_db_vars(self) -> None:
        url = build_database_url(
            env={
                "DB_HOST": "db.example.com",
                "DB_PORT": "5433",
                "DB_NAME": "bodysense_db",
                "DB_USER": "app_user",
                "DB_PASSWORD": "test-only-password",
            }
        )
        assert url == "postgresql://app_user:test-only-password@db.example.com:5433/bodysense_db"

    def test_database_url_wins_over_db_vars(self) -> None:
        url = build_database_url(
            env={
                "DATABASE_URL": "postgresql://u:pw@db1.example.com:5432/db1",
                "DB_HOST": "ignored",
                "DB_PASSWORD": "ignored",
            },
            url_overrides=("DATABASE_URL",),
        )
        assert url == "postgresql://u:pw@db1.example.com:5432/db1"

    def test_empty_override_is_skipped(self) -> None:
        url = build_database_url(
            env={
                "DATABASE_URL": "",
                "DB_HOST": "db.example.com",
                "DB_USER": "app_user",
                "DB_PASSWORD": "test-only-password",
            },
            url_overrides=("DATABASE_URL",),
        )
        assert url == "postgresql://app_user:test-only-password@db.example.com:5432/bodysense"

    def test_driver_suffixes_stripped(self) -> None:
        for suffix in ("+asyncpg", "+psycopg"):
            url = build_database_url(
                env={"DATABASE_URL": f"postgresql{suffix}://u:pw@db:5432/db"},
                url_overrides=("DATABASE_URL",),
            )
            assert url == "postgresql://u:pw@db:5432/db"

    def test_override_chain_priority(self) -> None:
        env = {
            "LANGGRAPH_CHECKPOINTER_URL": "postgresql://c:pw@checkpointer:5432/cp",
            "DATABASE_URL": "postgresql://d:pw@primary:5432/db",
        }
        url = build_database_url(
            env=env,
            url_overrides=("LANGGRAPH_CHECKPOINTER_URL", "DATABASE_URL"),
        )
        assert url == "postgresql://c:pw@checkpointer:5432/cp"
        # Without the higher-priority var, DATABASE_URL is used.
        url = build_database_url(
            env={"DATABASE_URL": env["DATABASE_URL"]},
            url_overrides=("LANGGRAPH_CHECKPOINTER_URL", "DATABASE_URL"),
        )
        assert url == "postgresql://d:pw@primary:5432/db"

    def test_special_characters_round_trip(self) -> None:
        password = "p@ss:w/rd?x#y&z"
        url = build_database_url(
            env={
                "DB_HOST": "db.example.com",
                "DB_USER": "app_user",
                "DB_PASSWORD": password,
            }
        )
        assert unquote(urlsplit(url).password) == password
        assert " " not in url
        # The raw password must not appear verbatim: it is percent-escaped.
        assert password not in url


class TestConsumersFailFast:
    """Both consumers must refuse to build a URL without DB_PASSWORD.

    These tests fail on the pre-fix code, which silently substituted a
    hard-coded default password instead of failing.
    """

    def test_knowledge_library_requires_db_password(self) -> None:
        with pytest.raises(DatabaseSettingsError) as exc_info:
            _knowledge_library_url()
        assert "DB_PASSWORD" in str(exc_info.value)

    def test_checkpointing_requires_db_password(self) -> None:
        with pytest.raises(DatabaseSettingsError) as exc_info:
            _checkpointing_url()
        assert "DB_PASSWORD" in str(exc_info.value)


class TestConsumerPrecedence:
    def test_knowledge_library_prefers_database_url(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setenv("DATABASE_URL", "postgresql+asyncpg://u:pw@db:5432/db")
        assert _knowledge_library_url() == "postgresql://u:pw@db:5432/db"

    def test_checkpointing_prefers_langgraph_url(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setenv("DATABASE_URL", "postgresql://u:pw@primary:5432/db")
        monkeypatch.setenv("LANGGRAPH_CHECKPOINTER_URL", "postgresql+psycopg://u:pw@cp:5432/cp")
        assert _checkpointing_url() == "postgresql://u:pw@cp:5432/cp"

    def test_checkpointing_falls_back_to_database_url(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        monkeypatch.setenv("DATABASE_URL", "postgresql+asyncpg://u:pw@db:5432/db")
        assert _checkpointing_url() == "postgresql://u:pw@db:5432/db"


class TestConsumerConsistency:
    def test_consumers_agree_when_db_vars_complete(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setenv("DB_HOST", "db.example.com")
        monkeypatch.setenv("DB_PORT", "5433")
        monkeypatch.setenv("DB_NAME", "bodysense_db")
        monkeypatch.setenv("DB_USER", "app_user")
        monkeypatch.setenv("DB_PASSWORD", "test-only-password")
        assert _knowledge_library_url() == _checkpointing_url()

    def test_consumers_agree_on_special_character_password(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        monkeypatch.setenv("DB_HOST", "db.example.com")
        monkeypatch.setenv("DB_USER", "app_user")
        monkeypatch.setenv("DB_PASSWORD", "p@ss:w/rd?x#y&z")
        knowledge_url = _knowledge_library_url()
        checkpointer_url = _checkpointing_url()
        assert knowledge_url == checkpointer_url
        assert unquote(urlsplit(knowledge_url).password) == "p@ss:w/rd?x#y&z"


class TestNoDefaultPasswordPattern:
    """Scan src so a getenv default for DB_PASSWORD can never come back."""

    def test_no_getenv_default_for_db_password_in_src(self) -> None:
        src_root = Path(__file__).resolve().parents[2] / "src"
        pattern = re.compile(r"""getenv\s*\(\s*["']DB_PASSWORD["']\s*,""")
        offenders = [
            f"{path}:{lineno}: {line.strip()}"
            for path in sorted(src_root.rglob("*.py"))
            for lineno, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1)
            if pattern.search(line)
        ]
        assert not offenders, "DB_PASSWORD must not have a getenv default: " + "; ".join(offenders)

    def test_no_hardcoded_default_credential_in_src(self) -> None:
        src_root = Path(__file__).resolve().parents[2] / "src"
        offenders = [
            f"{path}:{lineno}: {line.strip()}"
            for path in sorted(src_root.rglob("*.py"))
            for lineno, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1)
            if "bodysense123" in line
        ]
        assert not offenders, "Hard-coded default credential found: " + "; ".join(offenders)
