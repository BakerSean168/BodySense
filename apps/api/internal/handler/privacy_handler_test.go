package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakePrivacyHandlerService struct {
	plan       *service.PrivacyErasurePlan
	request    *model.PrivacyErasureRequest
	requestErr error
	calls      int
}

func (f *fakePrivacyHandlerService) Plan(_ context.Context, _ uuid.UUID) (*service.PrivacyErasurePlan, error) {
	return f.plan, nil
}
func (f *fakePrivacyHandlerService) Request(_ context.Context, _ uuid.UUID, _ string) (*model.PrivacyErasureRequest, error) {
	f.calls++
	return f.request, f.requestErr
}

func privacyContext(method, body string, userID uuid.UUID) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, "/api/v1/privacy/erasure", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("user_id", userID.String())
	return ctx, recorder
}

func TestPrivacyHandlerPlanIsNoStore(t *testing.T) {
	fake := &fakePrivacyHandlerService{plan: &service.PrivacyErasurePlan{
		Destructive: true,
		Counts:      []repository.PrivacyDataCount{{Name: "account", Count: 1}},
	}}
	h := NewPrivacyHandler(fake, NewAuthHandler(nil))
	ctx, recorder := privacyContext(http.MethodGet, "", uuid.New())

	h.PlanErasure(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q", got)
	}
}

func TestPrivacyHandlerRejectsConfirmationMismatch(t *testing.T) {
	fake := &fakePrivacyHandlerService{requestErr: service.ErrPrivacyErasureConfirmation}
	h := NewPrivacyHandler(fake, NewAuthHandler(nil))
	ctx, recorder := privacyContext(http.MethodPost, `{"confirmation":"wrong"}`, uuid.New())

	h.RequestErasure(ctx)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if fake.calls != 1 {
		t.Fatalf("calls=%d, want 1", fake.calls)
	}
}

func TestPrivacyHandlerAcceptedRequestClearsRefreshCookie(t *testing.T) {
	request := &model.PrivacyErasureRequest{ID: uuid.New(), Status: "completed", RequestedAt: time.Now()}
	fake := &fakePrivacyHandlerService{request: request}
	authHandler := NewAuthHandler(nil, AuthSecurityConfig{RefreshTTL: time.Hour, CookieSecure: true})
	h := NewPrivacyHandler(fake, authHandler)
	ctx, recorder := privacyContext(http.MethodPost, `{"confirmation":"DELETE ALL BODY DATA"}`, uuid.New())

	h.RequestErasure(ctx)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != defaultRefreshCookieName || cookies[0].MaxAge != -1 || !cookies[0].HttpOnly || !cookies[0].Secure {
		t.Fatalf("unexpected cleared cookie: %+v", cookies)
	}
}

func TestPrivacyHandlerInternalFailureDoesNotPretendAcceptance(t *testing.T) {
	fake := &fakePrivacyHandlerService{requestErr: errors.New("db down")}
	h := NewPrivacyHandler(fake, NewAuthHandler(nil))
	ctx, recorder := privacyContext(http.MethodPost, `{"confirmation":"DELETE ALL BODY DATA"}`, uuid.New())

	h.RequestErasure(ctx)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
