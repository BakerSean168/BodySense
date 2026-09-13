package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bodysense/api/internal/auth"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakePrivacyApplication struct {
	plan         *service.PrivacyErasurePlan
	planErr      error
	request      *model.PrivacyErasureRequest
	requestErr   error
	requestCalls int
	confirmation string
}

func (f *fakePrivacyApplication) Plan(_ context.Context, _ uuid.UUID) (*service.PrivacyErasurePlan, error) {
	return f.plan, f.planErr
}

func (f *fakePrivacyApplication) Request(_ context.Context, _ uuid.UUID, confirmation string) (*model.PrivacyErasureRequest, error) {
	f.requestCalls++
	f.confirmation = confirmation
	return f.request, f.requestErr
}

func newPrivacyOpenAPIRouter(t *testing.T, privacy privacyErasureApplication, secure bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSwagger()
	if err != nil {
		t.Fatalf("load generated OpenAPI spec: %v", err)
	}
	r := gin.New()
	g := r.Group("")
	g.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.NewString())
		c.Set("email", "user@example.com")
		c.Next()
	})
	g.Use(RequestValidator(spec))
	server := NewPublicServer(&fakeBodyStateFactService{}).WithPrivacy(privacy, auth.DefaultRefreshCookieName, secure)
	openapiv1.RegisterHandlers(g, StrictHandler(server))
	return r
}

func TestPrivacyErasurePlanIsValidatedAndNoStore(t *testing.T) {
	privacy := &fakePrivacyApplication{plan: &service.PrivacyErasurePlan{
		Destructive:        true,
		ConfirmationPhrase: service.PrivacyErasureConfirmationPhrase,
		Counts:             []repository.PrivacyDataCount{{Name: "account", Count: 1}},
		RetainedAudit:      []string{"anonymous erasure request status/timestamps"},
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/privacy/erasure-plan", nil)
	req.Header.Set("Authorization", "Bearer validated-by-auth-middleware")
	rec := httptest.NewRecorder()
	newPrivacyOpenAPIRouter(t, privacy, true).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q want=no-store", got)
	}
	assertOpenAPIResponse(t, req, rec)
	var body openapiv1.PrivacyErasurePlan
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode privacy plan: %v", err)
	}
	if !bool(body.Destructive) || len(body.Counts) != 1 || body.Counts[0].Count != 1 {
		t.Fatalf("privacy plan=%+v", body)
	}
}

func TestPrivacyErasureRejectsWrongConfirmationBeforeApplication(t *testing.T) {
	privacy := &fakePrivacyApplication{}
	rec := performJSONAt(
		newPrivacyOpenAPIRouter(t, privacy, true),
		http.MethodPost,
		"/api/v1/privacy/erasure",
		`{"confirmation":"wrong"}`,
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
	if privacy.requestCalls != 0 {
		t.Fatalf("privacy request calls=%d want=0", privacy.requestCalls)
	}
	assertErrorCode(t, rec, "INVALID_REQUEST")
}

func TestPrivacyErasureAcceptanceClearsRefreshCredentialAndIsNoStore(t *testing.T) {
	requestID := uuid.New()
	privacy := &fakePrivacyApplication{request: &model.PrivacyErasureRequest{
		ID: requestID, Status: "completed", RequestedAt: time.Now().UTC(),
	}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/privacy/erasure", nil)
	req.Header.Set("Authorization", "Bearer validated-by-auth-middleware")
	rec := performJSONAt(
		newPrivacyOpenAPIRouter(t, privacy, true),
		http.MethodPost,
		"/api/v1/privacy/erasure",
		`{"confirmation":"DELETE ALL BODY DATA"}`,
	)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d want=202 body=%s", rec.Code, rec.Body.String())
	}
	if privacy.requestCalls != 1 || privacy.confirmation != service.PrivacyErasureConfirmationPhrase {
		t.Fatalf("request calls=%d confirmation=%q", privacy.requestCalls, privacy.confirmation)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q want=no-store", got)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies=%d want=1 header=%q", len(cookies), rec.Header().Get("Set-Cookie"))
	}
	cookie := cookies[0]
	if cookie.Name != auth.DefaultRefreshCookieName || cookie.MaxAge != -1 || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != auth.RefreshCookiePath {
		t.Fatalf("cleared refresh cookie=%+v", cookie)
	}
	assertOpenAPIResponse(t, req, rec)
	var body openapiv1.PrivacyErasureAccepted
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode accepted response: %v", err)
	}
	if body.RequestId != requestID || body.Status != openapiv1.PrivacyErasureAcceptedStatusCompleted {
		t.Fatalf("accepted response=%+v", body)
	}
}
