#!/usr/bin/env python3
"""Fail closed before launching the dev gateway with an ambiguous supplier route.

This checks only the checked-in route *shape*. A passing result is not provider
identity verification, Virtual Key authorization, or upstream inference evidence.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import yaml

ORACLE2_BASE = "os.environ/ORACLE2_BODYSENSE_BASE_URL"
SUPPLIER_QUALIFIED = re.compile(r"openai/[a-z0-9][a-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*")
PIN_REQUIRED = (
    "bodysense-diagnosis-fallback",
    "bodysense-general-fallback",
    "bodysense-dev-oracle2-nanliang",
)


def has_supplier_qualified_routes(config_path: Path) -> bool:
    try:
        config = yaml.safe_load(config_path.read_text(encoding="utf-8"))
        if not isinstance(config, dict) or not isinstance(config.get("model_list"), list):
            return False
        models = config["model_list"]
        groups = {
            entry.get("model_name"): entry.get("litellm_params")
            for entry in models
            if isinstance(entry, dict)
        }
        for name in PIN_REQUIRED:
            params = groups.get(name)
            if not isinstance(params, dict):
                return False
            if params.get("api_base") != ORACLE2_BASE:
                return False
            model = params.get("model")
            if not isinstance(model, str) or not SUPPLIER_QUALIFIED.fullmatch(model):
                return False
        return True
    except (OSError, ValueError, yaml.YAMLError):
        return False


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: dev-oracle2-route-preflight.py CONFIG_PATH", file=sys.stderr)
        return 2

    if not has_supplier_qualified_routes(Path(sys.argv[1])):
        print(
            "Oracle2 dev activation blocked: DeepSeek fallback/probe routes are "
            "unqualified or the route config is invalid. Pin the verified, "
            "authorized supplier-qualified model ID before enabling this gateway.",
            file=sys.stderr,
        )
        return 1

    print(
        "Oracle2 route structure PASS (supplier identity, client authorization, "
        "and real inference remain unverified)."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
