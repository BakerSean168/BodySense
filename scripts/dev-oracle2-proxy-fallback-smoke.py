#!/usr/bin/env python3
"""Exercise the dev-only patched LiteLLM *proxy* with local synthetic HTTP.

Run inside the patched image with --network none. No provider credentials,
network access, or production services are involved. Complements the Router
SDK negative smoke by exercising the actual OpenAI-compatible proxy endpoint.
"""

from __future__ import annotations

import json
import os
import runpy
import socket
import subprocess
import tempfile
import threading
import time
from http.server import ThreadingHTTPServer
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

STUB = runpy.run_path(
    str(Path(__file__).with_name("dev-oracle2-fallback-negative-smoke.py"))
)["StubUpstream"]
AUTH = "sk-synthetic-proxy-smoke"
CASES = (200, 400, 401, 403, 408, 429, 500, 503)


def free_port() -> int:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


def proxy_call(url: str) -> tuple[int, str | None]:
    payload = json.dumps(
        {
            "model": "synthetic-primary",
            "messages": [{"role": "user", "content": "synthetic"}],
        }
    ).encode("utf-8")
    req = Request(
        url,
        method="POST",
        data=payload,
        headers={
            "Authorization": f"Bearer {AUTH}",
            "Content-Type": "application/json",
        },
    )
    try:
        with urlopen(req, timeout=12) as response:
            body = json.load(response)
            return response.status, body["choices"][0]["message"]["content"]
    except HTTPError as error:
        error.read()
        return error.code, None


def run() -> bool:
    server = ThreadingHTTPServer(("127.0.0.1", 0), STUB)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        with tempfile.TemporaryDirectory() as tmp:
            config = Path(tmp) / "litellm-proxy-synthetic.yaml"
            config.write_text(
                f"""model_list:
  - model_name: synthetic-primary
    litellm_params:
      model: openai/primary
      api_base: http://127.0.0.1:{server.server_port}/v1
      api_key: synthetic-not-a-real-key
      timeout: 4
  - model_name: synthetic-fallback
    litellm_params:
      model: openai/fallback
      api_base: http://127.0.0.1:{server.server_port}/v1
      api_key: synthetic-not-a-real-key
      timeout: 4
router_settings:
  num_retries: 0
  # Synthetic cases reuse one proxy process; never let one failure quarantine
  # the primary for later cases. This does NOT modify the real gateway config.
  disable_cooldowns: true
  allowed_fails: 100000
  fallbacks:
    - synthetic-primary:
        - synthetic-fallback
general_settings:
  master_key: {AUTH}
""",
                encoding="utf-8",
            )
            port = free_port()
            with (Path(tmp) / "proxy.log").open("wb") as log:
                process = subprocess.Popen(
                    [
                        "litellm",
                        "--config",
                        str(config),
                        "--host",
                        "127.0.0.1",
                        "--port",
                        str(port),
                    ],
                    env={
                        **os.environ,
                        "LITELLM_LOCAL_MODEL_COST_MAP": "True",
                        "LITELLM_MASTER_KEY": AUTH,
                    },
                    stdout=log,
                    stderr=subprocess.STDOUT,
                )
                try:
                    health = f"http://127.0.0.1:{port}/health/liveliness"
                    for _ in range(90):
                        if process.poll() is not None:
                            raise RuntimeError(
                                "synthetic LiteLLM proxy exited before health"
                            )
                        try:
                            with urlopen(health, timeout=1) as response:
                                if response.status == 200:
                                    break
                        except (URLError, TimeoutError):
                            time.sleep(0.5)
                    else:
                        raise RuntimeError(
                            "synthetic LiteLLM proxy did not become healthy"
                        )

                    ok_all = True
                    for upstream_status in CASES:
                        STUB.primary_status = upstream_status
                        STUB.attempts = []
                        status, answer = proxy_call(
                            f"http://127.0.0.1:{port}/v1/chat/completions"
                        )
                        attempts = list(STUB.attempts)
                        if upstream_status == 200:
                            ok = (
                                status == 200
                                and answer == "PRIMARY_OK"
                                and attempts == ["primary"]
                            )
                        elif upstream_status in (400, 401, 403):
                            ok = (
                                status == upstream_status
                                and answer is None
                                and attempts == ["primary"]
                            )
                        else:
                            ok = (
                                status == 200
                                and answer == "FALLBACK_OK"
                                and attempts
                                == [
                                    "primary",
                                    "fallback",
                                ]
                            )
                        print(
                            json.dumps(
                                {
                                    "upstream_status": upstream_status,
                                    "http_status": status,
                                    "attempts": attempts,
                                    "pass": ok,
                                },
                                sort_keys=True,
                            ),
                            flush=True,
                        )
                        ok_all &= ok
                    print(
                        "PROXY_FALLBACK_NEGATIVE_SMOKE="
                        + ("PASS" if ok_all else "FAIL"),
                        flush=True,
                    )
                    return ok_all
                finally:
                    process.terminate()
                    try:
                        process.wait(timeout=5)
                    except subprocess.TimeoutExpired:
                        process.kill()
                        process.wait(timeout=5)
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)


if __name__ == "__main__":
    raise SystemExit(0 if run() else 1)
