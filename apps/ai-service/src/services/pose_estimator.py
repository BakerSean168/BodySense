"""Pose landmark extraction and geometric posture metrics (Phase 2).

Design:
- Historical Posture v1 tolerated optional MediaPipe; current Posture v2 requires
  the pinned ``pose`` runtime and fails closed when its mechanism is unavailable.
- When the mechanism is verified but no person is detected, callers may continue
  with zero geometric metrics plus qualitative VLM analysis.
- Numeric ``metric`` values may **only** come from this module. The VLM must
  never invent angles — ``posture_analyzer`` enforces that post-check.

Implementation is split by responsibility, and this module keeps the public
surface stable for importers:

- ``pose_geometry``: landmarks, thresholds and geometric metric computation.
- ``pose_mechanism``: pinned mechanism verification and landmark extraction.

Coordinate convention (MediaPipe Pose):
- ``x, y`` in image-normalized [0, 1]; origin top-left; y grows downward.
- Side view assumes the subject faces the *right* of the frame (ear ahead of
  shoulder when forward-head is present). Front/back use left/right landmarks.
"""

from __future__ import annotations

import logging
from importlib import metadata

from ..configuration.posture_agent_config import PostureGeometryMechanismConfig
from .pose_geometry import (
    ASYMMETRY_MARKED,
    ASYMMETRY_MILD,
    ASYMMETRY_MODERATE,
    C7_PROXY_Y_OFFSET,
    CVA_MARKED,
    CVA_MILD,
    CVA_MODERATE,
    EAR_SHOULDER_MARKED,
    EAR_SHOULDER_MILD,
    EAR_SHOULDER_MODERATE,
    LANDMARK_VISIBILITY_MIN,
    LM_LEFT_ANKLE,
    LM_LEFT_EAR,
    LM_LEFT_HIP,
    LM_LEFT_SHOULDER,
    LM_NOSE,
    LM_RIGHT_ANKLE,
    LM_RIGHT_EAR,
    LM_RIGHT_HIP,
    LM_RIGHT_SHOULDER,
    PELVIC_TILT_MARKED,
    PELVIC_TILT_MILD,
    PELVIC_TILT_MODERATE,
    POSE_GEOMETRY_THRESHOLD_SPEC,
    SPINE_MIDLINE_MARKED,
    SPINE_MIDLINE_MILD,
    SPINE_MIDLINE_MODERATE,
    GeometricMetric,
    Landmark,
    compute_frontal_metrics,
    compute_metrics_for_view,
    compute_side_metrics,
    findings_from_metrics,
    geometry_threshold_sha256,
    metrics_to_dicts,
)
from .pose_mechanism import (
    PoseMechanismError,
    PoseMechanismIntegrityError,
    PoseMechanismUnavailableError,
    default_pose_model_path,
    extract_landmarks,
    verify_pose_mechanism,
)

__all__ = [
    "ASYMMETRY_MARKED",
    "ASYMMETRY_MILD",
    "ASYMMETRY_MODERATE",
    "C7_PROXY_Y_OFFSET",
    "CVA_MARKED",
    "CVA_MILD",
    "CVA_MODERATE",
    "EAR_SHOULDER_MARKED",
    "EAR_SHOULDER_MILD",
    "EAR_SHOULDER_MODERATE",
    "LANDMARK_VISIBILITY_MIN",
    "LM_LEFT_ANKLE",
    "LM_LEFT_EAR",
    "LM_LEFT_HIP",
    "LM_LEFT_SHOULDER",
    "LM_NOSE",
    "LM_RIGHT_ANKLE",
    "LM_RIGHT_EAR",
    "LM_RIGHT_HIP",
    "LM_RIGHT_SHOULDER",
    "PELVIC_TILT_MARKED",
    "PELVIC_TILT_MILD",
    "PELVIC_TILT_MODERATE",
    "POSE_GEOMETRY_THRESHOLD_SPEC",
    "SPINE_MIDLINE_MARKED",
    "SPINE_MIDLINE_MILD",
    "SPINE_MIDLINE_MODERATE",
    "GeometricMetric",
    "Landmark",
    "PoseMechanismError",
    "PoseMechanismIntegrityError",
    "PoseMechanismUnavailableError",
    "compute_frontal_metrics",
    "compute_metrics_for_view",
    "compute_side_metrics",
    "default_pose_model_path",
    "estimate_pose_metrics",
    "extract_landmarks",
    "findings_from_metrics",
    "geometry_threshold_sha256",
    "metadata",
    "metrics_to_dicts",
    "verify_pose_mechanism",
]

logger = logging.getLogger(__name__)


def estimate_pose_metrics(
    image_bytes: bytes,
    view: str,
    mechanism: PostureGeometryMechanismConfig,
) -> tuple[list[GeometricMetric], dict[str, str]]:
    """Run pinned geometry and return metrics with exact mechanism provenance."""

    landmarks, provenance = extract_landmarks(image_bytes, mechanism)
    if not landmarks:
        return [], provenance
    return compute_metrics_for_view(landmarks, view), provenance
