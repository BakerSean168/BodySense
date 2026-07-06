# ── Build stage ─────────────────────────────────────────
FROM golang:1.26-alpine AS builder

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
    CGO_ENABLED=0 go build -o /server ./cmd/server

# ── Runtime stage ──────────────────────────────────────
FROM alpine:3.20

ARG BUILD_DATE
ARG VCS_REF
LABEL org.opencontainers.image.title="bodysense-api" \
      org.opencontainers.image.description="BodySense Go API backend" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}"

RUN apk add --no-cache ca-certificates tzdata curl
WORKDIR /app
COPY --from=builder /server /app/server
COPY --from=builder /app/migrations /app/migrations
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
  CMD curl -f http://localhost:8080/api/health || exit 1
CMD ["/app/server"]
