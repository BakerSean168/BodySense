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
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[1]
CONFIG = ROOT / "docker" / "litellm" / "config.dev-oracle2.yaml"

DEFAULT_GATEWAY_URL = "http://127.0.0.1:20112/v1"
PROBE_TIMEOUT_SECONDS = 150


def local_gateway_url(raw: str) -> str:
    """Reject non-loopback endpoints before using the local gateway credential.

    IP literals avoid DNS rebinding. Exact matching also refuses URL userinfo,
    query strings, fragments, nonstandard paths and embedded whitespace.
    """
    try:
        parts = urlsplit(raw)
        port = parts.port
    except ValueError as exc:
        raise ValueError("invalid gateway URL") from exc

    if parts.scheme != "http" or parts.hostname not in ("127.0.0.1", "::1"):
        raise ValueError("gateway must be a literal loopback HTTP endpoint")
    if port is None or not 1 <= port <= 65535:
        raise ValueError("gateway requires an explicit TCP port")
    host = "127.0.0.1" if parts.hostname == "127.0.0.1" else "[::1]"
    canonical = f"http://{host}:{port}/v1"
    if raw not in (canonical, canonical + "/"):
        raise ValueError("noncanonical local gateway URL")
    return canonical


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    """Never forward a Bearer key from the loopback gateway to a Location."""

    def redirect_request(self, request, fp, code, msg, headers, newurl):
        return None


# No environment-provided HTTP proxy or cross-host redirect may receive the
# local gateway's Authorization header, even during a synthetic acceptance run.
LOCAL_GATEWAY_OPENER = urllib.request.build_opener(
    urllib.request.ProxyHandler({}), _NoRedirect()
)


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

# ── Exact expected probe answers ─────────────────────────────────────────────
# A capability passes only when it returns exactly this, so a plausible-looking
# but wrong answer (NOT PONG, wrong JSON, wrong tool label, unrelated SSE text)
# fails instead of passing on a non-empty preview.
EXPECTED_CHAT_ANSWER = "PONG"
EXPECTED_JSON_PAYLOAD: dict[str, Any] = {"probe": "synthetic", "ok": True}
EXPECTED_TOOL_NAME = "record_synthetic_probe"
EXPECTED_TOOL_ARGUMENTS: dict[str, Any] = {"label": "dev-probe"}
EXPECTED_STREAM_LINES = ("1", "2", "3")

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
    gateway_url = local_gateway_url(gateway_url)
    request = urllib.request.Request(
        f"{gateway_url}/chat/completions",
        data=json.dumps(payload).encode("utf-8"),
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {api_key}",
        },
        method="POST",
    )
    started = time.monotonic()
    try:
        with LOCAL_GATEWAY_OPENER.open(
            request, timeout=PROBE_TIMEOUT_SECONDS
        ) as response:
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
    # content_exact is the full normalized answer and is what the verdict uses.
    # content_preview is for humans only and must never drive a pass/fail.
    result.update(
        {
            "content_exact": content.strip(),
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
    result["content_exact"] = content.strip()
    try:
        parsed = json.loads(content)
    except json.JSONDecodeError:
        result["json_valid"] = False
        result["content_preview"] = content[:120]
    else:
        result["json_valid"] = isinstance(parsed, dict)
        # The parsed payload (not a preview) is what the verdict inspects.
        result["json_payload"] = parsed if isinstance(parsed, dict) else None
        result["json_keys"] = sorted(parsed) if isinstance(parsed, dict) else None
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
        raw_arguments = function.get("arguments")
        result["tool_arguments_raw"] = raw_arguments
        try:
            arguments = json.loads(raw_arguments or "{}")
        except json.JSONDecodeError:
            result["tool_arguments_valid_json"] = False
        else:
            result["tool_arguments_valid_json"] = isinstance(arguments, dict)
            # Parsed arguments are what the verdict inspects, so a matching
            # preview can never stand in for the real payload.
            result["tool_arguments"] = (
                arguments if isinstance(arguments, dict) else None
            )
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
    joined = "".join(text)
    # Ordered, non-empty lines of the reassembled stream. The verdict requires
    # exactly ["1", "2", "3"] in this order, so unrelated SSE text cannot pass.
    stream_lines = [line.strip() for line in joined.splitlines() if line.strip()]
    result.update(
        {
            "chunk_count": chunks,
            "saw_done": saw_done,
            "saw_usage": saw_usage,
            "content_exact": joined.strip(),
            "content_preview": joined.strip()[:60],
            "stream_lines": stream_lines,
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
    """The chat answer must be exactly PONG, with surrounding whitespace allowed."""
    return probe.get("content_exact") == EXPECTED_CHAT_ANSWER and bool(
        probe.get("finish_reason")
    )


def _json_ok(probe: dict[str, Any]) -> bool:
    """The JSON answer must equal the expected object with correct value types."""
    if probe.get("json_valid") is not True:
        return False
    payload = probe.get("json_payload")
    if not isinstance(payload, dict):
        return False
    # Exact key set: extra or missing keys are a failure.
    if set(payload) != set(EXPECTED_JSON_PAYLOAD):
        return False
    # type() is deliberate: in Python True == 1, so `ok` must be a real bool.
    if payload.get("probe") != EXPECTED_JSON_PAYLOAD["probe"]:
        return False
    return (
        type(payload.get("ok")) is bool and payload["ok"] is EXPECTED_JSON_PAYLOAD["ok"]
    )


def _tool_ok(probe: dict[str, Any]) -> bool:
    """The tool call must use the expected name and the exact expected label."""
    if int(probe.get("tool_call_count") or 0) < 1:
        return False
    if probe.get("tool_name") != EXPECTED_TOOL_NAME:
        return False
    if probe.get("tool_arguments_valid_json") is not True:
        return False
    arguments = probe.get("tool_arguments")
    if not isinstance(arguments, dict):
        return False
    if set(arguments) != set(EXPECTED_TOOL_ARGUMENTS):
        return False
    label = arguments.get("label")
    return type(label) is str and label == EXPECTED_TOOL_ARGUMENTS["label"]


def _stream_ok(probe: dict[str, Any]) -> bool:
    """The stream must deliver 1, 2, 3 in order and terminate with [DONE]."""
    if probe.get("saw_done") is not True:
        return False
    return probe.get("stream_lines") == list(EXPECTED_STREAM_LINES)


# HTTP 200 alone is not a pass: a 200 response can still be empty, malformed, or
# answer something other than what was asked. Each capability is judged on its
# own exact contract using full parsed values, never a truncated preview.
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
        if not probe.get("content_exact"):
            return "empty chat content"
        if not probe.get("finish_reason"):
            return "missing finish_reason"
        return f"answer is not {EXPECTED_CHAT_ANSWER!r}"
    if capability == "json":
        if probe.get("json_valid") is not True:
            return "invalid JSON body"
        payload = probe.get("json_payload")
        if not isinstance(payload, dict):
            return "JSON body is not an object"
        if set(payload) != set(EXPECTED_JSON_PAYLOAD):
            return f"unexpected JSON keys {sorted(payload)}"
        if type(payload.get("ok")) is not bool:
            return (
                f"JSON 'ok' is not a boolean (got {type(payload.get('ok')).__name__})"
            )
        return "JSON payload does not match the expected probe object"
    if capability == "tool":
        if int(probe.get("tool_call_count") or 0) < 1:
            return "no tool call returned"
        if probe.get("tool_name") != EXPECTED_TOOL_NAME:
            return f"unexpected tool name {probe.get('tool_name')!r}"
        if probe.get("tool_arguments_valid_json") is not True:
            return "tool arguments are not a JSON object"
        arguments = probe.get("tool_arguments")
        if not isinstance(arguments, dict):
            return "tool arguments are not a JSON object"
        if set(arguments) != set(EXPECTED_TOOL_ARGUMENTS):
            return f"unexpected tool argument keys {sorted(arguments)}"
        if arguments.get("label") != EXPECTED_TOOL_ARGUMENTS["label"]:
            return f"tool label is not {EXPECTED_TOOL_ARGUMENTS['label']!r}"
        return "tool arguments do not match the expected probe call"
    if capability == "stream":
        if probe.get("saw_done") is not True:
            return "stream missing [DONE]"
        lines = probe.get("stream_lines") or []
        if not lines:
            return "empty stream content"
        return f"stream content is not {list(EXPECTED_STREAM_LINES)} in order (got {lines[:6]})"
    return "unknown capability"


def evaluate(gateway_url: str, api_key: str) -> dict[str, Any]:
    report: dict[str, Any] = {
        "name": "bodysense-dev-oracle2-gateway-eval-v1",
        "config": str(CONFIG.relative_to(ROOT)),
        "gateway_url": gateway_url,
        "channels": {},
        "failures": [],
        "requalification_required": [],
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
                # deliberately, not a silent pass — so it fails the run.
                if passed:
                    report["requalification_required"].append(
                        f"{channel.name}/{capability} passed but the channel is pending"
                    )
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
    # A run is successful only when BOTH hold:
    #   1. every expected-working capability met its exact contract, and
    #   2. no pending channel unexpectedly started working — that demands
    #      deliberate re-qualification and is a failure, not a success.
    # `passed` is the single authoritative success signal so a consumer reading
    # the JSON cannot see a "passed" run that the CLI exits non-zero for.
    report["passed"] = not report["failures"] and not report["requalification_required"]
    # `ok` is kept as an explicit alias for consumers that read the older field.
    # It must never disagree with `passed`.
    report["ok"] = report["passed"]
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

    try:
        gateway_url = local_gateway_url(args.gateway_url)
    except ValueError:
        # Never echo an untrusted URL: it can contain embedded credentials.
        print(
            "Gateway URL must be http://127.0.0.1:<port>/v1 or http://[::1]:<port>/v1.",
            file=sys.stderr,
        )
        return 2

    api_key = (
        os.getenv("LITELLM_MASTER_KEY", "").strip()
        or os.getenv("LITELLM_API_KEY", "").strip()
    )
    if not api_key:
        print("LITELLM_MASTER_KEY or LITELLM_API_KEY is required", file=sys.stderr)
        return 2

    report = evaluate(gateway_url, api_key)
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
    if report["requalification_required"]:
        # A pending channel started answering. That is not a pass: it demands a
        # deliberate re-qualification before anything is wired into a live path.
        print(
            f"ORACLE2_GATEWAY_EVAL=REQUALIFY {report['requalification_required']}",
            file=sys.stderr,
        )
        return 1
    print("ORACLE2_GATEWAY_EVAL=PASS", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
