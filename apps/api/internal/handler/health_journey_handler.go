package handler

import (
	"net/http"

	"github.com/bodysense/api/internal/workflow"
	"github.com/gin-gonic/gin"
)

// HealthJourneyHandler handles health journey HTTP requests.
type HealthJourneyHandler struct {
	journey *workflow.HealthJourneyWorkflow
}

// NewHealthJourneyHandler creates a new HealthJourneyHandler.
func NewHealthJourneyHandler(journey *workflow.HealthJourneyWorkflow) *HealthJourneyHandler {
	return &HealthJourneyHandler{journey: journey}
}

// GetJourneyState handles GET /api/v1/journey
func (h *HealthJourneyHandler) GetJourneyState(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	state, err := h.journey.GetJourneyState(c.Request.Context(), uid)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to compute journey state")
		return
	}

	c.JSON(http.StatusOK, state)
}
