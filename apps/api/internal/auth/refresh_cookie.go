package auth

import (
	"net/http"
	"strings"
	"time"
)

const (
	DefaultRefreshCookieName = "bodysense_refresh"
	RefreshCookiePath        = "/api/v1/auth"
)

func normalizeRefreshCookieName(name string) string {
	if strings.TrimSpace(name) == "" {
		return DefaultRefreshCookieName
	}
	return name
}

func NewRefreshCookie(name, value string, ttl time.Duration, secure bool, now time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     normalizeRefreshCookieName(name),
		Value:    value,
		Path:     RefreshCookiePath,
		MaxAge:   int(ttl.Seconds()),
		Expires:  now.Add(ttl),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	}
}

func NewClearedRefreshCookie(name string, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     normalizeRefreshCookieName(name),
		Value:    "",
		Path:     RefreshCookiePath,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	}
}
