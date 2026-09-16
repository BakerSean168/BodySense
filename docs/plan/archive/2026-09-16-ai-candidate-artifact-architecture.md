# BodySense AI Candidate Artifact Architecture Refactor

Date: 2026-09-16
Status: complete
Owner: BodySense delivery platform

## Problem

`bodysense-ai-service` currently couples release identity to the heavy runtime filesystem. A release-only commit that does not change `apps/ai-service` still changes roughly 456 MB of compressed image layers because `BUILD_DATE` / `VCS_REF` are declared before filesystem-mutating instructions in the final stage. The candidate then uploads those regenerated layers from GitHub-hosted runners to Alibaba ACR, producing large and highly variable publication times.

The current Docker build also mounts the uv cache while invoking `uv sync --no-cache`, and GitHub-hosted AI builds continue to use the China-oriented Aliyun PyPI mirror by default.

## Decision target

Separate three identities:

1. **AI runtime-base identity** — system packages, locked Python dependencies, and pinned model artifacts. It changes only when runtime/dependency/model inputs change.
2. **AI application identity** — BodySense AI source tree for the exact candidate source revision.
3. **release metadata identity** — Git revision/build time expressed as OCI image config labels, never as filesystem cache inputs.

The runtime base is published as an immutable content-derived tag in the existing `bodysense-ai-service` ACR repository so its heavy blobs can be reused by candidate images without cross-repository blob mounting.

## Implementation

### Dockerfile

- pin the Python and uv source images by OCI digest;
- make `runtime-base` a first-class build target containing apt runtime packages, `.venv`, and verified model artifacts;
- use `uv sync --frozen` with a real BuildKit cache mount and remove `--no-cache`;
- pin apt package versions used by the current health-document runtime;
- make the default final stage derive from the internal `runtime-base` for local/prod-like builds;
- allow CI to replace that final base with an immutable ACR runtime-base digest;
- remove `BUILD_DATE` / `VCS_REF` from the AI Dockerfile entirely;
- keep application source in the final thin layer only.

### Runtime-base identity

Add a deterministic delivery helper that hashes:

- the marked runtime-base recipe in `apps/ai-service/Dockerfile`;
- `pyproject.toml` and `uv.lock`;
- model provisioning scripts;
- the current posture and health-document configuration loaders/manifests.

The output is `runtime-base-<sha256>`.

### Candidate workflow

- resolve the runtime-base tag alongside the exact-SHA candidate identity;
- add a dedicated `Prepare AI runtime base` job;
- reuse an existing immutable runtime-base tag when present;
- otherwise build/push only the `runtime-base` target;
- use `https://pypi.org/simple/` on GitHub-hosted runners;
- build AI candidate separately from Web/API/runtime so it no longer waits on Web static assets;
- build the thin AI image from `${runtime-base-repository}@${digest}`;
- inject dynamic OCI labels via `docker/build-push-action labels:`;
- retain remote manifest/config identity verification without pulling heavy layers.

### Governance

Add delivery tests that fail if:

- dynamic release metadata re-enters the AI Dockerfile;
- the runtime-base recipe loses content-derived identity;
- `uv --no-cache` returns;
- the GHA AI base build uses the Aliyun PyPI default;
- the thin candidate is not based on the immutable runtime-base digest;
- dynamic revision/build-date labels are not injected by the workflow.

## Acceptance

- existing delivery tests pass;
- AI unit/lint tests impacted by model provisioning scripts pass;
- Dockerfile syntax/build graph validates;
- a local `runtime-base` build succeeds;
- a local thin application build from an externally tagged runtime-base succeeds;
- changing only release metadata leaves the runtime-base identity unchanged;
- changing `uv.lock` or a pinned model manifest changes the runtime-base identity;
- candidate workflow YAML is syntactically valid and governance-tested;
- deployment architecture and ADR document the new artifact boundary.

## Rollout note

The first main revision after this change may still pay one heavy runtime-base upload because the new immutable base tag does not yet exist in ACR. Later exact-SHA AI candidates should reuse those heavy blobs and normally publish only the application layer plus image config/manifest.

## Completion evidence

- delivery governance: 43/43 tests pass;
- architecture boundary and mutation gates pass;
- AI provisioning scripts pass Ruff and the focused posture/document tests (20/20);
- Dockerfile build check passes;
- the final runtime base was built at about 500 MB and verified with MediaPipe 1.0.0, RapidOCR 3.9.2, ONNX Runtime 1.29.0, three health-document ONNX artifacts, the pinned pose model and Tesseract 5.5.0;
- thin application validation used an external registry-backed AI base, copied about 2 MB of build context, completed its source layer/export in under a second each, finished the local build in about five seconds, and passed runtime imports/model presence;
- staging-channel verification now compares remote source/destination digests plus OCI revision without pulling application filesystems.

The first main candidate after merge remains the canonical end-to-end performance proof because it creates the new ACR runtime-base tag once and then builds the exact-SHA thin AI candidate against its immutable digest.
