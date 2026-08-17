"""YAML configuration loading with environment variable interpolation."""

from __future__ import annotations

import logging
import os
import re
from dataclasses import dataclass, field
from pathlib import Path

import yaml

from .errors import ConfigError

logger = logging.getLogger(__name__)


@dataclass
class ModelConfig:
    id: str
    capabilities: set[str] = field(default_factory=set)


@dataclass
class ProviderConfig:
    type: str  # "openai-compatible"
    base_url: str
    api_key: str
    models: list[ModelConfig] = field(default_factory=list)


@dataclass
class RouteCandidate:
    provider: str
    model: str


@dataclass
class RouteDefaults:
    temperature: float | None = None
    max_tokens: int | None = None
    response_format: str | None = None


@dataclass
class RouteConfig:
    defaults: RouteDefaults = field(default_factory=RouteDefaults)
    candidates: list[RouteCandidate] = field(default_factory=list)


@dataclass
class RouterConfig:
    providers: dict[str, ProviderConfig] = field(default_factory=dict)
    routes: dict[str, RouteConfig] = field(default_factory=dict)


_ENV_VAR_PATTERN = re.compile(r"\$\{([^}]+)\}")


def _interpolate_env(value: str) -> str:
    """Replace ${VAR_NAME} with environment variable values."""

    def replacer(match: re.Match) -> str:
        var_name = match.group(1)
        env_value = os.environ.get(var_name)
        if env_value is None:
            raise ConfigError(f"Environment variable {var_name} is not set")
        return env_value

    return _ENV_VAR_PATTERN.sub(replacer, value)


def load_config(config_path: str | Path) -> RouterConfig:
    """Load and parse the YAML configuration file."""
    path = Path(config_path)
    if not path.exists():
        raise ConfigError(f"Config file not found: {config_path}")

    with open(path) as f:
        raw = yaml.safe_load(f)

    if not isinstance(raw, dict):
        raise ConfigError("Config file must be a YAML mapping")

    providers: dict[str, ProviderConfig] = {}
    for provider_id, provider_raw in raw.get("providers", {}).items():
        try:
            base_url = _interpolate_env(provider_raw["base_url"])
            api_key = _interpolate_env(provider_raw["api_key"])
        except ConfigError as e:
            # Skip providers with missing env vars instead of crashing
            logger.warning("Skipping provider %s: %s", provider_id, e)
            continue
        models = []
        for model_raw in provider_raw.get("models", []):
            models.append(
                ModelConfig(
                    id=model_raw["id"],
                    capabilities=set(model_raw.get("capabilities", [])),
                )
            )
        providers[provider_id] = ProviderConfig(
            type=provider_raw["type"],
            base_url=base_url,
            api_key=api_key,
            models=models,
        )

    routes: dict[str, RouteConfig] = {}
    for route_id, route_raw in raw.get("routes", {}).items():
        defaults_raw = route_raw.get("defaults", {})
        defaults = RouteDefaults(
            temperature=defaults_raw.get("temperature"),
            max_tokens=defaults_raw.get("max_tokens"),
            response_format=defaults_raw.get("response_format"),
        )
        candidates = []
        for cand_raw in route_raw.get("candidates", []):
            candidates.append(
                RouteCandidate(
                    provider=cand_raw["provider"],
                    model=cand_raw["model"],
                )
            )
        routes[route_id] = RouteConfig(defaults=defaults, candidates=candidates)

    return RouterConfig(providers=providers, routes=routes)
