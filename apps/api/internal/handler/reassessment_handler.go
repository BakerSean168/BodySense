package handler

import (
	"net/http"

	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ReassessmentHandler converts structured training feedback into Outcomes and,
// when deterministic policy recommends review, a new Treatment proposal.
type ReassessmentHandler struct {
	trainingService *service.TrainingService
}

func NewReassessmentHandler(trainingService *service.TrainingService) *ReassessmentHandler {
	return &ReassessmentHandler{trainingService: trainingService}
}

func (h *ReassessmentHandler) SubmitReassessment(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan id"})
		return
	}
	var req struct {
		Feedback struct {
			SymptomChanges  string     `json:"symptom_changes"`
			TrainingFeeling string     `json:"training_feeling"`
			Difficulties    string     `json:"difficulties"`
			BodyRegion      string     `json:"body_region"`
			ConcernKey      string     `json:"concern_key"`
			Trend           string     `json:"trend"`
			FactID          *uuid.UUID `json:"fact_id"`
		} `json:"feedback" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.trainingService.Reassess(c.Request.Context(), planID, uid, service.TrainingFeedbackInput{
		SymptomChanges: req.Feedback.SymptomChanges, TrainingFeeling: req.Feedback.TrainingFeeling,
		Difficulties: req.Feedback.Difficulties, BodyRegion: req.Feedback.BodyRegion,
		ConcernKey: req.Feedback.ConcernKey, Trend: req.Feedback.Trend, FactID: req.Feedback.FactID,
		Notes: req.Feedback.TrainingFeeling,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
