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
# Reviewed supplier identity is still unresolved. A qualified route name alone
# must not make this opt-in deployable: once the original Nanliang resource and
# dedicated client permission are independently verified, explicitly pin that
# resource here in a reviewed change. Do not supply this value from an env var.
APPROVED_DEEPSEEK_RESOURCE_ID: str | None = None
# A qualified DeepSeek name alone is insufficient: every other deployment
# must also retain its existing provider, credentials, and transport boundary.
# These are structural invariants, not a model-quality or supplier approval.
EXPECTED_STATIC_MODELS = {
    "bodysense-diagnosis": "openai/windhub-free/glm-5.3",
    "bodysense-consultation": "openai/windhub-free/glm-5.3",
    "bodysense-structured": "openai/windhub-free/glm-5.3",
    "bodysense-text": "openai/windhub-free/glm-5.3",
    "bodysense-dev-oracle2-windhub": "openai/windhub-free/glm-5.3",
    "bodysense-dev-oracle2-kunyou": "openai/gpt-6-luna",
    "bodysense-posture": "openrouter/qwen/qwen2.5-vl-72b-instruct",
    "bodysense-posture-fallback": "openrouter/google/gemini-2.0-flash-001",
}
POSTURE_GROUPS = {"bodysense-posture", "bodysense-posture-fallback"}
# Keep the exact reviewed latency ceilings: otherwise a supplier pin could hide
# an unreviewed timeout change while every other routing invariant still passes.
EXPECTED_TIMEOUTS = {
    "bodysense-diagnosis": 90,
    "bodysense-diagnosis-fallback": 60,
    "bodysense-consultation": 90,
    "bodysense-structured": 90,
    "bodysense-text": 90,
    "bodysense-general-fallback": 60,
    "bodysense-posture": 60,
    "bodysense-posture-fallback": 60,
    "bodysense-dev-oracle2-windhub": 90,
    "bodysense-dev-oracle2-kunyou": 20,
    "bodysense-dev-oracle2-nanliang": 60,
}
ORACLE2_PARAMS = {"model", "api_base", "api_key", "timeout"}
POSTURE_PARAMS = {"model", "api_key", "timeout"}
# Prevent a valid supplier pin from being bypassed by an added fallback edge
# to an unqualified, unauthorized, or known-unavailable model group.
EXPECTED_FALLBACKS = {
    "bodysense-diagnosis": "bodysense-diagnosis-fallback",
    "bodysense-consultation": "bodysense-general-fallback",
    "bodysense-structured": "bodysense-general-fallback",
    "bodysense-text": "bodysense-general-fallback",
    "bodysense-posture": "bodysense-posture-fallback",
}


def has_supplier_qualified_routes(config_path: Path) -> bool:
    try:
        config = yaml.safe_load(config_path.read_text(encoding="utf-8"))
        if (
            not isinstance(config, dict)
            or set(config) != {"model_list", "router_settings", "general_settings"}
            or not isinstance(config.get("model_list"), list)
            or config.get("general_settings")
            != {"master_key": "os.environ/LITELLM_MASTER_KEY"}
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
                set(entry) != {"model_name", "litellm_params"}
                or not isinstance(name, str)
                or not name
                or name in groups
                or not isinstance(params, dict)
            ):
                return False
            groups[name] = params

        if set(groups) != set(EXPECTED_STATIC_MODELS) | set(PIN_REQUIRED):
            return False
        for name, params in groups.items():
            is_posture = name in POSTURE_GROUPS
            if set(params) != (POSTURE_PARAMS if is_posture else ORACLE2_PARAMS):
                return False
            if (
                type(params["timeout"]) is not int
                or params["timeout"] != EXPECTED_TIMEOUTS[name]
            ):
                return False
            if is_posture:
                if (
                    params["api_key"] != "os.environ/OPENROUTER_API_KEY"
                    or params["model"] != EXPECTED_STATIC_MODELS[name]
                ):
                    return False
            elif params["api_base"] != ORACLE2_BASE or params["api_key"] != ORACLE2_KEY:
                return False
            if name in EXPECTED_STATIC_MODELS and (
                params["model"] != EXPECTED_STATIC_MODELS[name]
            ):
                return False

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

        router = config.get("router_settings")
        if (
            not isinstance(router, dict)
            or set(router) != {"num_retries", "retry_after", "fallbacks"}
            or type(router["num_retries"]) is not int
            or router["num_retries"] != 0
            or type(router["retry_after"]) is not int
            or router["retry_after"] != 1
        ):
            return False
        edges = router.get("fallbacks")
        if not isinstance(edges, list) or len(edges) != len(EXPECTED_FALLBACKS):
            return False
        seen = set()
        for edge in edges:
            if not isinstance(edge, dict) or len(edge) != 1:
                return False
            source, targets = next(iter(edge.items()))
            if source in seen or source not in EXPECTED_FALLBACKS:
                return False
            if targets != [EXPECTED_FALLBACKS[source]]:
                return False
            seen.add(source)
        return seen == set(EXPECTED_FALLBACKS)
    except (OSError, ValueError, yaml.YAMLError):
        return False


def matches_approved_supplier(config_path: Path, resource_id: str | None) -> bool:
    """Require all fallback/probe routes to match an explicitly reviewed resource."""
    if not resource_id or not re.fullmatch(r"[a-z0-9][a-z0-9_.-]*", resource_id):
        return False
    try:
        config = yaml.safe_load(config_path.read_text(encoding="utf-8"))
        models = {
            entry["model_name"]: entry["litellm_params"]["model"]
            for entry in config["model_list"]
        }
        expected = f"openai/{resource_id}/{DEEPSEEK_MODEL}"
        return all(models[name] == expected for name in PIN_REQUIRED)
    except (OSError, ValueError, TypeError, KeyError, yaml.YAMLError):
        return False


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: dev-oracle2-route-preflight.py CONFIG_PATH", file=sys.stderr)
        return 2

    if not has_supplier_qualified_routes(Path(sys.argv[1])):
        print(
            "Oracle2 dev activation blocked: DeepSeek fallback/probe routes are "
            "unqualified or the fallback graph/config is unsafe. Pin the verified, "
            "authorized supplier-qualified model ID and preserve allowed "
            "fallback edges before enabling this gateway.",
            file=sys.stderr,
        )
        return 1

    if not matches_approved_supplier(Path(sys.argv[1]), APPROVED_DEEPSEEK_RESOURCE_ID):
        print(
            "Oracle2 dev activation blocked: the fallback supplier identity "
            "has not been explicitly approved in reviewed source policy. "
            "A supplier-qualified route alone is insufficient.",
            file=sys.stderr,
        )
        return 1

    print(
        "Oracle2 route structure and reviewed supplier pin PASS (client "
        "authorization and real inference remain unverified)."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
