package handler

import (
	"log"
	"net/http"

	consultationruntime "github.com/bodysense/api/internal/consultation"
	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ConsultationHandler handles consultation HTTP requests.
type ConsultationHandler struct {
	consultationService *service.ConsultationService
	interactionService  *service.AgentInteractionService
	bodyStateService    *service.BodyStateService
	runtime             *consultationruntime.Runtime
}

// NewConsultationHandler creates a new ConsultationHandler.
func NewConsultationHandler(
	consultationService *service.ConsultationService,
	interactionService *service.AgentInteractionService,
	runtime *consultationruntime.Runtime,
	bodyStateServices ...*service.BodyStateService,
) *ConsultationHandler {
	var bodyStateService *service.BodyStateService
	if len(bodyStateServices) > 0 {
		bodyStateService = bodyStateServices[0]
	}
	return &ConsultationHandler{
		consultationService: consultationService,
		interactionService:  interactionService,
		bodyStateService:    bodyStateService,
		runtime:             runtime,
	}
}

// StartRun handles POST /api/v1/consultation-runs
func (h *ConsultationHandler) StartRun(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	var req dto.StartConsultationRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.runtime.StartRun(c.Request.Context(), c.Writer, uid, req); err != nil {
		respondError(c, err.Status, err.Code, err.Message)
		return
	}
}

// GetConsultation handles GET /api/v1/consultations/:id
func (h *ConsultationHandler) GetConsultation(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid consultation id")
		return
	}

	session, err := h.consultationService.GetConsultation(c.Request.Context(), conversationID, uid)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get consultation")
		return
	}
	if session == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "consultation not found")
		return
	}

	pendingInteractions, err := h.interactionService.GetPendingInteractions(c.Request.Context(), conversationID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get pending interactions")
		return
	}
	session.PendingInteractions = pendingInteractions

	c.JSON(http.StatusOK, session)
}

// ResumeInteraction handles POST /api/v1/consultations/:id/interrupts/:interactionId/answers
func (h *ConsultationHandler) ResumeInteraction(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	interactionID, err := uuid.Parse(c.Param("interactionId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid interaction id")
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid conversation id")
		return
	}

	var req dto.ResumeConsultationInteractionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.runtime.ResumeInteraction(
		c.Request.Context(),
		c.Writer,
		uid,
		conversationID,
		interactionID,
		req,
	); err != nil {
		respondError(c, err.Status, err.Code, err.Message)
	}
}

// GetInteractionMetrics handles GET /api/v1/consultations/:id/interaction-metrics
// T0-1 Phase C: lightweight projection over agent_interactions for this conversation.
func (h *ConsultationHandler) GetInteractionMetrics(c *gin.Context) {
	if _, ok := getUserUUID(c); !ok {
		return
	}
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid consultation id")
		return
	}
	metrics, err := h.interactionService.GetInteractionMetrics(c.Request.Context(), &conversationID)
	if err != nil {
		log.Printf("interaction metrics for %s: %v", conversationID, err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to compute interaction metrics")
		return
	}
	c.JSON(http.StatusOK, metrics)
}
