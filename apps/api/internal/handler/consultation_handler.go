package handler

import (
	"log"
	"net/http"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ConsultationHandler handles consultation HTTP requests.
type ConsultationHandler struct {
	consultationService *service.ConsultationService
}

// NewConsultationHandler creates a new ConsultationHandler.
func NewConsultationHandler(
	consultationService *service.ConsultationService,
) *ConsultationHandler {
	return &ConsultationHandler{
		consultationService: consultationService,
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
