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

// DiagnosisHandler handles diagnosis and treatment HTTP requests.
type DiagnosisHandler struct {
	consultationService *service.ConsultationService
	profileService      *service.ProfileService
	aiServiceURL        string
}

// NewDiagnosisHandler creates a new DiagnosisHandler.
func NewDiagnosisHandler(
	consultationService *service.ConsultationService,
	profileService *service.ProfileService,
) *DiagnosisHandler {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8000"
	}
	return &DiagnosisHandler{
		consultationService: consultationService,
		profileService:      profileService,
		aiServiceURL:        aiServiceURL,
	}
}

// AnalyzeDiagnosis handles POST /api/v1/consultation/:id/diagnosis
func (h *DiagnosisHandler) AnalyzeDiagnosis(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session id"})
		return
	}

	// Verify session exists and belongs to user
	session, err := h.consultationService.GetSession(c.Request.Context(), sessionID, uid)
	if err != nil || session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	// Get user profile
	profile, err := h.profileService.GetProfile(c.Request.Context(), uid)
	profileMap := map[string]any{}
	if err == nil && profile != nil {
		profileJSON, _ := json.Marshal(profile)
		_ = json.Unmarshal(profileJSON, &profileMap)
	}

	// Parse extracted info from session
	var extractedInfo []any
	if len(session.ExtractedInfo) > 0 {
		_ = json.Unmarshal(session.ExtractedInfo, &extractedInfo)
	}

	// Build request to AI service
	aiReq := map[string]any{
		"extracted_info":       extractedInfo,
		"profile":              profileMap,
		"conversation_summary": "",
	}

	aiReqBody, _ := json.Marshal(aiReq)

	resp, err := http.Post(
		h.aiServiceURL+"/api/diagnosis/analyze",
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

// GenerateTreatment handles POST /api/v1/consultation/:id/treatment
func (h *DiagnosisHandler) GenerateTreatment(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session id"})
		return
	}

	// Verify session exists and belongs to user
	session, err := h.consultationService.GetSession(c.Request.Context(), sessionID, uid)
	if err != nil || session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	// Parse request body (confirmed diagnosis)
	var reqBody struct {
		ConfirmedDiagnosis map[string]any `json:"confirmed_diagnosis" binding:"required"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user profile
	profile, err := h.profileService.GetProfile(c.Request.Context(), uid)
	profileMap := map[string]any{}
	if err == nil && profile != nil {
		profileJSON, _ := json.Marshal(profile)
		_ = json.Unmarshal(profileJSON, &profileMap)
	}

	// Parse extracted info from session
	var extractedInfo []any
	if len(session.ExtractedInfo) > 0 {
		_ = json.Unmarshal(session.ExtractedInfo, &extractedInfo)
	}

	// Build request to AI service
	aiReq := map[string]any{
		"confirmed_diagnosis": reqBody.ConfirmedDiagnosis,
		"extracted_info":      extractedInfo,
		"profile":             profileMap,
	}

	aiReqBody, _ := json.Marshal(aiReq)

	resp, err := http.Post(
		h.aiServiceURL+"/api/diagnosis/treatment",
		"application/json",
		bytes.NewBuffer(aiReqBody),
	)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to connect to AI service"})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// If successful, save treatment plan to session
	if resp.StatusCode == http.StatusOK {
		var treatmentResult map[string]any
		if json.Unmarshal(respBody, &treatmentResult) == nil {
			_ = h.consultationService.UpdateTreatmentPlan(c.Request.Context(), sessionID, treatmentResult)
		}
	}

	c.Data(resp.StatusCode, "application/json", respBody)
}
