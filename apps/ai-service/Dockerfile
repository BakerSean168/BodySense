# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e

# Local/prod-like builds use the internal runtime-base stage. Candidate CI replaces
# this with the immutable ACR runtime-base digest so release metadata never
# invalidates heavy dependency/model layers.
ARG AI_RUNTIME_BASE=runtime-base

# BODYSENSE_AI_RUNTIME_BASE_BEGIN
FROM python:3.13-slim@sha256:9d2e5553305c7c7b0097999bb17187c69b921ccd6bc9d40e4bb5ebe652c00285 AS dependency-builder
COPY --from=ghcr.io/astral-sh/uv:latest@sha256:62f8c047d0a0e9ece6b53fc63df902585a67a47a7f318ddec4a37db586edc8e3 /uv /uvx /bin/
WORKDIR /app

# China-hosted developer/build environments may keep the Aliyun default. GitHub
# candidate CI explicitly overrides this to https://pypi.org/simple/.
ARG UV_INDEX_URL=https://mirrors.aliyun.com/pypi/simple/
ARG UV_SYNC_EXTRAS="--extra ocr --extra pose --extra document-ocr"
ENV UV_INDEX_URL=${UV_INDEX_URL}

COPY pyproject.toml uv.lock ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-dev ${UV_SYNC_EXTRAS}

# Model provisioning is intentionally isolated from application source. The
# runtime-base identity therefore depends only on locked dependencies, the
# provisioner implementations and the pinned model manifests below.
COPY src/__init__.py src/__init__.py
COPY src/configuration/__init__.py src/configuration/__init__.py
COPY src/configuration/posture_agent_config.py src/configuration/posture_agent_config.py
COPY src/configuration/health_document_config.py src/configuration/health_document_config.py
COPY config/agents/posture-v2.yaml config/agents/posture-v2.yaml
COPY config/document-extraction/health-document-v20.yaml config/document-extraction/health-document-v20.yaml
COPY scripts/ensure_pose_model.py scripts/ensure_pose_model.py
COPY scripts/ensure_health_document_models.py scripts/ensure_health_document_models.py
RUN /app/.venv/bin/python scripts/ensure_pose_model.py \
    --output /app/models/pose_landmarker_lite-float16-v1.task
RUN /app/.venv/bin/python scripts/ensure_health_document_models.py \
    --output /app/models/health-document/ppocrv6-small-v1

FROM python:3.13-slim@sha256:9d2e5553305c7c7b0097999bb17187c69b921ccd6bc9d40e4bb5ebe652c00285 AS runtime-base

# Pin the system runtime contract. Silent Debian repository drift must fail the
# build rather than mutate an existing runtime-base identity.
RUN apt-get update && apt-get install -y --no-install-recommends \
    tesseract-ocr=5.5.0-1+b1 \
    tesseract-ocr-chi-sim=1:4.1.0-2 \
    libgl1=1.7.0-1+b2 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --link --from=dependency-builder /app/.venv /app/.venv
COPY --link --from=dependency-builder /app/models /app/models
ENV PATH="/app/.venv/bin:$PATH" \
    BODYSENSE_POSE_MODEL_PATH="/app/models/pose_landmarker_lite-float16-v1.task" \
    BODYSENSE_HEALTH_DOCUMENT_MODEL_ROOT="/app/models/health-document/ppocrv6-small-v1"
# BODYSENSE_AI_RUNTIME_BASE_END

# The final application layer is deliberately thin. Candidate CI supplies
# AI_RUNTIME_BASE=<repository>@<digest>; local builds fall back to the internal
# stage above. Git revision/build date are OCI config labels injected by CI.
FROM ${AI_RUNTIME_BASE} AS application
WORKDIR /app
COPY --link . .
EXPOSE 8100
HEALTHCHECK --interval=10s --timeout=5s --start-period=15s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8100/health')" || exit 1
CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8100"]
