package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ConsultationHandler handles consultation HTTP requests.
type ConsultationHandler struct {
	consultationService   *service.ConsultationService
	interactionService    *service.AgentInteractionService
	runService            *service.RunService
}

// NewConsultationHandler creates a new ConsultationHandler.
func NewConsultationHandler(
	consultationService *service.ConsultationService,
	interactionService *service.AgentInteractionService,
	runService *service.RunService,
) *ConsultationHandler {
	return &ConsultationHandler{
		consultationService:   consultationService,
		interactionService:    interactionService,
		runService:            runService,
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

	c.JSON(http.StatusOK, session)
}

// UpdateExtractedInfo handles PUT /api/v1/consultations/:id/extracted-info
func (h *ConsultationHandler) UpdateExtractedInfo(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid consultation id")
		return
	}

	var req dto.UpdateExtractedInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.consultationService.UpdateExtractedInfo(c.Request.Context(), conversationID, uid, req.ExtractedInfo); err != nil {
		log.Printf("failed to update extracted info for consultation %s: %v", conversationID, err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update extracted info")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "extracted info updated"})
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

// ResumeInteraction handles POST /api/v1/consultation/:conversationId/interactions/:interactionId/resume
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

	conversationID, err := uuid.Parse(c.Param("conversationId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid conversation id")
		return
	}

	// Verify conversation ownership
	session, err := h.consultationService.GetConsultation(c.Request.Context(), conversationID, uid)
	if err != nil || session == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "consultation not found")
		return
	}

	var req struct {
		Answer datatypes.JSON `json:"answer" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Get the interaction first to find the run ID (before marking as answered)
	interaction, getErr := h.interactionService.GetInteractionByID(c.Request.Context(), interactionID)
	if getErr != nil || interaction == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "interaction not found")
		return
	}

	if err := h.interactionService.ResumeInteraction(c.Request.Context(), interactionID, req.Answer); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Do NOT transition run to "running" here — the frontend will send a new
	// chat message which creates a fresh run. This avoids a broken "running" run
	// with no AI work behind it.

	// Extract answer text for the frontend to include in the follow-up message
	var answerText string
	var rawAnswer map[string]any
	if jsonErr := json.Unmarshal(req.Answer, &rawAnswer); jsonErr == nil {
		if t, ok := rawAnswer["text"].(string); ok {
			answerText = t
		} else if selected, ok := rawAnswer["selected"].([]any); ok {
			parts := make([]string, 0, len(selected))
			for _, s := range selected {
				if str, ok := s.(string); ok {
					parts = append(parts, str)
				}
			}
			answerText = strings.Join(parts, ", ")
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"action":      "send_message",
		"answer_text": answerText,
		"message":     "interaction answered — send a new chat message to continue",
	})
}
