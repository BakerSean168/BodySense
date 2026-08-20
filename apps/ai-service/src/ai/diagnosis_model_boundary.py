"""Migration-safe Diagnosis model boundary.

The target path is always the standalone LiteLLM gateway. A temporary legacy
branch protects servers whose Compose/runtime files have not yet been synchronized
when a new AI image is published. The deletion ledger makes this bridge explicit.
"""

from __future__ import annotations

import os

from pydantic_ai.models import Model

from .diagnosis_gateway_model import get_diagnosis_gateway_model
from .errors import NoAvailableProviderError

_BACKEND_ENV = "DIAGNOSIS_MODEL_BACKEND"


def get_diagnosis_runtime_model() -> Model:
    """Resolve the configured Diagnosis model backend during migration."""

    backend = os.getenv(_BACKEND_ENV, "legacy").strip().lower()
    if backend == "litellm":
        return get_diagnosis_gateway_model()
    if backend == "legacy":
        from .pydantic_model import get_pydantic_model

        return get_pydantic_model("llm.json")
    raise NoAvailableProviderError(
        f"Unsupported {_BACKEND_ENV}={backend!r}; expected 'litellm' or 'legacy'"
    )
