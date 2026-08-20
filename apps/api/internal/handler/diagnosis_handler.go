package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// DiagnosisHandler exposes the single BodyState-backed Diagnosis HTTP boundary.
type DiagnosisHandler struct {
	consultationService       *service.ConsultationService
	profileService            *service.ProfileService
	aiClient                  *service.AIClient
	outputReviewService       *service.OutputReviewService
	bodyStateService          *service.BodyStateService
	diagnosisAnalysisService  *service.DiagnosisAnalysisService
	diagnosisFreshnessService *service.DiagnosisFreshnessService
	agentDeploymentPolicy     *service.AgentDeploymentPolicy
	diagnosisReplayService    *service.DiagnosisReplayService
	diagnosisRolloutService   *service.DiagnosisRolloutService
}

func NewDiagnosisHandler(
	consultationService *service.ConsultationService,
	profileService *service.ProfileService,
	aiClient *service.AIClient,
	outputReviewService *service.OutputReviewService,
	bodyStateService *service.BodyStateService,
	diagnosisAnalysisService *service.DiagnosisAnalysisService,
	diagnosisFreshnessService *service.DiagnosisFreshnessService,
	agentDeploymentPolicy *service.AgentDeploymentPolicy,
	diagnosisReplayService *service.DiagnosisReplayService,
	diagnosisRolloutService *service.DiagnosisRolloutService,
) *DiagnosisHandler {
	return &DiagnosisHandler{
		consultationService:       consultationService,
		profileService:            profileService,
		aiClient:                  aiClient,
		outputReviewService:       outputReviewService,
		bodyStateService:          bodyStateService,
		diagnosisAnalysisService:  diagnosisAnalysisService,
		diagnosisFreshnessService: diagnosisFreshnessService,
		agentDeploymentPolicy:     agentDeploymentPolicy,
		diagnosisReplayService:    diagnosisReplayService,
		diagnosisRolloutService:   diagnosisRolloutService,
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

	// Diagnosis has one durable path: exact BodyState revision -> immutable
	// DiagnosisAnalysis. Refuse to run if that domain boundary is not wired rather
	// than falling back to consultation_sessions.diagnosis as a second truth.
	if h.bodyStateService == nil || h.diagnosisAnalysisService == nil {
		respondError(c, http.StatusServiceUnavailable, "DIAGNOSIS_DOMAIN_UNAVAILABLE", "BodyState-backed diagnosis services are not configured")
		return
	}

	profile, err := h.profileService.GetProfile(c.Request.Context(), uid)
	profileJSON := json.RawMessage("{}")
	if err == nil && profile != nil {
		if pj, marshalErr := json.Marshal(profile); marshalErr == nil {
			profileJSON = pj
		}
	}

	h.analyzeDiagnosisFromBodyState(c, uid, conversationID, profileJSON)
}

// analyzeDiagnosisFromBodyState is the new production boundary. It deliberately
// avoids broad Go-side RAG on every diagnosis run: Diagnosis primarily synthesizes
// durable BodyState + temporal history, while Python may later do targeted retrieval
// for explicit evidence gaps.
func (h *DiagnosisHandler) analyzeDiagnosisFromBodyState(
	c *gin.Context,
	uid uuid.UUID,
	conversationID uuid.UUID,
	profileJSON json.RawMessage,
) {
	snapshot, err := h.bodyStateService.GetSnapshot(c.Request.Context(), uid, 50)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load body state")
		return
	}
	if snapshot.CurrentRevision == 0 || (len(snapshot.Facts) == 0 && len(snapshot.Observations) == 0) {
		respondError(c, http.StatusConflict, "BODY_STATE_NOT_READY", "body state does not yet contain enough structured information for diagnosis")
		return
	}

	if h.agentDeploymentPolicy == nil {
		respondError(c, http.StatusServiceUnavailable, "AGENT_DEPLOYMENT_POLICY_UNAVAILABLE", "Diagnosis Agent deployment policy is not configured")
		return
	}
	routeSelection := h.agentDeploymentPolicy.SelectDiagnosisRoute(uid.String())
	configurationID := routeSelection.ServedConfigurationID
	decisionPolicyRevision := routeSelection.ServedDecisionPolicyRevision
	bodyStateJSON, err := json.Marshal(snapshot)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to encode body state")
		return
	}
	historyJSON, _ := json.Marshal(snapshot.RecentRevisions)
	replayInput, err := service.EncodeDiagnosisReplayInput(
		snapshot.CurrentRevision, bodyStateJSON, historyJSON, profileJSON,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to freeze Diagnosis replay input")
		return
	}

	// Safety is a Go-owned business gate, not merely an LLM suggestion. The
	// Phase-6 policy path evaluates durable safety state before any model call;
	// pre-envelope configurations retain their characterized legacy behavior for
	// historical replay until final promotion/retirement.
	if decisionPolicyRevision == service.DiagnosisDecisionPolicyV1 {
		probe := map[string]any{
			"status":     "completed",
			"candidates": []any{map[string]any{"name": "preflight", "confidence": "n/a"}},
			"governance": map[string]any{"verdict": "accepted"},
		}
		decision := service.EvaluateDiagnosisDecision(
			decisionPolicyRevision, snapshot.SafetyState, probe,
		)
		if decision.Outcome == service.DiagnosisBlock {
			blocked := service.ApplyDiagnosisDecision(map[string]any{
				"status":                 "safety_blocked",
				"scope":                  "full_body",
				"summary":                "当前身体状态包含需要优先处理的安全信号，暂不生成普通可能性候选。",
				"candidates":             []any{},
				"cross_concern_patterns": []any{},
				"information_gaps":       []any{},
				"safety_summary":         json.RawMessage(snapshot.SafetyState),
				"citations":              []any{},
				"agent_configuration": map[string]any{
					"id": configurationID, "role": "diagnosis",
					"decision_policy_revision": decisionPolicyRevision,
				},
				"execution_provenance": map[string]any{
					"status": "bypassed", "runtime": "go",
					"reason": "go_pre_agent_safety_gate",
				},
				"rollout_provenance": routeSelection,
				"governance": map[string]any{
					"kind": "diagnosis", "verdict": "rejected",
					"reasons": []string{"active_body_state_safety_concern"}, "issues": []any{},
				},
			}, decision)
			blockedRaw, _ := json.Marshal(blocked)
			analysis, persistErr := h.diagnosisAnalysisService.PersistAIResultWithReplayInput(
				c.Request.Context(), uid, snapshot.CurrentRevision, blockedRaw, replayInput,
			)
			if persistErr != nil {
				log.Printf("failed to persist decision-blocked diagnosis for user %s: %v", uid, persistErr)
				respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to persist diagnosis safety state")
				return
			}
			h.observeDiagnosisRollout(c.Request.Context(), uid, analysis, routeSelection)
			h.respondWithDiagnosisAnalysis(c, uid, analysis)
			return
		}
	} else {
		var safetyState struct {
			HasRedFlags bool   `json:"has_red_flags"`
			Status      string `json:"status"`
		}
		_ = json.Unmarshal(snapshot.SafetyState, &safetyState)
		if safetyState.HasRedFlags && safetyState.Status == "requires_review" {
			blockedRaw, _ := json.Marshal(map[string]any{
				"status": "safety_blocked", "scope": "full_body",
				"summary":    "当前身体状态包含需要优先处理的安全信号，暂不生成普通可能性候选。",
				"candidates": []any{}, "cross_concern_patterns": []any{},
				"information_gaps": []any{}, "safety_summary": json.RawMessage(snapshot.SafetyState),
				"citations": []any{},
				"agent_configuration": map[string]any{
					"id": configurationID, "role": "diagnosis",
					"decision_policy_revision": decisionPolicyRevision,
				},
				"execution_provenance": map[string]any{
					"status": "bypassed", "runtime": "go",
					"reason": "go_pre_agent_safety_gate",
				},
				"rollout_provenance": routeSelection,
				"governance": map[string]any{
					"kind": "diagnosis", "verdict": "rejected",
					"reasons": []string{"active_body_state_safety_concern"}, "issues": []any{},
				},
			})
			analysis, persistErr := h.diagnosisAnalysisService.PersistAIResultWithReplayInput(
				c.Request.Context(), uid, snapshot.CurrentRevision, blockedRaw, replayInput,
			)
			if persistErr != nil {
				log.Printf("failed to persist safety-blocked diagnosis for user %s: %v", uid, persistErr)
				respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to persist diagnosis safety state")
				return
			}
			h.observeDiagnosisRollout(c.Request.Context(), uid, analysis, routeSelection)
			h.respondWithDiagnosisAnalysis(c, uid, analysis)
			return
		}
	}

	result, err := h.aiClient.AnalyzeDiagnosis(c.Request.Context(), service.DiagnosisRequest{
		UserID:            uid.String(),
		ConfigurationID:   configurationID,
		BodyStateRevision: snapshot.CurrentRevision,
		BodyState:         bodyStateJSON,
		RelevantHistory:   historyJSON,
		Profile:           profileJSON,
	})
	if err != nil {
		log.Printf("AI diagnosis analysis failed for BodyState R%d user %s: %v", snapshot.CurrentRevision, uid, err)
		respondError(c, http.StatusBadGateway, "AI_SERVICE_ERROR", "failed to analyze diagnosis")
		return
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		respondError(c, http.StatusBadGateway, "INVALID_AI_RESPONSE", "diagnosis response was not valid JSON")
		return
	}
	if !diagnosisConfigurationMatches(parsed, configurationID) {
		respondError(c, http.StatusBadGateway, "INVALID_AGENT_CONFIGURATION", "diagnosis response did not match the selected Agent configuration")
		return
	}
	parsed["rollout_provenance"] = routeSelection
	result, err = json.Marshal(parsed)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to encode Diagnosis rollout provenance")
		return
	}
	// Diagnosis may discover a safety signal that was not previously committed by
	// Consultation. Promote the detector result back into the user-level BodyState
	// before persisting the analysis so future runs remain safety-constrained.
	if redFlags, ok := parsed["red_flags"].(map[string]any); ok {
		safetyPayload, marshalErr := json.Marshal(redFlags)
		if marshalErr != nil {
			respondError(c, http.StatusBadGateway, "INVALID_AI_RESPONSE", "diagnosis safety response was not valid")
			return
		}
		if err := h.bodyStateService.RecordSafetyEvent(c.Request.Context(), uid, safetyPayload); err != nil {
			log.Printf("failed to promote Diagnosis safety signal into BodyState for user %s: %v", uid, err)
			respondError(c, http.StatusInternalServerError, "SAFETY_STATE_PERSISTENCE_FAILED", "failed to persist diagnosis safety state")
			return
		}
	}
	h.recordGovernedOutput(c, "diagnosis", &uid, &conversationID, nil, parsed, result)
	if decisionPolicyRevision == service.DiagnosisDecisionPolicyV1 {
		decision := service.EvaluateDiagnosisDecision(
			decisionPolicyRevision, snapshot.SafetyState, parsed,
		)
		parsed = service.ApplyDiagnosisDecision(parsed, decision)
		result, err = json.Marshal(parsed)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to encode Diagnosis decision")
			return
		}
	} else if gov, ok := parsed["governance"].(map[string]any); ok {
		if verdict, _ := gov["verdict"].(string); verdict == "rejected" {
			h.observeDiagnosisRolloutFrozen(
				c.Request.Context(), uid, replayInput, result, routeSelection,
			)
			c.Data(http.StatusOK, "application/json", result)
			return
		}
	}

	// Persist traceable citations as Evidence before freezing candidate references.
	// This keeps literature/tool material separate from accepted user Facts.
	if normalized, evidenceErr := h.persistDiagnosisEvidence(c.Request.Context(), uid, parsed); evidenceErr == nil {
		result = normalized
	} else {
		log.Printf("failed to persist Diagnosis evidence for user %s: %v", uid, evidenceErr)
	}

	analysis, err := h.diagnosisAnalysisService.PersistAIResultWithReplayInput(
		c.Request.Context(), uid, snapshot.CurrentRevision, result, replayInput,
	)
	if err != nil {
		log.Printf("failed to persist diagnosis analysis for user %s BodyState R%d: %v", uid, snapshot.CurrentRevision, err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to persist diagnosis analysis")
		return
	}
	h.observeDiagnosisRollout(c.Request.Context(), uid, analysis, routeSelection)
	if hypothesisErr := h.commitDiagnosisHypotheses(c.Request.Context(), uid, analysis); hypothesisErr != nil {
		log.Printf("failed to project Diagnosis hypotheses for analysis %s: %v", analysis.ID, hypothesisErr)
	}
	publicPayload := h.diagnosisAnalysisService.PublicPayload(analysis)
	if h.diagnosisFreshnessService != nil {
		if freshness, freshErr := h.diagnosisFreshnessService.GetOrEvaluate(c.Request.Context(), uid, analysis); freshErr == nil {
			publicPayload["freshness"] = freshness
		}
	}

	if analysis.Status == "completed" || analysis.Status == "partial" {
		if err := h.consultationService.UpdatePhase(c.Request.Context(), conversationID, uid, "analysis_ready"); err != nil {
			log.Printf("failed to update consultation analysis phase for consultation %s: %v", conversationID, err)
		}
	}

	c.JSON(http.StatusOK, publicPayload)
}

// ListDiagnosisHistory handles GET /api/v1/diagnosis-analyses.
// Diagnosis history is now the user's analytical timeline; no separate
// MedicalRecord aggregate is required to preserve historical reasoning.

func (h *DiagnosisHandler) observeDiagnosisRolloutFrozen(
	ctx context.Context,
	uid uuid.UUID,
	replayInput json.RawMessage,
	baseline json.RawMessage,
	route service.DiagnosisRouteSelection,
) {
	if route.ShadowConfigurationID == "" || h.diagnosisReplayService == nil || h.diagnosisRolloutService == nil {
		return
	}
	report, err := h.diagnosisReplayService.CounterfactualFrozen(
		ctx, uid, replayInput, baseline, route.ServedConfigurationID, route.ShadowConfigurationID,
	)
	if recordErr := h.diagnosisRolloutService.RecordComparison(
		ctx, route, uuid.Nil, report, err,
	); recordErr != nil {
		log.Printf("failed to persist transient Diagnosis rollout observation: %v", recordErr)
	}
	if err != nil {
		log.Printf("Diagnosis %s transient comparison failed: %v", route.Stage, err)
	}
}

func (h *DiagnosisHandler) observeDiagnosisRollout(
	ctx context.Context,
	uid uuid.UUID,
	analysis *model.DiagnosisAnalysisRecord,
	route service.DiagnosisRouteSelection,
) {
	if route.ShadowConfigurationID == "" || h.diagnosisReplayService == nil || h.diagnosisRolloutService == nil {
		return
	}
	report, err := h.diagnosisReplayService.CounterfactualReplay(
		ctx, uid, analysis.ID, route.ShadowConfigurationID,
	)
	if recordErr := h.diagnosisRolloutService.RecordComparison(
		ctx, route, analysis.ID, report, err,
	); recordErr != nil {
		log.Printf("failed to persist Diagnosis rollout observation for analysis %s: %v", analysis.ID, recordErr)
	}
	if err != nil {
		log.Printf("Diagnosis %s comparison failed for analysis %s: %v", route.Stage, analysis.ID, err)
	}
}

func (h *DiagnosisHandler) respondWithDiagnosisAnalysis(
	c *gin.Context,
	uid uuid.UUID,
	analysis *model.DiagnosisAnalysisRecord,
) {
	payload := h.diagnosisAnalysisService.PublicPayload(analysis)
	if h.diagnosisFreshnessService != nil {
		if freshness, err := h.diagnosisFreshnessService.GetOrEvaluate(c.Request.Context(), uid, analysis); err == nil {
			payload["freshness"] = freshness
		}
	}
	c.JSON(http.StatusOK, payload)
}

func diagnosisConfigurationMatches(payload map[string]any, expectedID string) bool {
	configuration, ok := payload["agent_configuration"].(map[string]any)
	if !ok {
		return false
	}
	id, idOK := configuration["id"].(string)
	role, roleOK := configuration["role"].(string)
	return idOK && roleOK && id == expectedID && role == "diagnosis"
}

func (h *DiagnosisHandler) ListDiagnosisHistory(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	if h.diagnosisAnalysisService == nil {
		respondError(c, http.StatusServiceUnavailable, "DIAGNOSIS_HISTORY_UNAVAILABLE", "diagnosis history is not configured")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	analyses, err := h.diagnosisAnalysisService.List(c.Request.Context(), uid, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load diagnosis history")
		return
	}
	items := make([]map[string]any, 0, len(analyses))
	freshnessByID := map[uuid.UUID]model.DiagnosisAnalysisFreshness{}
	if h.diagnosisFreshnessService != nil {
		if values, freshErr := h.diagnosisFreshnessService.PreviewMany(c.Request.Context(), uid, analyses); freshErr == nil {
			freshnessByID = values
		}
	}
	for i := range analyses {
		payload := h.diagnosisAnalysisService.PublicPayload(&analyses[i])
		if freshness, exists := freshnessByID[analyses[i].ID]; exists {
			payload["freshness"] = freshness
		}
		items = append(items, payload)
	}
	c.JSON(http.StatusOK, gin.H{"analyses": items})
}

// GetDiagnosisAnalysis handles GET /api/v1/diagnosis-analyses/:analysisId.
func (h *DiagnosisHandler) GetDiagnosisAnalysis(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	analysisID, err := uuid.Parse(c.Param("analysisId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid analysis id")
		return
	}
	analysis, err := h.diagnosisAnalysisService.GetByID(c.Request.Context(), analysisID, uid)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load diagnosis analysis")
		return
	}
	if analysis == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "diagnosis analysis not found")
		return
	}
	assessments, err := h.diagnosisAnalysisService.ListAssessments(c.Request.Context(), uid, analysisID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load candidate assessments")
		return
	}
	payload := h.diagnosisAnalysisService.PublicPayload(analysis)
	payload["candidate_assessments"] = assessments
	if h.diagnosisFreshnessService != nil {
		if freshness, freshErr := h.diagnosisFreshnessService.Preview(c.Request.Context(), uid, analysis); freshErr == nil {
			payload["freshness"] = freshness
		}
	}
	c.JSON(http.StatusOK, payload)
}

// ReplayDiagnosisAnalysis handles POST /api/v1/diagnosis-analyses/:analysisId/replay.
// It is deliberately read-only: replay never mutates BodyState, Evidence,
// Hypothesis, consultation phase, or the source DiagnosisAnalysis.
func (h *DiagnosisHandler) ReplayDiagnosisAnalysis(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	if h.diagnosisReplayService == nil {
		respondError(c, http.StatusServiceUnavailable, "DIAGNOSIS_REPLAY_UNAVAILABLE", "diagnosis replay is not configured")
		return
	}
	analysisID, err := uuid.Parse(c.Param("analysisId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid analysis id")
		return
	}
	var req struct {
		Mode            string `json:"mode" binding:"required"`
		ConfigurationID string `json:"configuration_id,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var report *service.DiagnosisReplayReport
	switch strings.TrimSpace(req.Mode) {
	case "historical":
		report, err = h.diagnosisReplayService.HistoricalReplay(c.Request.Context(), uid, analysisID)
	case "counterfactual":
		if strings.TrimSpace(req.ConfigurationID) == "" {
			respondError(c, http.StatusBadRequest, "CONFIGURATION_REQUIRED", "counterfactual replay requires configuration_id")
			return
		}
		report, err = h.diagnosisReplayService.CounterfactualReplay(
			c.Request.Context(), uid, analysisID, req.ConfigurationID,
		)
	default:
		respondError(c, http.StatusBadRequest, "INVALID_REPLAY_MODE", "replay mode must be historical or counterfactual")
		return
	}
	if err != nil {
		h.respondDiagnosisReplayError(c, err)
		return
	}
	c.JSON(http.StatusOK, report)
}

// ExportDiagnosisRegressionCase exposes a source-controlled-dataset-shaped case
// envelope without mutating the repository. A developer can review/redact the
// frozen case before appending the nested `case` object to the regression split.
func (h *DiagnosisHandler) ExportDiagnosisRegressionCase(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	if h.diagnosisReplayService == nil {
		respondError(c, http.StatusServiceUnavailable, "DIAGNOSIS_REPLAY_UNAVAILABLE", "diagnosis replay is not configured")
		return
	}
	analysisID, err := uuid.Parse(c.Param("analysisId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid analysis id")
		return
	}
	exported, err := h.diagnosisReplayService.ExportRegressionCase(c.Request.Context(), uid, analysisID)
	if err != nil {
		h.respondDiagnosisReplayError(c, err)
		return
	}
	c.JSON(http.StatusOK, exported)
}

func (h *DiagnosisHandler) respondDiagnosisReplayError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDiagnosisReplayNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "diagnosis analysis not found")
	case errors.Is(err, service.ErrDiagnosisReplayUnavailable):
		respondError(c, http.StatusConflict, "DIAGNOSIS_REPLAY_INPUT_UNAVAILABLE", "this historical analysis predates frozen replay input")
	case strings.Contains(err.Error(), "unknown Diagnosis Agent configuration id"):
		respondError(c, http.StatusUnprocessableEntity, "UNKNOWN_AGENT_CONFIGURATION", err.Error())
	default:
		log.Printf("Diagnosis replay failed: %v", err)
		respondError(c, http.StatusBadGateway, "DIAGNOSIS_REPLAY_FAILED", "diagnosis replay failed")
	}
}

// AssessDiagnosisCandidates handles PUT /api/v1/diagnosis-analyses/:analysisId/assessment.
// Unmentioned candidates remain unassessed; they are never deleted simply because
// the user did not select them.
func (h *DiagnosisHandler) AssessDiagnosisCandidates(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	analysisID, err := uuid.Parse(c.Param("analysisId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid analysis id")
		return
	}
	var req struct {
		Candidates []struct {
			CandidateID string `json:"candidate_id" binding:"required"`
			State       string `json:"state" binding:"required"`
		} `json:"candidates" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	states := make(map[uuid.UUID]string, len(req.Candidates))
	for _, item := range req.Candidates {
		candidateID, err := uuid.Parse(item.CandidateID)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_CANDIDATE_ID", "invalid candidate id")
			return
		}
		states[candidateID] = item.State
	}
	assessments, err := h.diagnosisAnalysisService.AssessCandidates(c.Request.Context(), uid, analysisID, states)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_CANDIDATE_ASSESSMENT", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"analysis_id": analysisID, "candidate_assessments": assessments})
}

func (h *DiagnosisHandler) persistDiagnosisEvidence(
	ctx context.Context,
	userID uuid.UUID,
	parsed map[string]any,
) (json.RawMessage, error) {
	if h.bodyStateService == nil {
		return json.Marshal(parsed)
	}
	rawCitations, _ := parsed["citations"].([]any)
	if len(rawCitations) == 0 {
		return json.Marshal(parsed)
	}
	identityMap := map[string]string{}
	for index, rawCitation := range rawCitations {
		citation, ok := rawCitation.(map[string]any)
		if !ok {
			continue
		}
		sourceType := firstHandlerString(citation, "source_type", "type", "source")
		if sourceType == "" {
			sourceType = "knowledge"
		}
		sourceKey := firstHandlerString(citation, "source_key", "id", "source_id", "url", "uri")
		if sourceKey == "" {
			encoded, _ := json.Marshal(citation)
			digest := sha256.Sum256(encoded)
			sourceKey = "citation:" + hex.EncodeToString(digest[:12])
		}
		version := firstHandlerString(citation, "source_version", "version", "updated_at")
		metadata, _ := json.Marshal(citation)
		stored, err := h.bodyStateService.UpsertEvidence(ctx, userID, model.BodyStateEvidence{
			SourceType: sourceType, SourceKey: sourceKey, SourceVersion: version,
			Title:    firstHandlerString(citation, "title", "name"),
			Summary:  firstHandlerString(citation, "summary", "content"),
			Excerpt:  firstHandlerString(citation, "excerpt", "chunk", "text"),
			Metadata: datatypes.JSON(metadata),
		})
		if err != nil {
			return nil, err
		}
		storedID := stored.ID.String()
		citation["evidence_id"] = storedID
		citation["source_key"] = sourceKey
		rawCitations[index] = citation
		for _, key := range []string{sourceKey, firstHandlerString(citation, "id"), firstHandlerString(citation, "source_id"), firstHandlerString(citation, "url"), storedID} {
			if key != "" {
				identityMap[key] = storedID
			}
		}
	}
	parsed["citations"] = rawCitations
	if candidates, ok := parsed["candidates"].([]any); ok {
		for index, rawCandidate := range candidates {
			candidate, ok := rawCandidate.(map[string]any)
			if !ok {
				continue
			}
			if references, ok := candidate["supporting_evidence_ids"].([]any); ok {
				normalized := make([]string, 0, len(references))
				for _, rawReference := range references {
					reference := strings.TrimSpace(fmt.Sprint(rawReference))
					if mapped := identityMap[reference]; mapped != "" {
						normalized = append(normalized, mapped)
					} else if _, err := uuid.Parse(reference); err == nil {
						normalized = append(normalized, reference)
					}
				}
				candidate["supporting_evidence_ids"] = normalized
			}
			candidates[index] = candidate
		}
		parsed["candidates"] = candidates
	}
	return json.Marshal(parsed)
}

func (h *DiagnosisHandler) commitDiagnosisHypotheses(
	ctx context.Context,
	userID uuid.UUID,
	analysis *model.DiagnosisAnalysisRecord,
) error {
	if h.bodyStateService == nil || analysis == nil {
		return nil
	}
	for _, candidate := range analysis.Candidates {
		confidence := candidate.Confidence
		_, _, err := h.bodyStateService.AddDiagnosisHypothesis(ctx, userID, model.BodyStateHypothesis{
			ConcernKey:               candidate.ConcernKey,
			Statement:                candidate.Name,
			LifecycleState:           "active",
			Confidence:               &confidence,
			SupportingFactIDs:        candidate.BasisFactIDs,
			SupportingObservationIDs: candidate.BasisObservationIDs,
			SupportingEvidenceIDs:    candidate.SupportingEvidenceIDs,
			CounterevidenceIDs:       candidate.CounterevidenceIDs,
			SourceAnalysisID:         &analysis.ID,
			Provenance: datatypes.JSON(mustHandlerJSON(map[string]any{
				"source_type":       "diagnosis_analysis",
				"analysis_id":       analysis.ID,
				"candidate_id":      candidate.ID,
				"reasoning_summary": candidate.ReasoningSummary,
			})),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func firstHandlerString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fmt.Sprint(values[key])); value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func mustHandlerJSON(value any) []byte {
	encoded, _ := json.Marshal(value)
	return encoded
}

// recordGovernedOutput writes the P2 governance conclusion to ai_output_reviews.
// Non-blocking: OutputReviewService already logs persistence errors.
func (h *DiagnosisHandler) recordGovernedOutput(
	c *gin.Context,
	outputType string,
	userID, conversationID, jobID *uuid.UUID,
	parsed map[string]any,
	raw []byte,
) {
	if h.outputReviewService == nil {
		return
	}

	verdict := "unknown"
	issues := datatypes.JSON("[]")
	var validated datatypes.JSON

	if gov, ok := parsed["governance"].(map[string]any); ok {
		if v, ok := gov["verdict"].(string); ok && v != "" {
			verdict = v
		}
		if rawIssues, ok := gov["issues"]; ok {
			if b, err := json.Marshal(rawIssues); err == nil {
				issues = datatypes.JSON(b)
			}
		}
	}

	// Persist the deliverable surface only when it was allowed through the gate.
	if verdict == "accepted" || verdict == "degraded" {
		safe := make(map[string]any, len(parsed))
		for k, v := range parsed {
			if k == "governance" || k == "safety_fallback" {
				continue
			}
			safe[k] = v
		}
		if b, err := json.Marshal(safe); err == nil {
			validated = datatypes.JSON(b)
		}
	}

	h.outputReviewService.RecordReview(
		c.Request.Context(),
		outputType,
		verdict,
		userID,
		nil,
		jobID,
		conversationID,
		issues,
		validated,
		datatypes.JSON(raw),
	)
}
