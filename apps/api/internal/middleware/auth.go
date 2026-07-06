package middleware

import (
	"errors"
	"log"
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
//  1. Extract and validate JWT signature + expiry (existing logic)
//  2. Check Redis session cache — hit → allow immediately (hot path)
//  3. Cache miss → query DB to confirm user exists
//     - Exists → write to Redis cache → allow
//     - Not exists → 401 "user no longer exists"
//
// Degradation: if Redis is unavailable, falls back to DB-only check.
func AuthMiddleware(jwtConfig auth.JWTConfig, userRepo *repository.UserRepository, sessionCache *cache.UserSessionCache) gin.HandlerFunc {
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

		// ── Step 3: Verify user still exists (Redis cache → DB fallback) ──

		if err := verifyUserExists(c, userID, userRepo, sessionCache); err != nil {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error:   "unauthorized",
				Message: "User no longer exists",
			})
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", userID.String())
		c.Set("email", claims.Email)

		c.Next()
	}
}

// verifyUserExists checks if the user exists using a two-tier strategy:
//
//  1. Check Redis session cache (fast path, ~0.1ms)
//  2. On cache miss, query DB (slow path, ~1-3ms)
//  3. If DB confirms user exists, write back to Redis
//
// If Redis is unavailable, degrades to DB-only — does NOT reject the request.
func verifyUserExists(c *gin.Context, userID uuid.UUID, userRepo *repository.UserRepository, sessionCache *cache.UserSessionCache) error {
	ctx := c.Request.Context()

	// Try Redis cache first
	exists, err := sessionCache.Exists(ctx, userID)
	if err == nil && exists {
		// Cache hit — user is valid, skip DB query
		return nil
	}
	// If err != nil, Redis is down — fall through to DB (degrade gracefully)

	// Cache miss or Redis unavailable — query DB
	_, err = userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// User definitively doesn't exist
			return err
		}
		// DB error (connection issue, etc.) — log but don't block the request
		// This prevents a DB blip from logging out all users
		log.Printf("[AuthMiddleware] DB lookup failed for user %s: %v", userID, err)
		return nil
	}

	// User exists in DB — write to Redis cache for next time
	// Ignore write errors (best-effort)
	if cacheErr := sessionCache.Set(ctx, userID); cacheErr != nil {
		log.Printf("[AuthMiddleware] Failed to cache user session for %s: %v", userID, cacheErr)
	}

	return nil
}
