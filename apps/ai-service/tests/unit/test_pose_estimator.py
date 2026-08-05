"""Unit tests for geometric pose metrics (no MediaPipe required)."""

from __future__ import annotations

from src.services.pose_estimator import (
    LM_LEFT_EAR,
    LM_LEFT_HIP,
    LM_LEFT_SHOULDER,
    LM_RIGHT_EAR,
    LM_RIGHT_HIP,
    LM_RIGHT_SHOULDER,
    Landmark,
    compute_frontal_metrics,
    compute_side_metrics,
    findings_from_metrics,
    metrics_to_dicts,
)


def test_side_metrics_detect_forward_head():
    # Ear well ahead of shoulder/C7 proxy → smaller CVA → forward head.
    # Facing right: large anterior dx relative to vertical rise → CVA ~40°.
    landmarks = {
        LM_RIGHT_EAR: Landmark(x=0.70, y=0.28, visibility=0.95),
        LM_RIGHT_SHOULDER: Landmark(x=0.50, y=0.40, visibility=0.95),
        LM_RIGHT_HIP: Landmark(x=0.48, y=0.65, visibility=0.9),
    }
    metrics = compute_side_metrics(landmarks)
    names = {m.name for m in metrics}
    assert "craniovertebral_angle" in names
    cva = next(m for m in metrics if m.name == "craniovertebral_angle")
    assert cva.unit == "deg"
    assert cva.finding_key == "forward_head"
    assert cva.severity in {"mild", "moderate", "marked"}
    assert cva.value < 50


def test_side_metrics_upright_has_no_forward_head():
    # Nearly stacked ear over shoulder → high CVA → no forward-head finding.
    landmarks = {
        LM_RIGHT_EAR: Landmark(x=0.50, y=0.18, visibility=0.95),
        LM_RIGHT_SHOULDER: Landmark(x=0.50, y=0.40, visibility=0.95),
        LM_RIGHT_HIP: Landmark(x=0.50, y=0.65, visibility=0.9),
    }
    metrics = compute_side_metrics(landmarks)
    cva_metrics = [m for m in metrics if m.name == "craniovertebral_angle"]
    # Either no finding (normal) or only mild — never invent marked for upright.
    for m in cva_metrics:
        assert m.severity != "marked"


def test_frontal_metrics_shoulder_tilt():
    landmarks = {
        LM_LEFT_SHOULDER: Landmark(x=0.35, y=0.30, visibility=0.95),
        LM_RIGHT_SHOULDER: Landmark(x=0.65, y=0.36, visibility=0.95),  # right lower
        LM_LEFT_HIP: Landmark(x=0.40, y=0.60, visibility=0.9),
        LM_RIGHT_HIP: Landmark(x=0.60, y=0.60, visibility=0.9),
    }
    metrics = compute_frontal_metrics(landmarks, "front")
    shoulder = [m for m in metrics if m.name == "shoulder_height_diff"]
    assert shoulder, "expected shoulder asymmetry metric"
    assert shoulder[0].finding_key == "shoulder_tilt"
    assert shoulder[0].severity in {"mild", "moderate", "marked"}


def test_findings_from_metrics_shape():
    landmarks = {
        LM_LEFT_EAR: Landmark(x=0.60, y=0.22, visibility=0.9),
        LM_LEFT_SHOULDER: Landmark(x=0.50, y=0.42, visibility=0.9),
    }
    metrics = compute_side_metrics(landmarks)
    findings = findings_from_metrics(metrics)
    for f in findings:
        assert "key" in f
        assert f["metric"] is not None
        assert f["metric"]["name"]
        assert f["metric"]["unit"]
    assert len(metrics_to_dicts(metrics)) == len(metrics)


def test_low_visibility_landmarks_skipped():
    landmarks = {
        LM_LEFT_SHOULDER: Landmark(x=0.3, y=0.3, visibility=0.1),
        LM_RIGHT_SHOULDER: Landmark(x=0.7, y=0.4, visibility=0.1),
    }
    metrics = compute_frontal_metrics(landmarks, "front")
    assert metrics == []
