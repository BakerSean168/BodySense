"""Fail-closed checks for the direct-dev LiteLLM boundary (no real services)."""

from __future__ import annotations

import os
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[4]
PREFLIGHT = ROOT / "scripts/dev-litellm-gateway-preflight.sh"


def _run(
    tmp_path: Path, script: Path, *args: str, settings: str = ""
) -> subprocess.CompletedProcess[str]:
    env_file = tmp_path / "dev.env"
    env_file.write_text(settings, encoding="utf-8")
    # Tests must never launch Docker/processes even if a future guard regresses.
    stubs = tmp_path / "command-stubs"
    stubs.mkdir(exist_ok=True)
    for name in ("docker", "uv", "go", "pnpm", "curl", "setsid"):
        stub = stubs / name
        stub.write_text("#!/bin/sh\necho UNEXPECTED_SERVICE_START >&2\nexit 94\n", encoding="utf-8")
        stub.chmod(0o755)
    env = os.environ.copy()
    env["PATH"] = f"{stubs}:{env.get('PATH', '')}"
    for key in (
        "BODYSENSE_LITELLM_DEV_ORACLE2",
        "LITELLM_BASE_URL",
        "LITELLM_API_KEY",
        "LITELLM_MASTER_KEY",
        "LITELLM_PORT",
    ):
        env.pop(key, None)
    env["BODYSENSE_DEV_ENV_FILE"] = str(env_file)
    env["BODYSENSE_DEV_RUNTIME_DIR"] = str(tmp_path / "runtime")
    return subprocess.run(
        ["bash", str(script), *args],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=10,
        check=False,
    )


def test_local_default_gateway_is_allowed(tmp_path: Path) -> None:
    result = _run(tmp_path, PREFLIGHT)
    assert result.returncode == 0, result.stderr


@pytest.mark.parametrize(
    "settings",
    [
        "LITELLM_BASE_URL=https://oracle.taile92a8e.ts.net:10446/v1\n"
        "LITELLM_API_KEY=sk-synthetic-secret-for-test\n",
        "BODYSENSE_LITELLM_DEV_ORACLE2=1\n"
        "LITELLM_BASE_URL=https://oracle.taile92a8e.ts.net:10446/v1\n"
        "LITELLM_API_KEY=sk-synthetic-secret-for-test\n",
        "BODYSENSE_LITELLM_DEV_ORACLE2=1\n",
    ],
)
def test_unapproved_external_gateway_is_blocked(tmp_path: Path, settings: str) -> None:
    result = _run(tmp_path, PREFLIGHT, settings=settings)
    assert result.returncode != 0
    assert "BLOCKED" in result.stderr
    assert "sk-synthetic-secret-for-test" not in result.stdout + result.stderr


@pytest.mark.parametrize(
    ("script", "action"),
    [
        ("dev-runtime.sh", "up"),
        ("dev-runtime.sh", "restart"),
        ("dev-infra.sh", "up"),
        ("dev-infra.sh", "restart"),
    ],
)
def test_mutating_start_entrypoints_block_before_services(
    tmp_path: Path, script: str, action: str
) -> None:
    settings = (
        "BODYSENSE_LITELLM_DEV_ORACLE2=1\n"
        "LITELLM_BASE_URL=https://oracle.taile92a8e.ts.net:10446/v1\n"
        "LITELLM_API_KEY=sk-synthetic-secret-for-test\n"
    )
    result = _run(tmp_path, ROOT / "scripts" / script, action, settings=settings)
    assert result.returncode != 0
    assert "BLOCKED" in result.stderr
    assert "started pid=" not in result.stdout
    assert "sk-synthetic-secret-for-test" not in result.stdout + result.stderr


def test_ai_startup_no_longer_embeds_litellm_secret_in_process_command() -> None:
    source = (ROOT / "scripts/dev-runtime.sh").read_text(encoding="utf-8")
    assert "LITELLM_API_KEY='${LITELLM_MASTER_KEY}'" not in source
    assert "LITELLM_BASE_URL='http://127.0.0.1:" not in source
    assert "start_process ai" in source
