package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"

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

// AddEntryRequest represents the request to add a knowledge entry.
type AddEntryRequest struct {
	Category        string  `json:"category" binding:"required"`
	Title           string  `json:"title" binding:"required"`
	Content         string  `json:"content" binding:"required"`
	SourceVideo     *string `json:"source_video,omitempty"`
	SourceTimestamp *string `json:"source_timestamp,omitempty"`
}

// SearchRequest represents the request to search knowledge base.
type SearchRequest struct {
	Query    string  `json:"query" binding:"required"`
	TopK     int     `json:"top_k,omitempty"`
	TopN     int     `json:"top_n,omitempty"`
	Category *string `json:"category,omitempty"`
}

// AddEntry handles POST /api/knowledge/entries
func (h *KnowledgeHandler) AddEntry(c *gin.Context) {
	var req AddEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Forward to AI service
	body, err := json.Marshal(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal request"})
		return
	}

	resp, err := http.Post(
		h.aiServiceURL+"/api/knowledge/entries",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to connect to AI service"})
		return
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read response"})
		return
	}

	// Forward response
	c.Data(resp.StatusCode, "application/json", respBody)
}

// SearchKnowledge handles POST /api/knowledge/search
func (h *KnowledgeHandler) SearchKnowledge(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set defaults
	if req.TopK == 0 {
		req.TopK = 10
	}
	if req.TopN == 0 {
		req.TopN = 3
	}

	// Forward to AI service
	body, err := json.Marshal(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal request"})
		return
	}

	resp, err := http.Post(
		h.aiServiceURL+"/api/knowledge/search",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to connect to AI service"})
		return
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read response"})
		return
	}

	// Forward response
	c.Data(resp.StatusCode, "application/json", respBody)
}

// GetEntry handles GET /api/knowledge/entries/:id
func (h *KnowledgeHandler) GetEntry(c *gin.Context) {
	id := c.Param("id")

	resp, err := http.Get(h.aiServiceURL + "/api/knowledge/entries/" + id)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to connect to AI service"})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read response"})
		return
	}

	c.Data(resp.StatusCode, "application/json", respBody)
}

// DeleteEntry handles DELETE /api/knowledge/entries/:id
func (h *KnowledgeHandler) DeleteEntry(c *gin.Context) {
	id := c.Param("id")

	req, err := http.NewRequest(http.MethodDelete, h.aiServiceURL+"/api/knowledge/entries/"+id, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request"})
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to connect to AI service"})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read response"})
		return
	}

	c.Data(resp.StatusCode, "application/json", respBody)
}

// GetStats handles GET /api/knowledge/stats
func (h *KnowledgeHandler) GetStats(c *gin.Context) {
	resp, err := http.Get(h.aiServiceURL + "/api/knowledge/stats")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to connect to AI service"})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read response"})
		return
	}

	c.Data(resp.StatusCode, "application/json", respBody)
}
