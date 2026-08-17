package handler

import (
	"encoding/json"
	"net/http"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ThreadProjectionHandler exposes projection-backed consultation thread reads.
type ThreadProjectionHandler struct {
	threadProjectionService *service.ThreadProjectionService
	bodyStateService        *service.BodyStateService
}

// NewThreadProjectionHandler creates the thread read boundary with BodyState as the health source of truth.
func NewThreadProjectionHandler(
	threadProjectionService *service.ThreadProjectionService,
	bodyStateService *service.BodyStateService,
) *ThreadProjectionHandler {
	return &ThreadProjectionHandler{threadProjectionService: threadProjectionService, bodyStateService: bodyStateService}
}

// GetConsultationThread handles GET /api/v1/consultations/:id/thread.
func (h *ThreadProjectionHandler) GetConsultationThread(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid consultation id")
		return
	}

	projection, messages, toolCalls, activeTurnRunID, activeTurnEvents, err := h.threadProjectionService.RefreshAndGetThread(c.Request.Context(), conversationID, uid)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get consultation thread")
		return
	}
	if projection == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "consultation thread not found")
		return
	}

	var bodyState any
	if snapshot, loadErr := h.bodyStateService.GetSnapshot(c.Request.Context(), uid, 20); loadErr == nil {
		bodyState = snapshot
	}

	c.JSON(http.StatusOK, gin.H{
		"conversation_id": projection.ConversationID,
		"conversation": gin.H{
			"id":              projection.ConversationID,
			"title":           projection.Title,
			"title_status":    projection.TitleStatus,
			"status":          projection.Status,
			"pinned":          projection.Pinned,
			"pinned_at":       projection.PinnedAt,
			"default_model":   emptyStringToNull(projection.DefaultModel),
			"active_run_id":   projection.ActiveRunID,
			"last_message_at": projection.LastMessageAt,
			"metadata":        rawJSONOrEmptyObject(projection.Metadata),
			"message_count":   len(messages),
			"created_at":      projection.ConversationCreatedAt,
			"updated_at":      projection.ConversationUpdatedAt,
		},
		"phase":                projection.Phase,
		"extracted_info":       rawJSONOrEmptyArray(projection.ExtractedInfo),
		"body_state":           bodyState,
		"pending_interactions": rawJSONOrEmptyArray(projection.PendingInteractions),
		"interaction_history":  rawJSONOrEmptyArray(projection.InteractionHistory),
		"active_turn_run_id":   activeTurnRunID,
		"active_turn_events":   toStreamEvents(activeTurnEvents),
		"messages":             messages,
		"tool_calls":           toolCalls,
		"created_at":           projection.SessionCreatedAt,
		"updated_at":           projection.SessionUpdatedAt,
		"ended_at":             projection.EndedAt,
	})
}

func rawJSONOrEmptyObject(value []byte) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	return value
}

func rawJSONOrEmptyArray(value []byte) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`[]`)
	}
	return value
}

func emptyStringToNull(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func toStreamEvents(events []model.RuntimeEvent) []dto.StreamEvent {
	items := make([]dto.StreamEvent, 0, len(events))
	for _, event := range events {
		var ids dto.StreamEventIDs
		if len(event.IDs) > 0 {
			_ = json.Unmarshal(event.IDs, &ids)
		}

		payload := json.RawMessage(`{}`)
		if len(event.Payload) > 0 {
			payload = json.RawMessage(event.Payload)
		}

		items = append(items, dto.StreamEvent{
			Version: 1,
			Seq:     event.Seq,
			Channel: event.Channel,
			Type:    event.Type,
			IDs:     ids,
			Payload: payload,
		})
	}
	return items
}
