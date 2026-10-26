#!/usr/bin/env python3
"""Read-only, keyless OpenRouter price audit for dev Oracle2 Posture routes.

This is a billing safety check, NOT model evaluation, route selection, or API
authorization. A metadata-only PASS never authorizes real vision inference.
"""

from __future__ import annotations

import json
import sys
import urllib.error
import urllib.request
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Any

import yaml

ROOT = Path(__file__).resolve().parents[1]
DEV_CONFIG = ROOT / "docker" / "litellm" / "config.dev-oracle2.yaml"
CATALOG_URL = "https://openrouter.ai/api/v1/models"
POSTURE_GROUPS = ("bodysense-posture", "bodysense-posture-fallback")
MAX_CATALOG_BYTES = 8 * 1024 * 1024


class PriceAuditError(ValueError):
    """No affirmative free-only metadata evidence for a configured route."""


def configured_routes(config: Any) -> dict[str, str]:
    if not isinstance(config, dict) or not isinstance(config.get("model_list"), list):
        raise PriceAuditError("invalid development model configuration")
    groups: dict[str, str] = {}
    for entry in config["model_list"]:
        if not isinstance(entry, dict):
            raise PriceAuditError("invalid model entry")
        name = entry.get("model_name")
        if name not in POSTURE_GROUPS:
            continue
        params = entry.get("litellm_params")
        model = params.get("model") if isinstance(params, dict) else None
        if (
            name in groups
            or not isinstance(model, str)
            or not model.startswith("openrouter/")
        ):
            raise PriceAuditError("duplicate or non-OpenRouter Posture route")
        groups[name] = model.removeprefix("openrouter/")
    if set(groups) != set(POSTURE_GROUPS):
        raise PriceAuditError("missing Posture primary or fallback route")
    return groups


def verify_free_pricing(config: Any, catalog: Any) -> dict[str, str]:
    """Require every configured exact model ID to have only zero-valued prices.

    Missing fields, missing models and unknown price formats fail closed. This
    does not establish account-specific access, availability, or authorization.
    """
    routes = configured_routes(config)
    if not isinstance(catalog, dict) or not isinstance(catalog.get("data"), list):
        raise PriceAuditError("invalid OpenRouter catalog payload")

    # Validate both exact Posture routes before reporting. A nonfree primary
    # must not hide an unlisted or nonfree fallback from the operator.
    failures: list[str] = []
    for logical_group, model_id in routes.items():
        try:
            matches = [
                item
                for item in catalog["data"]
                if isinstance(item, dict) and item.get("id") == model_id
            ]
            if len(matches) != 1:
                raise PriceAuditError(
                    f"{logical_group}: exact catalog model not uniquely listed"
                )
            pricing = matches[0].get("pricing")
            if not isinstance(pricing, dict) or not {"prompt", "completion"} <= set(
                pricing
            ):
                raise PriceAuditError(
                    f"{logical_group}: missing required pricing fields"
                )
            for price_kind, raw_value in pricing.items():
                if isinstance(raw_value, bool) or not isinstance(
                    raw_value, (str, int, float)
                ):
                    raise PriceAuditError(
                        f"{logical_group}: unknown {price_kind} price"
                    )
                try:
                    value = Decimal(str(raw_value))
                except InvalidOperation as exc:
                    raise PriceAuditError(
                        f"{logical_group}: invalid {price_kind} price"
                    ) from exc
                if not value.is_finite() or value != 0:
                    raise PriceAuditError(
                        f"{logical_group}: nonzero or invalid {price_kind} price"
                    )
        except PriceAuditError as exc:
            failures.append(str(exc))
    if failures:
        raise PriceAuditError("; ".join(failures))
    return routes


def fetch_public_catalog() -> Any:
    # No API key, bearer token, prompt, image, user data or inference request.
    request = urllib.request.Request(
        CATALOG_URL,
        headers={
            "Accept": "application/json",
            "User-Agent": "BodySense-Dev-Price-Audit/1.0",
        },
        method="GET",
    )
    with urllib.request.urlopen(request, timeout=20) as response:
        if response.geturl() != CATALOG_URL:
            raise PriceAuditError("catalog redirected away from the pinned endpoint")
        payload = response.read(MAX_CATALOG_BYTES + 1)
    if len(payload) > MAX_CATALOG_BYTES:
        raise PriceAuditError("catalog response exceeds size limit")
    return json.loads(payload)


def main() -> int:
    try:
        config = yaml.safe_load(DEV_CONFIG.read_text(encoding="utf-8"))
        routes = verify_free_pricing(config, fetch_public_catalog())
    except (
        PriceAuditError,
        OSError,
        ValueError,
        urllib.error.URLError,
        yaml.YAMLError,
    ) as exc:
        print(f"POSTURE_FREE_PRICE_AUDIT=BLOCKED: {exc}", file=sys.stderr)
        return 1
    print(
        "POSTURE_FREE_PRICE_AUDIT=PASS_METADATA_ONLY "
        f"exact_routes={len(routes)}; account authorization and real inference NOT verified"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
