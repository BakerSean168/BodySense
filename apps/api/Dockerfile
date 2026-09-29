# ── Build stage ─────────────────────────────────────────
FROM golang:1.26-alpine@sha256:8ac98ca534ac3f51e1f420a1dd2c15e74c75cfa0f23f3ad27eb5d7236c349a0c AS builder

# Parameterized Go proxy — default is goproxy.cn for China-based CI,
# override with --build-arg GOPROXY=https://proxy.golang.org,direct on GHA runners.
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /server ./cmd/server && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /production-dr-manager ./cmd/production-dr-manager && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /domain-validator ./cmd/domain-validator && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /upload-storage-migrator ./cmd/upload-storage-migrator && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /knowledge-publication-manager ./cmd/knowledge-publication-manager && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /knowledge-source-manager ./cmd/knowledge-source-manager

# ── Runtime stage ──────────────────────────────────────
FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

ARG BUILD_DATE
ARG VCS_REF

RUN apk add --no-cache ca-certificates tzdata curl postgresql18-client
WORKDIR /app
COPY --from=builder /server /app/server
COPY --from=builder /production-dr-manager /app/production-dr-manager
COPY --from=builder /domain-validator /app/domain-validator
COPY --from=builder /upload-storage-migrator /app/upload-storage-migrator
COPY --from=builder /knowledge-publication-manager /app/knowledge-publication-manager
COPY --from=builder /knowledge-source-manager /app/knowledge-source-manager
COPY --from=builder /app/migrations /app/migrations
LABEL org.opencontainers.image.title="bodysense-api" \
      org.opencontainers.image.description="BodySense Go API backend" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}"
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
  CMD curl -f http://localhost:8080/api/health || exit 1
CMD ["/app/server"]
