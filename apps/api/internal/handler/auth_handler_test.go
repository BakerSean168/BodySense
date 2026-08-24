package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/dto"
	"github.com/gin-gonic/gin"
)

type fixedAuthRateLimiter struct {
	decision auth.RateLimitDecision
	err      error
	key      string
}

func (f *fixedAuthRateLimiter) Allow(_ context.Context, key string, _ auth.RateLimitPolicy) (auth.RateLimitDecision, error) {
	f.key = key
	return f.decision, f.err
}

func newAuthHandlerContext(method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, nil)
	return ctx, recorder
}

func TestAuthResponseSetsSecureHttpOnlyRefreshCookieAndHidesCredentialFromJSON(t *testing.T) {
	cfg := DefaultAuthSecurityConfig(24 * time.Hour)
	cfg.CookieSecure = true
	h := NewAuthHandler(nil, cfg)
	ctx, recorder := newAuthHandlerContext(http.MethodPost, "/api/v1/auth/login")

	h.writeAuthResponse(ctx, http.StatusOK, &dto.AuthResponse{
		AccessToken:  "access-token",
		RefreshToken: "refresh-secret",
		ExpiresIn:    900,
	})

	res := recorder.Result()
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies=%d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != defaultRefreshCookieName || cookie.Value != "refresh-secret" {
		t.Fatalf("unexpected refresh cookie: %+v", cookie)
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie flags = HttpOnly:%v Secure:%v SameSite:%v", cookie.HttpOnly, cookie.Secure, cookie.SameSite)
	}
	if cookie.Path != "/api/v1/auth" {
		t.Fatalf("cookie path=%q", cookie.Path)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, exists := body["refresh_token"]; exists {
		t.Fatal("refresh bearer credential leaked into JSON response")
	}
	if body["access_token"] != "access-token" {
		t.Fatalf("access_token=%v", body["access_token"])
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q, want no-store", got)
	}
}

func TestRequireTrustedOriginRejectsMissingAndCrossSiteOrigin(t *testing.T) {
	cfg := DefaultAuthSecurityConfig(time.Hour)
	cfg.RequireOrigin = true
	cfg.TrustedOrigins = []string{"https://body.example.com"}
	h := NewAuthHandler(nil, cfg)

	for _, origin := range []string{"", "https://evil.example.com"} {
		ctx, recorder := newAuthHandlerContext(http.MethodPost, "/api/v1/auth/refresh")
		if origin != "" {
			ctx.Request.Header.Set("Origin", origin)
		}
		if h.requireTrustedOrigin(ctx) {
			t.Fatalf("origin %q unexpectedly accepted", origin)
		}
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("origin %q status=%d, want 403", origin, recorder.Code)
		}
	}

	ctx, _ := newAuthHandlerContext(http.MethodPost, "/api/v1/auth/refresh")
	ctx.Request.Header.Set("Origin", "https://body.example.com")
	if !h.requireTrustedOrigin(ctx) {
		t.Fatal("trusted origin rejected")
	}
}

func TestAuthRateLimitReturnsDeterministic429(t *testing.T) {
	limiter := &fixedAuthRateLimiter{decision: auth.RateLimitDecision{
		Allowed:    false,
		Count:      11,
		RetryAfter: 37 * time.Second,
	}}
	cfg := DefaultAuthSecurityConfig(time.Hour)
	cfg.RateLimiter = limiter
	h := NewAuthHandler(nil, cfg)
	ctx, recorder := newAuthHandlerContext(http.MethodPost, "/api/v1/auth/login")
	ctx.Request.RemoteAddr = "203.0.113.10:54321"

	if h.allowAuthAttempt(ctx, "login", "user@example.com", cfg.LoginPolicy) {
		t.Fatal("rate-limited attempt unexpectedly allowed")
	}
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d, want 429", recorder.Code)
	}
	if got := recorder.Header().Get("Retry-After"); got != "37" {
		t.Fatalf("Retry-After=%q, want 37", got)
	}
	if limiter.key == "" {
		t.Fatal("rate limiter did not receive a composite dimension key")
	}
}
