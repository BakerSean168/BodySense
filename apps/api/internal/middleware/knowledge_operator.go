package middleware

import (
	"errors"
	"net/http"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RequireKnowledgeOperator gates global Knowledge administration. The role is
// loaded from the durable user record rather than trusted from client input or
// a long-lived token claim.
func RequireKnowledgeOperator(userRepo *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawUserID, ok := c.Get("user_id")
		if !ok || userRepo == nil {
			c.JSON(http.StatusServiceUnavailable, dto.ErrorResponse{Error: "authorization_unavailable", Message: "Authorization service is temporarily unavailable"})
			c.Abort()
			return
		}
		userIDText, ok := rawUserID.(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "unauthorized", Message: "Invalid authenticated user identity"})
			c.Abort()
			return
		}
		userID, err := uuid.Parse(userIDText)
		if err != nil {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "unauthorized", Message: "Invalid authenticated user identity"})
			c.Abort()
			return
		}
		user, err := userRepo.FindByID(c.Request.Context(), userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "unauthorized", Message: "Authenticated user no longer exists"})
			} else {
				c.JSON(http.StatusServiceUnavailable, dto.ErrorResponse{Error: "authorization_unavailable", Message: "Authorization service is temporarily unavailable"})
			}
			c.Abort()
			return
		}
		if user.Role != model.UserRoleOperator {
			c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "forbidden", Message: "Knowledge operator permission is required"})
			c.Abort()
			return
		}
		c.Set("knowledge_operator_id", user.ID.String())
		c.Next()
	}
}
