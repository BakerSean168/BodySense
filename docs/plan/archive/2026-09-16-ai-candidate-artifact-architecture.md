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

## First main-candidate observation

The first production-shaped main candidate after merge was Git revision `769332ad750d289bdd6f03b67e621c476de058fd`, GitHub Actions run `35048533292`. It completed successfully and promoted the coherent candidate to staging.

Observed timings:

- `Prepare AI runtime base`: 3m35s (`02:35:05Z` -> `02:38:40Z`), including the first build, ACR publication, and remote identity verification for `runtime-base-9fb479f776f61fba4f74d71d33d0a649c7b00de7e8dfc7bd932488a5f1b8ee4f`;
- `Build candidate aiService`: 2m01s (`02:38:43Z` -> `02:40:44Z`);
- the former worst observed AI candidate path was 39m45s, so the exact-SHA AI candidate stage fell by about 95%;
- compared with the prior 8m36s successful AI candidate, the exact-SHA AI candidate stage fell by about 77%.

Remote ACR manifests also prove the filesystem split rather than merely a faster rebuild:

- runtime base: 8 layers / 484.92 MiB compressed;
- exact-SHA candidate: 10 layers / 485.36 MiB compressed;
- the new BodySense application source layer is 458,126 bytes compressed (about 447 KiB), plus a 32-byte metadata/empty layer;
- candidate OCI labels bind the exact Git revision and the immutable runtime-base tag/digest.

GCP canonical staging then deployed all four application artifacts at the same revision and remained healthy.

## Warm-path probe

This evidence-only documentation change intentionally does not modify any `AI_RUNTIME_BASE_INPUTS`. Its merge is used as a second main-candidate probe. Acceptance is that the resolved runtime-base tag remains identical and `Prepare AI runtime base` reuses the already-published immutable base instead of executing the heavy build/push step. The new exact-SHA AI candidate should therefore publish only source/config/manifest changes on top of the same heavy base layers.
