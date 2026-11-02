"""Fail-closed checks for the direct-dev LiteLLM boundary (no real services)."""

from __future__ import annotations

import os
import subprocess
import time
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


def test_local_default_gateway_is_allowed(tmp_path: Path) -> None:
    result = _run(tmp_path, PREFLIGHT)
    assert result.returncode == 0, result.stderr

    # Offline local development must not silently grant the AI Service the
    # gateway infrastructure master credential.
    env = os.environ.copy()
    for key in ("LITELLM_API_KEY", "LITELLM_MASTER_KEY", "BODYSENSE_LITELLM_CLIENT_KEY"):
        env.pop(key, None)
    env_file = tmp_path / "dev.env"
    env["BODYSENSE_DEV_ENV_FILE"] = str(env_file)
    sourced = subprocess.run(
        [
            "bash",
            "-c",
            'source scripts/dev-env.sh; [[ -z "$LITELLM_API_KEY" && -n "$LITELLM_MASTER_KEY" ]]',
        ],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        check=False,
    )
    assert sourced.returncode == 0, sourced.stderr


def test_local_master_alias_is_blocked_without_provider_calls(tmp_path: Path) -> None:
    result = _run(
        tmp_path,
        PREFLIGHT,
        settings="LITELLM_MASTER_KEY=synthetic-master\nLITELLM_API_KEY=synthetic-master\n",
    )
    assert result.returncode != 0
    assert "master-key alias is forbidden" in result.stderr
    assert "synthetic-master" not in result.stdout + result.stderr


@pytest.mark.parametrize("legacy", ["synthetic-legacy", "synthetic-master"])
def test_unscoped_legacy_key_cannot_implicitly_authorize_ai(tmp_path: Path, legacy: str) -> None:
    result = _run(
        tmp_path,
        PREFLIGHT,
        settings=f"LITELLM_MASTER_KEY=synthetic-master\nLITELLM_API_KEY={legacy}\n",
    )
    assert result.returncode != 0
    assert "BLOCKED" in result.stderr
    assert legacy not in result.stdout + result.stderr


def test_scoped_client_takes_precedence_over_legacy_alias(tmp_path: Path) -> None:
    result = _run(
        tmp_path,
        PREFLIGHT,
        settings=(
            "LITELLM_MASTER_KEY=synthetic-master\n"
            "LITELLM_API_KEY=synthetic-master\n"
            "BODYSENSE_LITELLM_CLIENT_KEY=synthetic-client\n"
        ),
    )
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
    "settings",
    [
        # Both guard conditions must independently fail closed. Testing only
        # both together would miss regressions that drop either check.
        "BODYSENSE_LITELLM_DEV_ORACLE2=1\n",
        "LITELLM_BASE_URL=https://oracle.taile92a8e.ts.net:10446/v1\n"
        "LITELLM_API_KEY=sk-synthetic-secret-for-test\n",
        "BODYSENSE_LITELLM_DEV_ORACLE2=1\n"
        "LITELLM_BASE_URL=https://oracle.taile92a8e.ts.net:10446/v1\n"
        "LITELLM_API_KEY=sk-synthetic-secret-for-test\n",
    ],
)
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
    tmp_path: Path, script: str, action: str, settings: str
) -> None:
    result = _run(tmp_path, ROOT / "scripts" / script, action, settings=settings)
    assert result.returncode != 0
    assert "BLOCKED" in result.stderr
    assert "started pid=" not in result.stdout
    # A rejected restart must not tear down existing services first. Stubbed
    # Docker/service launches and the runtime's down() message are both
    # observable signs that the guard ran too late.
    assert "UNEXPECTED_SERVICE_START" not in result.stdout + result.stderr
    assert "persistent dev infrastructure left running" not in result.stdout
    assert "sk-synthetic-secret-for-test" not in result.stdout + result.stderr


@pytest.mark.parametrize(
    "port",
    ["20112@external.invalid", "20112/path", "0", "65536", "020112", "20112 "],
)
def test_malformed_litellm_port_never_passes_local_gateway_guard(tmp_path: Path, port: str) -> None:
    # A URI with 127.0.0.1:port@external.invalid actually targets the remote
    # hostname. Reject port injection even when BASE_URL uses the default.
    result = _run(tmp_path, PREFLIGHT, settings=f"LITELLM_PORT='{port}'\n")
    assert result.returncode != 0
    assert "port is invalid" in result.stderr
    assert "external.invalid" not in result.stderr


@pytest.mark.parametrize(
    ("script", "action"),
    [
        ("dev-runtime.sh", "up"),
        ("dev-runtime.sh", "restart"),
        ("dev-infra.sh", "up"),
        ("dev-infra.sh", "restart"),
    ],
)
def test_malformed_port_rejected_before_service_mutation(
    tmp_path: Path, script: str, action: str
) -> None:
    result = _run(
        tmp_path,
        ROOT / "scripts" / script,
        action,
        settings="LITELLM_PORT=20112@external.invalid\n",
    )
    assert result.returncode != 0
    assert "port is invalid" in result.stderr
    assert "UNEXPECTED_SERVICE_START" not in result.stdout + result.stderr
    assert "persistent dev infrastructure left running" not in result.stdout


@pytest.mark.parametrize("bind_host", ["0.0.0.0", "::", "192.0.2.1", "localhost"])
def test_direct_dev_never_exposes_gateway_bind(tmp_path: Path, bind_host: str) -> None:
    result = _run(tmp_path, PREFLIGHT, settings=f"DEV_BIND_HOST={bind_host}\n")
    assert result.returncode != 0
    assert "bind BLOCKED" in result.stderr


@pytest.mark.parametrize(
    ("script", "action"),
    [
        ("dev-runtime.sh", "up"),
        ("dev-runtime.sh", "restart"),
        ("dev-infra.sh", "up"),
        ("dev-infra.sh", "restart"),
    ],
)
def test_public_bind_rejected_before_mutation(tmp_path: Path, script: str, action: str) -> None:
    result = _run(
        tmp_path,
        ROOT / "scripts" / script,
        action,
        settings="DEV_BIND_HOST=0.0.0.0\n",
    )
    assert result.returncode != 0
    assert "bind BLOCKED" in result.stderr
    assert "UNEXPECTED_SERVICE_START" not in result.stdout + result.stderr
    assert "persistent dev infrastructure left running" not in result.stdout


def test_ai_startup_no_longer_embeds_litellm_secret_in_process_command() -> None:
    source = (ROOT / "scripts/dev-runtime.sh").read_text(encoding="utf-8")
    assert "LITELLM_API_KEY='${LITELLM_MASTER_KEY}'" not in source
    assert "LITELLM_BASE_URL='http://127.0.0.1:" not in source
    assert 'export DATABASE_URL="postgresql://${DB_USER}:${DB_PASSWORD}@127.0.0.1:' in source
    assert "start_process ai" in source


@pytest.mark.parametrize("client_is_master", [False, True])
@pytest.mark.parametrize("entrypoint", ["dev-runtime.sh", "dev-interactive.sh"])
def test_long_lived_services_only_inherit_role_credentials(
    tmp_path: Path, client_is_master: bool, entrypoint: str
) -> None:
    """Use shell command stubs to inspect child environments without launching services."""
    env_file = tmp_path / "dev.env"
    env_file.write_text(
        "LITELLM_MASTER_KEY=synthetic-master\n"
        + (
            "BODYSENSE_LITELLM_CLIENT_KEY=synthetic-master\n"
            if client_is_master
            else "LITELLM_API_KEY=synthetic-client\nBODYSENSE_LITELLM_CLIENT_KEY=synthetic-client\n"
        )
        + "PRIMARY_LLM_API_KEY=synthetic-primary\n"
        "OPENROUTER_API_KEY=synthetic-openrouter\n"
        "GROQ_API_KEY=synthetic-groq\n"
        "DB_PASSWORD=synthetic-db\n"
        "REDIS_PASSWORD=synthetic-redis\n"
        "JWT_SECRET_KEY=synthetic-jwt\n"
        "DATABASE_URL=postgresql://synthetic-user:synthetic-db@127.0.0.1/test\n"
        "LANGGRAPH_CHECKPOINTER_URL=postgresql://synthetic-checkpointer/test\n",
        encoding="utf-8",
    )
    stubs = tmp_path / "stubs"
    stubs.mkdir()
    for tool in ("docker", "curl"):
        stub = stubs / tool
        stub.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
        stub.chmod(0o755)
    setsid = stubs / "setsid"
    setsid.write_text(
        "#!/bin/sh\n"
        "env | grep -E '^(LITELLM_|BODYSENSE_LITELLM_CLIENT_KEY=|"
        "PRIMARY_LLM_API_KEY=|OPENROUTER_API_KEY=|GROQ_API_KEY=|"
        "DB_PASSWORD=|REDIS_PASSWORD=|JWT_SECRET_KEY=|DATABASE_URL=|"
        "LANGGRAPH_CHECKPOINTER_URL=)' > "
        '"$BODYSENSE_TEST_CAPTURE_DIR/$BODYSENSE_TEST_CAPTURE_ROLE" || true\n'
        'printf "%s\\n" "$@" > "$BODYSENSE_TEST_CAPTURE_DIR/$BODYSENSE_TEST_CAPTURE_ROLE.argv"\n',
        encoding="utf-8",
    )
    setsid.chmod(0o755)
    capture = tmp_path / "capture"
    capture.mkdir()

    env = os.environ.copy()
    for key in (
        "LITELLM_MASTER_KEY",
        "LITELLM_API_KEY",
        "LITELLM_BASE_URL",
        "BODYSENSE_LITELLM_CLIENT_KEY",
        "PRIMARY_LLM_API_KEY",
        "OPENROUTER_API_KEY",
        "GROQ_API_KEY",
        "DB_PASSWORD",
        "REDIS_PASSWORD",
        "JWT_SECRET_KEY",
        "DATABASE_URL",
        "LANGGRAPH_CHECKPOINTER_URL",
    ):
        env.pop(key, None)
    env.update(
        {
            "PATH": f"{stubs}:{env.get('PATH', '')}",
            "BODYSENSE_DEV_ENV_FILE": str(env_file),
            "BODYSENSE_DEV_RUNTIME_DIR": str(tmp_path / "runtime"),
            "BODYSENSE_TEST_CAPTURE_DIR": str(capture),
        }
    )
    if entrypoint == "dev-runtime.sh":
        script = ['source "$PWD/scripts/dev-runtime.sh" status']
    else:
        # Source only the helper body; never execute the interactive CLI.
        script = [
            'source "$PWD/scripts/dev-env.sh"',
            "pids=(); names=()",
            "source <(sed -n '/^start_service() {/,/^}/p' scripts/dev-interactive.sh)",
        ]
    for role in ("ai", "api", "document", "web"):
        command = (
            f"start_process {role} true"
            if entrypoint == "dev-runtime.sh"
            else f'start_service {role} "$PWD" true'
        )
        script.extend((f"export BODYSENSE_TEST_CAPTURE_ROLE={role}", command))
    result = subprocess.run(
        ["bash", "-c", "\n".join(script)],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=10,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    for _ in range(100):
        if all((capture / role).exists() for role in ("ai", "api", "document", "web")):
            break
        time.sleep(0.02)

    for role in ("ai", "api", "document", "web"):
        output = (capture / role).read_text(encoding="utf-8")
        assert "LITELLM_MASTER_KEY=" not in output, role
        assert "BODYSENSE_LITELLM_CLIENT_KEY=" not in output, role
        assert "PRIMARY_LLM_API_KEY=" not in output, role
        assert "OPENROUTER_API_KEY=" not in output, role
        assert "GROQ_API_KEY=" not in output, role
        assert ("LITELLM_API_KEY=synthetic-client" in output) == (
            role == "ai" and not client_is_master
        ), role
        assert "LITELLM_API_KEY=synthetic-master" not in output, role
        for key in ("DB_PASSWORD", "DATABASE_URL"):
            assert (f"{key}=" in output) == (role in {"ai", "api"}), (role, key)
        for key in ("REDIS_PASSWORD", "JWT_SECRET_KEY"):
            assert (f"{key}=" in output) == (role == "api"), (role, key)
        assert ("LANGGRAPH_CHECKPOINTER_URL=" in output) == (role in {"ai", "api"}), role
        argv = (capture / f"{role}.argv").read_text(encoding="utf-8")
        assert "synthetic-db" not in argv, role
        assert "synthetic-redis" not in argv, role
        assert "synthetic-jwt" not in argv, role


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
    for secret in ("DB_PASSWORD", "REDIS_PASSWORD", "JWT_SECRET_KEY", "DATABASE_URL"):
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
        )
    )
    assert ("LITELLM_API_KEY=synthetic-client" in output) == (role == "ai")
    assert ("DB_PASSWORD=synthetic-db" in output) == (role in {"api", "ai"})
    assert ("REDIS_PASSWORD=synthetic-redis" in output) == (role == "api")
    assert ("JWT_SECRET_KEY=synthetic-jwt" in output) == (role == "api")
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
        ):
            assert f"{secret}=" not in setup
