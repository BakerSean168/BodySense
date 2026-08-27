package handler

import (
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

const (
	clientDiagnosticSchemaVersion = 1
	maxDiagnosticAttributes       = 24
)

// ClientDiagnosticHandler accepts a deliberately small, privacy-safe browser
// diagnostic envelope. It is operational telemetry, not a health-data sink:
// clients must never include prompt/message/body-state content here.
type ClientDiagnosticHandler struct{}

type clientDiagnosticRequest struct {
	SchemaVersion       int            `json:"schemaVersion" binding:"omitempty,min=1,max=1"`
	Category            string         `json:"category" binding:"required,max=64"`
	Event               string         `json:"event" binding:"required,max=96"`
	Severity            string         `json:"severity" binding:"omitempty,max=16"`
	Code                string         `json:"code" binding:"omitempty,max=96"`
	Message             string         `json:"message" binding:"omitempty,max=512"`
	Phase               string         `json:"phase" binding:"omitempty,max=96"`
	ConversationID      string         `json:"conversationId" binding:"omitempty,max=128"`
	RunID               string         `json:"runId" binding:"omitempty,max=128"`
	RequestID           string         `json:"requestId" binding:"omitempty,max=128"`
	Resource            string         `json:"resource" binding:"omitempty,max=256"`
	DiagnosticSessionID string         `json:"diagnosticSessionId" binding:"omitempty,max=128"`
	AttemptID           string         `json:"attemptId" binding:"omitempty,max=128"`
	ElapsedMS           *float64       `json:"elapsedMs" binding:"omitempty,min=0"`
	Attributes          map[string]any `json:"attributes" binding:"omitempty"`
}

func NewClientDiagnosticHandler() *ClientDiagnosticHandler {
	return &ClientDiagnosticHandler{}
}

func (h *ClientDiagnosticHandler) Record(c *gin.Context) {
	if _, ok := getUserUUID(c); !ok {
		return
	}

	var req clientDiagnosticRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_DIAGNOSTIC", "invalid client diagnostic payload")
		return
	}
	if req.SchemaVersion == 0 {
		req.SchemaVersion = clientDiagnosticSchemaVersion
	}

	switch req.Category {
	case "chat.transport", "body3d.viewer", "app.runtime":
	default:
		respondError(c, http.StatusBadRequest, "INVALID_DIAGNOSTIC", "unsupported diagnostic category")
		return
	}

	level := slog.LevelInfo
	if req.Severity == "" {
		req.Severity = "info"
	}
	switch req.Severity {
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		respondError(c, http.StatusBadRequest, "INVALID_DIAGNOSTIC", "unsupported diagnostic severity")
		return
	}

	attrs, ok := sanitizeDiagnosticAttributes(req.Attributes)
	if !ok {
		respondError(c, http.StatusBadRequest, "INVALID_DIAGNOSTIC", "invalid client diagnostic attributes")
		return
	}

	logAttrs := []slog.Attr{
		slog.String("kind", "client_diagnostic"),
		slog.Int("schema_version", req.SchemaVersion),
		slog.String("http_request_id", requestid.Get(c)),
		slog.String("category", strings.TrimSpace(req.Category)),
		slog.String("event", strings.TrimSpace(req.Event)),
		slog.String("code", strings.TrimSpace(req.Code)),
		slog.String("phase", strings.TrimSpace(req.Phase)),
		slog.String("conversation_id", strings.TrimSpace(req.ConversationID)),
		slog.String("run_id", strings.TrimSpace(req.RunID)),
		slog.String("request_id", strings.TrimSpace(req.RequestID)),
		slog.String("diagnostic_session_id", strings.TrimSpace(req.DiagnosticSessionID)),
		slog.String("attempt_id", strings.TrimSpace(req.AttemptID)),
		slog.String("resource", sanitizeDiagnosticResource(req.Resource)),
		slog.String("client_user_agent", c.Request.UserAgent()),
	}
	if message := strings.TrimSpace(req.Message); message != "" {
		logAttrs = append(logAttrs, slog.String("error_message", message))
	}
	if req.ElapsedMS != nil {
		logAttrs = append(logAttrs, slog.Float64("elapsed_ms", *req.ElapsedMS))
	}
	if len(attrs) > 0 {
		logAttrs = append(logAttrs, slog.Attr{Key: "attributes", Value: slog.GroupValue(attrs...)})
	}

	slog.LogAttrs(c.Request.Context(), level, "client diagnostic", logAttrs...)
	c.Status(http.StatusNoContent)
}

func sanitizeDiagnosticAttributes(input map[string]any) ([]slog.Attr, bool) {
	if len(input) == 0 {
		return nil, true
	}
	if len(input) > maxDiagnosticAttributes {
		return nil, false
	}

	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	attrs := make([]slog.Attr, 0, len(keys))
	for _, rawKey := range keys {
		key := strings.TrimSpace(rawKey)
		if key == "" || len(key) > 64 {
			return nil, false
		}
		switch value := input[rawKey].(type) {
		case string:
			if len(value) > 256 {
				return nil, false
			}
			attrs = append(attrs, slog.String(key, value))
		case bool:
			attrs = append(attrs, slog.Bool(key, value))
		case float64:
			if math.IsInf(value, 0) || math.IsNaN(value) {
				return nil, false
			}
			attrs = append(attrs, slog.Float64(key, value))
		case nil:
			continue
		default:
			// Deliberately reject nested objects/arrays. The browser telemetry
			// contract is flat, bounded and safe to index in Loki/OTel backends.
			return nil, false
		}
	}
	return attrs, true
}

func sanitizeDiagnosticResource(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return value
	}
	if parsed.Path != "" {
		return parsed.Path
	}
	return value
}
