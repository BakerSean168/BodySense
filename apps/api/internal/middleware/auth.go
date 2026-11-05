package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// IdentityProvisioner is the only storage seam used by the auth middleware.
// It materializes the identity after verifying the Supabase JWT and enforces
// durable account-erasure tombstones.
type IdentityProvisioner interface {
	EnsureIdentity(ctx context.Context, id uuid.UUID, email string) (*model.User, error)
}

// AuthMiddleware validates hosted Supabase access tokens (ES256/JWKS) and
// resolves a local, owner-scoped business identity. No refresh cookies or
// locally minted JWTs are accepted.
func AuthMiddleware(verifier *auth.SupabaseVerifier, users IdentityProvisioner) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") || len(header) <= len("Bearer ") {
			c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("UNAUTHORIZED", "Supabase bearer access token required"))
			c.Abort()
			return
		}

		claims, err := verifier.Verify(c.Request.Context(), strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("UNAUTHORIZED", "Invalid or expired Supabase session"))
			c.Abort()
			return
		}
		id, err := uuid.Parse(claims.Subject)
		if err != nil || id == uuid.Nil {
			c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("UNAUTHORIZED", "Invalid Supabase account identity"))
			c.Abort()
			return
		}
		if _, err := users.EnsureIdentity(c.Request.Context(), id, strings.ToLower(strings.TrimSpace(claims.Email))); err != nil {
			// Includes account-erasure tombstones and datastore unavailability:
			// fail closed instead of granting access to erased health data.
			c.JSON(http.StatusServiceUnavailable, dto.NewErrorResponse("AUTHENTICATION_UNAVAILABLE", "Account access is unavailable"))
			c.Abort()
			return
		}
		c.Set("user_id", id.String())
		c.Set("email", claims.Email)
		c.Next()
	}
}
