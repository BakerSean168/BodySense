package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// respondError writes a structured error response in the format
// {"error": {"code": "...", "message": "..."}} as required by the design doc.
func respondError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
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
