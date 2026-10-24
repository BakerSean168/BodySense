#!/usr/bin/env python3
"""Fail-closed, build-time patch for the pinned DEV-ONLY LiteLLM Router.

The upstream v1.102.1 Router retries via *fallbacks* after any exception,
including invalid requests and authentication failures. This patch limits
cross-model-group fallback to transient HTTP failures or transport errors.

No runtime monkeypatch is used. An upstream source change fails the Docker
build instead of silently disabling the check.
"""

from __future__ import annotations

import hashlib
from pathlib import Path

ROUTER_PATH = Path("/app/.venv/lib/python3.13/site-packages/litellm/router.py")
PINNED_SHA256 = "c914de7cf3a4523707d7bc01204a7a9683f6212db2579725ab897c05cb00565e"
MARKER = "        if disable_fallbacks is True or original_model_group is None:\n            raise e\n"
CHECK = """        # BodySense dev-only: no fallback on auth, validation or unknown errors.
        # Only transient upstream statuses and transport failures can switch
        # model groups. In particular, HTTP 400/401/403 must surface unchanged.
        _fallback_status = getattr(original_exception, "status_code", None)
        _transient_status = _fallback_status in (408, 429, 500, 502, 503, 504)
        _transport_failure = _fallback_status is None and isinstance(
            original_exception, (litellm.Timeout, litellm.APIConnectionError)
        )
        if not (_transient_status or _transport_failure):
            raise original_exception

"""


def main() -> None:
    source = ROUTER_PATH.read_bytes()
    actual_sha256 = hashlib.sha256(source).hexdigest()
    if actual_sha256 != PINNED_SHA256:
        raise SystemExit("Unsupported LiteLLM router.py SHA256; refusing to patch")
    text = source.decode("utf-8")
    if text.count(MARKER) != 1:
        raise SystemExit(
            "LiteLLM Router fallback insertion point changed; refusing to patch"
        )
    ROUTER_PATH.write_text(text.replace(MARKER, MARKER + CHECK), encoding="utf-8")
    print("DEV_ORACLE2_FAIL_CLOSED_FALLBACK_PATCH=APPLIED")


if __name__ == "__main__":
    main()
