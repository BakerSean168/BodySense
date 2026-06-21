# ── Build stage ─────────────────────────────────────────
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

# ── Runtime stage ──────────────────────────────────────
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata curl
COPY --from=builder /server /server
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
  CMD curl -f http://localhost:8080/api/health || exit 1
CMD ["/server"]
