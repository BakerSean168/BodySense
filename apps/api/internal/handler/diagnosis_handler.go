package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DiagnosisHandler handles diagnosis and treatment HTTP requests.
type DiagnosisHandler struct {
	consultationService *service.ConsultationService
	profileService      *service.ProfileService
	aiClient            *service.AIClient
	aiServiceURL        string
}

// NewDiagnosisHandler creates a new DiagnosisHandler.
func NewDiagnosisHandler(
	consultationService *service.ConsultationService,
	profileService *service.ProfileService,
	aiClient *service.AIClient,
) *DiagnosisHandler {
	return &DiagnosisHandler{
		consultationService: consultationService,
		profileService:      profileService,
		aiClient:            aiClient,
		aiServiceURL:        aiClient.BaseURL(),
	}
}

// AnalyzeDiagnosis handles POST /api/v1/consultations/:id/diagnosis
func (h *DiagnosisHandler) AnalyzeDiagnosis(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid consultation id")
		return
	}

	// Verify session exists and belongs to user
	session, err := h.consultationService.GetConsultation(c.Request.Context(), conversationID, uid)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get consultation")
		return
	}
	if session == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "consultation not found")
		return
	}

	// Get user profile
	profile, err := h.profileService.GetProfile(c.Request.Context(), uid)
	profileJSON := json.RawMessage("{}")
	if err == nil && profile != nil {
		if pj, marshalErr := json.Marshal(profile); marshalErr == nil {
			profileJSON = pj
		}
	}

	// Parse extracted info from session
	extractedInfoJSON := json.RawMessage("[]")
	if len(session.ExtractedInfo) > 0 {
		extractedInfoJSON = json.RawMessage(session.ExtractedInfo)
	}

	// Build RAG context from knowledge search
	diagReq := service.DiagnosisRequest{
		ExtractedInfo: extractedInfoJSON,
		Profile:       profileJSON,
		UseCase:       "llm.json",
	}

	// Knowledge search for RAG context
	var extractedInfoList []any
	_ = json.Unmarshal(extractedInfoJSON, &extractedInfoList)
	if query := buildDiagnosisKnowledgeQuery(extractedInfoList); query != "" {
		ragResults, searchErr := searchKnowledge(c.Request.Context(), h.aiServiceURL, query)
		if searchErr == nil && len(ragResults) > 0 {
			diagReq.RAGContext = buildKnowledgeContext(ragResults)
			ragResultsJSON, _ := json.Marshal(ragResults)
			diagReq.RAGResults = ragResultsJSON
		}
	}

	// Call AI service
	result, err := h.aiClient.AnalyzeDiagnosis(c.Request.Context(), diagReq)
	if err != nil {
		log.Printf("AI diagnosis analysis failed for consultation %s: %v", conversationID, err)
		respondError(c, http.StatusBadGateway, "AI_SERVICE_ERROR", "failed to analyze diagnosis")
		return
	}

	// Parse and persist the result
	var diagnosisResult map[string]any
	if json.Unmarshal(result, &diagnosisResult) == nil {
		if err := h.consultationService.UpdateDiagnosis(c.Request.Context(), conversationID, uid, diagnosisResult); err != nil {
			log.Printf("failed to save diagnosis for consultation %s: %v", conversationID, err)
		}
		if err := h.consultationService.UpdatePhase(c.Request.Context(), conversationID, uid, "analysis_ready"); err != nil {
			log.Printf("failed to update phase for consultation %s: %v", conversationID, err)
		}
	}

	c.Data(http.StatusOK, "application/json", result)
}

// GenerateTreatment handles POST /api/v1/consultations/:id/treatment
func (h *DiagnosisHandler) GenerateTreatment(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid consultation id")
		return
	}

	// Verify session exists and belongs to user
	session, err := h.consultationService.GetConsultation(c.Request.Context(), conversationID, uid)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get consultation")
		return
	}
	if session == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "consultation not found")
		return
	}

	// Validate that the current phase allows treatment generation
	if session.Phase != "diagnosis_confirmed" {
		respondError(c, http.StatusConflict, "INVALID_PHASE", "diagnosis must be confirmed before generating treatment plan")
		return
	}

	// Parse request body (confirmed diagnosis)
	var reqBody struct {
		ConfirmedDiagnosis map[string]any `json:"confirmedDiagnosis" binding:"required"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Get user profile
	profile, err := h.profileService.GetProfile(c.Request.Context(), uid)
	profileJSON := json.RawMessage("{}")
	if err == nil && profile != nil {
		if pj, marshalErr := json.Marshal(profile); marshalErr == nil {
			profileJSON = pj
		}
	}

	// Parse extracted info from session
	extractedInfoJSON := json.RawMessage("[]")
	if len(session.ExtractedInfo) > 0 {
		extractedInfoJSON = json.RawMessage(session.ExtractedInfo)
	}

	confirmedDiagnosisJSON, err := json.Marshal(reqBody.ConfirmedDiagnosis)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid confirmed diagnosis")
		return
	}

	treatmentReq := service.TreatmentRequest{
		ConfirmedDiagnosis: confirmedDiagnosisJSON,
		ExtractedInfo:      extractedInfoJSON,
		Profile:            profileJSON,
		UseCase:            "llm.json",
	}

	// Knowledge search for RAG context
	var extractedInfoList []any
	_ = json.Unmarshal(extractedInfoJSON, &extractedInfoList)
	if query := buildTreatmentKnowledgeQuery(reqBody.ConfirmedDiagnosis, extractedInfoList); query != "" {
		ragResults, searchErr := searchKnowledge(c.Request.Context(), h.aiServiceURL, query)
		if searchErr == nil && len(ragResults) > 0 {
			treatmentReq.RAGContext = buildKnowledgeContext(ragResults)
			ragResultsJSON, _ := json.Marshal(ragResults)
			treatmentReq.RAGResults = ragResultsJSON
		}
	}

	// Call AI service
	result, err := h.aiClient.GenerateTreatment(c.Request.Context(), treatmentReq)
	if err != nil {
		log.Printf("AI treatment generation failed for consultation %s: %v", conversationID, err)
		respondError(c, http.StatusBadGateway, "AI_SERVICE_ERROR", "failed to generate treatment")
		return
	}

	// Parse and persist the result
	var treatmentResult map[string]any
	if json.Unmarshal(result, &treatmentResult) == nil {
		var treatmentPlanToSave any
		if planObj, ok := treatmentResult["treatment_plan"].(map[string]any); ok {
			treatmentPlanToSave = planObj
		} else {
			treatmentPlanToSave = treatmentResult
		}
		if err := h.consultationService.UpdateTreatmentPlan(c.Request.Context(), conversationID, uid, treatmentPlanToSave); err != nil {
			log.Printf("failed to save treatment plan for consultation %s: %v", conversationID, err)
		}
		if err := h.consultationService.UpdatePhase(c.Request.Context(), conversationID, uid, "plan_ready"); err != nil {
			log.Printf("failed to update phase for consultation %s: %v", conversationID, err)
		}
	}

	c.Data(http.StatusOK, "application/json", result)
}
