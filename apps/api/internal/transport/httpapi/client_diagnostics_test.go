package httpapi

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/gin-gonic/gin"
)

func newClientDiagnosticOpenAPIRouter(t *testing.T) *gin.Engine {
	t.Helper()
	return newOpenAPITestRouter(t, &fakeBodyStateFactService{})
}

func TestClientDiagnosticOpenAPIRejectsNestedAttributes(t *testing.T) {
	rec := performJSONAt(
		newClientDiagnosticOpenAPIRouter(t),
		http.MethodPost,
		"/api/v1/client-diagnostics",
		`{"schemaVersion":1,"category":"app.runtime","event":"render_failed","attributes":{"nested":{"secret":"no"}}}`,
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec, "INVALID_REQUEST")
}

func TestClientDiagnosticOpenAPIRejectsUnknownCategory(t *testing.T) {
	rec := performJSONAt(
		newClientDiagnosticOpenAPIRouter(t),
		http.MethodPost,
		"/api/v1/client-diagnostics",
		`{"schemaVersion":1,"category":"health.payload","event":"should_not_log"}`,
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec, "INVALID_REQUEST")
}

func TestClientDiagnosticAcceptsBoundedScalarEnvelope(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/client-diagnostics", strings.NewReader(
		`{"schemaVersion":1,"category":"chat.transport","event":"reconnect","severity":"warn","resource":"https://example.test/assets/model.glb?token=secret","elapsedMs":12.75,"attributes":{"retry":2,"cached":false,"note":"bounded","optional":null}}`,
	))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer validated-by-auth-middleware")
	request.Header.Set("User-Agent", "BodySense-Test/1")
	rec := httptest.NewRecorder()
	newClientDiagnosticOpenAPIRouter(t).ServeHTTP(rec, request)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want=204 body=%s", rec.Code, rec.Body.String())
	}
}

func TestSanitizeDiagnosticAttributesRejectsUnsafeShapesAndNonFiniteNumbers(t *testing.T) {
	if _, ok := sanitizeDiagnosticAttributes(map[string]any{"nested": map[string]any{"x": 1}}); ok {
		t.Fatal("nested object unexpectedly accepted")
	}
	if _, ok := sanitizeDiagnosticAttributes(map[string]any{"nan": math.NaN()}); ok {
		t.Fatal("NaN unexpectedly accepted")
	}
	if _, ok := sanitizeDiagnosticAttributes(map[string]any{"too_long": strings.Repeat("x", 257)}); ok {
		t.Fatal("oversized string unexpectedly accepted")
	}
}

func TestSanitizeDiagnosticResourceDropsOriginAndQuery(t *testing.T) {
	if got := sanitizeDiagnosticResource("https://assets.example.test/models/body.glb?token=secret#mesh"); got != "/models/body.glb" {
		t.Fatalf("resource=%q want=/models/body.glb", got)
	}
}

func TestGeneratedClientDiagnosticRetainsFloat64TelemetryPrecision(t *testing.T) {
	var value openapiv1.ClientDiagnosticAttributes1 = 12.1234567890123
	if math.Abs(float64(value)-12.1234567890123) > 1e-12 {
		t.Fatalf("generated telemetry number lost float64 precision: %.15f", value)
	}
}
