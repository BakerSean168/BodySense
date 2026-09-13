package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

const maxDiagnosticAttributes = 24

func (s *PublicServer) RecordClientDiagnostic(
	ctx context.Context,
	request openapiv1.RecordClientDiagnosticRequestObject,
) (openapiv1.RecordClientDiagnosticResponseObject, error) {
	if _, err := authenticatedUserID(ctx); err != nil {
		return recordClientDiagnosticError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if request.Body == nil {
		return recordClientDiagnosticError(http.StatusBadRequest, "INVALID_DIAGNOSTIC", "request body is required"), nil
	}

	attributes, err := diagnosticAttributesFromOpenAPI(request.Body.Attributes)
	if err != nil {
		return recordClientDiagnosticError(http.StatusBadRequest, "INVALID_DIAGNOSTIC", "invalid client diagnostic attributes"), nil
	}
	attrs, ok := sanitizeDiagnosticAttributes(attributes)
	if !ok {
		return recordClientDiagnosticError(http.StatusBadRequest, "INVALID_DIAGNOSTIC", "invalid client diagnostic attributes"), nil
	}

	severity := "info"
	if request.Body.Severity != nil {
		severity = string(*request.Body.Severity)
	}
	level := slog.LevelInfo
	switch severity {
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return recordClientDiagnosticError(http.StatusBadRequest, "INVALID_DIAGNOSTIC", "unsupported diagnostic severity"), nil
	}

	requestID := ""
	userAgent := ""
	if ginContext, ok := ctx.(*gin.Context); ok {
		requestID = requestid.Get(ginContext)
		if ginContext.Request != nil {
			userAgent = ginContext.Request.UserAgent()
		}
	}

	logAttrs := []slog.Attr{
		slog.String("kind", "client_diagnostic"),
		slog.Int("schema_version", int(request.Body.SchemaVersion)),
		slog.String("http_request_id", requestID),
		slog.String("category", strings.TrimSpace(string(request.Body.Category))),
		slog.String("event", strings.TrimSpace(request.Body.Event)),
		slog.String("code", optionalTrimmedString(request.Body.Code)),
		slog.String("phase", optionalTrimmedString(request.Body.Phase)),
		slog.String("conversation_id", optionalTrimmedString(request.Body.ConversationId)),
		slog.String("run_id", optionalTrimmedString(request.Body.RunId)),
		slog.String("request_id", optionalTrimmedString(request.Body.RequestId)),
		slog.String("diagnostic_session_id", optionalTrimmedString(request.Body.DiagnosticSessionId)),
		slog.String("attempt_id", optionalTrimmedString(request.Body.AttemptId)),
		slog.String("resource", sanitizeDiagnosticResource(optionalTrimmedString(request.Body.Resource))),
		slog.String("client_user_agent", userAgent),
	}
	if message := optionalTrimmedString(request.Body.Message); message != "" {
		logAttrs = append(logAttrs, slog.String("error_message", message))
	}
	if request.Body.ElapsedMs != nil {
		logAttrs = append(logAttrs, slog.Float64("elapsed_ms", *request.Body.ElapsedMs))
	}
	if len(attrs) > 0 {
		logAttrs = append(logAttrs, slog.Attr{Key: "attributes", Value: slog.GroupValue(attrs...)})
	}

	slog.LogAttrs(ctx, level, "client diagnostic", logAttrs...)
	return openapiv1.RecordClientDiagnostic204Response{}, nil
}

func diagnosticAttributesFromOpenAPI(
	input *map[string]*openapiv1.ClientDiagnostic_Attributes_AdditionalProperties,
) (map[string]any, error) {
	if input == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(*input)
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
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

func optionalTrimmedString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func recordClientDiagnosticError(status int, code, message string) openapiv1.RecordClientDiagnosticResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.RecordClientDiagnostic400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.RecordClientDiagnostic401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	default:
		return openapiv1.RecordClientDiagnostic500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
