package httpapi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bodysense/api/internal/auth"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
)

type authAccountApplication interface {
	Register(ctx context.Context, email, password string) (*service.AuthSessionResult, error)
	Login(ctx context.Context, email, password string) (*service.AuthSessionResult, error)
	RefreshSession(ctx context.Context, refreshToken string) (*service.AuthSessionResult, error)
	Logout(ctx context.Context, refreshToken string) error
}

// WithAuth attaches the account session application and the browser/session
// edge policy used by the public auth routes.
func (s *PublicServer) WithAuth(accounts authAccountApplication, security auth.SecurityConfig) *PublicServer {
	s.accounts = accounts
	s.authSecurity = security.Normalized()
	return s
}

// authReject is a transport-level auth edge rejection (origin, rate limit) the
// adapter maps onto the generated response type of its operation.
type authReject struct {
	code       string
	message    string
	retryAfter *string
}

// RegisterAccount registers a new account and establishes a browser session
// family. The refresh credential only ever leaves the server through the
// HttpOnly cookie; the JSON body carries the access credential alone.
func (s *PublicServer) RegisterAccount(
	ctx context.Context,
	request openapiv1.RegisterAccountRequestObject,
) (openapiv1.RegisterAccountResponseObject, error) {
	ginCtx := ginContextOrNil(ctx)
	if reject := s.requireTrustedOrigin(ginCtx); reject != nil {
		return registerAccountError403(reject), nil
	}
	if request.Body == nil {
		return registerAccountError400("VALIDATION_ERROR", "request body is required"), nil
	}
	email := normalizeAccount(request.Body.Email)
	if email == "" || request.Body.Password == "" {
		return registerAccountError400("VALIDATION_ERROR", "email and password are required"), nil
	}
	if reject := s.allowAuthAttempt(ginCtx, "register", email, s.authSecurity.RegisterPolicy); reject != nil {
		return registerAccountError(reject), nil
	}

	session, err := s.accounts.Register(ctx, email, request.Body.Password)
	if err != nil {
		if errors.Is(err, service.ErrRegistrationFailed) {
			return registerAccountError409("REGISTRATION_FAILED", "registration failed"), nil
		}
		return registerAccountError503("AUTH_SERVICE_UNAVAILABLE", "registration is temporarily unavailable"), nil
	}

	s.setRefreshCookie(ginCtx, session.RefreshToken)
	return openapiv1.RegisterAccount201JSONResponse{
		Body: authSessionFromOpenAPI(session),
		Headers: openapiv1.RegisterAccount201ResponseHeaders{
			CacheControl: authNoStoreHeader(),
			Pragma:       authNoCacheHeader(),
		},
	}, nil
}

// LoginAccount authenticates an existing account and establishes a browser
// session family under the same credential policy as registration.
func (s *PublicServer) LoginAccount(
	ctx context.Context,
	request openapiv1.LoginAccountRequestObject,
) (openapiv1.LoginAccountResponseObject, error) {
	ginCtx := ginContextOrNil(ctx)
	if reject := s.requireTrustedOrigin(ginCtx); reject != nil {
		return loginAccountError403(reject), nil
	}
	if request.Body == nil {
		return loginAccountError400("VALIDATION_ERROR", "request body is required"), nil
	}
	email := normalizeAccount(request.Body.Email)
	if email == "" || request.Body.Password == "" {
		return loginAccountError400("VALIDATION_ERROR", "email and password are required"), nil
	}
	if reject := s.allowAuthAttempt(ginCtx, "login", email, s.authSecurity.LoginPolicy); reject != nil {
		return loginAccountError(reject), nil
	}

	session, err := s.accounts.Login(ctx, email, request.Body.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return loginAccountError401("AUTHENTICATION_FAILED", "invalid email or password"), nil
		}
		return loginAccountError503("AUTH_SERVICE_UNAVAILABLE", "authentication is temporarily unavailable"), nil
	}

	s.setRefreshCookie(ginCtx, session.RefreshToken)
	return openapiv1.LoginAccount200JSONResponse{
		Body: authSessionFromOpenAPI(session),
		Headers: openapiv1.LoginAccount200ResponseHeaders{
			CacheControl: authNoStoreHeader(),
			Pragma:       authNoCacheHeader(),
		},
	}, nil
}

// RefreshAccountSession rotates the HttpOnly refresh credential and returns
// only the short-lived access credential in JSON.
func (s *PublicServer) RefreshAccountSession(
	ctx context.Context,
	_ openapiv1.RefreshAccountSessionRequestObject,
) (openapiv1.RefreshAccountSessionResponseObject, error) {
	ginCtx := ginContextOrNil(ctx)
	if reject := s.requireTrustedOrigin(ginCtx); reject != nil {
		return refreshAccountSessionError403(reject), nil
	}
	refreshToken := ""
	if ginCtx != nil {
		refreshToken, _ = ginCtx.Cookie(s.authSecurity.RefreshCookieName)
	}
	if refreshToken == "" {
		return refreshAccountSessionError401("REFRESH_FAILED", "refresh session is missing or expired"), nil
	}
	if reject := s.allowAuthAttempt(ginCtx, "refresh", refreshToken, s.authSecurity.RefreshPolicy); reject != nil {
		return refreshAccountSessionError(reject), nil
	}

	session, err := s.accounts.RefreshSession(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, service.ErrInvalidRefresh) || errors.Is(err, service.ErrRefreshReuse) {
			s.clearRefreshCookie(ginCtx)
			return refreshAccountSessionError401("REFRESH_FAILED", "invalid or expired refresh session"), nil
		}
		return refreshAccountSessionError503("AUTH_SERVICE_UNAVAILABLE", "authentication is temporarily unavailable"), nil
	}

	s.setRefreshCookie(ginCtx, session.RefreshToken)
	return openapiv1.RefreshAccountSession200JSONResponse{
		Body: authSessionFromOpenAPI(session),
		Headers: openapiv1.RefreshAccountSession200ResponseHeaders{
			CacheControl: authNoStoreHeader(),
			Pragma:       authNoCacheHeader(),
		},
	}, nil
}

// LogoutAccount revokes the current refresh/session family and clears the
// browser credential even when the server-side revocation path is degraded.
func (s *PublicServer) LogoutAccount(
	ctx context.Context,
	_ openapiv1.LogoutAccountRequestObject,
) (openapiv1.LogoutAccountResponseObject, error) {
	ginCtx := ginContextOrNil(ctx)
	if reject := s.requireTrustedOrigin(ginCtx); reject != nil {
		return logoutAccountError403(reject), nil
	}
	refreshToken := ""
	if ginCtx != nil {
		refreshToken, _ = ginCtx.Cookie(s.authSecurity.RefreshCookieName)
	}
	s.clearRefreshCookie(ginCtx)
	if refreshToken == "" {
		return logoutAccountAcknowledgement(), nil
	}

	if err := s.accounts.Logout(ctx, refreshToken); err != nil {
		return logoutAccountError503("LOGOUT_FAILED", "logout revocation is temporarily unavailable"), nil
	}
	return logoutAccountAcknowledgement(), nil
}

func ginContextOrNil(ctx context.Context) *gin.Context {
	if ginCtx, ok := ctx.(*gin.Context); ok {
		return ginCtx
	}
	return nil
}

func authSessionFromOpenAPI(session *service.AuthSessionResult) openapiv1.AuthSession {
	return openapiv1.AuthSession{AccessToken: session.AccessToken, ExpiresIn: int(session.ExpiresIn)}
}

func authNoStoreHeader() *string {
	value := "no-store"
	return &value
}

func authNoCacheHeader() *string {
	value := "no-cache"
	return &value
}

func (s *PublicServer) setRefreshCookie(ginCtx *gin.Context, token string) {
	if ginCtx == nil {
		return
	}
	http.SetCookie(ginCtx.Writer, auth.NewRefreshCookie(
		s.authSecurity.RefreshCookieName, token, s.authSecurity.RefreshTTL, s.authSecurity.CookieSecure, time.Now()))
}

func (s *PublicServer) clearRefreshCookie(ginCtx *gin.Context) {
	if ginCtx == nil {
		return
	}
	http.SetCookie(ginCtx.Writer, auth.NewClearedRefreshCookie(
		s.authSecurity.RefreshCookieName, s.authSecurity.CookieSecure))
}

func (s *PublicServer) requireTrustedOrigin(ginCtx *gin.Context) *authReject {
	if !s.authSecurity.RequireOrigin {
		return nil
	}
	origin := ""
	if ginCtx != nil {
		origin = strings.TrimSpace(ginCtx.GetHeader("Origin"))
	}
	if origin == "" {
		return &authReject{code: "ORIGIN_REQUIRED", message: "trusted browser origin required"}
	}
	for _, allowed := range s.authSecurity.TrustedOrigins {
		if origin == strings.TrimSpace(allowed) {
			return nil
		}
	}
	return &authReject{code: "ORIGIN_FORBIDDEN", message: "request origin is not allowed"}
}

func (s *PublicServer) allowAuthAttempt(ginCtx *gin.Context, action, dimension string, policy auth.RateLimitPolicy) *authReject {
	if s.authSecurity.RateLimiter == nil {
		return nil
	}
	if ginCtx == nil || ginCtx.Request == nil {
		return &authReject{code: "AUTH_SERVICE_UNAVAILABLE", message: "authentication is temporarily unavailable"}
	}
	key := fmt.Sprintf("%s|ip=%s|dimension=%s", action, ginCtx.ClientIP(), dimension)
	decision, err := s.authSecurity.RateLimiter.Allow(ginCtx.Request.Context(), key, policy)
	if err != nil {
		return &authReject{code: "AUTH_SERVICE_UNAVAILABLE", message: "authentication is temporarily unavailable"}
	}
	if decision.Allowed {
		return nil
	}
	retrySeconds := int(math.Ceil(decision.RetryAfter.Seconds()))
	if retrySeconds < 1 {
		retrySeconds = 1
	}
	retryAfter := strconv.Itoa(retrySeconds)
	return &authReject{code: "RATE_LIMITED", message: "too many authentication attempts", retryAfter: &retryAfter}
}

func normalizeAccount(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func registerAccountError400(code, message string) openapiv1.RegisterAccount400JSONResponse {
	return openapiv1.RegisterAccount400JSONResponse{
		InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(errorEnvelope(code, message)),
	}
}

func registerAccountError403(reject *authReject) openapiv1.RegisterAccount403JSONResponse {
	return openapiv1.RegisterAccount403JSONResponse{
		ForbiddenJSONResponse: openapiv1.ForbiddenJSONResponse(errorEnvelope(reject.code, reject.message)),
	}
}

func registerAccountError409(code, message string) openapiv1.RegisterAccount409JSONResponse {
	return openapiv1.RegisterAccount409JSONResponse{
		ConflictJSONResponse: openapiv1.ConflictJSONResponse(errorEnvelope(code, message)),
	}
}

func registerAccountError503(code, message string) openapiv1.RegisterAccount503JSONResponse {
	return openapiv1.RegisterAccount503JSONResponse{
		ServiceUnavailableJSONResponse: openapiv1.ServiceUnavailableJSONResponse(errorEnvelope(code, message)),
	}
}

func registerAccountError(reject *authReject) openapiv1.RegisterAccountResponseObject {
	if reject.code == "RATE_LIMITED" {
		return openapiv1.RegisterAccount429JSONResponse{
			TooManyRequestsJSONResponse: openapiv1.TooManyRequestsJSONResponse{
				Body:    errorEnvelope(reject.code, reject.message),
				Headers: openapiv1.TooManyRequestsResponseHeaders{RetryAfter: reject.retryAfter},
			},
		}
	}
	return registerAccountError503(reject.code, reject.message)
}

func loginAccountError400(code, message string) openapiv1.LoginAccount400JSONResponse {
	return openapiv1.LoginAccount400JSONResponse{
		InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(errorEnvelope(code, message)),
	}
}

func loginAccountError401(code, message string) openapiv1.LoginAccount401JSONResponse {
	return openapiv1.LoginAccount401JSONResponse{
		UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message)),
	}
}

func loginAccountError403(reject *authReject) openapiv1.LoginAccount403JSONResponse {
	return openapiv1.LoginAccount403JSONResponse{
		ForbiddenJSONResponse: openapiv1.ForbiddenJSONResponse(errorEnvelope(reject.code, reject.message)),
	}
}

func loginAccountError503(code, message string) openapiv1.LoginAccount503JSONResponse {
	return openapiv1.LoginAccount503JSONResponse{
		ServiceUnavailableJSONResponse: openapiv1.ServiceUnavailableJSONResponse(errorEnvelope(code, message)),
	}
}

func loginAccountError(reject *authReject) openapiv1.LoginAccountResponseObject {
	if reject.code == "RATE_LIMITED" {
		return openapiv1.LoginAccount429JSONResponse{
			TooManyRequestsJSONResponse: openapiv1.TooManyRequestsJSONResponse{
				Body:    errorEnvelope(reject.code, reject.message),
				Headers: openapiv1.TooManyRequestsResponseHeaders{RetryAfter: reject.retryAfter},
			},
		}
	}
	return loginAccountError503(reject.code, reject.message)
}

func refreshAccountSessionError401(code, message string) openapiv1.RefreshAccountSession401JSONResponse {
	return openapiv1.RefreshAccountSession401JSONResponse{
		UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message)),
	}
}

func refreshAccountSessionError403(reject *authReject) openapiv1.RefreshAccountSession403JSONResponse {
	return openapiv1.RefreshAccountSession403JSONResponse{
		ForbiddenJSONResponse: openapiv1.ForbiddenJSONResponse(errorEnvelope(reject.code, reject.message)),
	}
}

func refreshAccountSessionError503(code, message string) openapiv1.RefreshAccountSession503JSONResponse {
	return openapiv1.RefreshAccountSession503JSONResponse{
		ServiceUnavailableJSONResponse: openapiv1.ServiceUnavailableJSONResponse(errorEnvelope(code, message)),
	}
}

func refreshAccountSessionError(reject *authReject) openapiv1.RefreshAccountSessionResponseObject {
	if reject.code == "RATE_LIMITED" {
		return openapiv1.RefreshAccountSession429JSONResponse{
			TooManyRequestsJSONResponse: openapiv1.TooManyRequestsJSONResponse{
				Body:    errorEnvelope(reject.code, reject.message),
				Headers: openapiv1.TooManyRequestsResponseHeaders{RetryAfter: reject.retryAfter},
			},
		}
	}
	return refreshAccountSessionError503(reject.code, reject.message)
}

func logoutAccountError403(reject *authReject) openapiv1.LogoutAccount403JSONResponse {
	return openapiv1.LogoutAccount403JSONResponse{
		ForbiddenJSONResponse: openapiv1.ForbiddenJSONResponse(errorEnvelope(reject.code, reject.message)),
	}
}

func logoutAccountError503(code, message string) openapiv1.LogoutAccount503JSONResponse {
	return openapiv1.LogoutAccount503JSONResponse{
		ServiceUnavailableJSONResponse: openapiv1.ServiceUnavailableJSONResponse(errorEnvelope(code, message)),
	}
}

func logoutAccountAcknowledgement() openapiv1.LogoutAccount200JSONResponse {
	return openapiv1.LogoutAccount200JSONResponse{
		Body: openapiv1.LogoutAcknowledgement{Message: "Logged out successfully"},
	}
}
