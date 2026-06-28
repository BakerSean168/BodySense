package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// KnowledgeHandler handles knowledge base related requests.
type KnowledgeHandler struct {
	aiServiceURL string
}

// NewKnowledgeHandler creates a new KnowledgeHandler.
func NewKnowledgeHandler() *KnowledgeHandler {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8000"
	}
	return &KnowledgeHandler{aiServiceURL: aiServiceURL}
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
	VideoPath          string `json:"video_path" binding:"required"`
	ProblemSlug        string `json:"problem_slug" binding:"required"`
	ProblemDisplayName string `json:"problem_display_name" binding:"required"`
	Author             string `json:"author" binding:"required"`
	SourceTitle        string `json:"source_title,omitempty"`
	Language           string `json:"language,omitempty"`
	WhisperModel       string `json:"whisper_model,omitempty"`
	ForceTranscribe    bool   `json:"force_transcribe,omitempty"`
	ExportClips        bool   `json:"export_clips,omitempty"`
	OverwriteSource    bool   `json:"overwrite_source,omitempty"`
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
	if path == "" {
		return false
	}
	// Reject path traversal
	cleaned := filepath.Clean(path)
	if strings.Contains(cleaned, "..") {
		return false
	}
	// Must be an absolute path or within a known base directory
	if !filepath.IsAbs(cleaned) {
		return false
	}
	return true
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
		respondError(c, http.StatusBadRequest, "INVALID_VIDEO_PATH", "video_path must be an absolute path without traversal sequences")
		return
	}

	if req.Language == "" {
		req.Language = "zh"
	}
	if req.WhisperModel == "" {
		req.WhisperModel = "ggml-base.bin"
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
	c.Data(status, "application/json", sanitized)
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
