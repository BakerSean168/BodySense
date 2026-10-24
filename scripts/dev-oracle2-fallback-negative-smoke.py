#!/usr/bin/env python3
"""Reproduce LiteLLM fallback error handling with synthetic local HTTP responses.

Dev-only security regression, NOT a model-quality evaluation. No external API,
credentials, production database or real model request is used.

Run inside the WSL-only pinned and patched LiteLLM image:
  docker run --rm --network none --entrypoint python \
    -e LITELLM_LOCAL_MODEL_COST_MAP=True \
    -v "$PWD/scripts/dev-oracle2-fallback-negative-smoke.py:/tmp/fallback-smoke.py:ro" \
    bodysense/litellm-dev-oracle2:v1.102.1-safe-fallback /tmp/fallback-smoke.py

Exit 0 only when non-transient errors fail closed and eligible transient errors
can use fallback. The unpatched upstream v1.102.1 image must FAIL.
"""

from __future__ import annotations

import asyncio
import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import ClassVar

from litellm import Router

# Auth/business invalidity must not be masked by another model's answer.
NON_FALLBACK_STATUSES = (400, 401, 402, 403, 404, 409, 422)
# Provider failures eligible for fallback, if the fallback is healthy.
FALLBACK_STATUSES = (408, 429, 500, 502, 503, 504)


class StubUpstream(BaseHTTPRequestHandler):
    primary_status = 200
    attempts: ClassVar[list[str]] = []

    def do_POST(self) -> None:
        if self.path != "/v1/chat/completions":
            self.send_error(404)
            return
        size = int(self.headers.get("Content-Length", "0"))
        body = json.loads(self.rfile.read(size))
        model = str(body.get("model", ""))
        self.attempts.append(model)
        status = self.primary_status if model == "primary" else 200
        if model not in ("primary", "fallback"):
            status = 404
        if status == 200:
            payload = {
                "id": "chatcmpl-local-synthetic",
                "object": "chat.completion",
                "created": 0,
                "model": model,
                "choices": [
                    {
                        "index": 0,
                        "finish_reason": "stop",
                        "message": {
                            "role": "assistant",
                            "content": "FALLBACK_OK"
                            if model == "fallback"
                            else "PRIMARY_OK",
                        },
                    }
                ],
                "usage": {
                    "prompt_tokens": 1,
                    "completion_tokens": 1,
                    "total_tokens": 2,
                },
            }
        else:
            payload = {
                "error": {
                    "message": "synthetic upstream status",
                    "type": "invalid_request_error" if status == 400 else "api_error",
                    "code": str(status),
                }
            }
        encoded = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def log_message(self, fmt: str, *args: object) -> None:
        # Do not emit request headers (even with synthetic credentials).
        pass


async def check_case(base_url: str, status: int) -> tuple[bool, dict]:
    StubUpstream.primary_status = status
    StubUpstream.attempts = []
    router = Router(
        model_list=[
            {
                "model_name": "synthetic-primary",
                "litellm_params": {
                    "model": "openai/primary",
                    "api_base": base_url,
                    "api_key": "synthetic-not-a-real-key",
                    "timeout": 4,
                },
            },
            {
                "model_name": "synthetic-fallback",
                "litellm_params": {
                    "model": "openai/fallback",
                    "api_base": base_url,
                    "api_key": "synthetic-not-a-real-key",
                    "timeout": 4,
                },
            },
        ],
        num_retries=0,
        fallbacks=[{"synthetic-primary": ["synthetic-fallback"]}],
    )
    result = None
    error_status = None
    try:
        response = await router.acompletion(
            model="synthetic-primary",
            messages=[{"role": "user", "content": "synthetic"}],
        )
        result = response.choices[0].message.content
    except Exception as error:  # noqa: BLE001 - inspect every Router failure as test data
        error_status = getattr(error, "status_code", None)
    finally:
        close = getattr(router, "close", None)
        if callable(close):
            close()
    attempts = list(StubUpstream.attempts)
    if status == 200:
        passed = result == "PRIMARY_OK" and attempts == ["primary"]
    elif status in NON_FALLBACK_STATUSES:
        passed = result is None and error_status == status and attempts == ["primary"]
    else:
        passed = result == "FALLBACK_OK" and attempts == ["primary", "fallback"]
    return passed, {
        "upstream_status": status,
        "attempts": attempts,
        "response": result,
        "returned_error_status": error_status,
        "pass": passed,
    }


async def run_tests() -> bool:
    server = ThreadingHTTPServer(("127.0.0.1", 0), StubUpstream)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        base_url = f"http://127.0.0.1:{server.server_port}/v1"
        passed_all = True
        for status in (200, *NON_FALLBACK_STATUSES, *FALLBACK_STATUSES):
            ok, outcome = await check_case(base_url, status)
            print(json.dumps(outcome, sort_keys=True), flush=True)
            passed_all &= ok
        print(
            "FALLBACK_NEGATIVE_SMOKE=" + ("PASS" if passed_all else "FAIL"), flush=True
        )
        return passed_all
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)


if __name__ == "__main__":
    raise SystemExit(0 if asyncio.run(run_tests()) else 1)
