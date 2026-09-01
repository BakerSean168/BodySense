#!/usr/bin/env python3
"""Provision the exact Posture geometry model outside the request path.

Production executes this at image build time. Local development executes it
before starting the AI service. The model URI is versioned in posture-v2 and the
bytes must match the manifest SHA256 before they are made visible atomically.
"""

from __future__ import annotations

import argparse
import hashlib
import os
import sys
import tempfile
import urllib.request
from pathlib import Path

SERVICE_ROOT = Path(__file__).resolve().parents[1]
if str(SERVICE_ROOT) not in sys.path:
    sys.path.insert(0, str(SERVICE_ROOT))

from src.configuration.posture_agent_config import get_default_posture_configuration  # noqa: E402
from src.services.pose_estimator import default_pose_model_path  # noqa: E402


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()

    config = get_default_posture_configuration()
    mechanism = config.geometry_mechanism
    if mechanism is None or not mechanism.required:
        raise SystemExit(
            "current Posture configuration does not bind a required geometry mechanism"
        )
    if "/latest/" in mechanism.model_uri:
        raise SystemExit("refusing unversioned Posture geometry model URI")

    target = args.output or default_pose_model_path(mechanism)
    target = target.expanduser().resolve()
    target.parent.mkdir(parents=True, exist_ok=True)

    if target.is_file() and target.stat().st_size > 0:
        digest = sha256_file(target)
        if digest == mechanism.model_sha256:
            print(f"POSE_MODEL_READY path={target} sha256={digest}")
            return

    fd, tmp_name = tempfile.mkstemp(prefix=target.name + ".", suffix=".partial", dir=target.parent)
    os.close(fd)
    tmp = Path(tmp_name)
    try:
        urllib.request.urlretrieve(mechanism.model_uri, tmp)  # noqa: S310 - manifest-pinned URI
        digest = sha256_file(tmp)
        if digest != mechanism.model_sha256:
            raise SystemExit(
                f"Posture model SHA256 mismatch: got {digest} want {mechanism.model_sha256}"
            )
        tmp.replace(target)
    finally:
        tmp.unlink(missing_ok=True)

    print(f"POSE_MODEL_READY path={target} sha256={mechanism.model_sha256}")


if __name__ == "__main__":
    main()
