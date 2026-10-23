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

import pytest
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
    for command, output in (("getent", "103.84.155.217  STREAM oracle.taile92a8e.ts.net"),):
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


# ── Probe verdicts: HTTP 200 is not a pass ───────────────────────────────────
# A gateway can answer 200 with an empty body, malformed JSON, no tool call, or a
# truncated stream. Those must be reported as failures.


def _eval_module():
    """Import the eval script as a module (it lives outside the package)."""
    import importlib.util
    import sys

    name = "oracle2_gateway_eval_under_test"
    if name in sys.modules:
        return sys.modules[name]
    spec = importlib.util.spec_from_file_location(name, EVAL_SCRIPT)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    # dataclasses resolves string annotations through sys.modules, so register
    # the module before executing it.
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


@pytest.mark.parametrize(
    ("capability", "probe"),
    [
        # 200 but empty assistant content.
        ("chat", {"http_status": 200, "content_preview": "", "finish_reason": "stop"}),
        # 200 but no terminal finish reason.
        ("chat", {"http_status": 200, "content_preview": "PONG", "finish_reason": None}),
        # 200 but the body was not valid JSON.
        ("json", {"http_status": 200, "json_valid": False}),
        # 200 but json_valid was never established.
        ("json", {"http_status": 200}),
        # 200 but no tool call was returned.
        ("tool", {"http_status": 200, "tool_call_count": 0}),
        # 200 but the wrong tool was called.
        (
            "tool",
            {
                "http_status": 200,
                "tool_call_count": 1,
                "tool_name": "some_other_tool",
                "tool_arguments_valid_json": True,
            },
        ),
        # 200 with unparseable tool arguments.
        (
            "tool",
            {
                "http_status": 200,
                "tool_call_count": 1,
                "tool_name": "record_synthetic_probe",
                "tool_arguments_valid_json": False,
            },
        ),
        # 200 but the stream never terminated.
        ("stream", {"http_status": 200, "saw_done": False, "content_preview": "1\n2\n3"}),
        # 200, terminated, but no content.
        ("stream", {"http_status": 200, "saw_done": True, "content_preview": ""}),
        # Transport/HTTP failures are never passes.
        ("chat", {"http_status": 0, "content_preview": "PONG", "finish_reason": "stop"}),
        ("json", {"http_status": 503, "json_valid": True}),
        ("tool", {"http_status": 401, "tool_call_count": 1}),
        ("stream", {"http_status": 500, "saw_done": True, "content_preview": "x"}),
    ],
)
def test_http_200_but_invalid_response_is_a_probe_failure(capability: str, probe: dict) -> None:
    module = _eval_module()
    assert module.probe_passed(capability, probe) is False
    assert module.probe_failure_reason(capability, probe)


@pytest.mark.parametrize(
    ("capability", "probe"),
    [
        ("chat", {"http_status": 200, "content_preview": "PONG", "finish_reason": "stop"}),
        ("json", {"http_status": 200, "json_valid": True}),
        (
            "tool",
            {
                "http_status": 200,
                "tool_call_count": 1,
                "tool_name": "record_synthetic_probe",
                "tool_arguments_valid_json": True,
            },
        ),
        ("stream", {"http_status": 200, "saw_done": True, "content_preview": "1\n2\n3"}),
    ],
)
def test_valid_probe_responses_pass(capability: str, probe: dict) -> None:
    module = _eval_module()
    assert module.probe_passed(capability, probe) is True


def test_tool_probe_forces_tool_choice_required() -> None:
    """An optional tool choice would let a model answer in prose and look fine."""
    text = EVAL_SCRIPT.read_text(encoding="utf-8")
    assert '"tool_choice": "required"' in text
    assert '"tool_choice": "auto"' not in text


def test_pending_channel_is_never_reported_as_qualified(monkeypatch) -> None:
    module = _eval_module()
    # Every probe "passes", including Kunyou's, which is still pending.
    monkeypatch.setattr(
        module,
        "probe_chat",
        lambda *a, **k: {
            "http_status": 200,
            "content_preview": "PONG",
            "finish_reason": "stop",
        },
    )
    monkeypatch.setattr(
        module,
        "probe_json",
        lambda *a, **k: {
            "http_status": 200,
            "json_valid": True,
        },
    )
    monkeypatch.setattr(
        module,
        "probe_tool",
        lambda *a, **k: {
            "http_status": 200,
            "tool_call_count": 1,
            "tool_name": "record_synthetic_probe",
            "tool_arguments_valid_json": True,
        },
    )
    monkeypatch.setattr(
        module,
        "probe_stream",
        lambda *a, **k: {
            "http_status": 200,
            "saw_done": True,
            "content_preview": "1\n2\n3",
        },
    )
    module.PROBES.update(
        chat=module.probe_chat,
        json=module.probe_json,
        tool=module.probe_tool,
        stream=module.probe_stream,
    )

    report = module.evaluate("http://127.0.0.1:1/v1", "placeholder")

    kunyou = report["channels"]["kunyou"]
    assert kunyou["pending"] is True
    assert kunyou["qualified"] is False
    assert any("kunyou" in note for note in report["notes"])
    assert report["channels"]["windhub"]["qualified"] is True
    assert report["channels"]["nanliang"]["qualified"] is True


def test_failing_capability_fails_the_run_even_when_http_is_200(monkeypatch) -> None:
    module = _eval_module()
    monkeypatch.setattr(
        module,
        "probe_chat",
        lambda *a, **k: {
            "http_status": 200,
            "content_preview": "",
            "finish_reason": "stop",
        },
    )
    monkeypatch.setattr(
        module,
        "probe_json",
        lambda *a, **k: {
            "http_status": 200,
            "json_valid": False,
        },
    )
    monkeypatch.setattr(
        module,
        "probe_tool",
        lambda *a, **k: {
            "http_status": 200,
            "tool_call_count": 0,
        },
    )
    monkeypatch.setattr(
        module,
        "probe_stream",
        lambda *a, **k: {
            "http_status": 200,
            "saw_done": False,
            "content_preview": "",
        },
    )
    module.PROBES.update(
        chat=module.probe_chat,
        json=module.probe_json,
        tool=module.probe_tool,
        stream=module.probe_stream,
    )

    report = module.evaluate("http://127.0.0.1:1/v1", "placeholder")

    assert report["passed"] is False
    assert report["failures"]
    assert all("HTTP200" not in failure for failure in report["failures"])


# ── Authorized upstream endpoint (single allowed destination) ────────────────
# The dev gateway carries the Oracle2 Virtual Key and real BodySense request
# content to whatever ORACLE2_BODYSENSE_BASE_URL names. Only the authorized
# endpoint may be reachable, so anything else must fail closed at env-load time.

AUTHORIZED_BASE_URL = "https://oracle.taile92a8e.ts.net:10446/v1"
AUTHORIZED_HOSTNAME = "oracle.taile92a8e.ts.net"


def _opt_in_env(**overrides: str) -> dict[str, str]:
    base = {
        "BODYSENSE_LITELLM_DEV_ORACLE2": "1",
        # Synthetic placeholder only. No real credential is used or required.
        "ORACLE2_BODYSENSE_API_KEY": "synthetic-not-real-key",
        "ORACLE2_BODYSENSE_TAILSCALE_IP": "100.68.237.116",
    }
    base.update(overrides)
    return base


def test_authorized_endpoint_enables_the_overlay() -> None:
    for url in (AUTHORIZED_BASE_URL, AUTHORIZED_BASE_URL + "/"):
        completed = _run_dev_env(
            _opt_in_env(ORACLE2_BODYSENSE_BASE_URL=url),
            call=(
                'printf "overlay=%s\\nbase=%s\\nhost=%s\\n" '
                '"$BODYSENSE_DEV_COMPOSE_OVERLAY" "$ORACLE2_BODYSENSE_BASE_URL" '
                '"$ORACLE2_BODYSENSE_HOSTNAME"'
            ),
        )
        assert completed.returncode == 0, (url, completed.stderr)
        assert "overlay=" + str(DEV_ORACLE2_COMPOSE) in completed.stdout
        # A tolerated variant is normalized back to the authorized value.
        assert f"base={AUTHORIZED_BASE_URL}" in completed.stdout
        assert f"host={AUTHORIZED_HOSTNAME}" in completed.stdout


@pytest.mark.parametrize(
    "unauthorized",
    [
        # Different scheme.
        "http://oracle.taile92a8e.ts.net:10446/v1",
        # Different host entirely.
        "https://evil.example.com/v1",
        # Lookalike suffix host.
        "https://oracle.taile92a8e.ts.net.evil.com:10446/v1",
        # Wrong port / default port.
        "https://oracle.taile92a8e.ts.net/v1",
        "https://oracle.taile92a8e.ts.net:443/v1",
        "https://oracle.taile92a8e.ts.net:10446:10446/v1",
        # Extra or different path.
        "https://oracle.taile92a8e.ts.net:10446/v1/extra",
        "https://oracle.taile92a8e.ts.net:10446",
        "https://oracle.taile92a8e.ts.net:10446/v1//",
        # Userinfo.
        "https://user:pass@oracle.taile92a8e.ts.net:10446/v1",
        "https://oracle.taile92a8e.ts.net:10446@evil.example.com/v1",
        # Query and fragment.
        "https://oracle.taile92a8e.ts.net:10446/v1?x=1",
        "https://oracle.taile92a8e.ts.net:10446/v1#frag",
        # Internal/metadata targets.
        "http://127.0.0.1:8000/v1",
        "http://169.254.169.254/latest/meta-data",
        # Whitespace and case tricks.
        " https://oracle.taile92a8e.ts.net:10446/v1",
        "https://ORACLE.taile92a8e.ts.net:10446/v1",
    ],
)
def test_unauthorized_base_url_fails_closed_without_exporting_overlay(
    unauthorized: str,
) -> None:
    completed = _run_dev_env(
        _opt_in_env(ORACLE2_BODYSENSE_BASE_URL=unauthorized),
        call=(
            'printf "overlay=[%s]\\nkey=[%s]\\n" "$BODYSENSE_DEV_COMPOSE_OVERLAY" '
            '"${ORACLE2_BODYSENSE_API_KEY:-UNSET}"'
        ),
    )
    assert completed.returncode != 0, unauthorized
    assert "not the authorized Oracle2 endpoint" in completed.stderr, completed.stderr
    # Fail closed: the overlay is never exported, so Compose cannot mount the
    # Oracle2 config, and the key is not left in the environment.
    assert "overlay=[]" not in completed.stdout
    assert "overlay=[" + str(DEV_ORACLE2_COMPOSE) + "]" not in completed.stdout


@pytest.mark.parametrize(
    "unauthorized_host",
    [
        "evil.example.com",
        "oracle.taile92a8e.ts.net.evil.com",
        "oracle.taile92a8e.ts.net:10446",
        "ORACLE.taile92a8e.ts.net",
        "*.taile92a8e.ts.net",
        "oracle",
        "100.68.237.116",
    ],
)
def test_unauthorized_hostname_fails_closed(unauthorized_host: str) -> None:
    completed = _run_dev_env(
        _opt_in_env(ORACLE2_BODYSENSE_HOSTNAME=unauthorized_host),
        call='printf "overlay=[%s]\\n" "$BODYSENSE_DEV_COMPOSE_OVERLAY"',
    )
    assert completed.returncode != 0, unauthorized_host
    assert "not the authorized Oracle2 MagicDNS name" in completed.stderr, completed.stderr
    assert "overlay=[" + str(DEV_ORACLE2_COMPOSE) + "]" not in completed.stdout


def test_unauthorized_target_is_refused_before_dns_resolution() -> None:
    """An unauthorized target must not even trigger a MagicDNS lookup."""
    fake_bin = Path(tempfile.mkdtemp())
    marker = fake_bin / "getent-was-called"
    getent = fake_bin / "getent"
    getent.write_text(f"#!/bin/sh\ntouch {marker}\nexit 0\n", encoding="utf-8")
    getent.chmod(0o755)

    completed = _run_dev_env(
        {
            "BODYSENSE_LITELLM_DEV_ORACLE2": "1",
            "ORACLE2_BODYSENSE_API_KEY": "synthetic-not-real-key",
            "ORACLE2_BODYSENSE_BASE_URL": "https://evil.example.com/v1",
            "PATH": f"{fake_bin}:{os.environ.get('PATH', '/usr/bin:/bin')}",
        }
    )
    assert completed.returncode != 0
    assert "not the authorized Oracle2 endpoint" in completed.stderr
    assert not marker.exists(), "getent ran for an unauthorized target"


def test_authorized_target_still_resolves_tailscale_ip() -> None:
    """The authorized path keeps the existing CGNAT resolution behaviour."""
    completed = _run_dev_env(
        {
            "BODYSENSE_LITELLM_DEV_ORACLE2": "1",
            "ORACLE2_BODYSENSE_API_KEY": "synthetic-not-real-key",
        },
        call='printf "ip=[%s]\\n" "${ORACLE2_BODYSENSE_TAILSCALE_IP:-}"',
    )
    # Either it resolved a CGNAT address, or the host has no Tailscale DNS and it
    # fails closed with the documented message. Never a silent public address.
    if completed.returncode == 0:
        assert re.fullmatch(
            r"100\.(6[4-9]|[7-9][0-9]|1[01][0-9]|12[0-7])\.\d+\.\d+",
            completed.stdout.split("ip=[")[1].split("]")[0],
        )
    else:
        assert "Could not resolve" in completed.stderr


def test_dev_env_declares_the_single_authorized_endpoint() -> None:
    text = DEV_ENV.read_text(encoding="utf-8")
    assert "ORACLE2_AUTHORIZED_HOST='oracle.taile92a8e.ts.net'" in text
    assert "ORACLE2_AUTHORIZED_PORT='10446'" in text
    assert (
        'ORACLE2_AUTHORIZED_BASE_URL="https://${ORACLE2_AUTHORIZED_HOST}:${ORACLE2_AUTHORIZED_PORT}/v1"'
        in text
    )
    # The authorization check must run before the overlay is exported.
    auth_check = text.index("oracle2_is_authorized_base_url")
    overlay_export = text.index(
        "export BODYSENSE_DEV_COMPOSE_OVERLAY=$ROOT/docker/docker-compose.dev-oracle2.yml"
    )
    assert auth_check < overlay_export
