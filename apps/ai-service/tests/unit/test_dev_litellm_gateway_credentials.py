"""Fail-closed checks for the direct-dev LiteLLM boundary (no real services)."""

from __future__ import annotations

import os
import subprocess
from pathlib import Path

import pytest
import yaml

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
        "DEV_BIND_HOST",
        "BODYSENSE_LITELLM_DEV_ORACLE2",
        "LITELLM_BASE_URL",
        "LITELLM_API_KEY",
        "LITELLM_MASTER_KEY",
        "BODYSENSE_LITELLM_CLIENT_KEY",
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


def test_model_setup_helpers_never_inherit_gateway_or_application_credentials(
    tmp_path: Path,
) -> None:
    """Model artifact preparation is not an authorized gateway client."""
    secrets = (
        "LITELLM_MASTER_KEY",
        "LITELLM_API_KEY",
        "BODYSENSE_LITELLM_CLIENT_KEY",
        "PRIMARY_LLM_API_KEY",
        "OPENROUTER_API_KEY",
        "GROQ_API_KEY",
        "DB_PASSWORD",
        "DATABASE_URL",
        "LANGGRAPH_CHECKPOINTER_URL",
        "REDIS_PASSWORD",
        "JWT_SECRET_KEY",
        "SUPABASE_SECRET_KEY",
        "SUPABASE_SERVICE_ROLE_KEY",
    )
    env = os.environ.copy()
    env.update({name: f"synthetic-{name}" for name in secrets})
    env["BODYSENSE_DEV_ENV_FILE"] = str(tmp_path / "missing.env")
    result = subprocess.run(
        ["bash", "-c", "source scripts/dev-env.sh; run_model_setup env"],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=10,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    child_keys = {line.partition("=")[0] for line in result.stdout.splitlines()}
    assert not child_keys.intersection(secrets)
    for name in ("dev-runtime.sh", "dev-interactive.sh"):
        source = (ROOT / "scripts" / name).read_text(encoding="utf-8")
        prep_lines = [line for line in source.splitlines() if "python scripts/ensure_" in line]
        assert prep_lines
        for line in prep_lines:
            assert line.count("run_model_setup uv run") == line.count("python scripts/ensure_")


def test_go_api_launch_inherits_secrets_without_interpolating_them_into_argv() -> None:
    source = (ROOT / "scripts/dev-runtime.sh").read_text(encoding="utf-8")
    api_launch = next(
        line for line in source.splitlines() if line.strip().startswith("start_process api ")
    )
    assert "go run ./cmd/server" in api_launch
    for secret in (
        "DB_PASSWORD",
        "REDIS_PASSWORD",
        "JWT_SECRET_KEY",
        "DATABASE_URL",
        "SUPABASE_SECRET_KEY",
        "SUPABASE_SERVICE_ROLE_KEY",
    ):
        assert "${" + secret + "}" not in api_launch


@pytest.mark.parametrize("explicit_alias", [False, True])
def test_unscoped_local_master_alias_never_unblocks_external_route(
    tmp_path: Path, explicit_alias: bool
) -> None:
    settings = (
        "LITELLM_MASTER_KEY=synthetic-shared-master\n"
        + ("LITELLM_API_KEY=synthetic-shared-master\n" if explicit_alias else "")
        + "LITELLM_BASE_URL=https://oracle.taile92a8e.ts.net:10446/v1\n"
    )
    result = _run(tmp_path, PREFLIGHT, settings=settings)
    assert result.returncode != 0
    assert "BLOCKED" in result.stderr
    assert "synthetic-shared-master" not in result.stdout + result.stderr


def test_raw_compose_is_loopback_only_and_has_no_external_upstream() -> None:
    """The Docker Compose entrypoint bypasses shell preflight: guard the config itself."""
    compose = yaml.safe_load((ROOT / "docker/docker-compose.yml").read_text(encoding="utf-8"))
    services = compose["services"]
    published = [port for service in services.values() for port in service.get("ports", [])]
    assert published
    assert all(port.startswith("127.0.0.1:") for port in published)

    gateway = services["litellm-gateway"]
    assert "./litellm/config.dev-offline.yaml:/app/config.yaml:ro" in gateway["volumes"]
    assert not {"PRIMARY_LLM_API_KEY", "OPENROUTER_API_KEY", "GROQ_API_KEY"} & set(
        gateway["environment"]
    )
    offline = yaml.safe_load(
        (ROOT / "docker/litellm/config.dev-offline.yaml").read_text(encoding="utf-8")
    )
    assert offline["model_list"] == []
    assert not offline.get("router_settings", {}).get("fallbacks")


def test_raw_compose_uses_a_separate_client_key_source() -> None:
    compose = yaml.safe_load((ROOT / "docker/docker-compose.yml").read_text(encoding="utf-8"))
    services = compose["services"]
    master = services["litellm-gateway"]["environment"]["LITELLM_MASTER_KEY"]
    client = services["ai-service"]["environment"]["LITELLM_API_KEY"]
    assert client == "${BODYSENSE_LITELLM_CLIENT_KEY:-}"
    assert client != master
    assert "LITELLM_MASTER_KEY" not in services["ai-service"]["environment"]


@pytest.mark.parametrize("role", ["web", "api", "ai", "document"])
def test_process_compose_pane_blocks_external_gateway(tmp_path: Path, role: str) -> None:
    """No Process Compose service may bypass the offline gateway guard."""
    result = _run(
        tmp_path,
        ROOT / "scripts/dev-pane.sh",
        role,
        settings="LITELLM_BASE_URL=https://unapproved.invalid/v1\n",
    )
    assert result.returncode != 0
    assert "BLOCKED" in result.stderr
    assert "UNEXPECTED_SERVICE_START" not in result.stderr


@pytest.mark.parametrize("role", ["web", "api", "ai", "document"])
def test_process_compose_pane_keeps_service_and_model_setup_secrets_scoped(
    tmp_path: Path, role: str
) -> None:
    """The Process Compose path must enforce the same boundary as dev-runtime."""
    env_file = tmp_path / "synthetic.env"
    env_file.write_text(
        "LITELLM_MASTER_KEY=synthetic-master\n"
        "BODYSENSE_LITELLM_CLIENT_KEY=synthetic-client\n"
        "PRIMARY_LLM_API_KEY=synthetic-primary\n"
        "OPENROUTER_API_KEY=synthetic-openrouter\n"
        "GROQ_API_KEY=synthetic-groq\n"
        "DB_PASSWORD=synthetic-db\n"
        "REDIS_PASSWORD=synthetic-redis\n"
        "JWT_SECRET_KEY=synthetic-jwt\n"
        "SUPABASE_SECRET_KEY=synthetic-sb-secret\n"
        "SUPABASE_SERVICE_ROLE_KEY=synthetic-sb-legacy\n"
        "LANGGRAPH_CHECKPOINTER_URL=postgresql://synthetic-checkpoint\n",
        encoding="utf-8",
    )
    stubs = tmp_path / "stubs"
    capture = tmp_path / "capture"
    stubs.mkdir()
    capture.mkdir()
    stub_source = (
        "#!/usr/bin/env bash\n"
        "set -Eeuo pipefail\n"
        "kind=service\n"
        '[[ " $* " == *"scripts/ensure_"* ]] && kind=setup\n'
        'env | sort > "$BODYSENSE_TEST_CAPTURE_DIR/$BODYSENSE_TEST_ROLE.$kind.env"\n'
        'printf "%s\\\n" "$@" > "$BODYSENSE_TEST_CAPTURE_DIR/$BODYSENSE_TEST_ROLE.$kind.argv"\n'
    )
    for tool in ("pnpm", "air", "uv"):
        stub = stubs / tool
        stub.write_text(stub_source, encoding="utf-8")
        stub.chmod(0o755)
    env = os.environ.copy()
    for name in (
        "LITELLM_MASTER_KEY",
        "LITELLM_API_KEY",
        "BODYSENSE_LITELLM_CLIENT_KEY",
        "LITELLM_BASE_URL",
        "PRIMARY_LLM_API_KEY",
        "OPENROUTER_API_KEY",
        "GROQ_API_KEY",
        "DB_PASSWORD",
        "REDIS_PASSWORD",
        "JWT_SECRET_KEY",
        "SUPABASE_SECRET_KEY",
        "SUPABASE_SERVICE_ROLE_KEY",
        "LANGGRAPH_CHECKPOINTER_URL",
        "BODYSENSE_LITELLM_DEV_ORACLE2",
    ):
        env.pop(name, None)
    env.update(
        {
            "BODYSENSE_DEV_ENV_FILE": str(env_file),
            "BODYSENSE_TEST_CAPTURE_DIR": str(capture),
            "BODYSENSE_TEST_ROLE": role,
            "PATH": f"{stubs}:{env.get('PATH', '')}",
        }
    )
    result = subprocess.run(
        ["bash", "scripts/dev-pane.sh", role],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=10,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    output = (capture / f"{role}.service.env").read_text(encoding="utf-8")
    assert not any(
        f"{name}=" in output
        for name in (
            "LITELLM_MASTER_KEY",
            "BODYSENSE_LITELLM_CLIENT_KEY",
            "PRIMARY_LLM_API_KEY",
            "OPENROUTER_API_KEY",
            "GROQ_API_KEY",
            "SUPABASE_SERVICE_ROLE_KEY",
        )
    )
    assert ("LITELLM_API_KEY=synthetic-client" in output) == (role == "ai")
    assert ("DB_PASSWORD=synthetic-db" in output) == (role in {"api", "ai"})
    assert ("REDIS_PASSWORD=synthetic-redis" in output) == (role == "api")
    assert ("JWT_SECRET_KEY=synthetic-jwt" in output) == (role == "api")
    assert ("SUPABASE_SECRET_KEY=synthetic-sb-secret" in output) == (role == "api")
    if role in {"ai", "document"}:
        setup = (capture / f"{role}.setup.env").read_text(encoding="utf-8")
        for secret in (
            "LITELLM_MASTER_KEY",
            "LITELLM_API_KEY",
            "OPENROUTER_API_KEY",
            "GROQ_API_KEY",
            "DB_PASSWORD",
            "REDIS_PASSWORD",
            "JWT_SECRET_KEY",
            "SUPABASE_SECRET_KEY",
            "SUPABASE_SERVICE_ROLE_KEY",
        ):
            assert f"{secret}=" not in setup
