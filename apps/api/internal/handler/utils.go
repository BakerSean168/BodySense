package handler

import (
	"errors"
	"net/http"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// respondError writes a structured error response in the format
// {"error": {"code": "...", "message": "..."}} as required by the design doc.
func respondError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, dto.NewErrorResponse(code, message))
}

// getUserUUID extracts and validates the user ID from the gin context.
// Returns the parsed UUID and true on success, or writes an error response and returns false.
func getUserUUID(c *gin.Context) (uuid.UUID, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return uuid.Nil, false
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_USER_ID", "invalid user id format")
		return uuid.Nil, false
	}

	return uid, true
}

// bodyStateHandleMutationError centralizes BodyState mutation errors for the
// remaining non-OpenAPI handlers that call the BodyState application service.
// Public BodyState routes themselves use transport/httpapi typed responses.
func bodyStateHandleMutationError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, repository.ErrBodyStateRevisionConflict):
		respondError(c, http.StatusConflict, "BODY_STATE_REVISION_CONFLICT", err.Error())
	case errors.Is(err, service.ErrUnknownBodyRegionID):
		respondError(c, http.StatusBadRequest, "INVALID_BODY_REGION_ID", err.Error())
	case errors.Is(err, service.ErrBodyRegionIDValidationUnavailable):
		respondError(c, http.StatusServiceUnavailable, "BODY_REGION_VALIDATION_UNAVAILABLE", err.Error())
	case errors.Is(err, gorm.ErrRecordNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "body state item not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update body state")
	}
	return true
}
