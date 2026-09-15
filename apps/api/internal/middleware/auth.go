package middleware

import (
	"net/http"
	"strings"

	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/cache"
	"github.com/bodysense/api/internal/dto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuthMiddleware creates a middleware for JWT authentication.
//
// Validation flow:
//  1. Extract and validate JWT signature + expiry.
//  2. Require the canonical session-aware access-token contract.
//  3. Check the session id against the revocation cache:
//     - present → allow (hot path)
//     - absent → 401 (session was revoked on logout / global sign-out)
//     - Redis unavailable → fail closed with 503; revocation authority is unavailable.
func AuthMiddleware(jwtConfig auth.JWTConfig, sessionCache cache.SessionCache) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ── Step 1: Extract Bearer token ──

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("UNAUTHORIZED", "Authorization header is required"))
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("UNAUTHORIZED", "Invalid authorization format. Use: Bearer <token>"))
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("UNAUTHORIZED", "Token is required"))
			c.Abort()
			return
		}

		// ── Step 2: Validate JWT signature + expiry ──

		claims, err := auth.ValidateAccessToken(jwtConfig, tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("UNAUTHORIZED", "Invalid or expired token"))
			c.Abort()
			return
		}

		userID := claims.UserID

		// ── Step 3: Verify the canonical session is still live ──

		if claims.SessionID == uuid.Nil {
			c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("UNAUTHORIZED", "Access token is missing its session identity, please sign in again"))
			c.Abort()
			return
		}
		exists, cacheErr := sessionCache.Exists(c.Request.Context(), claims.SessionID)
		if cacheErr != nil {
			// Session authority is the revocation boundary. Failing open here would
			// let a previously revoked bearer token through during a Redis outage.
			c.JSON(http.StatusServiceUnavailable, dto.NewErrorResponse("AUTHENTICATION_UNAVAILABLE", "Authentication service is temporarily unavailable"))
			c.Abort()
			return
		} else if !exists {
			// Definitive miss: the session was revoked (logout / global sign-out).
			c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("UNAUTHORIZED", "Session has been revoked, please sign in again"))
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", userID.String())
		c.Set("email", claims.Email)

		c.Next()
	}
}
