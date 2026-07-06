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
	runtime             *consultationruntime.Runtime
}

// NewConsultationHandler creates a new ConsultationHandler.
func NewConsultationHandler(
	consultationService *service.ConsultationService,
	interactionService *service.AgentInteractionService,
	runtime *consultationruntime.Runtime,
) *ConsultationHandler {
	return &ConsultationHandler{
		consultationService: consultationService,
		interactionService:  interactionService,
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

// UpdateHealthFeatures handles PUT /api/v1/consultations/:id/health-features
func (h *ConsultationHandler) UpdateHealthFeatures(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid consultation id")
		return
	}

	var req dto.UpdateHealthFeaturesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.consultationService.UpdateHealthFeatures(c.Request.Context(), conversationID, uid, req.HealthFeatures); err != nil {
		log.Printf("failed to update health features for consultation %s: %v", conversationID, err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update health features")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "health features updated"})
}

// ConfirmDiagnosis handles PUT /api/v1/consultations/:id/confirm
func (h *ConsultationHandler) ConfirmDiagnosis(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid consultation id")
		return
	}

	var req dto.ConfirmDiagnosisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.consultationService.UpdateDiagnosis(c.Request.Context(), conversationID, uid, req.Diagnosis); err != nil {
		log.Printf("failed to update diagnosis for consultation %s: %v", conversationID, err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to confirm diagnosis")
		return
	}

	if err := h.consultationService.UpdatePhase(c.Request.Context(), conversationID, uid, "diagnosis_confirmed"); err != nil {
		log.Printf("failed to update phase for consultation %s: %v", conversationID, err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update consultation phase")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "diagnosis confirmed"})
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
