package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ConversationHandler handles conversation HTTP requests.
type ConversationHandler struct {
	conversationService *service.ConversationService
	shareService        *service.ShareService
}

// NewConversationHandler creates a new ConversationHandler.
func NewConversationHandler(
	conversationService *service.ConversationService,
	shareService *service.ShareService,
) *ConversationHandler {
	return &ConversationHandler{
		conversationService: conversationService,
		shareService:        shareService,
	}
}

// ListConversations handles GET /api/v1/conversations
func (h *ConversationHandler) ListConversations(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	limit := 20
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	var cursor *time.Time
	if cursorStr := c.Query("cursor"); cursorStr != "" {
		t, err := time.Parse(time.RFC3339, cursorStr)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_CURSOR", "cursor must be RFC3339 timestamp")
			return
		}
		cursor = &t
	}

	conversations, hasMore, err := h.conversationService.ListConversations(c.Request.Context(), uid, cursor, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list conversations")
		return
	}

	var nextCursor *string
	if hasMore && len(conversations) > 0 {
		last := conversations[len(conversations)-1]
		ts := last.UpdatedAt.Format(time.RFC3339)
		nextCursor = &ts
	}

	c.JSON(http.StatusOK, gin.H{
		"conversations": conversations,
		"hasMore":       hasMore,
		"nextCursor":    nextCursor,
	})
}

// GetConversation handles GET /api/v1/conversations/:id
func (h *ConversationHandler) GetConversation(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid conversation id")
		return
	}

	conversation, messages, err := h.conversationService.GetConversation(c.Request.Context(), conversationID, uid)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get conversation")
		return
	}
	if conversation == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "conversation not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"conversation": conversation,
		"messages":     messages,
	})
}

// DeleteConversation handles DELETE /api/v1/conversations/:id
func (h *ConversationHandler) DeleteConversation(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid conversation id")
		return
	}

	if err := h.conversationService.DeleteConversation(c.Request.Context(), conversationID, uid); err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "conversation deleted"})
}

// PinConversation handles PATCH /api/v1/conversations/:id/pin
func (h *ConversationHandler) PinConversation(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid conversation id")
		return
	}

	var req dto.PinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.conversationService.PinConversation(c.Request.Context(), conversationID, uid, req.Pinned); err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "conversation pin updated"})
}

// RenameTitle handles PUT /api/v1/conversations/:id/title
func (h *ConversationHandler) RenameTitle(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid conversation id")
		return
	}

	var req dto.RenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Title == "" {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "title is required")
		return
	}

	if err := h.conversationService.RenameTitle(c.Request.Context(), conversationID, uid, req.Title); err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "title updated"})
}

// GenerateTitle handles POST /api/v1/conversations/:id/title
func (h *ConversationHandler) GenerateTitle(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid conversation id")
		return
	}

	if err := h.conversationService.GenerateTitle(c.Request.Context(), conversationID, uid); err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "title generation started"})
}

// ShareConversation handles POST /api/v1/conversations/:id/share
func (h *ConversationHandler) ShareConversation(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid conversation id")
		return
	}

	share, shareURL, err := h.shareService.ShareConversation(c.Request.Context(), conversationID, uid)
	if err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	c.JSON(http.StatusCreated, dto.ShareResponse{
		ShareToken: share.ShareToken,
		ShareURL:   shareURL,
	})
}

// UnshareConversation handles DELETE /api/v1/conversations/:id/share
func (h *ConversationHandler) UnshareConversation(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid conversation id")
		return
	}

	if err := h.shareService.UnshareConversation(c.Request.Context(), conversationID, uid); err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "conversation unshared"})
}

// UpdateConversation handles PATCH /api/v1/conversations/:id
func (h *ConversationHandler) UpdateConversation(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid conversation id")
		return
	}

	var req struct {
		Status *string `json:"status,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Status != nil {
		validStatuses := map[string]bool{"active": true, "archived": true, "deleted": true}
		if !validStatuses[*req.Status] {
			respondError(c, http.StatusBadRequest, "INVALID_STATUS", "status must be active, archived, or deleted")
			return
		}
		if err := h.conversationService.UpdateConversationStatus(c.Request.Context(), conversationID, uid, *req.Status); err != nil {
			respondError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "conversation updated"})
}

// ListRuns handles GET /api/v1/conversations/:id/runs
func (h *ConversationHandler) ListRuns(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid conversation id")
		return
	}

	runs, err := h.conversationService.ListRuns(c.Request.Context(), conversationID, uid)
	if err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

// GetSharedConversation handles GET /api/v1/conversations/share/:token
func (h *ConversationHandler) GetSharedConversation(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		respondError(c, http.StatusBadRequest, "INVALID_TOKEN", "share token is required")
		return
	}

	share, err := h.shareService.GetSharedConversation(c.Request.Context(), token)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get shared conversation")
		return
	}
	if share == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "shared conversation not found")
		return
	}

	c.JSON(http.StatusOK, dto.SharedConversationResponse{
		Title:    share.SnapshotTitle,
		Messages: json.RawMessage(share.SnapshotMessages),
	})
}
