"""Guard the dev-only Oracle2 gateway opt-in.

These are structural/schema tests: no network, no credentials. They lock the
invariants that make the opt-in safe:

- production/staging configs and Compose files are unchanged by the opt-in;
- the dev Oracle2 config keeps the business logical groups, pins no random
  load-balancing, and leaves posture on its pre-existing vision provider;
- Kunyou (HTTP503 "no channel") is declared but never wired into a fallback;
- no key value is committed anywhere;
- the eval script only probes groups that have no fallback edge.
"""

from __future__ import annotations

import os
import re
import subprocess
import tempfile
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[4]
PROD_CONFIG = ROOT / "docker" / "litellm" / "config.yaml"
STAGING_CONFIG = ROOT / "docker" / "litellm" / "config.staging.yaml"
DEV_ORACLE2_CONFIG = ROOT / "docker" / "litellm" / "config.dev-oracle2.yaml"
BASE_COMPOSE = ROOT / "docker" / "docker-compose.yml"
DEV_INFRA_COMPOSE = ROOT / "docker" / "docker-compose.dev-infra.yml"
DEV_ORACLE2_COMPOSE = ROOT / "docker" / "docker-compose.dev-oracle2.yml"
PROD_COMPOSE = ROOT / "docker" / "docker-compose.prod.yml"
STAGING_COMPOSE = ROOT / "docker" / "docker-compose.staging.yml"
DEV_ENV = ROOT / "scripts" / "dev-env.sh"
DEV_INFRA = ROOT / "scripts" / "dev-infra.sh"
DEV_RUNTIME = ROOT / "scripts" / "dev-runtime.sh"
EVAL_SCRIPT = ROOT / "scripts" / "dev-oracle2-gateway-eval.py"

PUBLIC_GROUPS = {
    "bodysense-diagnosis",
    "bodysense-consultation",
    "bodysense-structured",
    "bodysense-text",
    "bodysense-posture",
}
FALLBACK_GROUPS = {
    "bodysense-diagnosis-fallback",
    "bodysense-general-fallback",
    "bodysense-posture-fallback",
}
PROBE_GROUPS = {
    "bodysense-dev-oracle2-windhub",
    "bodysense-dev-oracle2-kunyou",
    "bodysense-dev-oracle2-nanliang",
}

# Measured on Oracle2 2026-10-22.
WINDHUB_MODEL = "openai/windhub-free/glm-5.3"
NANLIANG_MODEL = "openai/deepseek-v4.1-flash-1017"
KUNYOU_MODEL = "openai/gpt-6-luna"
POSTURE_MODEL = "openrouter/qwen/qwen2.5-vl-72b-instruct"

def _load(path: Path) -> dict:
    with path.open(encoding="utf-8") as handle:
        return yaml.safe_load(handle)

def _groups(config: dict) -> dict[str, dict]:
    return {item["model_name"]: item["litellm_params"] for item in config["model_list"]}

def test_dev_oracle2_config_keeps_every_business_logical_group() -> None:
    config = _load(DEV_ORACLE2_CONFIG)
    assert set(_groups(config)) == PUBLIC_GROUPS | FALLBACK_GROUPS | PROBE_GROUPS

def test_dev_oracle2_routes_compatible_groups_to_oracle2_and_keeps_posture_vision() -> None:
    groups = _groups(_load(DEV_ORACLE2_CONFIG))
    for logical_model in (
        "bodysense-diagnosis",
        "bodysense-consultation",
        "bodysense-structured",
        "bodysense-text",
    ):
        params = groups[logical_model]
        assert params["model"] == WINDHUB_MODEL
        assert params["api_base"] == "os.environ/ORACLE2_BODYSENSE_BASE_URL"
        assert params["api_key"] == "os.environ/ORACLE2_BODYSENSE_API_KEY"
        # WindHub is a slow free channel (measured chat ~42s, JSON ~28s, tool ~27s).
        assert int(params["timeout"]) >= 90

    assert groups["bodysense-general-fallback"]["model"] == NANLIANG_MODEL
    assert groups["bodysense-diagnosis-fallback"]["model"] == NANLIANG_MODEL

    # Posture must keep its pre-existing vision provider: Oracle2 has no
    # qualified vision channel.
    assert groups["bodysense-posture"]["model"] == POSTURE_MODEL
    assert groups["bodysense-posture"]["api_key"] == "os.environ/OPENROUTER_API_KEY"
    assert groups["bodysense-posture-fallback"]["api_key"] == "os.environ/OPENROUTER_API_KEY"

def test_dev_oracle2_uses_deterministic_ordered_fallback_without_load_balancing() -> None:
    config = _load(DEV_ORACLE2_CONFIG)
    router = config["router_settings"]

    assert router["num_retries"] == 0
    assert router["fallbacks"] == [
        {"bodysense-diagnosis": ["bodysense-diagnosis-fallback"]},
        {"bodysense-consultation": ["bodysense-general-fallback"]},
        {"bodysense-structured": ["bodysense-general-fallback"]},
        {"bodysense-text": ["bodysense-general-fallback"]},
        {"bodysense-posture": ["bodysense-posture-fallback"]},
    ]
    # No random load balancing anywhere in the file.
    for forbidden in ("routing_strategy", "weight", "num_retries_per_model"):
        assert forbidden not in router, forbidden
    assert "simple-shuffle" not in DEV_ORACLE2_CONFIG.read_text(encoding="utf-8")
    # One deployment per logical group: no duplicate model_name entries.
    names = [item["model_name"] for item in config["model_list"]]
    assert len(names) == len(set(names))

def test_kunyou_is_declared_pending_and_never_in_a_live_fallback_path() -> None:
    config = _load(DEV_ORACLE2_CONFIG)
    groups = _groups(config)
    # Declared for review/measurement only.
    assert groups["bodysense-dev-oracle2-kunyou"]["model"] == KUNYOU_MODEL

    # Never a serving group and never a fallback target: Oracle2 answers HTTP503
    # "no channel" for gpt-6-luna, so a live path must not depend on it.
    fallback_targets = {
        target
        for entry in config["router_settings"]["fallbacks"]
        for targets in entry.values()
        for target in targets
    }
    assert "bodysense-dev-oracle2-kunyou" not in fallback_targets
    for name in PUBLIC_GROUPS:
        assert groups[name]["model"] != KUNYOU_MODEL, name
    for name in FALLBACK_GROUPS:
        assert groups[name]["model"] != KUNYOU_MODEL, name

def test_probe_groups_have_no_fallback_edge() -> None:
    config = _load(DEV_ORACLE2_CONFIG)
    fallback_keys = {key for entry in config["router_settings"]["fallbacks"] for key in entry}
    assert PROBE_GROUPS.isdisjoint(fallback_keys)

def test_production_and_staging_gateway_configs_are_unaffected() -> None:
    production = _load(PROD_CONFIG)
    staging = _load(STAGING_CONFIG)
    for config in (production, staging):
        for params in _groups(config).values():
            assert "ORACLE2" not in str(params), params
    assert _groups(production)["bodysense-consultation"]["model"] == "openai/gemini-3.7-flash"
    assert _groups(staging)["bodysense-structured"]["api_base"] == (
        "os.environ/STAGING_DIAGNOSIS_BASE_URL"
    )

def test_only_dev_compose_references_the_oracle2_overlay() -> None:
    for path in (PROD_COMPOSE, STAGING_COMPOSE, BASE_COMPOSE, DEV_INFRA_COMPOSE):
        assert "dev-oracle2" not in path.read_text(encoding="utf-8"), path.name

    for script in (DEV_INFRA, DEV_RUNTIME):
        text = script.read_text(encoding="utf-8")
        assert "BODYSENSE_DEV_COMPOSE_OVERLAY" in text, script.name

def test_overlay_injects_key_by_environment_and_fails_closed_without_it() -> None:
    overlay = _load(DEV_ORACLE2_COMPOSE)
    gateway = overlay["services"]["litellm-gateway"]

    assert gateway["environment"]["ORACLE2_BODYSENSE_API_KEY"] == (
        "${ORACLE2_BODYSENSE_API_KEY:?set ORACLE2_BODYSENSE_API_KEY"
        " (dev-only Oracle2 Virtual Key; never commit it)}"
    )
    assert gateway["environment"]["ORACLE2_BODYSENSE_BASE_URL"] == (
        "${ORACLE2_BODYSENSE_BASE_URL:-https://oracle.taile92a8e.ts.net:10446/v1}"
    )
    # The overlay must not touch the network boundary or port publishing.
    assert "network_mode" not in gateway
    assert "ports" not in gateway
    assert "networks" not in gateway
    # The container config path is replaced, not appended.
    assert gateway["volumes"] == ["./litellm/config.dev-oracle2.yaml:/app/config.yaml:ro"]

def test_overlay_pins_tailscale_address_into_extra_hosts() -> None:
    overlay = _load(DEV_ORACLE2_COMPOSE)
    extra_hosts = overlay["services"]["litellm-gateway"]["extra_hosts"]
    assert extra_hosts == [
        "${ORACLE2_BODYSENSE_HOSTNAME:-oracle.taile92a8e.ts.net}"
        ":${ORACLE2_BODYSENSE_TAILSCALE_IP:-100.68.237.116}"
    ]

def test_dev_env_opt_in_requires_key_and_validates_tailscale_range() -> None:
    text = DEV_ENV.read_text(encoding="utf-8")
    # Default stays off so ordinary dev, staging and production are unaffected.
    assert "BODYSENSE_LITELLM_DEV_ORACLE2=${BODYSENSE_LITELLM_DEV_ORACLE2:-0}" in text
    # The key must never be defaulted to a value.
    assert not re.search(r"ORACLE2_BODYSENSE_API_KEY:-[^}\s]", text)
    # Fail closed without a key.
    assert "ORACLE2_BODYSENSE_API_KEY is empty" in text
    # Only the tailnet CGNAT range is accepted.
    assert "a == 100 && b >= 64 && b <= 127" in text
    assert "ORACLE2_BODYSENSE_TAILSCALE_IP" in text

def test_no_oracle2_secret_material_is_committed() -> None:
    for path in (
        DEV_ORACLE2_CONFIG,
        DEV_ORACLE2_COMPOSE,
        DEV_ENV,
        EVAL_SCRIPT,
        ROOT / "docs" / "runbooks" / "dev-oracle2-gateway.md",
    ):
        if not path.exists():
            continue
        text = path.read_text(encoding="utf-8")
        # Only the placeholder variable name is allowed; no literal key values.
        assert "sk-" not in text.replace("sk-bodysense-dev-gateway", "").replace(
            "sk-synthetic-not-a-real-key", ""
        ), path.name
        # A documented assignment is allowed only in angle-bracket placeholder
        # form (e.g. ORACLE2_BODYSENSE_API_KEY=<dev-only Virtual Key>); a literal
        # value is not.
        for match in re.finditer(r"ORACLE2_BODYSENSE_API_KEY\s*=\s*(\S+)", text):
            assert match.group(1).startswith("<"), (path.name, match.group(1))

def test_eval_script_targets_only_fixed_non_fallback_groups() -> None:
    text = EVAL_SCRIPT.read_text(encoding="utf-8")
    assert "bodysense-dev-oracle2-windhub" in text
    assert "bodysense-dev-oracle2-kunyou" in text
    assert "bodysense-dev-oracle2-nanliang" in text
    # Pending channels must be marked pending so a surprise 200 is surfaced.
    assert "pending=True" in text
    for capability in ("chat", "json", "tool", "stream"):
        assert f'"{capability}"' in text, capability
    for metric in ("latency_seconds", "usage", "json_valid", "tool_call_count", "saw_done"):
        assert metric in text, metric

def _run_dev_env(
    env_overrides: dict[str, str], *, call: str = ""
) -> subprocess.CompletedProcess[str]:
    """Source scripts/dev-env.sh in a clean shell; never touches the network."""
    env = {
        "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
        # Point at a non-existent env file so a developer's .env.dev.local cannot
        # influence the assertion.
        "BODYSENSE_DEV_ENV_FILE": "/nonexistent/bodysense-test.env",
        **env_overrides,
    }
    script = f'source "{DEV_ENV}" && {call}' if call else f'source "{DEV_ENV}" && echo SOURCED_OK'
    return subprocess.run(
        ["bash", "-c", script],
        capture_output=True,
        text=True,
        env=env,
        check=False,
    )

def test_tailscale_ip_validation_accepts_only_cgnat() -> None:
    cases = {
        "100.68.237.116": 0,
        "100.64.0.1": 0,
        "100.127.255.254": 0,
        # Outside 100.64.0.0/10.
        "100.63.255.255": 1,
        "100.128.0.1": 1,
        "103.84.155.217": 1,
        "10.0.0.1": 1,
        "not-an-ip": 1,
        "100.68.237": 1,
        "100.68.237.999": 1,
        "": 1,
    }
    for value, expected in cases.items():
        completed = _run_dev_env({}, call=f'oracle2_is_tailscale_ipv4 "{value}"')
        assert completed.returncode == expected, (value, completed.stderr)

def test_opt_in_rejects_explicit_override_outside_tailscale_range() -> None:
    for bad in ("103.84.155.217", "10.0.0.1", "not-an-ip"):
        completed = _run_dev_env(
            {
                "BODYSENSE_LITELLM_DEV_ORACLE2": "1",
                "ORACLE2_BODYSENSE_API_KEY": "placeholder-not-a-real-key",
                "ORACLE2_BODYSENSE_TAILSCALE_IP": bad,
            }
        )
        assert completed.returncode != 0, bad
        assert "is not a Tailscale 100.64.0.0/10 IPv4 address" in completed.stderr, completed.stderr
        assert "BODYSENSE_DEV_COMPOSE_OVERLAY=" not in completed.stdout

def test_opt_in_accepts_explicit_tailscale_override_and_resolves_overlay() -> None:
    completed = _run_dev_env(
        {
            "BODYSENSE_LITELLM_DEV_ORACLE2": "1",
            "ORACLE2_BODYSENSE_API_KEY": "placeholder-not-a-real-key",
            "ORACLE2_BODYSENSE_TAILSCALE_IP": "100.68.237.116",
        },
        call=(
            'printf "overlay=%s\\nip=%s\\n" '
            '"$BODYSENSE_DEV_COMPOSE_OVERLAY" "$ORACLE2_BODYSENSE_TAILSCALE_IP"'
        ),
    )
    assert completed.returncode == 0, completed.stderr
    assert "overlay=" + str(DEV_ORACLE2_COMPOSE) in completed.stdout
    assert "ip=100.68.237.116" in completed.stdout

def test_opt_in_without_key_fails_closed() -> None:
    completed = _run_dev_env(
        {
            "BODYSENSE_LITELLM_DEV_ORACLE2": "1",
            "ORACLE2_BODYSENSE_TAILSCALE_IP": "100.68.237.116",
        }
    )
    assert completed.returncode != 0
    assert "ORACLE2_BODYSENSE_API_KEY is empty" in completed.stderr

def test_opt_in_off_by_default_leaves_overlay_empty() -> None:
    completed = _run_dev_env(
        {"ORACLE2_BODYSENSE_TAILSCALE_IP": "100.68.237.116"},
        call='printf "overlay=[%s]\\n" "$BODYSENSE_DEV_COMPOSE_OVERLAY"',
    )
    assert completed.returncode == 0, completed.stderr
    assert "overlay=[]" in completed.stdout

def test_auto_resolution_ignores_non_tailscale_dns_answer(monkeypatch) -> None:
    """A resolver that answers with a public address must not be accepted."""
    fake_bin = Path(tempfile.mkdtemp())
    for command, output in (
        ("getent", "103.84.155.217  STREAM oracle.taile92a8e.ts.net"),
    ):
        script = fake_bin / command
        script.write_text(f"#!/bin/sh\nprintf '%s\\n' '{output}'\n", encoding="utf-8")
        script.chmod(0o755)
    completed = _run_dev_env(
        {
            "BODYSENSE_LITELLM_DEV_ORACLE2": "1",
            "ORACLE2_BODYSENSE_API_KEY": "placeholder-not-a-real-key",
            "PATH": f"{fake_bin}:{os.environ.get('PATH', '/usr/bin:/bin')}",
        }
    )
    assert completed.returncode != 0
    assert "Could not resolve" in completed.stderr
