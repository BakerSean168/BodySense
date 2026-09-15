package auth

import "time"

// SecurityConfig owns the browser/session edge policy for the public auth
// routes. Keeping it explicit prevents cookie, origin and abuse-control
// behavior from drifting between the transport adapter and main wiring.
type SecurityConfig struct {
	RefreshCookieName string
	RefreshTTL        time.Duration
	CookieSecure      bool
	RequireOrigin     bool
	TrustedOrigins    []string
	RateLimiter       RateLimiter
	LoginPolicy       RateLimitPolicy
	RegisterPolicy    RateLimitPolicy
	RefreshPolicy     RateLimitPolicy
}

// DefaultSecurityConfig returns the production abuse-control policy for the
// public auth endpoints.
func DefaultSecurityConfig(refreshTTL time.Duration) SecurityConfig {
	return SecurityConfig{
		RefreshCookieName: DefaultRefreshCookieName,
		RefreshTTL:        refreshTTL,
		LoginPolicy:       RateLimitPolicy{Limit: 10, Window: 5 * time.Minute},
		RegisterPolicy:    RateLimitPolicy{Limit: 5, Window: 15 * time.Minute},
		RefreshPolicy:     RateLimitPolicy{Limit: 60, Window: 5 * time.Minute},
	}
}

// Normalized fills defaulted fields so focused tests can pass partial configs.
func (c SecurityConfig) Normalized() SecurityConfig {
	if c.RefreshCookieName == "" {
		c.RefreshCookieName = DefaultRefreshCookieName
	}
	if c.RefreshTTL <= 0 {
		c.RefreshTTL = 30 * 24 * time.Hour
	}
	return c
}
