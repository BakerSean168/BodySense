package handler

import (
	"net/http"
	"strconv"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RuntimeEventHandler exposes durable runtime event queries.
type RuntimeEventHandler struct {
	runtimeEventService *service.RuntimeEventService
	conversationService *service.ConversationService
}

// NewRuntimeEventHandler creates a new RuntimeEventHandler.
func NewRuntimeEventHandler(
	runtimeEventService *service.RuntimeEventService,
	conversationService *service.ConversationService,
) *RuntimeEventHandler {
	return &RuntimeEventHandler{
		runtimeEventService: runtimeEventService,
		conversationService: conversationService,
	}
}

// ListRunEvents handles GET /api/v1/conversations/:id/runs/:runId/events
func (h *RuntimeEventHandler) ListRunEvents(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid conversation id")
		return
	}

	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid run id")
		return
	}

	conversation, err := h.conversationService.GetConversationByID(c.Request.Context(), conversationID, uid)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load conversation")
		return
	}
	if conversation == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "conversation not found")
		return
	}

	afterSeq := 0
	if raw := c.Query("after_seq"); raw != "" {
		value, parseErr := strconv.Atoi(raw)
		if parseErr != nil || value < 0 {
			respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "after_seq must be a non-negative integer")
			return
		}
		afterSeq = value
	}

	limit := 200
	if raw := c.Query("limit"); raw != "" {
		value, parseErr := strconv.Atoi(raw)
		if parseErr != nil || value <= 0 || value > 1000 {
			respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "limit must be between 1 and 1000")
			return
		}
		limit = value
	}

	events, hasMore, err := h.runtimeEventService.ListRunEvents(c.Request.Context(), conversationID, runID, afterSeq, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list runtime events")
		return
	}

	items := make([]dto.RuntimeEventDTO, 0, len(events))
	var nextAfterSeq *int
	for _, event := range events {
		items = append(items, dto.RuntimeEventDTO{
			Seq:       event.Seq,
			Channel:   event.Channel,
			Type:      event.Type,
			IDs:       []byte(event.IDs),
			Payload:   []byte(event.Payload),
			CreatedAt: event.CreatedAt,
		})
	}
	if len(events) > 0 {
		last := events[len(events)-1].Seq
		nextAfterSeq = &last
	}

	c.JSON(http.StatusOK, dto.ListRuntimeEventsResponse{
		Events:       items,
		HasMore:      hasMore,
		NextAfterSeq: nextAfterSeq,
	})
}
