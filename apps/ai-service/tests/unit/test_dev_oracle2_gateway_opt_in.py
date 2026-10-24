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
import sys
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
    assert gateway["image"] == "bodysense/litellm-dev-oracle2:v1.102.1-safe-fallback"
    assert gateway["build"] == {
        "context": "./litellm",
        "dockerfile": "Dockerfile.dev-oracle2-gateway",
    }
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


def test_invalid_tailscale_override_never_leaks_input_to_logs() -> None:
    marker = "synthetic-sensitive-value-not-a-real-key"
    completed = _run_dev_env(
        {
            "BODYSENSE_LITELLM_DEV_ORACLE2": "1",
            "ORACLE2_BODYSENSE_API_KEY": "placeholder-not-a-real-key",
            "ORACLE2_BODYSENSE_TAILSCALE_IP": marker,
        }
    )
    assert completed.returncode != 0
    assert "is not a Tailscale 100.64.0.0/10 IPv4 address" in completed.stderr
    assert marker not in completed.stdout + completed.stderr
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
        (
            "chat",
            {
                "http_status": 200,
                "content_exact": "PONG",
                "content_preview": "PONG",
                "finish_reason": "stop",
            },
        ),
        (
            "json",
            {
                "http_status": 200,
                "json_valid": True,
                "json_payload": {"probe": "synthetic", "ok": True},
            },
        ),
        (
            "tool",
            {
                "http_status": 200,
                "tool_call_count": 1,
                "tool_name": "record_synthetic_probe",
                "tool_arguments_valid_json": True,
                "tool_arguments": {"label": "dev-probe"},
            },
        ),
        (
            "stream",
            {"http_status": 200, "saw_done": True, "stream_lines": ["1", "2", "3"]},
        ),
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
    _patch_all_probes_passing(module, monkeypatch)

    report = module.evaluate("http://127.0.0.1:1/v1", "placeholder")

    kunyou = report["channels"]["kunyou"]
    assert kunyou["pending"] is True
    assert kunyou["qualified"] is False
    assert any("kunyou" in item for item in report["requalification_required"])
    assert any("kunyou" in note for note in report["notes"])
    # A pending channel passing must not read as an overall success.
    assert report["ok"] is False
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


def _patch_all_probes_passing(module, monkeypatch) -> None:
    monkeypatch.setattr(
        module,
        "probe_chat",
        lambda *a, **k: {
            "http_status": 200,
            "content_exact": "PONG",
            "finish_reason": "stop",
        },
    )
    monkeypatch.setattr(
        module,
        "probe_json",
        lambda *a, **k: {
            "http_status": 200,
            "json_valid": True,
            "json_payload": {"probe": "synthetic", "ok": True},
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
            "tool_arguments": {"label": "dev-probe"},
        },
    )
    monkeypatch.setattr(
        module,
        "probe_stream",
        lambda *a, **k: {
            "http_status": 200,
            "saw_done": True,
            "stream_lines": ["1", "2", "3"],
        },
    )
    module.PROBES.update(
        chat=module.probe_chat,
        json=module.probe_json,
        tool=module.probe_tool,
        stream=module.probe_stream,
    )


def test_report_passed_is_false_when_a_pending_channel_passes(monkeypatch) -> None:
    """The JSON success flag must agree with the CLI, never report a clean run.

    A pending channel that unexpectedly answers correctly leaves `failures`
    empty, so a verdict derived only from failures would wrongly claim success.
    """
    module = _eval_module()
    _patch_all_probes_passing(module, monkeypatch)

    report = module.evaluate("http://127.0.0.1:1/v1", "placeholder")

    assert report["failures"] == []
    assert report["requalification_required"] == ["kunyou/chat passed but the channel is pending"]
    # The authoritative success flag must be False, not just the CLI exit code.
    assert report["passed"] is False
    # `ok` must never disagree with `passed`.
    assert report["ok"] is False
    # Kunyou stays unqualified and is still reported as pending.
    kunyou = report["channels"]["kunyou"]
    assert kunyou["pending"] is True
    assert kunyou["qualified"] is False


def test_report_passed_and_ok_always_agree(monkeypatch) -> None:
    """`passed` and `ok` are aliases; no scenario may make them disagree."""
    module = _eval_module()

    # Case 1: everything correct and no pending channel passing.
    def only_kunyou_fails(_g, _k, logical_model):
        if "kunyou" in logical_model:
            return {"http_status": 503, "error": "no channel"}
        return {"http_status": 200, "content_exact": "PONG", "finish_reason": "stop"}

    _patch_all_probes_passing(module, monkeypatch)
    monkeypatch.setattr(module, "probe_chat", only_kunyou_fails)
    module.PROBES.update(chat=only_kunyou_fails)
    clean = module.evaluate("http://127.0.0.1:1/v1", "placeholder")
    assert clean["passed"] is True
    assert clean["ok"] is True

    # Case 2: a pending channel passes -> both flags must be False.
    _patch_all_probes_passing(module, monkeypatch)
    requalify = module.evaluate("http://127.0.0.1:1/v1", "placeholder")
    assert requalify["passed"] is False
    assert requalify["ok"] is False


def test_pending_channel_passing_forces_a_nonzero_exit(monkeypatch, capsys) -> None:
    """Kunyou passing must re-qualify, not silently succeed: exit code 1."""
    module = _eval_module()
    _patch_all_probes_passing(module, monkeypatch)
    monkeypatch.setenv("LITELLM_MASTER_KEY", "synthetic-not-real-key")
    monkeypatch.setattr(sys, "argv", ["dev-oracle2-gateway-eval.py"])

    exit_code = module.main()

    assert exit_code == 1
    err = capsys.readouterr().err
    assert "ORACLE2_GATEWAY_EVAL=REQUALIFY" in err
    assert "kunyou" in err
    assert "ORACLE2_GATEWAY_EVAL=PASS" not in err


def test_all_channels_healthy_exits_zero(monkeypatch, capsys) -> None:
    """Only non-pending channels are expected to work, so this is a clean pass."""
    module = _eval_module()
    _patch_all_probes_passing(module, monkeypatch)

    # Kunyou stays pending and would otherwise pass, so mark only its probe as
    # failing the way the real Oracle2 HTTP503 currently does.
    def failing_chat(_gateway_url, _api_key, logical_model):
        if "kunyou" in logical_model:
            return {"http_status": 503, "error": "no channel"}
        return {"http_status": 200, "content_exact": "PONG", "finish_reason": "stop"}

    monkeypatch.setattr(module, "probe_chat", failing_chat)
    module.PROBES.update(chat=failing_chat)
    monkeypatch.setenv("LITELLM_MASTER_KEY", "synthetic-not-real-key")
    monkeypatch.setattr(sys, "argv", ["dev-oracle2-gateway-eval.py"])

    exit_code = module.main()

    assert exit_code == 0
    assert "ORACLE2_GATEWAY_EVAL=PASS" in capsys.readouterr().err


def test_failing_expected_channel_exits_nonzero(monkeypatch, capsys) -> None:
    module = _eval_module()
    _patch_all_probes_passing(module, monkeypatch)

    # WindHub (not pending) answers 200 with the wrong chat text.
    def wrong_chat(gateway_url, api_key, logical_model):
        if "windhub" in logical_model:
            return {"http_status": 200, "content_exact": "NOT PONG", "finish_reason": "stop"}
        return {"http_status": 503, "error": "no channel"}

    monkeypatch.setattr(module, "probe_chat", wrong_chat)
    module.PROBES.update(chat=wrong_chat)
    monkeypatch.setenv("LITELLM_MASTER_KEY", "synthetic-not-real-key")
    monkeypatch.setattr(sys, "argv", ["dev-oracle2-gateway-eval.py"])

    exit_code = module.main()

    assert exit_code == 1
    err = capsys.readouterr().err
    assert "ORACLE2_GATEWAY_EVAL=FAIL" in err
    assert "windhub/chat" in err


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


def test_rejected_upstream_values_are_never_echoed_to_logs() -> None:
    """Reject credential-looking inputs without logging untrusted content."""
    canary = "synthetic-sensitive-marker-not-a-key"
    attempts = (
        {
            "ORACLE2_BODYSENSE_BASE_URL": (
                f"https://user:{canary}@oracle.taile92a8e.ts.net:10446/v1"
            )
        },
        {"ORACLE2_BODYSENSE_HOSTNAME": f"{canary}.example.invalid"},
    )
    for overrides in attempts:
        completed = _run_dev_env(_opt_in_env(**overrides))
        assert completed.returncode != 0
        assert canary not in completed.stderr
        assert canary not in completed.stdout
        assert "authorized" in completed.stderr


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


# ── Exact capability contracts ───────────────────────────────────────────────
# Verdicts must use the full parsed payload, never a truncated preview, so a
# plausible-but-wrong answer fails.


def _probe_chat_with(content: str, finish_reason: str | None = "stop") -> dict:
    """Build the probe dict probe_chat would produce for a given content."""
    return {
        "http_status": 200,
        "content_exact": content.strip(),
        "content_preview": content.strip()[:60],
        "finish_reason": finish_reason,
    }


def _probe_json_with(content: str) -> dict:
    import json as _json

    probe: dict = {"http_status": 200, "content_exact": content.strip()}
    try:
        parsed = _json.loads(content)
    except _json.JSONDecodeError:
        probe["json_valid"] = False
        return probe
    probe["json_valid"] = isinstance(parsed, dict)
    probe["json_payload"] = parsed if isinstance(parsed, dict) else None
    return probe


def _probe_tool_with(name: str, arguments: object, raw: str | None = None) -> dict:
    probe: dict = {
        "http_status": 200,
        "tool_call_count": 1,
        "tool_name": name,
        "tool_arguments_raw": raw,
    }
    if isinstance(arguments, dict):
        probe["tool_arguments_valid_json"] = True
        probe["tool_arguments"] = arguments
    else:
        probe["tool_arguments_valid_json"] = False
    return probe


def _probe_stream_with(lines: list[str], saw_done: bool = True) -> dict:
    return {
        "http_status": 200,
        "saw_done": saw_done,
        "content_exact": "\n".join(lines),
        "content_preview": "\n".join(lines)[:60],
        "stream_lines": lines,
    }


def test_chat_passes_only_for_exact_pong() -> None:
    module = _eval_module()
    assert module.probe_passed("chat", _probe_chat_with("PONG")) is True
    # Surrounding whitespace is tolerated.
    assert module.probe_passed("chat", _probe_chat_with("  PONG\n")) is True
    # NOT PONG must fail even though the content is non-empty.
    for wrong in ("NOT PONG", "PONG!", "pong", "PONG PONG", "PONG.", "Sure: PONG"):
        probe = _probe_chat_with(wrong)
        assert module.probe_passed("chat", probe) is False, wrong
        assert "not 'PONG'" in module.probe_failure_reason("chat", probe), wrong
    # A PONG without a terminal finish reason is still a failure.
    assert module.probe_passed("chat", _probe_chat_with("PONG", None)) is False


def test_json_passes_only_for_the_exact_expected_object() -> None:
    module = _eval_module()
    assert module.probe_passed("json", _probe_json_with('{"probe":"synthetic","ok":true}')) is True
    wrong_payloads = [
        '{"probe":"synthetic","ok":false}',
        '{"probe":"synthetic"}',
        '{"probe":"synthetic","ok":true,"extra":1}',
        '{"probe":"other","ok":true}',
        '{"probe":"synthetic","ok":"true"}',
        '{"probe":"synthetic","ok":1}',
        "not json at all",
        '["probe","synthetic"]',
        "null",
    ]
    for content in wrong_payloads:
        probe = _probe_json_with(content)
        assert module.probe_passed("json", probe) is False, content
        assert module.probe_failure_reason("json", probe), content


def test_json_rejects_non_boolean_ok_even_when_truthy() -> None:
    module = _eval_module()
    # 1 == True in Python, so the verdict must check the type explicitly.
    probe = _probe_json_with('{"probe":"synthetic","ok":1}')
    assert module.probe_passed("json", probe) is False
    assert "not a boolean" in module.probe_failure_reason("json", probe)


def test_tool_passes_only_for_expected_name_and_exact_label() -> None:
    module = _eval_module()
    assert (
        module.probe_passed(
            "tool", _probe_tool_with("record_synthetic_probe", {"label": "dev-probe"})
        )
        is True
    )
    wrong = [
        ("record_synthetic_probe", {"label": "prod-probe"}),
        ("record_synthetic_probe", {"label": "dev-probe-2"}),
        ("record_synthetic_probe", {"label": ""}),
        ("record_synthetic_probe", {"label": 123}),
        ("record_synthetic_probe", {"other": "dev-probe"}),
        ("record_synthetic_probe", {"label": "dev-probe", "extra": 1}),
        ("wrong_tool", {"label": "dev-probe"}),
    ]
    for name, arguments in wrong:
        probe = _probe_tool_with(name, arguments)
        assert module.probe_passed("tool", probe) is False, (name, arguments)
        assert module.probe_failure_reason("tool", probe), (name, arguments)
    # No call at all.
    assert module.probe_passed("tool", {"http_status": 200, "tool_call_count": 0}) is False


def test_stream_passes_only_for_ordered_1_2_3_with_done() -> None:
    module = _eval_module()
    assert module.probe_passed("stream", _probe_stream_with(["1", "2", "3"])) is True
    wrong = [
        (["1", "2"], True),
        (["1", "2", "3", "4"], True),
        (["3", "2", "1"], True),
        (["1", "1", "1"], True),
        (["hello", "world"], True),
        ([], True),
        (["1", "2", "3"], False),
    ]
    for lines, saw_done in wrong:
        probe = _probe_stream_with(lines, saw_done)
        assert module.probe_passed("stream", probe) is False, (lines, saw_done)
        assert module.probe_failure_reason("stream", probe), (lines, saw_done)


def test_unrelated_sse_text_cannot_pass_the_stream_probe() -> None:
    """A stream carrying unrelated text must fail even though it is non-empty."""
    module = _eval_module()
    probe = _probe_stream_with(["Here", "are", "the", "numbers", "1", "2", "3"])
    assert module.probe_passed("stream", probe) is False
    assert "[DONE]" not in module.probe_failure_reason("stream", probe)


@pytest.mark.parametrize(
    "probe",
    [
        # A long content preview that starts with PONG but is not PONG.
        {
            "http_status": 200,
            "content_exact": "PONG" + " and here is a long explanation" * 5,
            "content_preview": "PONG and here is a long explanation"[:60],
            "finish_reason": "stop",
        },
        # A truncated JSON preview that looks right but parses to the wrong thing.
        {
            "http_status": 200,
            "json_valid": True,
            "json_payload": {"probe": "synthetic", "ok": False},
            "content_preview": '{"probe":"synthetic","ok":true}'[:60],
        },
    ],
)
def test_truncated_preview_cannot_rescue_a_wrong_answer(probe: dict) -> None:
    module = _eval_module()
    capability = "chat" if "content_exact" in probe and "finish_reason" in probe else "json"
    assert module.probe_passed(capability, probe) is False


@pytest.mark.parametrize(
    ("entrypoint", "action"),
    [
        (DEV_INFRA, "up"),
        (DEV_INFRA, "restart"),
        (DEV_INFRA, "restart-gateway"),
        (DEV_RUNTIME, "up"),
        (DEV_RUNTIME, "restart"),
    ],
)
def test_unqualified_route_blocks_all_mutating_start_paths_before_docker(
    tmp_path: Path, entrypoint: Path, action: str
) -> None:
    fake_docker = tmp_path / "docker"
    fake_docker.write_text(
        '#!/bin/sh\\nprintf "%s\\n" "$*" >> "$DOCKER_TRACE"\\n',
        encoding="utf-8",
    )
    fake_docker.chmod(0o755)
    trace = tmp_path / "docker-trace"
    env = {
        **os.environ,
        "PATH": f"{tmp_path}:{os.environ['PATH']}",
        "DOCKER_TRACE": str(trace),
        "BODYSENSE_DEV_ENV_FILE": str(tmp_path / "missing"),
        "BODYSENSE_DEV_RUNTIME_DIR": str(tmp_path / "runtime"),
        "BODYSENSE_LITELLM_DEV_ORACLE2": "1",
        "ORACLE2_BODYSENSE_API_KEY": "synthetic-not-a-real-key",
        "ORACLE2_BODYSENSE_TAILSCALE_IP": "100.68.237.116",
        "ORACLE2_BODYSENSE_BASE_URL": AUTHORIZED_BASE_URL,
        "ORACLE2_BODYSENSE_HOSTNAME": AUTHORIZED_HOSTNAME,
    }
    result = subprocess.run(
        ["bash", str(entrypoint), action],
        env=env,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode != 0
    assert "supplier-qualified" in result.stderr
    assert not trace.exists(), "start or restart reached Docker before route validation"


def _supplier_route_preflight(path: Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(ROOT / "scripts/dev-oracle2-route-preflight.py"), str(path)],
        capture_output=True,
        text=True,
        check=False,
    )


def test_supplier_route_preflight_blocks_current_ambiguous_deepseek() -> None:
    result = _supplier_route_preflight(DEV_ORACLE2_CONFIG)
    assert result.returncode == 1
    assert "supplier-qualified" in result.stderr
    assert "deepseek-v4.1-flash-1017" not in result.stderr


def test_supplier_route_preflight_accepts_only_structural_pin(tmp_path: Path) -> None:
    source = DEV_ORACLE2_CONFIG.read_text(encoding="utf-8")
    old = "model: openai/deepseek-v4.1-flash-1017"
    assert source.count(old) == 3
    synthetic = source.replace(
        old,
        "model: openai/synthetic-resource/deepseek-v4.1-flash-1017",
    )
    path = tmp_path / "qualified-only.yaml"
    path.write_text(synthetic, encoding="utf-8")
    result = _supplier_route_preflight(path)
    assert result.returncode == 0, result.stderr
    assert "remain unverified" in result.stdout


def test_supplier_route_preflight_rejects_partial_pin_and_invalid_config(tmp_path: Path) -> None:
    source = DEV_ORACLE2_CONFIG.read_text(encoding="utf-8")
    path = tmp_path / "unsafe.yaml"
    path.write_text(
        source.replace(
            "model: openai/deepseek-v4.1-flash-1017",
            "model: openai/synthetic-resource/deepseek-v4.1-flash-1017",
            1,
        ),
        encoding="utf-8",
    )
    assert _supplier_route_preflight(path).returncode == 1
    path.write_text("model_list: [", encoding="utf-8")
    result = _supplier_route_preflight(path)
    assert result.returncode == 1
    assert "Traceback" not in result.stderr


@pytest.mark.parametrize(
    "unsafe_case",
    ["duplicate", "mixed_supplier", "other_model", "other_key", "invalid_entry"],
)
def test_supplier_route_preflight_rejects_deceptive_structural_pins(
    tmp_path: Path, unsafe_case: str
) -> None:
    source = DEV_ORACLE2_CONFIG.read_text(encoding="utf-8").replace(
        "model: openai/deepseek-v4.1-flash-1017",
        "model: openai/synthetic-resource/deepseek-v4.1-flash-1017",
    )
    config = yaml.safe_load(source)
    assert isinstance(config, dict)
    models = config["model_list"]
    fallback = next(
        group for group in models if group["model_name"] == "bodysense-general-fallback"
    )
    if unsafe_case == "duplicate":
        # A dictionary-based check would hide one deployment in this group.
        models.append(
            {
                "model_name": fallback["model_name"],
                "litellm_params": {
                    **fallback["litellm_params"],
                    "model": "openai/deepseek-v4.1-flash-1017",
                },
            }
        )
    elif unsafe_case == "mixed_supplier":
        fallback["litellm_params"]["model"] = "openai/synthetic-other/deepseek-v4.1-flash-1017"
    elif unsafe_case == "other_model":
        fallback["litellm_params"]["model"] = "openai/synthetic-resource/paid-model"
    elif unsafe_case == "other_key":
        fallback["litellm_params"]["api_key"] = "os.environ/UNRELATED_KEY"
    else:
        models.append(None)

    path = tmp_path / f"{unsafe_case}.yaml"
    path.write_text(yaml.safe_dump(config), encoding="utf-8")
    result = _supplier_route_preflight(path)
    assert result.returncode == 1
    assert "Traceback" not in result.stderr


@pytest.mark.parametrize(
    "unsafe_case",
    [
        "extra_edge",
        "duplicate_edge",
        "retarget_to_pending_kunyou",
        "missing_edge",
        "context_window_fallback",
        "default_fallback",
        "retry_enabled",
    ],
)
def test_supplier_route_preflight_rejects_fallback_graph_bypass(
    tmp_path: Path, unsafe_case: str
) -> None:
    source = DEV_ORACLE2_CONFIG.read_text(encoding="utf-8").replace(
        "model: openai/deepseek-v4.1-flash-1017",
        "model: openai/synthetic-resource/deepseek-v4.1-flash-1017",
    )
    config = yaml.safe_load(source)
    router = config["router_settings"]
    edges = router["fallbacks"]
    if unsafe_case == "extra_edge":
        edges.append({"bodysense-dev-oracle2-nanliang": ["bodysense-dev-oracle2-kunyou"]})
    elif unsafe_case == "duplicate_edge":
        edges.append({"bodysense-diagnosis": ["bodysense-diagnosis-fallback"]})
    elif unsafe_case == "retarget_to_pending_kunyou":
        edges[0]["bodysense-diagnosis"] = ["bodysense-dev-oracle2-kunyou"]
    elif unsafe_case == "missing_edge":
        edges.pop()
    elif unsafe_case == "context_window_fallback":
        router["context_window_fallbacks"] = [
            {"bodysense-diagnosis": ["bodysense-dev-oracle2-kunyou"]}
        ]
    elif unsafe_case == "default_fallback":
        router["default_fallbacks"] = ["bodysense-dev-oracle2-kunyou"]
    else:
        router["num_retries"] = 1
    path = tmp_path / f"{unsafe_case}.yaml"
    path.write_text(yaml.safe_dump(config), encoding="utf-8")
    result = _supplier_route_preflight(path)
    assert result.returncode == 1
    assert "Traceback" not in result.stderr


@pytest.mark.parametrize("opt_in", ["0", "1"])
def test_restart_gateway_never_restarts_other_dev_services(tmp_path: Path, opt_in: str) -> None:
    """The safe gateway restart must never call Compose down or recreate DB/Redis."""
    fake_docker = tmp_path / "docker"
    fake_docker.write_text(
        '#!/usr/bin/env bash\nprintf "%s\\n" "$*" >> "$DOCKER_TRACE"\n',
        encoding="utf-8",
    )
    fake_docker.chmod(0o755)
    trace = tmp_path / "docker-trace"
    env = {
        **os.environ,
        "PATH": f"{tmp_path}:{os.environ['PATH']}",
        "DOCKER_TRACE": str(trace),
        "BODYSENSE_DEV_ENV_FILE": str(tmp_path / "not-present"),
        "COMPOSE_PROJECT_NAME": "bodysense-oracle2-test",
        "BODYSENSE_LITELLM_DEV_ORACLE2": opt_in,
        "ORACLE2_BODYSENSE_API_KEY": "synthetic-not-a-real-key",
        "ORACLE2_BODYSENSE_TAILSCALE_IP": "100.68.237.116",
        "ORACLE2_BODYSENSE_BASE_URL": "https://oracle.taile92a8e.ts.net:10446/v1",
        "ORACLE2_BODYSENSE_HOSTNAME": "oracle.taile92a8e.ts.net",
    }

    result = subprocess.run(
        ["bash", str(DEV_INFRA), "restart-gateway"],
        env=env,
        capture_output=True,
        text=True,
        check=False,
    )
    if opt_in == "1":
        # Shared, unqualified DeepSeek currently blocks all Oracle2 start/restart
        # actions, before any Docker invocation or shared infra mutation.
        assert result.returncode != 0
        assert "supplier-qualified" in result.stderr
        assert not trace.exists()
        return

    assert result.returncode == 0, (result.stdout, result.stderr)
    commands = trace.read_text(encoding="utf-8").splitlines()
    assert len(commands) == 2
    restart, status = commands
    assert (
        "up -d --no-deps --build --force-recreate --wait --wait-timeout 120 litellm-gateway"
        in restart
    )
    assert "postgres-dev" not in restart
    assert "redis-dev" not in restart
    assert " down " not in f" {restart} "
    assert "--remove-orphans" not in restart
    assert "ps postgres-dev redis-dev litellm-gateway" in status
    assert ("docker-compose.dev-oracle2.yml" in restart) is (opt_in == "1")
    assert "synthetic-not-a-real-key" not in "\n".join(commands)


# Local gateway master keys must never be sent to another host by the
# deferred channel-acceptance helper, even if a CLI URL or redirect is hostile.
@pytest.mark.parametrize(
    "raw",
    [
        "http://127.0.0.1:20112/v1",
        "http://127.0.0.1:20112/v1/",
        "http://[::1]:20112/v1",
    ],
)
def test_oracle2_eval_accepts_only_literal_loopback_gateway(raw: str) -> None:
    module = _eval_module()
    assert module.local_gateway_url(raw) == raw.rstrip("/")


@pytest.mark.parametrize(
    "raw",
    [
        "https://127.0.0.1:20112/v1",
        "http://localhost:20112/v1",
        "http://127.0.0.2:20112/v1",
        "http://example.invalid:20112/v1",
        "http://127.0.0.1:20112/v1?access=secret-marker",
        "http://127.0.0.1:20112/v1#secret-marker",
        "http://user:secret-marker@127.0.0.1:20112/v1",
        "http://127.0.0.1:20112/v1/../v1",
        "http://127.0.0.1:20112/v1//",
        "http://127.0.0.1:0/v1",
        "http://127.0.0.1/v1",
        "http://[::1:20112/v1",
        "http://127.0.0.1:20112/v1\n",
    ],
)
def test_oracle2_eval_rejects_untrusted_or_noncanonical_urls(raw: str) -> None:
    module = _eval_module()
    with pytest.raises(ValueError):
        module.local_gateway_url(raw)


def test_oracle2_eval_refuses_untrusted_url_without_logging_secret(monkeypatch, capsys) -> None:
    module = _eval_module()
    marker = "secret-marker-must-not-log"
    monkeypatch.setenv("LITELLM_MASTER_KEY", "synthetic-not-a-real-key")
    monkeypatch.setattr(
        sys,
        "argv",
        [
            "dev-oracle2-gateway-eval.py",
            "--gateway-url",
            f"https://user:{marker}@example.invalid/v1",
        ],
    )
    calls = []
    monkeypatch.setattr(module, "evaluate", lambda *args: calls.append(args))

    assert module.main() == 2
    captured = capsys.readouterr()
    assert marker not in captured.err + captured.out
    assert calls == []


def test_oracle2_eval_does_not_follow_redirect_with_local_bearer_key() -> None:
    from http.server import BaseHTTPRequestHandler, HTTPServer
    from threading import Thread

    module = _eval_module()
    redirected_requests: list[str] = []

    class TargetHandler(BaseHTTPRequestHandler):
        def do_GET(self):
            redirected_requests.append(self.headers.get("Authorization", ""))
            self.send_response(200)
            self.end_headers()

        def do_POST(self):
            self.do_GET()

        def log_message(self, *_args):
            pass

    target = HTTPServer(("127.0.0.1", 0), TargetHandler)

    class OriginHandler(BaseHTTPRequestHandler):
        def do_POST(self):
            self.send_response(302)
            self.send_header("Location", f"http://127.0.0.1:{target.server_port}/capture")
            self.end_headers()

        def log_message(self, *_args):
            pass

    origin = HTTPServer(("127.0.0.1", 0), OriginHandler)
    threads = [Thread(target=server.serve_forever, daemon=True) for server in (target, origin)]
    for thread in threads:
        thread.start()
    try:
        status, _body, _elapsed = module._post(
            f"http://127.0.0.1:{origin.server_port}/v1",
            "synthetic-not-a-real-key",
            {"model": "synthetic-only", "messages": []},
        )
        assert status == 302
        assert redirected_requests == []
    finally:
        for server in (origin, target):
            server.shutdown()
            server.server_close()
        for thread in threads:
            thread.join(timeout=3)
