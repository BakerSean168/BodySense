"""Shared pytest fixtures for BodySense AI Service tests."""

import asyncio
from typing import AsyncGenerator, Generator

import pytest
from fastapi.testclient import TestClient

from src.main import app


@pytest.fixture(scope="session")
def event_loop() -> Generator:
    """Create an instance of the default event loop for the test session."""
    loop = asyncio.get_event_loop_policy().new_event_loop()
    yield loop
    loop.close()


@pytest.fixture
def client():
    """FastAPI test client."""
    return TestClient(app)


@pytest.fixture
def mock_embedding() -> list[float]:
    """Return a mock embedding vector."""
    return [0.1] * 1536


@pytest.fixture
def mock_database_url() -> str:
    """Return a mock database URL for testing."""
    return "postgresql://test:test@localhost/test_db"
