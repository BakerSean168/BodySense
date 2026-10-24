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
ORACLE2_KEY = "os.environ/ORACLE2_BODYSENSE_API_KEY"
DEEPSEEK_MODEL = "deepseek-v4.1-flash-1017"
SUPPLIER_QUALIFIED = re.compile(
    rf"openai/[a-z0-9][a-z0-9_.-]*/{re.escape(DEEPSEEK_MODEL)}"
)
PIN_REQUIRED = (
    "bodysense-diagnosis-fallback",
    "bodysense-general-fallback",
    "bodysense-dev-oracle2-nanliang",
)


def has_supplier_qualified_routes(config_path: Path) -> bool:
    try:
        config = yaml.safe_load(config_path.read_text(encoding="utf-8"))
        if not isinstance(config, dict) or not isinstance(
            config.get("model_list"), list
        ):
            return False

        # Duplicate model_name entries are separate LiteLLM deployments. A dict
        # comprehension would silently discard all but one and falsely certify
        # the group even if another deployment used a shared supplier alias.
        groups = {}
        for entry in config["model_list"]:
            if not isinstance(entry, dict):
                return False
            name = entry.get("model_name")
            params = entry.get("litellm_params")
            if (
                not isinstance(name, str)
                or not name
                or name in groups
                or not isinstance(params, dict)
            ):
                return False
            groups[name] = params

        pinned_model = None
        for name in PIN_REQUIRED:
            params = groups.get(name)
            if not isinstance(params, dict):
                return False
            if params.get("api_base") != ORACLE2_BASE:
                return False
            if params.get("api_key") != ORACLE2_KEY:
                return False
            model = params.get("model")
            if not isinstance(model, str) or not SUPPLIER_QUALIFIED.fullmatch(model):
                return False
            if pinned_model is not None and model != pinned_model:
                return False
            pinned_model = model
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
