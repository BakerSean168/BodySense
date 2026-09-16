# ADR 0016: Separate the AI runtime-base identity from exact-SHA release identity

- Status: Accepted
- Date: 2026-09-16
- Related: ADR 0008, ADR 0012, ADR 0013, ADR 0015

## Context

BodySense publishes one immutable coherent candidate set after exhaustive `main` CI. The Python AI image is materially larger than the other application images because it carries system OCR libraries, the locked Python environment, MediaPipe, RapidOCR/ONNX Runtime and pinned posture/document model artifacts.

The original AI Dockerfile placed `BUILD_DATE` and `VCS_REF` before the final stage's filesystem-mutating instructions. BuildKit therefore treated every Git revision as a new cache environment even when the `apps/ai-service` tree was byte-identical. Evidence from the `50d5a3e5...` and `1f69693f...` candidates showed about 502 MB of compressed layers per image, only about 46 MB reused, and about 456 MB regenerated even though the AI source tree had the same Git tree object. The release candidate consequently took nearly forty minutes when GitHub-to-Alibaba ACR upload throughput was poor.

Increasing a timeout protects correctness but does not fix this artifact-identity error.

## Decision

### 1. Give heavy AI runtime state its own content-derived identity

The AI Dockerfile exposes a `runtime-base` target containing:

- a digest-pinned Python base image;
- pinned system OCR/graphics packages;
- the `uv.lock`-resolved Python environment;
- pinned/verified posture and health-document model artifacts.

`scripts/delivery/ai-runtime-base.mjs` hashes the marked runtime-base Dockerfile recipe and the exact dependency/model provisioning inputs. The resulting immutable tag is:

```text
runtime-base-<sha256-of-runtime-input-contract>
```

The runtime base is stored in the existing `bodysense-ai-service` ACR repository. Keeping base and application manifests in one repository maximizes registry blob reuse and avoids relying on cross-repository blob mounting.

### 2. Keep exact Git revision identity in the thin application image

The final AI image contains the runtime base plus the BodySense AI application source layer. Candidate CI references the runtime base by immutable digest:

```text
AI_RUNTIME_BASE=<acr-repository>@sha256:<digest>
```

A normal source revision must not rebuild or republish heavy dependency/model layers when their content identity is unchanged.

### 3. Release metadata is OCI config, not a filesystem build input

`org.opencontainers.image.created` and `org.opencontainers.image.revision` are injected by `docker/build-push-action` through `labels:`. The AI Dockerfile contains no `BUILD_DATE` or `VCS_REF` build arguments.

Therefore a release-only commit may change image config/manifest identity while retaining every unchanged filesystem layer.

### 4. Runtime-base publication is build-once/reuse-many

Candidate publication resolves the runtime-base content tag first. If ACR already contains that immutable tag and its identity label matches, CI reuses its digest. Otherwise CI builds only the `runtime-base` target, publishes it once, and verifies the remote digest/identity before allowing the AI candidate build.

The heavy base job and thin application job are separate. AI publication no longer waits on Web static-asset publication.

### 5. Dependency restoration must use real cache semantics

The runtime-base build uses a BuildKit cache mount for uv and runs `uv sync --frozen` without `--no-cache`. GitHub-hosted runners explicitly use `https://pypi.org/simple/`; the Dockerfile may retain the Aliyun mirror as the default for China-hosted local/build environments.

## Consequences

### Positive

- release metadata cannot invalidate AI dependency/model filesystem layers;
- unchanged dependencies/models are represented by one durable ACR runtime-base artifact;
- ordinary AI source changes normally add only a small application layer;
- release-only commits can reuse the application filesystem layer as well;
- GitHub Actions cache stops accumulating a new copy of the 300+ MB Python environment for every revision;
- ACR upload volume and candidate latency become proportional to real AI changes rather than Git commit frequency;
- model integrity remains governed by the existing version/SHA-256 contracts.

### Trade-offs

- candidate publication owns one additional immutable runtime-base tag class;
- the first candidate after a runtime-base identity change may still pay the full dependency/model build and upload cost;
- the runtime-base input list is a delivery contract and must evolve with the marked Dockerfile recipe and model provisioning boundary;
- a future registry-retention policy must preserve runtime-base tags referenced by candidate/release images.

## Rejected alternatives

### Only increase the AI timeout

Rejected because it tolerates unnecessary 400+ MB layer churn instead of correcting it.

### Move candidate builds to the Alibaba production host

Rejected because it mixes untrusted/source build authority with the production runtime mutation plane and treats network locality as a substitute for correct artifact boundaries.

### Keep Git revision as a runtime-base key

Rejected because dependency/model identity is intentionally independent from source/release identity.

### Store models as request-time downloads

Rejected because posture/document serving already requires pinned artifacts to be provisioned and verified before requests are accepted.
