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
	replayService       *service.ConsultationReplayService
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

// WithReplayService attaches the run-level decision authority replay service.
func (h *ConsultationHandler) WithReplayService(replay *service.ConsultationReplayService) *ConsultationHandler {
	h.replayService = replay
	return h
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

// ReplayRun handles POST /api/v1/consultation-runs/:id/replay. Read-only:
// recomputes the Go decision authority for a completed run without a model call.
func (h *ConsultationHandler) ReplayRun(c *gin.Context) {
	if h.replayService == nil {
		respondError(c, http.StatusServiceUnavailable, "REPLAY_UNAVAILABLE", "replay service not configured")
		return
	}
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	runID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid run id")
		return
	}
	decision, err := h.replayService.HistoricalReplay(c.Request.Context(), uid, runID)
	if err != nil {
		switch {
		case err == service.ErrConsultationReplayUnavailable:
			respondError(c, http.StatusUnprocessableEntity, "REPLAY_UNAVAILABLE", "run predates North-Star provenance")
		default:
			respondError(c, http.StatusInternalServerError, "REPLAY_FAILED", err.Error())
		}
		return
	}
	c.JSON(http.StatusOK, decision)
}

// ReplayRunCounterfactual handles POST /api/v1/consultation-runs/:id/replay/counterfactual
// with {"configuration_id": "..."}. Read-only.
func (h *ConsultationHandler) ReplayRunCounterfactual(c *gin.Context) {
	if h.replayService == nil {
		respondError(c, http.StatusServiceUnavailable, "REPLAY_UNAVAILABLE", "replay service not configured")
		return
	}
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	runID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid run id")
		return
	}
	var req struct {
		ConfigurationID string `json:"configuration_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ConfigurationID == "" {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "configuration_id is required")
		return
	}
	decision, err := h.replayService.CounterfactualReplay(c.Request.Context(), uid, runID, req.ConfigurationID)
	if err != nil {
		switch {
		case err == service.ErrConsultationReplayUnavailable:
			respondError(c, http.StatusUnprocessableEntity, "REPLAY_UNAVAILABLE", "run predates North-Star provenance")
		default:
			respondError(c, http.StatusInternalServerError, "REPLAY_FAILED", err.Error())
		}
		return
	}
	c.JSON(http.StatusOK, decision)
}
