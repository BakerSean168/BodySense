package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/auth"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/service"
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

type fakeAuthAccountService struct {
	registerEmail, registerPassword string
	loginEmail, loginPassword       string
	refreshToken                    string
	logoutToken                     string
	session                         *service.AuthSessionResult
	err                             error
}

func (f *fakeAuthAccountService) Register(_ context.Context, email, password string) (*service.AuthSessionResult, error) {
	f.registerEmail, f.registerPassword = email, password
	if f.err != nil {
		return nil, f.err
	}
	return f.session, nil
}

func (f *fakeAuthAccountService) Login(_ context.Context, email, password string) (*service.AuthSessionResult, error) {
	f.loginEmail, f.loginPassword = email, password
	if f.err != nil {
		return nil, f.err
	}
	return f.session, nil
}

func (f *fakeAuthAccountService) RefreshSession(_ context.Context, refreshToken string) (*service.AuthSessionResult, error) {
	f.refreshToken = refreshToken
	if f.err != nil {
		return nil, f.err
	}
	return f.session, nil
}

func (f *fakeAuthAccountService) Logout(_ context.Context, refreshToken string) error {
	f.logoutToken = refreshToken
	return f.err
}

func newAuthTestRouter(t *testing.T, accounts authAccountApplication, security auth.SecurityConfig, extra func(r *gin.Engine)) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSwagger()
	if err != nil {
		t.Fatalf("load generated OpenAPI spec: %v", err)
	}
	r := gin.New()
	RegisterRoutes(r, StrictHandler(NewPublicServer(nil).WithAuth(accounts, security)), RouteSecurity{
		Auth: func(c *gin.Context) {
			c.Set("user_id", "11111111-1111-1111-1111-111111111111")
			c.Set("email", "user@example.com")
			c.Next()
		},
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	if extra != nil {
		extra(r)
	}
	return r
}

func authJSONBody(rec *httptest.ResponseRecorder) map[string]any {
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		panic(fmt.Sprintf("decode auth response: %v body=%s", err, rec.Body.String()))
	}
	return body
}

func TestAuthLoginSetsSecureHttpOnlyRefreshCookieAndHidesCredentialFromJSON(t *testing.T) {
	security := auth.SecurityConfig{
		RefreshCookieName: auth.DefaultRefreshCookieName,
		RefreshTTL:        24 * time.Hour,
		CookieSecure:      true,
	}
	accounts := &fakeAuthAccountService{session: &service.AuthSessionResult{
		AccessToken: "access-token", RefreshToken: "refresh-secret", ExpiresIn: 900,
	}}
	r := newAuthTestRouter(t, accounts, security, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"user@example.com","password":"hunter2222"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies=%d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != auth.DefaultRefreshCookieName || cookie.Value != "refresh-secret" {
		t.Fatalf("unexpected refresh cookie: %+v", cookie)
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie flags = HttpOnly:%v Secure:%v SameSite:%v", cookie.HttpOnly, cookie.Secure, cookie.SameSite)
	}
	if cookie.Path != "/api/v1/auth" {
		t.Fatalf("cookie path=%q", cookie.Path)
	}
	if accounts.loginEmail != "user@example.com" || accounts.loginPassword != "hunter2222" {
		t.Fatalf("service received email=%q password=%q", accounts.loginEmail, accounts.loginPassword)
	}

	body := authJSONBody(rec)
	if _, exists := body["refresh_token"]; exists {
		t.Fatal("refresh bearer credential leaked into JSON response")
	}
	if body["access_token"] != "access-token" || body["expires_in"] != float64(900) {
		t.Fatalf("session payload=%v", body)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q, want no-store", got)
	}
	if got := rec.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("Pragma=%q, want no-cache", got)
	}
}

func TestAuthRegisterCreatesAccountWith201(t *testing.T) {
	accounts := &fakeAuthAccountService{session: &service.AuthSessionResult{AccessToken: "a", RefreshToken: "r", ExpiresIn: 60}}
	r := newAuthTestRouter(t, accounts, auth.SecurityConfig{RefreshCookieName: auth.DefaultRefreshCookieName, RefreshTTL: time.Hour}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"new@example.com","password":"hunter2222"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if accounts.registerEmail != "new@example.com" {
		t.Fatalf("register email=%q", accounts.registerEmail)
	}
	if len(rec.Result().Cookies()) != 1 {
		t.Fatal("register must seed the refresh cookie")
	}
}

func TestAuthRegisterMapsConflictAndUnavailable(t *testing.T) {
	security := auth.SecurityConfig{RefreshCookieName: auth.DefaultRefreshCookieName, RefreshTTL: time.Hour}
	r := newAuthTestRouter(t, &fakeAuthAccountService{err: service.ErrRegistrationFailed}, security, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"dup@example.com","password":"hunter2222"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d", rec.Code)
	}
	assertErrorCode(t, rec, "REGISTRATION_FAILED")

	r = newAuthTestRouter(t, &fakeAuthAccountService{err: errors.New("db down")}, security, nil)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"x@example.com","password":"hunter2222"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rec.Code)
	}
	assertErrorCode(t, rec, "AUTH_SERVICE_UNAVAILABLE")
}

func TestAuthTrustedOriginPolicy(t *testing.T) {
	security := auth.SecurityConfig{
		RefreshCookieName: auth.DefaultRefreshCookieName,
		RefreshTTL:        time.Hour,
		RequireOrigin:     true,
		TrustedOrigins:    []string{"https://body.example.com"},
	}
	accounts := &fakeAuthAccountService{session: &service.AuthSessionResult{AccessToken: "a", RefreshToken: "r", ExpiresIn: 60}}
	r := newAuthTestRouter(t, accounts, security, nil)

	for _, tc := range []struct{ origin, code string }{
		{"", "ORIGIN_REQUIRED"},
		{"https://evil.example.com", "ORIGIN_FORBIDDEN"},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"user@example.com","password":"hunter2222"}`))
		req.Header.Set("Content-Type", "application/json")
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("origin %q status=%d", tc.origin, rec.Code)
		}
		assertErrorCode(t, rec, tc.code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"user@example.com","password":"hunter2222"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://body.example.com")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("trusted origin rejected: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthRateLimitReturnsDeterministic429(t *testing.T) {
	limiter := &fixedAuthRateLimiter{decision: auth.RateLimitDecision{Allowed: false, Count: 11, RetryAfter: 37 * time.Second}}
	security := auth.SecurityConfig{
		RefreshCookieName: auth.DefaultRefreshCookieName,
		RefreshTTL:        time.Hour,
		RateLimiter:       limiter,
	}
	accounts := &fakeAuthAccountService{session: &service.AuthSessionResult{AccessToken: "a", RefreshToken: "r", ExpiresIn: 60}}
	r := newAuthTestRouter(t, accounts, security, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"user@example.com","password":"hunter2222"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:54321"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d, want 429", rec.Code)
	}
	assertErrorCode(t, rec, "RATE_LIMITED")
	if got := rec.Header().Get("Retry-After"); got != "37" {
		t.Fatalf("Retry-After=%q, want 37", got)
	}
	if !strings.Contains(limiter.key, "ip=203.0.113.10") || !strings.Contains(limiter.key, "user@example.com") {
		t.Fatalf("rate limiter key=%q, want composite ip+dimension", limiter.key)
	}
	if accounts.loginEmail != "" {
		t.Fatal("rate-limited attempt must not reach the account service")
	}
}

func TestAuthRefreshRotatesCookieAndClearsItOnReuse(t *testing.T) {
	security := auth.SecurityConfig{RefreshCookieName: auth.DefaultRefreshCookieName, RefreshTTL: time.Hour}
	rotated := &fakeAuthAccountService{session: &service.AuthSessionResult{AccessToken: "next-access", RefreshToken: "next-refresh", ExpiresIn: 900}}
	r := newAuthTestRouter(t, rotated, security, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: auth.DefaultRefreshCookieName, Value: "stale-refresh"})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rotated.refreshToken != "stale-refresh" {
		t.Fatalf("service refresh token=%q", rotated.refreshToken)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value != "next-refresh" {
		t.Fatalf("rotated cookies=%v", cookies)
	}
	if body := authJSONBody(rec); body["access_token"] != "next-access" {
		t.Fatalf("rotated payload=%v", body)
	}

	reused := &fakeAuthAccountService{err: service.ErrRefreshReuse}
	r = newAuthTestRouter(t, reused, security, nil)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: auth.DefaultRefreshCookieName, Value: "consumed-refresh"})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
	assertErrorCode(t, rec, "REFRESH_FAILED")
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == auth.DefaultRefreshCookieName && cookie.Value != "" && cookie.MaxAge != -1 {
			t.Fatalf("reuse must clear the refresh cookie, got %+v", cookie)
		}
	}

	// Missing cookie is rejected before the service.
	missing := &fakeAuthAccountService{}
	r = newAuthTestRouter(t, missing, security, nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil))
	if rec.Code != http.StatusUnauthorized || missing.refreshToken != "" {
		t.Fatalf("missing cookie status=%d", rec.Code)
	}
}

func TestAuthLogoutClearsCookieAndAcknowledges(t *testing.T) {
	security := auth.SecurityConfig{RefreshCookieName: auth.DefaultRefreshCookieName, RefreshTTL: time.Hour}
	accounts := &fakeAuthAccountService{}
	r := newAuthTestRouter(t, accounts, security, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: auth.DefaultRefreshCookieName, Value: "doomed-refresh"})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if accounts.logoutToken != "doomed-refresh" {
		t.Fatalf("logout token=%q", accounts.logoutToken)
	}
	cleared := false
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == auth.DefaultRefreshCookieName && cookie.MaxAge == -1 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("logout must clear the refresh cookie")
	}
	if body := authJSONBody(rec); body["message"] != "Logged out successfully" {
		t.Fatalf("logout payload=%v", body)
	}

	// Logout keeps 200 even without a cookie.
	empty := &fakeAuthAccountService{}
	r = newAuthTestRouter(t, empty, security, nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))
	if rec.Code != http.StatusOK || empty.logoutToken != "" {
		t.Fatalf("cookie-less logout status=%d", rec.Code)
	}
}

func TestAuthRoutesRejectSpecInvalidPayloadBeforeService(t *testing.T) {
	security := auth.SecurityConfig{RefreshCookieName: auth.DefaultRefreshCookieName, RefreshTTL: time.Hour}
	accounts := &fakeAuthAccountService{}
	r := newAuthTestRouter(t, accounts, security, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"not-an-email","password":"hunter2222"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec, "INVALID_REQUEST")
	if accounts.registerEmail != "" {
		t.Fatal("spec-invalid payload must not reach the account service")
	}
}

func TestAuthRoutesStayPublicWhileProtectedRoutesRequireAuthentication(t *testing.T) {
	security := auth.SecurityConfig{RefreshCookieName: auth.DefaultRefreshCookieName, RefreshTTL: time.Hour}
	accounts := &fakeAuthAccountService{session: &service.AuthSessionResult{AccessToken: "a", RefreshToken: "r", ExpiresIn: 60}}
	spec, err := openapiv1.GetSwagger()
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}

	var authSeen *gin.Context
	r := gin.New()
	RegisterRoutes(r, StrictHandler(NewPublicServer(nil).WithAuth(accounts, security)), RouteSecurity{
		Auth: func(c *gin.Context) {
			authSeen = c
			c.AbortWithStatus(http.StatusTeapot)
		},
		Operator: func(c *gin.Context) {
			c.AbortWithStatus(http.StatusTeapot)
		},
		Validator: RequestValidator(spec),
	})

	// The auth endpoints must not run the authentication middleware at all.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"user@example.com","password":"hunter2222"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("auth route status=%d body=%s", rec.Code, rec.Body.String())
	}
	if authSeen != nil {
		t.Fatal("auth route must not pass through the authenticated middleware partition")
	}

	// A protected generated route must pass through the same middleware.
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/body-state", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("protected route status=%d, want 418 via auth middleware", rec.Code)
	}
}

// ginPathToSpecPath converts gin's ":param" path segments to OpenAPI's
// "{param}" form so router inventory can be compared against the spec.
func ginPathToSpecPath(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, ":") {
			segments[i] = "{" + segment[1:] + "}"
		}
	}
	return strings.Join(segments, "/")
}

func TestRegisterRoutesRegistersEverySpecOperationExactlyOnce(t *testing.T) {
	security := auth.SecurityConfig{RefreshCookieName: auth.DefaultRefreshCookieName, RefreshTTL: time.Hour}
	spec, err := openapiv1.GetSwagger()
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	r := gin.New()
	RegisterRoutes(r, StrictHandler(NewPublicServer(nil).WithAuth(&fakeAuthAccountService{}, security)), RouteSecurity{
		Auth:      func(c *gin.Context) { c.Next() },
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})

	registered := map[string]bool{}
	for _, route := range r.Routes() {
		key := route.Method + " " + ginPathToSpecPath(route.Path)
		if registered[key] {
			t.Fatalf("route registered twice: %s", key)
		}
		registered[key] = true
	}

	missing := 0
	for path, item := range spec.Paths.Map() {
		ops := map[string]bool{
			http.MethodGet: item.Get != nil, http.MethodPost: item.Post != nil,
			http.MethodPut: item.Put != nil, http.MethodPatch: item.Patch != nil,
			http.MethodDelete: item.Delete != nil,
		}
		for method, present := range ops {
			if !present {
				continue
			}
			if !registered[method+" "+path] {
				t.Errorf("spec operation missing from router: %s %s", method, path)
				missing++
			}
		}
	}
	if missing > 0 {
		t.Fatalf("%d spec operations missing from RegisterRoutes", missing)
	}
}
