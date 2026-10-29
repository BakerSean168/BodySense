"""Read-only pricing checks use synthetic catalogs; no provider requests."""

from __future__ import annotations

import runpy
from pathlib import Path

import pytest
import yaml

ROOT = Path(__file__).resolve().parents[4]
SCRIPT = ROOT / "scripts" / "dev-oracle2-posture-free-audit.py"
CONFIG = ROOT / "docker" / "litellm" / "config.dev-oracle2.yaml"


def _module() -> dict:
    return runpy.run_path(str(SCRIPT), run_name="test_only")


def _configured() -> dict:
    return yaml.safe_load(CONFIG.read_text(encoding="utf-8"))


def _catalog(pricing: dict | None = None, include_fallback: bool = True) -> dict:
    rows = [
        {
            "id": "qwen/qwen2.5-vl-72b-instruct",
            "pricing": pricing if pricing is not None else {"prompt": "0", "completion": "0"},
            "architecture": {
                "input_modalities": ["text", "image"],
                "output_modalities": ["text"],
            },
        },
    ]
    if include_fallback:
        rows.append(
            {
                "id": "google/gemini-2.0-flash-001",
                "pricing": {"prompt": "0", "completion": "0", "image": "0"},
                "architecture": {
                    "input_modalities": ["text", "image"],
                    "output_modalities": ["text"],
                },
            }
        )
    return {"data": rows}


def test_exact_configured_posture_routes_only() -> None:
    result = _module()["configured_routes"](_configured())
    assert result == {
        "bodysense-posture": "qwen/qwen2.5-vl-72b-instruct",
        "bodysense-posture-fallback": "google/gemini-2.0-flash-001",
    }


def test_both_exact_models_with_all_zero_prices_pass_metadata_only() -> None:
    assert len(_module()["verify_free_pricing"](_configured(), _catalog())) == 2


@pytest.mark.parametrize(
    "pricing",
    [
        {"prompt": "0.0000008", "completion": "0"},
        {"prompt": "0", "completion": "0", "image": "0.01"},
        {"prompt": "0"},
        {"prompt": "0", "completion": None},
        {"prompt": "free", "completion": "0"},
        {"prompt": "NaN", "completion": "0"},
        {"prompt": "0", "completion": -0.0001},
        {"prompt": False, "completion": "0"},
        {},
    ],
)
def test_nonfree_or_unknown_price_fails_closed(pricing: dict) -> None:
    module = _module()
    with pytest.raises(module["PriceAuditError"]):
        module["verify_free_pricing"](_configured(), _catalog(pricing=pricing))


@pytest.mark.parametrize(
    ("architecture", "reason"),
    [
        (None, "missing architecture metadata"),
        (
            {"input_modalities": ["text"], "output_modalities": ["text"]},
            "image input capability not verified",
        ),
        (
            {"input_modalities": "text+image", "output_modalities": ["text"]},
            "image input capability not verified",
        ),
        (
            {"input_modalities": ["text", "image"], "output_modalities": ["image"]},
            "text output capability not verified",
        ),
    ],
)
def test_free_metadata_without_posture_vision_capability_fails_closed(
    architecture: dict | None, reason: str
) -> None:
    module = _module()
    catalog = _catalog()
    catalog["data"][0]["architecture"] = architecture
    with pytest.raises(module["PriceAuditError"], match=reason):
        module["verify_free_pricing"](_configured(), catalog)


def test_missing_fallback_cannot_be_mistaken_for_free() -> None:
    module = _module()
    with pytest.raises(module["PriceAuditError"], match="not uniquely listed"):
        module["verify_free_pricing"](_configured(), _catalog(include_fallback=False))


def test_nonfree_primary_and_missing_fallback_report_both_blockers() -> None:
    module = _module()
    catalog = _catalog(
        pricing={"prompt": "0.0000008", "completion": "0"},
        include_fallback=False,
    )
    with pytest.raises(module["PriceAuditError"]) as error:
        module["verify_free_pricing"](_configured(), catalog)
    assert "bodysense-posture: nonzero or invalid prompt price" in str(error.value)
    assert "bodysense-posture-fallback: exact catalog model not uniquely listed" in str(error.value)


def test_nonfree_primary_and_nonfree_fallback_report_both_blockers() -> None:
    module = _module()
    catalog = _catalog(pricing={"prompt": "0.0000008", "completion": "0"})
    catalog["data"][1]["pricing"]["image"] = "0.002"
    with pytest.raises(module["PriceAuditError"]) as error:
        module["verify_free_pricing"](_configured(), catalog)
    assert "bodysense-posture: nonzero or invalid prompt price" in str(error.value)
    assert "bodysense-posture-fallback: nonzero or invalid image price" in str(error.value)


def test_duplicate_id_in_catalog_is_refused() -> None:
    module = _module()
    catalog = _catalog()
    catalog["data"].append(catalog["data"][0])
    with pytest.raises(module["PriceAuditError"], match="not uniquely listed"):
        module["verify_free_pricing"](_configured(), catalog)


def test_incomplete_or_misdirected_posture_config_is_refused() -> None:
    module = _module()
    config = _configured()
    groups = {entry["model_name"]: entry["litellm_params"] for entry in config["model_list"]}
    groups["bodysense-posture"]["model"] = "openai/unreviewed/paid"
    with pytest.raises(module["PriceAuditError"], match="non-OpenRouter"):
        module["verify_free_pricing"](config, _catalog())


def test_malformed_catalog_is_refused() -> None:
    module = _module()
    with pytest.raises(module["PriceAuditError"], match="invalid OpenRouter catalog"):
        module["verify_free_pricing"](_configured(), {"data": {}})
