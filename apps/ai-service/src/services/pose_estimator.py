"""Pose landmark extraction and geometric posture metrics (Phase 2).

Design:
- Historical Posture v1 tolerated optional MediaPipe; current Posture v2 requires
  the pinned ``pose`` runtime and fails closed when its mechanism is unavailable.
- When the mechanism is verified but no person is detected, callers may continue
  with zero geometric metrics plus qualitative VLM analysis.
- Numeric ``metric`` values may **only** come from this module. The VLM must
  never invent angles — ``posture_analyzer`` enforces that post-check.

Coordinate convention (MediaPipe Pose):
- ``x, y`` in image-normalized [0, 1]; origin top-left; y grows downward.
- Side view assumes the subject faces the *right* of the frame (ear ahead of
  shoulder when forward-head is present). Front/back use left/right landmarks.
"""

from __future__ import annotations

import hashlib
import json
import logging
import math
import os
from dataclasses import dataclass
from importlib import metadata
from pathlib import Path
from typing import Any

from ..configuration.posture_agent_config import PostureGeometryMechanismConfig

logger = logging.getLogger(__name__)

# MediaPipe Pose landmark indices (BlazePose 33-point topology).
LM_NOSE = 0
LM_LEFT_EAR = 7
LM_RIGHT_EAR = 8
LM_LEFT_SHOULDER = 11
LM_RIGHT_SHOULDER = 12
LM_LEFT_HIP = 23
LM_RIGHT_HIP = 24
LM_LEFT_ANKLE = 27
LM_RIGHT_ANKLE = 28


@dataclass(frozen=True)
class Landmark:
    x: float
    y: float
    visibility: float = 1.0


@dataclass(frozen=True)
class GeometricMetric:
    name: str
    value: float
    unit: str
    # Finding key this metric supports (aligned with VIEW_ALLOWED_KEYS / KEY_LABELS).
    finding_key: str
    severity: str  # mild | moderate | marked
    confidence: str  # high | medium | low
    evidence: str


# ---------------------------------------------------------------------------
# Threshold contract
# ---------------------------------------------------------------------------
# Every behavior-significant numeric threshold used by geometric perception is
# represented in this canonical spec. Current Posture manifests pin its SHA256;
# changing a threshold without advancing the manifest therefore fails closed.

POSE_GEOMETRY_THRESHOLD_SPEC: dict[str, Any] = {
    "landmark_visibility_min": 0.5,
    "c7_proxy_y_offset": 0.02,
    "craniovertebral_angle_deg": {"mild": 50.0, "moderate": 45.0, "marked": 40.0},
    "shoulder_hip_asymmetry_norm": {"mild": 0.015, "moderate": 0.025, "marked": 0.04},
    "ear_shoulder_offset_norm": {"mild": 0.02, "moderate": 0.04, "marked": 0.06},
    "pelvic_tilt_proxy_norm": {"mild": 0.03, "moderate": 0.05, "marked": 0.08},
    "spine_midline_offset_norm": {"mild": 0.015, "moderate": 0.03, "marked": 0.05},
    "pose_landmarker": {
        "num_poses": 1,
        "min_pose_detection_confidence": 0.5,
        "min_pose_presence_confidence": 0.5,
        "min_tracking_confidence": 0.5,
    },
}


def geometry_threshold_sha256() -> str:
    payload = json.dumps(
        POSE_GEOMETRY_THRESHOLD_SPEC,
        sort_keys=True,
        separators=(",", ":"),
        ensure_ascii=True,
    )
    return hashlib.sha256(payload.encode()).hexdigest()


CVA_MILD = float(POSE_GEOMETRY_THRESHOLD_SPEC["craniovertebral_angle_deg"]["mild"])
CVA_MODERATE = float(POSE_GEOMETRY_THRESHOLD_SPEC["craniovertebral_angle_deg"]["moderate"])
CVA_MARKED = float(POSE_GEOMETRY_THRESHOLD_SPEC["craniovertebral_angle_deg"]["marked"])
ASYMMETRY_MILD = float(POSE_GEOMETRY_THRESHOLD_SPEC["shoulder_hip_asymmetry_norm"]["mild"])
ASYMMETRY_MODERATE = float(POSE_GEOMETRY_THRESHOLD_SPEC["shoulder_hip_asymmetry_norm"]["moderate"])
ASYMMETRY_MARKED = float(POSE_GEOMETRY_THRESHOLD_SPEC["shoulder_hip_asymmetry_norm"]["marked"])
EAR_SHOULDER_MILD = float(POSE_GEOMETRY_THRESHOLD_SPEC["ear_shoulder_offset_norm"]["mild"])
EAR_SHOULDER_MODERATE = float(POSE_GEOMETRY_THRESHOLD_SPEC["ear_shoulder_offset_norm"]["moderate"])
EAR_SHOULDER_MARKED = float(POSE_GEOMETRY_THRESHOLD_SPEC["ear_shoulder_offset_norm"]["marked"])
PELVIC_TILT_MILD = float(POSE_GEOMETRY_THRESHOLD_SPEC["pelvic_tilt_proxy_norm"]["mild"])
PELVIC_TILT_MODERATE = float(POSE_GEOMETRY_THRESHOLD_SPEC["pelvic_tilt_proxy_norm"]["moderate"])
PELVIC_TILT_MARKED = float(POSE_GEOMETRY_THRESHOLD_SPEC["pelvic_tilt_proxy_norm"]["marked"])
SPINE_MIDLINE_MILD = float(POSE_GEOMETRY_THRESHOLD_SPEC["spine_midline_offset_norm"]["mild"])
SPINE_MIDLINE_MODERATE = float(
    POSE_GEOMETRY_THRESHOLD_SPEC["spine_midline_offset_norm"]["moderate"]
)
SPINE_MIDLINE_MARKED = float(POSE_GEOMETRY_THRESHOLD_SPEC["spine_midline_offset_norm"]["marked"])
LANDMARK_VISIBILITY_MIN = float(POSE_GEOMETRY_THRESHOLD_SPEC["landmark_visibility_min"])
C7_PROXY_Y_OFFSET = float(POSE_GEOMETRY_THRESHOLD_SPEC["c7_proxy_y_offset"])


def _severity_from_bands(
    value: float,
    *,
    mild: float,
    moderate: float,
    marked: float,
    lower_is_worse: bool,
) -> str | None:
    """Map a continuous metric onto mild/moderate/marked, or None if normal."""
    if lower_is_worse:
        if value < marked:
            return "marked"
        if value < moderate:
            return "moderate"
        if value < mild:
            return "mild"
        return None
    # higher absolute deviation is worse
    if value >= marked:
        return "marked"
    if value >= moderate:
        return "moderate"
    if value >= mild:
        return "mild"
    return None


def _angle_deg(ax: float, ay: float, bx: float, by: float, cx: float, cy: float) -> float:
    """Interior angle ABC in degrees."""
    bax, bay = ax - bx, ay - by
    bcx, bcy = cx - bx, cy - by
    dot = bax * bcx + bay * bcy
    mag_a = math.hypot(bax, bay)
    mag_c = math.hypot(bcx, bcy)
    if mag_a < 1e-9 or mag_c < 1e-9:
        return float("nan")
    cos_a = max(-1.0, min(1.0, dot / (mag_a * mag_c)))
    return math.degrees(math.acos(cos_a))


def _midpoint(a: Landmark, b: Landmark) -> Landmark:
    return Landmark(
        x=(a.x + b.x) / 2,
        y=(a.y + b.y) / 2,
        visibility=min(a.visibility, b.visibility),
    )


def _visible(lm: Landmark | None, min_vis: float = LANDMARK_VISIBILITY_MIN) -> bool:
    return lm is not None and lm.visibility >= min_vis


def compute_side_metrics(landmarks: dict[int, Landmark]) -> list[GeometricMetric]:
    """Geometric metrics for a side-view standing photo."""
    metrics: list[GeometricMetric] = []

    # Prefer the ear that is more visible (camera-side).
    left_ear = landmarks.get(LM_LEFT_EAR)
    right_ear = landmarks.get(LM_RIGHT_EAR)
    ear = None
    if _visible(left_ear) and _visible(right_ear):
        ear = left_ear if left_ear.visibility >= right_ear.visibility else right_ear  # type: ignore[union-attr]
    elif _visible(left_ear):
        ear = left_ear
    elif _visible(right_ear):
        ear = right_ear

    left_sh = landmarks.get(LM_LEFT_SHOULDER)
    right_sh = landmarks.get(LM_RIGHT_SHOULDER)
    shoulder = None
    if _visible(left_sh) and _visible(right_sh):
        shoulder = _midpoint(left_sh, right_sh)  # type: ignore[arg-type]
    elif _visible(left_sh):
        shoulder = left_sh
    elif _visible(right_sh):
        shoulder = right_sh

    # C7 proxy: slightly above shoulder midpoint (no true C7 in BlazePose).
    c7 = None
    if shoulder is not None:
        c7 = Landmark(
            x=shoulder.x, y=shoulder.y - C7_PROXY_Y_OFFSET, visibility=shoulder.visibility
        )

    if ear is not None and c7 is not None:
        # Clinical CVA: angle at C7 between the horizontal and the C7→tragus
        # line. With y growing downward and the subject facing *right*:
        #   dx = ear.x - c7.x  (positive when ear is anterior)
        #   dy = c7.y - ear.y  (positive when ear is above C7)
        # upright ≈ 90°, forward-head lowers CVA (typical FHP < ~50°).
        dx = ear.x - c7.x
        dy = c7.y - ear.y
        if abs(dx) < 1e-9 and abs(dy) < 1e-9:
            cva = float("nan")
        else:
            cva = math.degrees(math.atan2(dy, dx))
            # atan2 range (-180, 180]; fold into the clinical 0–90 band used
            # for standing side-view (ignore behind-the-camera flips).
            if cva < 0:
                cva = abs(cva)
            if cva > 90:
                cva = 180.0 - cva
            cva = max(15.0, min(90.0, cva))
        if not math.isnan(cva):
            sev = _severity_from_bands(
                cva,
                mild=CVA_MILD,
                moderate=CVA_MODERATE,
                marked=CVA_MARKED,
                lower_is_worse=True,
            )
            conf = "high" if ear.visibility > 0.7 and c7.visibility > 0.7 else "medium"
            if sev:
                metrics.append(
                    GeometricMetric(
                        name="craniovertebral_angle",
                        value=round(cva, 1),
                        unit="deg",
                        finding_key="forward_head",
                        severity=sev,
                        confidence=conf,
                        evidence=f"侧面观估算颅椎角约 {cva:.1f}°（几何测量）",
                    )
                )

    # Rounded shoulder proxy: horizontal offset ear vs shoulder.
    if ear is not None and shoulder is not None:
        offset = abs(ear.x - shoulder.x)
        sev = _severity_from_bands(
            offset,
            mild=EAR_SHOULDER_MILD,
            moderate=EAR_SHOULDER_MODERATE,
            marked=EAR_SHOULDER_MARKED,
            lower_is_worse=False,
        )
        if sev:
            metrics.append(
                GeometricMetric(
                    name="ear_shoulder_offset",
                    value=round(offset, 3),
                    unit="norm",
                    finding_key="rounded_shoulders",
                    severity=sev,
                    confidence="medium",
                    evidence=f"耳-肩水平偏移约 {offset:.3f}（归一化）",
                )
            )

    left_hip = landmarks.get(LM_LEFT_HIP)
    right_hip = landmarks.get(LM_RIGHT_HIP)
    hip = None
    if _visible(left_hip) and _visible(right_hip):
        hip = _midpoint(left_hip, right_hip)  # type: ignore[arg-type]
    elif _visible(left_hip):
        hip = left_hip
    elif _visible(right_hip):
        hip = right_hip

    if shoulder is not None and hip is not None:
        # Anterior pelvic tilt proxy: relative horizontal shift hip vs shoulder.
        tilt = abs(hip.x - shoulder.x)
        sev = _severity_from_bands(
            tilt,
            mild=PELVIC_TILT_MILD,
            moderate=PELVIC_TILT_MODERATE,
            marked=PELVIC_TILT_MARKED,
            lower_is_worse=False,
        )
        if sev:
            metrics.append(
                GeometricMetric(
                    name="pelvic_tilt_proxy",
                    value=round(tilt, 3),
                    unit="norm",
                    finding_key="anterior_pelvic_tilt",
                    severity=sev,
                    confidence="low",
                    evidence=f"侧视骨盆相对肩部水平偏移约 {tilt:.3f}（近似）",
                )
            )

    return metrics


def compute_frontal_metrics(landmarks: dict[int, Landmark], view: str) -> list[GeometricMetric]:
    """Geometric metrics for front or back standing photos."""
    metrics: list[GeometricMetric] = []
    left_sh = landmarks.get(LM_LEFT_SHOULDER)
    right_sh = landmarks.get(LM_RIGHT_SHOULDER)
    left_hip = landmarks.get(LM_LEFT_HIP)
    right_hip = landmarks.get(LM_RIGHT_HIP)

    if _visible(left_sh) and _visible(right_sh):
        # y grows downward: higher shoulder has smaller y.
        diff = abs(left_sh.y - right_sh.y)  # type: ignore[union-attr]
        sev = _severity_from_bands(
            diff,
            mild=ASYMMETRY_MILD,
            moderate=ASYMMETRY_MODERATE,
            marked=ASYMMETRY_MARKED,
            lower_is_worse=False,
        )
        if sev:
            higher = "左" if left_sh.y < right_sh.y else "右"  # type: ignore[union-attr]
            metrics.append(
                GeometricMetric(
                    name="shoulder_height_diff",
                    value=round(diff, 3),
                    unit="norm",
                    finding_key="shoulder_tilt",
                    severity=sev,
                    confidence="high",
                    evidence=f"{view}面观肩高差约 {diff:.3f}，{higher}侧偏高（几何测量）",
                )
            )

    if _visible(left_hip) and _visible(right_hip):
        diff = abs(left_hip.y - right_hip.y)  # type: ignore[union-attr]
        sev = _severity_from_bands(
            diff,
            mild=ASYMMETRY_MILD,
            moderate=ASYMMETRY_MODERATE,
            marked=ASYMMETRY_MARKED,
            lower_is_worse=False,
        )
        if sev:
            metrics.append(
                GeometricMetric(
                    name="hip_height_diff",
                    value=round(diff, 3),
                    unit="norm",
                    finding_key="pelvic_obliquity",
                    severity=sev,
                    confidence="medium",
                    evidence=f"{view}面观骨盆高低差约 {diff:.3f}（几何测量）",
                )
            )

    # Spine midline offset: mid-shoulders vs mid-hips horizontal distance.
    if _visible(left_sh) and _visible(right_sh) and _visible(left_hip) and _visible(right_hip):
        mid_sh = _midpoint(left_sh, right_sh)  # type: ignore[arg-type]
        mid_hip = _midpoint(left_hip, right_hip)  # type: ignore[arg-type]
        offset = abs(mid_sh.x - mid_hip.x)
        sev = _severity_from_bands(
            offset,
            mild=SPINE_MIDLINE_MILD,
            moderate=SPINE_MIDLINE_MODERATE,
            marked=SPINE_MIDLINE_MARKED,
            lower_is_worse=False,
        )
        if sev:
            metrics.append(
                GeometricMetric(
                    name="spine_midline_offset",
                    value=round(offset, 3),
                    unit="norm",
                    finding_key="scoliosis_tendency",
                    severity=sev,
                    confidence="low",
                    evidence=f"{view}面观肩-髋中线偏移约 {offset:.3f}（几何近似，需专业评估）",
                )
            )

    return metrics


def compute_metrics_for_view(
    landmarks: dict[int, Landmark],
    view: str,
) -> list[GeometricMetric]:
    if view == "side":
        return compute_side_metrics(landmarks)
    if view in ("front", "back"):
        return compute_frontal_metrics(landmarks, view)
    return []


def metrics_to_dicts(metrics: list[GeometricMetric]) -> list[dict[str, Any]]:
    return [
        {
            "name": m.name,
            "value": m.value,
            "unit": m.unit,
            "finding_key": m.finding_key,
            "severity": m.severity,
            "confidence": m.confidence,
            "evidence": m.evidence,
        }
        for m in metrics
    ]


def findings_from_metrics(metrics: list[GeometricMetric]) -> list[dict[str, Any]]:
    """Project geometric metrics into the posture finding schema."""
    findings: list[dict[str, Any]] = []
    for m in metrics:
        findings.append(
            {
                "key": m.finding_key,
                "label": "",  # filled by KEY_LABELS in govern
                "severity": m.severity,
                "confidence": m.confidence,
                "evidence": m.evidence,
                "metric": {
                    "name": m.name,
                    "value": m.value,
                    "unit": m.unit,
                },
            }
        )
    return findings


# ---------------------------------------------------------------------------
# MediaPipe extraction (required by current Posture v2)
# ---------------------------------------------------------------------------
# Production images bake the pinned artifact at build time. Local development
# may provision the same versioned artifact before starting the service, but the
# request path never downloads a model and never accepts an unverified cache.

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
    except Exception:
        logger.exception("pose landmark extraction failed")
        return None, provenance


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
