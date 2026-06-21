package middleware

import (
	"net/http"
	"strings"

	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/dto"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware creates a middleware for JWT authentication.
func AuthMiddleware(jwtConfig auth.JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error:   "unauthorized",
				Message: "Authorization header is required",
			})
			c.Abort()
			return
		}

		// Check if it starts with "Bearer "
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error:   "unauthorized",
				Message: "Invalid authorization format. Use: Bearer <token>",
			})
			c.Abort()
			return
		}

		// Extract token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error:   "unauthorized",
				Message: "Token is required",
			})
			c.Abort()
			return
		}

		// Validate token
		claims, err := auth.ValidateAccessToken(jwtConfig, tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error:   "unauthorized",
				Message: "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", claims.UserID.String())
		c.Set("email", claims.Email)

		c.Next()
	}
}
