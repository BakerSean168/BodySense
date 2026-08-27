# BodySense Observability Foundation — 2026-08-27

Status: active — foundation + Body Explorer diagnostics

## Problem

BodySense currently has durable domain/runtime events, container logs, nginx access logs,
and a small browser diagnostic endpoint, but these are not yet one coherent observability
system. The 3D Body Explorer can currently stall after a successful GLB response without
leaving evidence for the client-side parse / GPU upload / first-ready interval.

## External patterns reviewed

- OpenTelemetry logging model: keep mature language logging APIs, bridge/collect into one
  telemetry model rather than inventing a new logger API.
  https://opentelemetry.io/docs/specs/otel/logs/
- Dify: structured logging + trace/span correlation + OpenTelemetry extension.
  https://github.com/langgenius/dify/blob/main/api/app_factory.py
- Open WebUI: JSON-capable Python logging plus opt-in OpenTelemetry traces/metrics.
  https://github.com/open-webui/open-webui/blob/main/backend/open_webui/env.py
  https://github.com/open-webui/open-webui/blob/main/backend/open_webui/utils/telemetry/setup.py
- Langfuse: LLM/application tracing and OTLP interoperability; useful for AI-specific
  observations, not as the sole infrastructure logger.
  https://langfuse.com/docs/observability/overview
- Grafana Alloy: OTLP collector/router for Loki/Tempo/Prometheus-compatible backends.
  https://grafana.com/docs/alloy/latest/collect/opentelemetry-to-lgtm-stack/

## Decision

### Application logging

- Go API: standard `log/slog` JSON logger. Use official `gin-contrib/requestid` and
  `gin-contrib/slog` rather than bespoke request logging middleware.
- Existing `log.Printf` call sites migrate incrementally: `slog.SetDefault` keeps legacy
  standard-library logging flowing through the structured default logger.
- Python AI service: keep Python stdlib `logging`; later bridge/export through OTel rather
  than introducing a second application logging abstraction solely for formatting.
- Browser: prefer Grafana Faro Web SDK as the mature OSS RUM layer. Faro already captures
  browser performance/resource timing, errors, events and optional OTel traces, and Grafana
  Alloy has a first-class `faro.receiver`. Do not enable Session Replay or broad console/input
  capture by default in a health product.
- The current privacy-bounded typed diagnostic endpoint remains only as the immediate staging
  compatibility path until the Faro/Alloy receiver is deployed; do not grow it into a custom
  observability platform.

### Correlation

Stable operational correlation fields:
- `http_request_id`
- `trace_id` / `span_id` once tracing exporter is enabled
- consultation `request_id`, `run_id`, `conversation_id` where applicable
- browser `diagnostic_session_id` and per-mount/retry `attempt_id`

Health prompt text, BodyState values, auth headers/cookies, and raw request bodies are not
operational-log fields.

### Collection / storage target

Future deployment profile:
- server: `slog/Python logging -> stdout or OTLP -> Grafana Alloy -> Loki + Tempo`
- browser: `Grafana Faro Web SDK -> Alloy faro.receiver -> Loki + Tempo`
- LLM/RAG/tool observations: optional self-hosted Langfuse / OTLP export.

For staging diagnosis, Alloy can initially route Faro logs to `loki.echo` without deploying a
full Loki/Tempo stack. This keeps the application boundary on mature OSS while preserving a
small operational footprint.

## Immediate implementation

1. Replace ad-hoc API logger bootstrap with `slog` JSON.
2. Add `gin-contrib/requestid` and `gin-contrib/slog` request middleware.
3. Convert client diagnostic ingestion to structured `slog` attributes with bounded
   arbitrary primitive attributes; remove raw user id from emitted diagnostics.
4. Add browser diagnostic session/attempt correlation.
5. Instrument Body Explorer boundaries:
   - WebGL capability check
   - atlas catalog/metadata ready
   - model load start
   - progress milestones
   - `onModelReady` (GLTF parsed/mounted)
   - aggregate `onReady`
   - WebGL lost/restored
   - watchdog when model/viewer readiness exceeds threshold
   - Resource Timing summary for model URL
   - main-thread long-task summary when supported
6. Upgrade nginx access log to structured JSON including `request_time`, bytes and gzip ratio,
   so static anatomy transfer time is observable server-side.

## Acceptance

- One 3D attempt can be followed by `diagnostic_session_id + attempt_id`.
- A stall after HTTP 200 distinguishes network complete vs model-ready vs viewer-ready.
- Nginx exposes request duration and transfer byte counts for GLB requests.
- API logs are structured JSON and include HTTP request IDs without request bodies/secrets.
- Existing tests/typecheck/lint/build pass.
- Staging emits enough evidence for the next real browser refresh before making further 3D
  performance changes.
