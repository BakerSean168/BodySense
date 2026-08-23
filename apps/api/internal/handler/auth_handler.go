package handler

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
)

const defaultRefreshCookieName = "bodysense_refresh"

// AuthSecurityConfig owns the browser/session edge policy. Keeping it explicit
// prevents cookie, origin and abuse-control behavior from drifting across handlers.
type AuthSecurityConfig struct {
	RefreshCookieName string
	RefreshTTL        time.Duration
	CookieSecure      bool
	RequireOrigin     bool
	TrustedOrigins    []string
	RateLimiter       auth.RateLimiter
	LoginPolicy       auth.RateLimitPolicy
	RegisterPolicy    auth.RateLimitPolicy
	RefreshPolicy     auth.RateLimitPolicy
}

func DefaultAuthSecurityConfig(refreshTTL time.Duration) AuthSecurityConfig {
	return AuthSecurityConfig{
		RefreshCookieName: defaultRefreshCookieName,
		RefreshTTL:        refreshTTL,
		LoginPolicy:       auth.RateLimitPolicy{Limit: 10, Window: 5 * time.Minute},
		RegisterPolicy:    auth.RateLimitPolicy{Limit: 5, Window: 15 * time.Minute},
		RefreshPolicy:     auth.RateLimitPolicy{Limit: 60, Window: 5 * time.Minute},
	}
}

// AuthHandler handles authentication HTTP requests.
type AuthHandler struct {
	authService *service.AuthService
	security    AuthSecurityConfig
}

// NewAuthHandler creates a new AuthHandler. The variadic config preserves a
// narrow compatibility path for focused tests while production passes an
// explicit security policy from main.
func NewAuthHandler(authService *service.AuthService, security ...AuthSecurityConfig) *AuthHandler {
	cfg := DefaultAuthSecurityConfig(30 * 24 * time.Hour)
	if len(security) > 0 {
		cfg = security[0]
	}
	if cfg.RefreshCookieName == "" {
		cfg.RefreshCookieName = defaultRefreshCookieName
	}
	if cfg.RefreshTTL <= 0 {
		cfg.RefreshTTL = 30 * 24 * time.Hour
	}
	return &AuthHandler{authService: authService, security: cfg}
}

// Register handles user registration.
func (h *AuthHandler) Register(c *gin.Context) {
	if !h.requireTrustedOrigin(c) {
		return
	}
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if !h.allowAuthAttempt(c, "register", normalizeAccount(req.Email), h.security.RegisterPolicy) {
		return
	}

	resp, err := h.authService.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrRegistrationFailed) {
			respondError(c, http.StatusConflict, "REGISTRATION_FAILED", "registration failed")
			return
		}
		respondError(c, http.StatusServiceUnavailable, "AUTH_SERVICE_UNAVAILABLE", "registration is temporarily unavailable")
		return
	}

	h.writeAuthResponse(c, http.StatusCreated, resp)
}

// Login handles user login.
func (h *AuthHandler) Login(c *gin.Context) {
	if !h.requireTrustedOrigin(c) {
		return
	}
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if !h.allowAuthAttempt(c, "login", normalizeAccount(req.Email), h.security.LoginPolicy) {
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			respondError(c, http.StatusUnauthorized, "AUTHENTICATION_FAILED", "invalid email or password")
			return
		}
		respondError(c, http.StatusServiceUnavailable, "AUTH_SERVICE_UNAVAILABLE", "authentication is temporarily unavailable")
		return
	}

	h.writeAuthResponse(c, http.StatusOK, resp)
}

// RefreshToken rotates the HttpOnly refresh credential and returns only the
// short-lived access credential in JSON.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	if !h.requireTrustedOrigin(c) {
		return
	}
	refreshToken, err := c.Cookie(h.security.RefreshCookieName)
	if err != nil || refreshToken == "" {
		respondError(c, http.StatusUnauthorized, "REFRESH_FAILED", "refresh session is missing or expired")
		return
	}
	if !h.allowAuthAttempt(c, "refresh", refreshToken, h.security.RefreshPolicy) {
		return
	}

	resp, err := h.authService.RefreshToken(c.Request.Context(), dto.RefreshRequest{RefreshToken: refreshToken})
	if err != nil {
		if errors.Is(err, service.ErrInvalidRefresh) || errors.Is(err, service.ErrRefreshReuse) {
			h.clearRefreshCookie(c)
			respondError(c, http.StatusUnauthorized, "REFRESH_FAILED", "invalid or expired refresh session")
			return
		}
		respondError(c, http.StatusServiceUnavailable, "AUTH_SERVICE_UNAVAILABLE", "authentication is temporarily unavailable")
		return
	}

	h.writeAuthResponse(c, http.StatusOK, resp)
}

// Logout revokes the current refresh/session family and clears the browser
// credential even when the server-side revocation path is temporarily degraded.
func (h *AuthHandler) Logout(c *gin.Context) {
	if !h.requireTrustedOrigin(c) {
		return
	}
	refreshToken, _ := c.Cookie(h.security.RefreshCookieName)
	defer h.clearRefreshCookie(c)
	if refreshToken == "" {
		c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
		return
	}

	if err := h.authService.Logout(c.Request.Context(), refreshToken); err != nil {
		respondError(c, http.StatusServiceUnavailable, "LOGOUT_FAILED", "logout revocation is temporarily unavailable")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// Me returns the current authenticated user info.
func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	uid, ok := userID.(string)
	if !ok {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "invalid user id type")
		return
	}

	email, _ := c.Get("email")
	emailStr, _ := email.(string)
	c.JSON(http.StatusOK, dto.UserResponse{ID: uid, Email: emailStr})
}

func (h *AuthHandler) writeAuthResponse(c *gin.Context, status int, resp *dto.AuthResponse) {
	h.setRefreshCookie(c, resp.RefreshToken)
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(status, resp)
}

func (h *AuthHandler) setRefreshCookie(c *gin.Context, token string) {
	maxAge := int(h.security.RefreshTTL.Seconds())
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.security.RefreshCookieName,
		Value:    token,
		Path:     "/api/v1/auth",
		MaxAge:   maxAge,
		Expires:  time.Now().Add(h.security.RefreshTTL),
		HttpOnly: true,
		Secure:   h.security.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *AuthHandler) clearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.security.RefreshCookieName,
		Value:    "",
		Path:     "/api/v1/auth",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   h.security.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *AuthHandler) requireTrustedOrigin(c *gin.Context) bool {
	if !h.security.RequireOrigin {
		return true
	}
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin == "" {
		respondError(c, http.StatusForbidden, "ORIGIN_REQUIRED", "trusted browser origin required")
		return false
	}
	for _, allowed := range h.security.TrustedOrigins {
		if origin == strings.TrimSpace(allowed) {
			return true
		}
	}
	respondError(c, http.StatusForbidden, "ORIGIN_FORBIDDEN", "request origin is not allowed")
	return false
}

func (h *AuthHandler) allowAuthAttempt(c *gin.Context, action, dimension string, policy auth.RateLimitPolicy) bool {
	if h.security.RateLimiter == nil {
		return true
	}
	key := fmt.Sprintf("%s|ip=%s|dimension=%s", action, c.ClientIP(), dimension)
	decision, err := h.security.RateLimiter.Allow(c.Request.Context(), key, policy)
	if err != nil {
		respondError(c, http.StatusServiceUnavailable, "AUTH_SERVICE_UNAVAILABLE", "authentication is temporarily unavailable")
		return false
	}
	if decision.Allowed {
		return true
	}
	retrySeconds := int(math.Ceil(decision.RetryAfter.Seconds()))
	if retrySeconds < 1 {
		retrySeconds = 1
	}
	c.Header("Retry-After", strconv.Itoa(retrySeconds))
	respondError(c, http.StatusTooManyRequests, "RATE_LIMITED", "too many authentication attempts")
	return false
}

func normalizeAccount(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
