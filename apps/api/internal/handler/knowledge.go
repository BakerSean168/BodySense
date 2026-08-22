package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// KnowledgeHandler handles knowledge base related requests.
type knowledgeAgentDeployment interface {
	KnowledgeCuratorConfigurationID() string
	KnowledgeCuratorDecisionPolicyRevision() string
	KnowledgeCuratorLogicalModel() string
	KnowledgeSplitterConfigurationID() string
	KnowledgeSplitterDecisionPolicyRevision() string
	KnowledgeSplitterLogicalModel() string
}

type KnowledgeHandler struct {
	aiServiceURL string
	deployment   knowledgeAgentDeployment
}

// NewKnowledgeHandler creates a new KnowledgeHandler.
func NewKnowledgeHandler(deployment knowledgeAgentDeployment) *KnowledgeHandler {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8100"
	}
	return &KnowledgeHandler{aiServiceURL: aiServiceURL, deployment: deployment}
}

// SearchRequest represents the request to search knowledge base.
type SearchRequest struct {
	Query       string  `json:"query" binding:"required"`
	TopK        int     `json:"top_k,omitempty"`
	ProblemSlug *string `json:"problem_slug,omitempty"`
	UnitType    *string `json:"unit_type,omitempty"`
}

// IngestVideoRequest represents the request to ingest a local video source.
type IngestVideoRequest struct {
	VideoPath               string `json:"video_path" binding:"required"`
	ProblemSlug             string `json:"problem_slug" binding:"required"`
	ProblemDisplayName      string `json:"problem_display_name" binding:"required"`
	Author                  string `json:"author" binding:"required"`
	SourceTitle             string `json:"source_title,omitempty"`
	Language                string `json:"language,omitempty"`
	WhisperModel            string `json:"whisper_model,omitempty"`
	ForceTranscribe         bool   `json:"force_transcribe,omitempty"`
	ExportClips             bool   `json:"export_clips,omitempty"`
	OverwriteSource         bool   `json:"overwrite_source,omitempty"`
	SplitterProvider        string `json:"splitter_provider,omitempty"`
	AIRefine                bool   `json:"ai_refine,omitempty"`
	SplitterConfigurationID string `json:"splitter_configuration_id,omitempty"`
	CuratorConfigurationID  string `json:"curator_configuration_id,omitempty"`
}

// sanitizeProxyResponse returns a generic error message for non-2xx AI service responses
// to avoid leaking internal details (stack traces, internal paths, etc.) to the client.
func sanitizeProxyResponse(statusCode int, body []byte) (int, []byte) {
	if statusCode >= 200 && statusCode < 300 {
		return statusCode, body
	}
	genericErr, _ := json.Marshal(gin.H{"error": "AI service request failed", "status": statusCode})
	return statusCode, genericErr
}

// validateVideoPath checks that the video path is within an allowed directory
// and does not contain path traversal sequences.
func validateVideoPath(path string) bool {
	normalized := strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
	if normalized == "" || strings.HasPrefix(normalized, "/") {
		return false
	}
	for _, part := range strings.Split(normalized, "/") {
		if part == ".." {
			return false
		}
	}
	cleaned := filepath.Clean(normalized)
	return !filepath.IsAbs(cleaned) && cleaned != "." && cleaned != ".."
}

// SearchKnowledge handles POST /api/knowledge/search
func (h *KnowledgeHandler) SearchKnowledge(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Set defaults
	if req.TopK == 0 {
		req.TopK = 5
	}

	// Forward to AI service
	body, err := json.Marshal(req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to marshal request")
		return
	}

	resp, err := http.Post(
		h.aiServiceURL+"/api/knowledge/search",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		respondError(c, http.StatusBadGateway, "AI_SERVICE_UNAVAILABLE", "failed to connect to AI service")
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read response")
		return
	}

	status, sanitized := sanitizeProxyResponse(resp.StatusCode, respBody)
	c.Data(status, "application/json", sanitized)
}

// IngestVideo handles POST /api/knowledge/ingestions/video
func (h *KnowledgeHandler) IngestVideo(c *gin.Context) {
	var req IngestVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Validate video path to prevent path traversal
	if !validateVideoPath(req.VideoPath) {
		respondError(c, http.StatusBadRequest, "INVALID_VIDEO_PATH", "video_path must be relative to the knowledge data root without traversal sequences")
		return
	}

	if req.Language == "" {
		req.Language = "zh"
	}
	if req.WhisperModel == "" {
		req.WhisperModel = "ggml-base.bin"
	}
	if req.SplitterProvider == "" {
		req.SplitterProvider = "heuristic"
	}
	if req.SplitterProvider != "heuristic" && req.SplitterProvider != "llm" {
		respondError(c, http.StatusBadRequest, "INVALID_SPLITTER_PROVIDER", "splitter_provider must be heuristic or llm")
		return
	}
	// North-Star: callers choose the capability, never an immutable Agent id.
	req.SplitterConfigurationID = ""
	req.CuratorConfigurationID = ""
	if req.SplitterProvider == "llm" {
		if h.deployment == nil {
			respondError(c, http.StatusServiceUnavailable, "AGENT_DEPLOYMENT_UNAVAILABLE", "knowledge Agent deployment policy is unavailable")
			return
		}
		req.SplitterConfigurationID = h.deployment.KnowledgeSplitterConfigurationID()
	}
	if req.AIRefine {
		if h.deployment == nil {
			respondError(c, http.StatusServiceUnavailable, "AGENT_DEPLOYMENT_UNAVAILABLE", "knowledge Agent deployment policy is unavailable")
			return
		}
		req.CuratorConfigurationID = h.deployment.KnowledgeCuratorConfigurationID()
	}

	body, err := json.Marshal(req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to marshal request")
		return
	}

	resp, err := http.Post(
		h.aiServiceURL+"/api/knowledge/ingestions/video",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		respondError(c, http.StatusBadGateway, "AI_SERVICE_UNAVAILABLE", "failed to connect to AI service")
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read response")
		return
	}

	status, sanitized := sanitizeProxyResponse(resp.StatusCode, respBody)
	if status >= 200 && status < 300 {
		if err := h.validateKnowledgeAgentExecution(respBody, req); err != nil {
			respondError(c, http.StatusBadGateway, "AGENT_IDENTITY_MISMATCH", "knowledge Agent execution identity validation failed")
			return
		}
	}
	c.Data(status, "application/json", sanitized)
}

func (h *KnowledgeHandler) validateKnowledgeAgentExecution(body []byte, req IngestVideoRequest) error {
	if req.SplitterProvider != "llm" && !req.AIRefine {
		return nil
	}
	var response struct {
		AgentExecution map[string]struct {
			AgentConfiguration  map[string]any `json:"agent_configuration"`
			ExecutionProvenance map[string]any `json:"execution_provenance"`
		} `json:"agent_execution"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return err
	}
	check := func(key, expectedID, expectedRole, expectedPolicy, expectedModel string) error {
		record, ok := response.AgentExecution[key]
		if !ok {
			return fmt.Errorf("missing %s execution record", key)
		}
		id, _ := record.AgentConfiguration["id"].(string)
		role, _ := record.AgentConfiguration["role"].(string)
		policy, _ := record.AgentConfiguration["decision_policy_revision"].(string)
		logicalModel, _ := record.AgentConfiguration["logical_model"].(string)
		if id != expectedID || role != expectedRole || policy != expectedPolicy || logicalModel != expectedModel {
			return fmt.Errorf("%s immutable configuration mismatch", key)
		}
		executionStatus, _ := record.ExecutionProvenance["status"].(string)
		executionModel, _ := record.ExecutionProvenance["logical_model"].(string)
		if (executionStatus != "executed" && executionStatus != "degraded") || executionModel != expectedModel {
			return fmt.Errorf("%s execution provenance mismatch", key)
		}
		return nil
	}
	if req.SplitterProvider == "llm" {
		if err := check(
			"knowledge_splitter",
			req.SplitterConfigurationID,
			"knowledge_splitter",
			h.deployment.KnowledgeSplitterDecisionPolicyRevision(),
			h.deployment.KnowledgeSplitterLogicalModel(),
		); err != nil {
			return err
		}
	}
	if req.AIRefine {
		if err := check(
			"knowledge_curator",
			req.CuratorConfigurationID,
			"knowledge_curator",
			h.deployment.KnowledgeCuratorDecisionPolicyRevision(),
			h.deployment.KnowledgeCuratorLogicalModel(),
		); err != nil {
			return err
		}
	}
	return nil
}

// ListSources handles GET /api/knowledge/sources
func (h *KnowledgeHandler) ListSources(c *gin.Context) {
	resp, err := http.Get(h.aiServiceURL + "/api/knowledge/sources")
	if err != nil {
		respondError(c, http.StatusBadGateway, "AI_SERVICE_UNAVAILABLE", "failed to connect to AI service")
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read response")
		return
	}

	status, sanitized := sanitizeProxyResponse(resp.StatusCode, respBody)
	c.Data(status, "application/json", sanitized)
}

// GetStats handles GET /api/knowledge/stats
func (h *KnowledgeHandler) GetStats(c *gin.Context) {
	resp, err := http.Get(h.aiServiceURL + "/api/knowledge/stats")
	if err != nil {
		respondError(c, http.StatusBadGateway, "AI_SERVICE_UNAVAILABLE", "failed to connect to AI service")
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read response")
		return
	}

	status, sanitized := sanitizeProxyResponse(resp.StatusCode, respBody)
	c.Data(status, "application/json", sanitized)
}
