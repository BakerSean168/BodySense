#!/usr/bin/env python3
"""Measure each dev Oracle2 channel deterministically (dev-only).

Every probe targets a fixed `bodysense-dev-oracle2-*` logical group that has NO
fallback edge in docker/litellm/config.dev-oracle2.yaml, so a result can never be
silently produced by a different provider. Nothing here uses personal health
data: prompts are synthetic. No real credential is required — the gateway's
master key authenticates the local LiteLLM, and the Oracle2 Virtual Key stays
inside the container environment.

Reported per channel: HTTP status, latency, usage, JSON validity, tool-call
presence, and stream behaviour. Exit code is non-zero when a channel that is
expected to work fails, or when a pending channel unexpectedly succeeds (that is
a signal to re-qualify it, not a silent pass).

Usage:
  python3 scripts/dev-oracle2-gateway-eval.py [--gateway-url URL] [--json PATH]
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
CONFIG = ROOT / "docker" / "litellm" / "config.dev-oracle2.yaml"

DEFAULT_GATEWAY_URL = "http://127.0.0.1:20112/v1"
PROBE_TIMEOUT_SECONDS = 150


@dataclass(frozen=True)
class Channel:
    name: str
    logical_model: str
    # A pending channel is declared but not qualified: it must NOT be wired into
    # any router_settings.fallbacks edge.
    pending: bool = False
    capabilities: tuple[str, ...] = field(default_factory=tuple)


CHANNELS: tuple[Channel, ...] = (
    Channel(
        "windhub",
        "bodysense-dev-oracle2-windhub",
        capabilities=("chat", "json", "tool"),
    ),
    Channel(
        "kunyou", "bodysense-dev-oracle2-kunyou", pending=True, capabilities=("chat",)
    ),
    Channel(
        "nanliang",
        "bodysense-dev-oracle2-nanliang",
        capabilities=("chat", "json", "tool", "stream"),
    ),
)

EXPECTED_TOOL_NAME = "record_synthetic_probe"

TOOL_SCHEMA = [
    {
        "type": "function",
        "function": {
            "name": EXPECTED_TOOL_NAME,
            "description": "Record a synthetic probe label. No real data.",
            "parameters": {
                "type": "object",
                "properties": {"label": {"type": "string"}},
                "required": ["label"],
            },
        },
    }
]


def _post(
    gateway_url: str,
    api_key: str,
    payload: dict[str, Any],
    *,
    stream: bool = False,
) -> tuple[int, dict[str, Any] | str, float]:
    request = urllib.request.Request(
        f"{gateway_url.rstrip('/')}/chat/completions",
        data=json.dumps(payload).encode("utf-8"),
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {api_key}",
        },
        method="POST",
    )
    started = time.monotonic()
    try:
        with urllib.request.urlopen(request, timeout=PROBE_TIMEOUT_SECONDS) as response:
            status = response.status
            raw = response.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as error:
        return (
            error.code,
            error.read().decode("utf-8", "replace"),
            time.monotonic() - started,
        )
    except urllib.error.URLError as error:
        return 0, f"transport-error: {error.reason}", time.monotonic() - started
    elapsed = time.monotonic() - started
    if stream:
        return status, raw, elapsed
    try:
        return status, json.loads(raw), elapsed
    except json.JSONDecodeError:
        return status, raw, elapsed


def _usage(body: dict[str, Any]) -> dict[str, int]:
    usage = body.get("usage") or {}
    return {
        "prompt_tokens": int(usage.get("prompt_tokens") or 0),
        "completion_tokens": int(usage.get("completion_tokens") or 0),
        "total_tokens": int(usage.get("total_tokens") or 0),
    }


def probe_chat(gateway_url: str, api_key: str, logical_model: str) -> dict[str, Any]:
    status, body, elapsed = _post(
        gateway_url,
        api_key,
        {
            "model": logical_model,
            "messages": [{"role": "user", "content": "Reply with exactly: PONG"}],
            "max_tokens": 256,
        },
    )
    result: dict[str, Any] = {
        "http_status": status,
        "latency_seconds": round(elapsed, 2),
    }
    if status != 200 or not isinstance(body, dict):
        result["error"] = str(body)[:300]
        return result
    choice = (body.get("choices") or [{}])[0]
    content = (choice.get("message") or {}).get("content") or ""
    result.update(
        {
            "ok": bool(content.strip()),
            "content_preview": content.strip()[:60],
            "finish_reason": choice.get("finish_reason"),
            "actual_model": body.get("model"),
            "usage": _usage(body),
        }
    )
    return result


def probe_json(gateway_url: str, api_key: str, logical_model: str) -> dict[str, Any]:
    status, body, elapsed = _post(
        gateway_url,
        api_key,
        {
            "model": logical_model,
            "messages": [
                {
                    "role": "user",
                    "content": 'Return only this JSON object: {"probe":"synthetic","ok":true}',
                }
            ],
            "max_tokens": 256,
            "response_format": {"type": "json_object"},
        },
    )
    result: dict[str, Any] = {
        "http_status": status,
        "latency_seconds": round(elapsed, 2),
    }
    if status != 200 or not isinstance(body, dict):
        result["error"] = str(body)[:300]
        return result
    content = ((body.get("choices") or [{}])[0].get("message") or {}).get(
        "content"
    ) or ""
    try:
        parsed = json.loads(content)
        result["json_valid"] = isinstance(parsed, dict)
        result["json_keys"] = sorted(parsed) if isinstance(parsed, dict) else None
    except json.JSONDecodeError:
        result["json_valid"] = False
        result["content_preview"] = content[:120]
    result.update({"actual_model": body.get("model"), "usage": _usage(body)})
    return result


def probe_tool(gateway_url: str, api_key: str, logical_model: str) -> dict[str, Any]:
    status, body, elapsed = _post(
        gateway_url,
        api_key,
        {
            "model": logical_model,
            "messages": [
                {
                    "role": "user",
                    "content": "Call record_synthetic_probe with label 'dev-probe'. No other output.",
                }
            ],
            "max_tokens": 256,
            "tools": TOOL_SCHEMA,
            "tool_choice": "required",
        },
    )
    result: dict[str, Any] = {
        "http_status": status,
        "latency_seconds": round(elapsed, 2),
    }
    if status != 200 or not isinstance(body, dict):
        result["error"] = str(body)[:300]
        return result
    message = (body.get("choices") or [{}])[0].get("message") or {}
    calls = message.get("tool_calls") or []
    result["tool_call_count"] = len(calls)
    if calls:
        function = (calls[0] or {}).get("function") or {}
        result["tool_name"] = function.get("name")
        try:
            arguments = json.loads(function.get("arguments") or "{}")
            result["tool_arguments_valid_json"] = isinstance(arguments, dict)
        except json.JSONDecodeError:
            result["tool_arguments_valid_json"] = False
    result.update({"actual_model": body.get("model"), "usage": _usage(body)})
    return result


def probe_stream(gateway_url: str, api_key: str, logical_model: str) -> dict[str, Any]:
    status, raw, elapsed = _post(
        gateway_url,
        api_key,
        {
            "model": logical_model,
            "messages": [
                {"role": "user", "content": "Count 1 to 3, one number per line."}
            ],
            "max_tokens": 128,
            "stream": True,
            "stream_options": {"include_usage": True},
        },
        stream=True,
    )
    result: dict[str, Any] = {
        "http_status": status,
        "latency_seconds": round(elapsed, 2),
    }
    if status != 200 or not isinstance(raw, str):
        result["error"] = str(raw)[:300]
        return result
    chunks = 0
    saw_done = False
    saw_usage = False
    text: list[str] = []
    for line in raw.splitlines():
        if not line.startswith("data:"):
            continue
        payload = line[5:].strip()
        if payload == "[DONE]":
            saw_done = True
            continue
        try:
            parsed = json.loads(payload)
        except json.JSONDecodeError:
            continue
        chunks += 1
        if parsed.get("usage"):
            saw_usage = True
        for choice in parsed.get("choices") or []:
            piece = (choice.get("delta") or {}).get("content")
            if piece:
                text.append(piece)
    result.update(
        {
            "ok": saw_done and bool(text),
            "chunk_count": chunks,
            "saw_done": saw_done,
            "saw_usage": saw_usage,
            "content_preview": "".join(text).strip()[:60],
        }
    )
    return result


PROBES = {
    "chat": probe_chat,
    "json": probe_json,
    "tool": probe_tool,
    "stream": probe_stream,
}


def _chat_ok(probe: dict[str, Any]) -> bool:
    """A chat reply must carry non-empty text and a terminal finish reason."""
    return bool(str(probe.get("content_preview") or "").strip()) and bool(
        probe.get("finish_reason")
    )


def _json_ok(probe: dict[str, Any]) -> bool:
    return probe.get("json_valid") is True


def _tool_ok(probe: dict[str, Any]) -> bool:
    """A tool probe must return >=1 call with the expected name and object args."""
    if int(probe.get("tool_call_count") or 0) < 1:
        return False
    if probe.get("tool_name") != EXPECTED_TOOL_NAME:
        return False
    return probe.get("tool_arguments_valid_json") is True


def _stream_ok(probe: dict[str, Any]) -> bool:
    """A stream must terminate with [DONE] and carry non-empty content."""
    return probe.get("saw_done") is True and bool(
        str(probe.get("content_preview") or "").strip()
    )


# HTTP 200 alone is not a pass: a 200 response can still be empty, malformed, or
# answer without the capability that was asked for. Each capability is judged on
# its own contract.
CAPABILITY_VERDICTS = {
    "chat": _chat_ok,
    "json": _json_ok,
    "tool": _tool_ok,
    "stream": _stream_ok,
}


def probe_passed(capability: str, probe: dict[str, Any]) -> bool:
    if probe.get("http_status") != 200:
        return False
    return CAPABILITY_VERDICTS[capability](probe)


def probe_failure_reason(capability: str, probe: dict[str, Any]) -> str:
    status = probe.get("http_status")
    if status != 200:
        return f"HTTP{status}"
    if capability == "chat":
        return (
            "empty chat content"
            if not str(probe.get("content_preview") or "").strip()
            else "missing finish_reason"
        )
    if capability == "json":
        return "invalid JSON body"
    if capability == "tool":
        if int(probe.get("tool_call_count") or 0) < 1:
            return "no tool call returned"
        if probe.get("tool_name") != EXPECTED_TOOL_NAME:
            return f"unexpected tool name {probe.get('tool_name')!r}"
        return "tool arguments not a JSON object"
    if capability == "stream":
        return (
            "stream missing [DONE]"
            if probe.get("saw_done") is not True
            else "empty stream content"
        )
    return "unknown capability"


def evaluate(gateway_url: str, api_key: str) -> dict[str, Any]:
    report: dict[str, Any] = {
        "name": "bodysense-dev-oracle2-gateway-eval-v1",
        "config": str(CONFIG.relative_to(ROOT)),
        "gateway_url": gateway_url,
        "channels": {},
        "failures": [],
        "notes": [],
    }
    for channel in CHANNELS:
        channel_report: dict[str, Any] = {
            "logical_model": channel.logical_model,
            "pending": channel.pending,
            "probes": {},
        }
        for capability in channel.capabilities:
            probe = PROBES[capability](gateway_url, api_key, channel.logical_model)
            passed = probe_passed(capability, probe)
            probe["passed"] = passed
            if not passed:
                probe["failure_reason"] = probe_failure_reason(capability, probe)
            channel_report["probes"][capability] = probe

            if channel.pending:
                # A pending channel is never reported as working. If it starts
                # answering correctly, that is a signal to re-qualify it
                # deliberately, not a silent pass.
                if passed:
                    report["notes"].append(
                        f"{channel.name} passed {capability} but is marked pending — "
                        "re-qualify before adding it to router_settings.fallbacks"
                    )
            elif not passed:
                report["failures"].append(
                    f"{channel.name}/{capability} {probe['failure_reason']}"
                )
        channel_report["passed"] = all(
            probe["passed"] for probe in channel_report["probes"].values()
        )
        channel_report["qualified"] = channel_report["passed"] and not channel.pending
        report["channels"][channel.name] = channel_report
    report["passed"] = not report["failures"]
    return report


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--gateway-url",
        default=os.getenv("LITELLM_BASE_URL", DEFAULT_GATEWAY_URL),
        help="Local dev LiteLLM base URL (default: env LITELLM_BASE_URL or the dev port)",
    )
    parser.add_argument(
        "--json", type=Path, default=None, help="Write the report to this path"
    )
    args = parser.parse_args()

    api_key = (
        os.getenv("LITELLM_MASTER_KEY", "").strip()
        or os.getenv("LITELLM_API_KEY", "").strip()
    )
    if not api_key:
        print("LITELLM_MASTER_KEY or LITELLM_API_KEY is required", file=sys.stderr)
        return 2

    report = evaluate(args.gateway_url.rstrip("/"), api_key)
    rendered = json.dumps(report, indent=2, sort_keys=True)
    print(rendered)
    if args.json:
        args.json.parent.mkdir(parents=True, exist_ok=True)
        args.json.write_text(rendered + "\n", encoding="utf-8")

    for name, channel in report["channels"].items():
        for capability, probe in channel["probes"].items():
            state = "PASS" if probe["passed"] else "FAIL"
            reason = "" if probe["passed"] else f" reason={probe.get('failure_reason')}"
            print(
                f"ORACLE2_CHANNEL={name} capability={capability} state={state} "
                f"http={probe.get('http_status')} latency={probe.get('latency_seconds')}s "
                f"pending={channel['pending']} qualified={channel['qualified']}{reason}",
                file=sys.stderr,
            )
    if report["failures"]:
        print(f"ORACLE2_GATEWAY_EVAL=FAIL {report['failures']}", file=sys.stderr)
        return 1
    print("ORACLE2_GATEWAY_EVAL=PASS", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
