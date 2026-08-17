package handler

import (
	"net/http"

	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TrainingHandler handles training HTTP requests.
type TrainingHandler struct {
	trainingService *service.TrainingService
}

func NewTrainingHandler(trainingService *service.TrainingService) *TrainingHandler {
	return &TrainingHandler{trainingService: trainingService}
}

// GetPlan handles GET /api/v1/training/:id
func (h *TrainingHandler) GetPlan(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan id"})
		return
	}

	plan, err := h.trainingService.GetPlan(c.Request.Context(), planID, uid)
	if err != nil || plan == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plan not found"})
		return
	}

	c.JSON(http.StatusOK, plan)
}

// ListPlans handles GET /api/v1/training
func (h *TrainingHandler) ListPlans(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	plans, err := h.trainingService.ListPlans(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list plans"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"plans": plans})
}

// GetTodayTask handles GET /api/v1/training/:id/today
func (h *TrainingHandler) GetTodayTask(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan id"})
		return
	}

	task, err := h.trainingService.GetTodayTask(c.Request.Context(), planID, uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// CheckIn handles POST /api/v1/training/:id/checkin
func (h *TrainingHandler) CheckIn(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan id"})
		return
	}

	if err := h.trainingService.CheckIn(c.Request.Context(), planID, uid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "checked in"})
}

// UpdateLog handles PUT /api/v1/training/:id/log
func (h *TrainingHandler) UpdateLog(c *gin.Context) {
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
		Notes           string     `json:"notes"`
		Exercises       any        `json:"exercises"`
		SymptomChanges  string     `json:"symptom_changes"`
		TrainingFeeling string     `json:"training_feeling"`
		Difficulties    string     `json:"difficulties"`
		BodyRegion      string     `json:"body_region"`
		ConcernKey      string     `json:"concern_key"`
		Trend           string     `json:"trend"`
		FactID          *uuid.UUID `json:"fact_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.trainingService.UpdateLogWithFeedback(c.Request.Context(), planID, uid, service.TrainingFeedbackInput{
		Notes: req.Notes, Exercises: req.Exercises, SymptomChanges: req.SymptomChanges,
		TrainingFeeling: req.TrainingFeeling, Difficulties: req.Difficulties,
		BodyRegion: req.BodyRegion, ConcernKey: req.ConcernKey, Trend: req.Trend, FactID: req.FactID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := gin.H{
		"message":      "log updated",
		"has_proposal": false,
		"result":       result,
	}
	if proposal, exists := result["proposal"]; exists && proposal != nil {
		response["has_proposal"] = true
		response["proposal"] = proposal
	}
	c.JSON(http.StatusOK, response)
}

// GetProgress handles GET /api/v1/training/:id/progress
func (h *TrainingHandler) GetProgress(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan id"})
		return
	}

	progress, err := h.trainingService.GetProgress(c.Request.Context(), planID, uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, progress)
}
