package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ConsultationHandler handles consultation HTTP requests.
type ConsultationHandler struct {
	consultationService *service.ConsultationService
	profileService      *service.ProfileService
	aiServiceURL        string
}

type knowledgeSearchResult struct {
	Title           string           `json:"title"`
	Summary         string           `json:"summary"`
	BodyMarkdown    string           `json:"body_markdown"`
	Category        string           `json:"category"`
	ProblemSlug     string           `json:"problem_slug"`
	UnitType        string           `json:"unit_type"`
	SourceTitle     string           `json:"source_title"`
	SourceAuthor    string           `json:"source_author"`
	SourceTimestamp string           `json:"source_timestamp"`
	Tags            []string         `json:"tags"`
	Clips           []map[string]any `json:"clips"`
	Similarity      float64          `json:"similarity"`
}

type knowledgeSearchResponse struct {
	Results []knowledgeSearchResult `json:"results"`
}

// NewConsultationHandler creates a new ConsultationHandler.
func NewConsultationHandler(
	consultationService *service.ConsultationService,
	profileService *service.ProfileService,
) *ConsultationHandler {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8000"
	}
	return &ConsultationHandler{
		consultationService: consultationService,
		profileService:      profileService,
		aiServiceURL:        aiServiceURL,
	}
}

// CreateSession handles POST /api/v1/consultation
func (h *ConsultationHandler) CreateSession(c *gin.Context) {
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

	session, err := h.consultationService.CreateSession(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	c.JSON(http.StatusCreated, session)
}

// GetSession handles GET /api/v1/consultation/:id
func (h *ConsultationHandler) GetSession(c *gin.Context) {
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

	session, err := h.consultationService.GetSession(c.Request.Context(), sessionID, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get session"})
		return
	}
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// ListSessions handles GET /api/v1/consultation
func (h *ConsultationHandler) ListSessions(c *gin.Context) {
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

	// Parse pagination params
	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	sessions, total, err := h.consultationService.ListSessions(c.Request.Context(), uid, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// SendMessage handles POST /api/v1/consultation/:id/message (SSE proxy)
func (h *ConsultationHandler) SendMessage(c *gin.Context) {
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

	// Parse request body
	var req dto.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user profile for context
	profile, err := h.profileService.GetProfile(c.Request.Context(), uid)
	profileMap := map[string]any{}
	if err == nil && profile != nil {
		profileJSON, _ := json.Marshal(profile)
		_ = json.Unmarshal(profileJSON, &profileMap)
	}

	// Parse existing messages and extracted info from session
	var messages []any
	if len(session.Messages) > 0 {
		_ = json.Unmarshal(session.Messages, &messages)
	}
	if messages == nil {
		messages = []any{}
	}

	var extractedInfoList []any
	if len(session.ExtractedInfo) > 0 {
		_ = json.Unmarshal(session.ExtractedInfo, &extractedInfoList)
	}
	if extractedInfoList == nil {
		extractedInfoList = []any{}
	}

	// Save user message to session
	userMsg := map[string]any{
		"role":    "user",
		"content": req.Content,
	}
	messages = append(messages, userMsg)
	_ = h.consultationService.AppendMessage(c.Request.Context(), sessionID, userMsg)

	// Build request to AI service
	aiReq := map[string]any{
		"session_id":     sessionID.String(),
		"user_id":        uid.String(),
		"content":        req.Content,
		"profile":        profileMap,
		"messages":       messages,
		"extracted_info": extractedInfoList,
	}

	ragResults, err := h.searchKnowledge(c.Request.Context(), req.Content)
	if err == nil && len(ragResults) > 0 {
		aiReq["rag_results"] = ragResults
	}

	aiReqBody, err := json.Marshal(aiReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal request"})
		return
	}

	// Forward to AI service SSE endpoint
	resp, err := http.Post(
		h.aiServiceURL+"/api/chat/stream",
		"application/json",
		bytes.NewBuffer(aiReqBody),
	)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to connect to AI service"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.Data(resp.StatusCode, "application/json", body)
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Stream the response and collect assistant message
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	var assistantText string
	var latestExtractedInfo []any

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// Forward the SSE line
		fmt.Fprintf(c.Writer, "%s\n", line)
		if line == "" {
			flusher.Flush()
		}

		// Parse SSE data to collect assistant response
		if len(line) > 6 && line[:6] == "data: " {
			dataStr := line[6:]
			var data map[string]any
			if json.Unmarshal([]byte(dataStr), &data) == nil {
				if data["type"] == "text" {
					if content, ok := data["content"].(string); ok {
						assistantText += content
					}
				}
				if data["type"] == "extracted_info" {
					if info, ok := data["info"]; ok {
						latestExtractedInfo = append(latestExtractedInfo, info)
					}
				}
			}
		}
	}

	// Save assistant message to session
	if assistantText != "" {
		assistantMsg := map[string]any{
			"role":    "assistant",
			"content": assistantText,
		}
		_ = h.consultationService.AppendMessage(c.Request.Context(), sessionID, assistantMsg)
	}

	// Update extracted info if new info was found
	if len(latestExtractedInfo) > 0 {
		// Merge with existing
		merged := append(extractedInfoList, latestExtractedInfo...)
		_ = h.consultationService.UpdateExtractedInfo(c.Request.Context(), sessionID, merged)
	}
}

// UpdateExtractedInfo handles PUT /api/v1/consultation/:id/extracted-info
func (h *ConsultationHandler) UpdateExtractedInfo(c *gin.Context) {
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

	var req dto.UpdateExtractedInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.consultationService.UpdateExtractedInfo(c.Request.Context(), sessionID, req.ExtractedInfo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update extracted info"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// ConfirmDiagnosis handles PUT /api/v1/consultation/:id/confirm
func (h *ConsultationHandler) ConfirmDiagnosis(c *gin.Context) {
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

	var req dto.ConfirmDiagnosisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.consultationService.UpdateDiagnosis(c.Request.Context(), sessionID, req.Diagnosis); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update diagnosis"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "diagnosis confirmed"})
}

func (h *ConsultationHandler) searchKnowledge(ctx context.Context, query string) ([]map[string]any, error) {
	body, err := json.Marshal(SearchRequest{
		Query: query,
		TopK:  5,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		h.aiServiceURL+"/api/knowledge/search",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("knowledge search failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var payload knowledgeSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	sortKnowledgeResults(payload.Results, query)

	results := make([]map[string]any, 0, len(payload.Results))
	for _, result := range payload.Results {
		results = append(results, map[string]any{
			"title":            result.Title,
			"summary":          result.Summary,
			"content":          result.BodyMarkdown,
			"body_markdown":    result.BodyMarkdown,
			"category":         result.Category,
			"problem_slug":     result.ProblemSlug,
			"unit_type":        result.UnitType,
			"source_title":     result.SourceTitle,
			"source_author":    result.SourceAuthor,
			"source_timestamp": result.SourceTimestamp,
			"tags":             result.Tags,
			"clips":            result.Clips,
		})
	}

	return results, nil
}

func sortKnowledgeResults(results []knowledgeSearchResult, query string) {
	if len(results) < 2 {
		return
	}

	preferredUnitTypes := []string{}
	switch {
	case containsAny(query, []string{"自测", "测试", "判断", "检查"}):
		preferredUnitTypes = []string{"self_check"}
	case containsAny(query, []string{"是什么", "定义", "什么意思"}):
		preferredUnitTypes = []string{"definition"}
	case containsAny(query, []string{"怎么", "如何", "处理", "改善", "纠正", "矫正", "训练", "动作", "缓解"}):
		preferredUnitTypes = []string{"exercise", "recommendation"}
	}

	sort.SliceStable(results, func(i, j int) bool {
		leftScore := unitTypeIntentScore(results[i].UnitType, preferredUnitTypes)
		rightScore := unitTypeIntentScore(results[j].UnitType, preferredUnitTypes)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		return results[i].Similarity > results[j].Similarity
	})
}

func containsAny(query string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(query, keyword) {
			return true
		}
	}
	return false
}

func unitTypeIntentScore(unitType string, preferred []string) int {
	for idx, value := range preferred {
		if unitType == value {
			return len(preferred) - idx
		}
	}
	return 0
}
