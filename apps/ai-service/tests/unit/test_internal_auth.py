"""Inbound internal-token authentication tests (issue #625, R2-02).

Once a token is configured the middleware must reject unauthenticated
callers on every business route of both FastAPI apps; while no token is
configured the pre-token deployment mode must keep working, and the
/health probe surface must stay reachable in both modes.
"""

from __future__ import annotations

import logging

import pytest
from fastapi.testclient import TestClient

from src.api.internal_auth import log_auth_configuration
from src.document_main import app as document_app
from src.main import app
from src.settings import internal_token

FAKE_TOKEN = "unit-test-internal-token"

BUSINESS_URL = "/api/assessment/generate"


@pytest.fixture(autouse=True)
def clear_token_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("AI_SERVICE_INTERNAL_TOKEN", raising=False)


@pytest.fixture
def configured(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("AI_SERVICE_INTERNAL_TOKEN", FAKE_TOKEN)


class TestInternalTokenSetting:
    def test_unset_returns_none(self) -> None:
        assert internal_token(env={}) is None

    def test_blank_returns_none(self) -> None:
        assert internal_token(env={"AI_SERVICE_INTERNAL_TOKEN": "  "}) is None

    def test_configured_value_is_stripped(self) -> None:
        env = {"AI_SERVICE_INTERNAL_TOKEN": " abc "}
        assert internal_token(env=env) == "abc"


class TestMainAppMiddleware:
    def test_unconfigured_allows_business_route(self) -> None:
        response = TestClient(app).post(BUSINESS_URL, json={})
        # 422 means the request reached route validation instead of
        # being rejected by the middleware.
        assert response.status_code == 422

    def test_missing_token_is_rejected(self, configured: None) -> None:
        response = TestClient(app).post(BUSINESS_URL, json={})
        assert response.status_code == 401

    def test_wrong_token_is_rejected(self, configured: None) -> None:
        response = TestClient(app).post(
            BUSINESS_URL, json={}, headers={"X-Internal-Token": "wrong"}
        )
        assert response.status_code == 401

    def test_correct_token_passes(self, configured: None) -> None:
        response = TestClient(app).post(
            BUSINESS_URL, json={}, headers={"X-Internal-Token": FAKE_TOKEN}
        )
        assert response.status_code == 422

    def test_health_is_exempt(self, configured: None) -> None:
        response = TestClient(app).get("/health")
        assert response.status_code == 200


class TestDocumentAppMiddleware:
    def test_missing_token_is_rejected(self, configured: None) -> None:
        response = TestClient(document_app).post("/api/ocr/extract")
        assert response.status_code == 401

    def test_health_is_exempt(self, configured: None) -> None:
        response = TestClient(document_app).get("/health")
        assert response.status_code == 200


class TestStartupWarning:
    def test_unconfigured_logs_warning(self, caplog: pytest.LogCaptureFixture) -> None:
        with caplog.at_level(logging.WARNING, logger="src.api.internal_auth"):
            log_auth_configuration("test service")
        assert "内部鉴权未启用" in caplog.text

    def test_configured_logs_no_warning(
        self, configured: None, caplog: pytest.LogCaptureFixture
    ) -> None:
        with caplog.at_level(logging.WARNING, logger="src.api.internal_auth"):
            log_auth_configuration("test service")
        assert "内部鉴权未启用" not in caplog.text
