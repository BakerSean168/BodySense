"""Pinned MediaPipe Pose mechanism verification and landmark extraction.

Design:
- Historical Posture v1 tolerated optional MediaPipe; current Posture v2 requires
  the pinned ``pose`` runtime and fails closed when its mechanism is unavailable.
- When the mechanism is verified but no person is detected, callers may continue
  with zero geometric metrics plus qualitative VLM analysis.
- Production images bake the pinned artifact at build time. Local development
  may provision the same versioned artifact before starting the service, but the
  request path never downloads a model and never accepts an unverified cache.
"""

from __future__ import annotations

import hashlib
import os
from importlib import metadata
from pathlib import Path
from typing import Any

from ..configuration.posture_agent_config import PostureGeometryMechanismConfig
from .pose_geometry import (
    POSE_GEOMETRY_THRESHOLD_SPEC,
    Landmark,
    geometry_threshold_sha256,
)

_POSE_MODEL_PATH_ENV = "BODYSENSE_POSE_MODEL_PATH"


class PoseMechanismError(RuntimeError):
    """Base class for a Posture geometric-mechanism contract failure."""


class PoseMechanismUnavailableError(PoseMechanismError):
    """Required engine/model is not installed or available."""


class PoseMechanismIntegrityError(PoseMechanismError):
    """Pinned engine/model/threshold identity does not match runtime reality."""


_landmarker = None
_landmarker_key: tuple[str, ...] | None = None
_landmarker_provenance: dict[str, str] | None = None


def default_pose_model_path(mechanism: PostureGeometryMechanismConfig) -> Path:
    override = os.getenv(_POSE_MODEL_PATH_ENV, "").strip()
    if override:
        return Path(override)
    return (
        Path.home()
        / ".cache"
        / "bodysense"
        / "mediapipe"
        / f"pose-landmarker-{mechanism.model_sha256[:16]}.task"
    )


def _sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def verify_pose_mechanism(
    mechanism: PostureGeometryMechanismConfig,
) -> dict[str, str]:
    """Verify the exact non-LLM mechanism bound by the Posture manifest."""

    actual_threshold_sha = geometry_threshold_sha256()
    if actual_threshold_sha != mechanism.threshold_sha256:
        raise PoseMechanismIntegrityError(
            "posture geometry threshold fingerprint mismatch: "
            f"got {actual_threshold_sha} want {mechanism.threshold_sha256}"
        )

    try:
        engine_version = metadata.version("mediapipe")
    except metadata.PackageNotFoundError as exc:
        raise PoseMechanismUnavailableError("required mediapipe package is not installed") from exc
    if engine_version != mechanism.engine_version:
        raise PoseMechanismIntegrityError(
            f"mediapipe version mismatch: got {engine_version} want {mechanism.engine_version}"
        )

    model_path = default_pose_model_path(mechanism)
    if not model_path.is_file() or model_path.stat().st_size <= 0:
        raise PoseMechanismUnavailableError(f"pinned pose model is missing: {model_path}")
    actual_model_sha = _sha256_file(model_path)
    if actual_model_sha != mechanism.model_sha256:
        raise PoseMechanismIntegrityError(
            f"pose model sha256 mismatch: got {actual_model_sha} want {mechanism.model_sha256}"
        )

    return {
        "status": "verified",
        "mechanism_revision": mechanism.mechanism_revision,
        "engine": mechanism.engine,
        "engine_version": engine_version,
        "model_uri": mechanism.model_uri,
        "model_sha256": actual_model_sha,
        "threshold_revision": mechanism.threshold_revision,
        "threshold_sha256": actual_threshold_sha,
    }


def _landmarker_for(
    mechanism: PostureGeometryMechanismConfig,
) -> tuple[Any, dict[str, str]]:
    global _landmarker, _landmarker_key, _landmarker_provenance

    provenance = verify_pose_mechanism(mechanism)
    key = (
        mechanism.mechanism_revision,
        mechanism.engine_version,
        mechanism.model_sha256,
        mechanism.threshold_sha256,
        str(default_pose_model_path(mechanism)),
    )
    if _landmarker is not None and _landmarker_key == key and _landmarker_provenance is not None:
        return _landmarker, dict(_landmarker_provenance)

    try:
        from mediapipe.tasks.python import vision  # pyright: ignore[reportMissingImports]
        from mediapipe.tasks.python.core import (  # pyright: ignore[reportMissingImports]
            base_options as base_options_module,
        )
    except Exception as exc:  # noqa: BLE001
        raise PoseMechanismUnavailableError("mediapipe Tasks API is unavailable") from exc

    pose_options = POSE_GEOMETRY_THRESHOLD_SPEC["pose_landmarker"]
    try:
        options = vision.PoseLandmarkerOptions(
            base_options=base_options_module.BaseOptions(
                model_asset_path=str(default_pose_model_path(mechanism)),
            ),
            running_mode=vision.RunningMode.IMAGE,
            num_poses=int(pose_options["num_poses"]),
            min_pose_detection_confidence=float(pose_options["min_pose_detection_confidence"]),
            min_pose_presence_confidence=float(pose_options["min_pose_presence_confidence"]),
            min_tracking_confidence=float(pose_options["min_tracking_confidence"]),
        )
        landmarker = vision.PoseLandmarker.create_from_options(options)
    except Exception as exc:  # noqa: BLE001
        raise PoseMechanismUnavailableError("unable to construct pinned PoseLandmarker") from exc

    _landmarker = landmarker
    _landmarker_key = key
    _landmarker_provenance = dict(provenance)
    return landmarker, provenance


def extract_landmarks(
    image_bytes: bytes,
    mechanism: PostureGeometryMechanismConfig,
) -> tuple[dict[int, Landmark] | None, dict[str, str]]:
    """Run the pinned Pose Landmarker and return landmarks + mechanism provenance."""

    landmarker, provenance = _landmarker_for(mechanism)

    try:
        import cv2  # type: ignore
        import numpy as np  # type: ignore
        from mediapipe.tasks.python.vision.core import (  # pyright: ignore[reportMissingImports]
            image as mp_image_module,
        )
    except Exception as exc:  # noqa: BLE001
        raise PoseMechanismUnavailableError(
            "pose image-decoding dependencies are unavailable"
        ) from exc

    try:
        arr = np.frombuffer(image_bytes, dtype=np.uint8)
        bgr = cv2.imdecode(arr, cv2.IMREAD_COLOR)
        if bgr is None:
            return None, provenance
        rgb = cv2.cvtColor(bgr, cv2.COLOR_BGR2RGB)
        mp_image = mp_image_module.Image(
            image_format=mp_image_module.ImageFormat.SRGB,
            data=np.ascontiguousarray(rgb),
        )
        result = landmarker.detect(mp_image)
        if not result.pose_landmarks:
            return None, provenance

        pose = result.pose_landmarks[0]
        out: dict[int, Landmark] = {}
        for idx, lm in enumerate(pose):
            visibility = float(
                getattr(lm, "visibility", None) or getattr(lm, "presence", None) or 1.0
            )
            out[idx] = Landmark(x=float(lm.x), y=float(lm.y), visibility=visibility)
        return out, provenance
    except PoseMechanismError:
        raise
    except Exception as exc:
        raise PoseMechanismError("pose landmark extraction failed") from exc
