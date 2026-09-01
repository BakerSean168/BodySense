# ---- Builder: install Python dependencies (cache stays here) ----
FROM python:3.13-slim AS builder
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/
WORKDIR /app

# Parameterized PyPI mirror — default is Aliyun for China-based CI,
# override with --build-arg UV_INDEX_URL=https://pypi.org/simple/ on GHA runners.
ARG UV_INDEX_URL=https://mirrors.aliyun.com/pypi/simple/
ENV UV_INDEX_URL=${UV_INDEX_URL}

COPY pyproject.toml uv.lock ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --no-dev --extra ocr --extra pose --no-cache

COPY . .
RUN /app/.venv/bin/python scripts/ensure_pose_model.py \
    --output /app/models/pose_landmarker_lite-float16-v1.task

# ---- Final: runtime image (no uv cache, no runtime model downloads) ----
FROM python:3.13-slim

ARG BUILD_DATE
ARG VCS_REF
LABEL org.opencontainers.image.title="bodysense-ai-service" \
      org.opencontainers.image.description="BodySense AI service (FastAPI + LangChain)" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}"

RUN apt-get update && apt-get install -y --no-install-recommends \
    tesseract-ocr \
    tesseract-ocr-chi-sim \
    libgl1 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/.venv /app/.venv
COPY . .
COPY --from=builder /app/models /app/models
ENV PATH="/app/.venv/bin:$PATH" \
    BODYSENSE_POSE_MODEL_PATH="/app/models/pose_landmarker_lite-float16-v1.task"
EXPOSE 8100
HEALTHCHECK --interval=10s --timeout=5s --start-period=15s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8100/health')" || exit 1
CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8100"]
