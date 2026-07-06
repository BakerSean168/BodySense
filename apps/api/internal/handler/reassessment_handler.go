package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ReassessmentHandler handles reassessment HTTP requests.
type ReassessmentHandler struct {
	trainingService *service.TrainingService
	aiServiceURL    string
}

func NewReassessmentHandler(trainingService *service.TrainingService) *ReassessmentHandler {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8100"
	}
	return &ReassessmentHandler{
		trainingService: trainingService,
		aiServiceURL:    aiServiceURL,
	}
}

// SubmitReassessment handles POST /api/v1/training/:id/reassess
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

	// Verify plan belongs to user
	plan, err := h.trainingService.GetPlan(c.Request.Context(), planID, uid)
	if err != nil || plan == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plan not found"})
		return
	}

	// Parse feedback
	var reqBody struct {
		Feedback map[string]any `json:"feedback" binding:"required"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get training logs
	logs, _ := h.trainingService.GetLogsByPlanID(c.Request.Context(), planID)
	logsJSON, _ := json.Marshal(logs)

	// Get plan as map
	planJSON, _ := json.Marshal(plan)
	var planMap map[string]any
	_ = json.Unmarshal(planJSON, &planMap)

	// Call AI service
	aiReq := map[string]any{
		"feedback":      reqBody.Feedback,
		"training_logs": json.RawMessage(logsJSON),
		"current_plan":  planMap,
	}
	aiReqBody, _ := json.Marshal(aiReq)

	resp, err := http.Post(
		h.aiServiceURL+"/api/reassessment/analyze",
		"application/json",
		bytes.NewBuffer(aiReqBody),
	)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to connect to AI service"})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", respBody)
}
