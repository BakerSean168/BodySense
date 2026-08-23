package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/cache"
	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuthMiddleware creates a middleware for JWT authentication.
//
// Validation flow:
//  1. Extract and validate JWT signature + expiry
//  2. Tokens carrying a session id are checked against the session cache:
//     - present → allow (hot path)
//     - absent → 401 (session was revoked on logout / global sign-out)
//     - Redis unavailable → fail closed with 503; revocation authority is unavailable
//  3. Legacy tokens without a session id fall back to a DB user-exists check,
//     but are never written back into the session cache (a token minted before
//     session tracking must not re-arm a revoked session).
func AuthMiddleware(jwtConfig auth.JWTConfig, userRepo *repository.UserRepository, sessionCache cache.SessionCache) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ── Step 1: Extract Bearer token ──

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error:   "unauthorized",
				Message: "Authorization header is required",
			})
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error:   "unauthorized",
				Message: "Invalid authorization format. Use: Bearer <token>",
			})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error:   "unauthorized",
				Message: "Token is required",
			})
			c.Abort()
			return
		}

		// ── Step 2: Validate JWT signature + expiry ──

		claims, err := auth.ValidateAccessToken(jwtConfig, tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error:   "unauthorized",
				Message: "Invalid or expired token",
			})
			c.Abort()
			return
		}

		userID := claims.UserID

		// ── Step 3: Verify the session is still live ──

		if claims.SessionID == uuid.Nil {
			// Legacy token minted before session tracking: only check the user
			// still exists. Never write it back into the session cache. Legacy
			// verification also fails closed: a DB outage is not proof that the
			// account remains authorized.
			if err := verifyUserExistsNoCache(c, userID, userRepo); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
						Error:   "unauthorized",
						Message: "User no longer exists",
					})
				} else {
					c.JSON(http.StatusServiceUnavailable, dto.ErrorResponse{
						Error:   "authentication_unavailable",
						Message: "Authentication service is temporarily unavailable",
					})
				}
				c.Abort()
				return
			}
		} else {
			exists, cacheErr := sessionCache.Exists(c.Request.Context(), claims.SessionID)
			if cacheErr != nil {
				// Session authority is the revocation boundary. Failing open here would
				// let a previously revoked bearer token through during a Redis outage.
				c.JSON(http.StatusServiceUnavailable, dto.ErrorResponse{
					Error:   "authentication_unavailable",
					Message: "Authentication service is temporarily unavailable",
				})
				c.Abort()
				return
			} else if !exists {
				// Definitive miss: the session was revoked (logout / global sign-out).
				c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
					Error:   "unauthorized",
					Message: "Session has been revoked, please sign in again",
				})
				c.Abort()
				return
			}
		}

		// Set user info in context
		c.Set("user_id", userID.String())
		c.Set("email", claims.Email)

		c.Next()
	}
}

// verifyUserExistsNoCache queries the DB to confirm the user still exists. It
// never writes back to the session cache, so a revoked session cannot be revived
// by a legacy token or a Redis outage.
func verifyUserExistsNoCache(c *gin.Context, userID uuid.UUID, userRepo *repository.UserRepository) error {
	if userRepo == nil {
		return errors.New("user repository unavailable")
	}
	_, err := userRepo.FindByID(c.Request.Context(), userID)
	return err
}
