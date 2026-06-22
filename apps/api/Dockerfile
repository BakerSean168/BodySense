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
WORKDIR /app
COPY --from=builder /server /app/server
COPY --from=builder /app/migrations /app/migrations
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
  CMD curl -f http://localhost:8080/api/health || exit 1
CMD ["/app/server"]
