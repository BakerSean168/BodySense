package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func newOpenAPITestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	return newOpenAPITestRouterWith(t, NewPublicServer(), uuid.NewString())
}

// newOpenAPITestRouterWith serves the generated contract (with request
// validation) for one authenticated user id.
func newOpenAPITestRouterWith(t *testing.T, server *PublicServer, userID string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatalf("load generated OpenAPI spec: %v", err)
	}
	r := gin.New()
	g := r.Group("")
	g.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("email", "user@example.com")
		c.Next()
	})
	g.Use(RequestValidator(spec))
	openapiv1.RegisterHandlers(g, StrictHandler(server))
	return r
}

func performJSONAt(r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer validated-by-auth-middleware")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestResolveSafetyOpenAPIRequiresExpectedRevisionBeforeAdapter(t *testing.T) {
	rec := performJSONAt(
		newOpenAPITestRouter(t),
		http.MethodPost,
		"/api/v1/body-state/safety/resolve",
		`{"resolution":"resolved","note":"reviewed"}`,
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec, "INVALID_REQUEST")
}

// The legacy BodyState fact writers are gone: body statements use the V2
// ledger (/body-state/records) and equipment uses /equipment.
func TestLegacyFactWriteRoutesAreGone(t *testing.T) {
	router := newOpenAPITestRouter(t)
	for _, path := range []string{"/api/v1/body-state/facts", "/api/v1/body-state/facts/" + uuid.NewString() + "/correct"} {
		rec := performJSONAt(router, http.MethodPost, path, `{"expected_revision":1,"fact":{"kind":"equipment.available","value":"band"}}`)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("legacy route %s must not exist: status=%d", path, rec.Code)
		}
	}
}

func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v body=%s", err, rec.Body.String())
	}
	if response.Error.Code != want {
		t.Fatalf("error.code=%q want=%q body=%s", response.Error.Code, want, rec.Body.String())
	}
}
