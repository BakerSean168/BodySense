#!/usr/bin/env python3
"""Inspect the production Diagnosis provider route without exposing credentials.

This script is intended to run on the production host. It reads only boolean
credential presence, Docker health, configured model identities, and LiteLLM
route metadata returned by a harmless logical-model probe. Secret values are
never written to the report.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path
from typing import Any

LOGICAL_MODEL = "bodysense-diagnosis"
PRIMARY_CREDENTIAL = "MIMO_API_KEY"
FALLBACK_CREDENTIAL = "OPENROUTER_API_KEY"


def run(
    command: list[str],
    *,
    input_text: str | None = None,
    check: bool = True,
) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        command,
        input=input_text,
        text=True,
        capture_output=True,
        check=check,
    )


def compose_prefix(root: Path) -> list[str]:
    return [
        "docker",
        "compose",
        "-f",
        str(root / "docker/docker-compose.prod.yml"),
        "--env-file",
        str(root / ".env.production"),
        "--env-file",
        str(root / ".env.production.local"),
    ]


def gateway_id(root: Path) -> str:
    completed = run([*compose_prefix(root), "ps", "-q", "litellm-gateway"])
    value = completed.stdout.strip()
    if not value:
        raise RuntimeError("production LiteLLM gateway container not found")
    return value


def docker_health(container_id: str) -> str:
    completed = run(
        [
            "docker",
            "inspect",
            container_id,
            "--format",
            "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}",
        ]
    )
    return completed.stdout.strip()


def env_present(container_id: str, name: str) -> bool:
    completed = run(
        [
            "docker",
            "exec",
            container_id,
            "sh",
            "-lc",
            f'test -n "$(printenv {name})"',
        ],
        check=False,
    )
    return completed.returncode == 0


def configured_model(config_text: str, model_name: str) -> str | None:
    lines = config_text.splitlines()
    inside = False
    for raw in lines:
        stripped = raw.strip()
        if stripped.startswith("- model_name:"):
            current = stripped.split(":", 1)[1].strip()
            inside = current == model_name
            continue
        if inside:
            match = re.match(r"model:\s*(\S+)", stripped)
            if match:
                return match.group(1)
    return None


def logical_probe(container_id: str) -> dict[str, Any]:
    code = r'''
import json
import os
import urllib.error
import urllib.request

payload = json.dumps({
    "model": "bodysense-diagnosis",
    "messages": [{"role": "user", "content": "Reply with OK only."}],
    "temperature": 0,
    "max_tokens": 8,
}).encode()
request = urllib.request.Request(
    "http://127.0.0.1:4000/v1/chat/completions",
    data=payload,
    headers={
        "Authorization": "Bearer " + os.environ["LITELLM_MASTER_KEY"],
        "Content-Type": "application/json",
    },
)
result = {
    "http_status": None,
    "actual_model": None,
    "model_group": None,
    "attempted_retries": None,
    "attempted_fallbacks": None,
    "primary_auth_missing": False,
    "fallback_auth_expired": False,
}
try:
    with urllib.request.urlopen(request, timeout=60) as response:
        result["http_status"] = response.status
        result["actual_model"] = response.headers.get("x-litellm-model-name")
        result["model_group"] = response.headers.get("x-litellm-model-group")
        retries = response.headers.get("x-litellm-attempted-retries")
        fallbacks = response.headers.get("x-litellm-attempted-fallbacks")
        result["attempted_retries"] = int(retries or 0)
        result["attempted_fallbacks"] = int(fallbacks or 0)
except urllib.error.HTTPError as error:
    result["http_status"] = error.code
    body = error.read().decode("utf-8", errors="replace").lower()
    result["primary_auth_missing"] = "api_key client option must be set" in body
    result["fallback_auth_expired"] = "api key expired" in body
except Exception:
    result["http_status"] = 0
print(json.dumps(result))
'''
    completed = run(
        ["docker", "exec", "-i", container_id, "python", "-"],
        input_text=code,
        check=False,
    )
    if completed.returncode != 0:
        return {
            "http_status": 0,
            "actual_model": None,
            "model_group": None,
            "attempted_retries": None,
            "attempted_fallbacks": None,
            "primary_auth_missing": False,
            "fallback_auth_expired": False,
            "probe_runtime_error": True,
        }
    return json.loads(completed.stdout)


def evaluate(root: Path) -> dict[str, Any]:
    config_path = root / "docker/litellm/config.yaml"
    config_text = config_path.read_text(encoding="utf-8")
    primary_model = configured_model(config_text, LOGICAL_MODEL)
    fallback_model = configured_model(config_text, "bodysense-diagnosis-fallback")

    gateway = gateway_id(root)
    health = docker_health(gateway)
    primary_present = env_present(gateway, PRIMARY_CREDENTIAL)
    fallback_present = env_present(gateway, FALLBACK_CREDENTIAL)
    probe = logical_probe(gateway)

    reasons: list[str] = []
    if health != "healthy":
        reasons.append("gateway_not_healthy")
    if not primary_present:
        reasons.append("primary_credential_missing")
    if primary_model is None:
        reasons.append("primary_model_not_configured")
    if probe.get("http_status") != 200:
        reasons.append("logical_probe_failed")
    if probe.get("actual_model") != primary_model:
        reasons.append("logical_probe_not_primary_model")
    if int(probe.get("attempted_fallbacks") or 0) != 0:
        reasons.append("logical_probe_used_fallback")
    if probe.get("primary_auth_missing") is True:
        reasons.append("primary_authentication_unavailable")
    if probe.get("fallback_auth_expired") is True:
        reasons.append("fallback_credential_expired")

    ready = not reasons
    return {
        "name": "diagnosis-production-provider-preflight-v1",
        "environment": "production",
        "logical_model": LOGICAL_MODEL,
        "configured_primary_model": primary_model,
        "configured_fallback_model": fallback_model,
        "gateway_healthy": health == "healthy",
        "primary_credential_present": primary_present,
        "fallback_credential_present": fallback_present,
        "logical_probe": probe,
        "ready_for_primary_qualification": ready,
        "reasons": reasons,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path("/opt/bodysense"))
    parser.add_argument("--json-output", type=Path)
    parser.add_argument("--require-ready", action="store_true")
    args = parser.parse_args()

    report = evaluate(args.root)
    encoded = json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    if args.json_output:
        args.json_output.parent.mkdir(parents=True, exist_ok=True)
        args.json_output.write_text(encoded, encoding="utf-8")
    else:
        sys.stdout.write(encoded)

    if args.require_ready and not report["ready_for_primary_qualification"]:
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
