package httpapi

import (
	"context"
	"encoding/json"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"net/http"
	"strings"
	"time"
)

func (s *PublicServer) GetConsultationThread(
	ctx context.Context,
	request openapiv1.GetConsultationThreadRequestObject,
) (openapiv1.GetConsultationThreadResponseObject, error) {
	uid, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return consultationHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "missing authenticated user"}, nil
	}
	if s.consultationThreads == nil {
		return consultationHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "CONSULTATION_UNAVAILABLE", message: "consultation thread service is not configured"}, nil
	}
	projection, messages, toolCalls, activeTurnRunID, activeTurnEvents, err := s.consultationThreads.RefreshAndGetThread(applicationRequestContext(ctx), request.Id, uid)
	if err != nil {
		return internalConsultationError("failed to get consultation thread"), nil
	}
	if projection == nil {
		return consultationHTTPErrorResponse{status: http.StatusNotFound, code: "NOT_FOUND", message: "consultation thread not found"}, nil
	}
	wire := consultationThreadWire{
		ConversationID: projection.ConversationID,
		Conversation: consultationThreadConversationWire{
			ID:            projection.ConversationID,
			Status:        projection.Status,
			DefaultModel:  nullableString(projection.DefaultModel),
			LastMessageAt: projection.LastMessageAt,
			Metadata:      rawJSONObject(projection.Metadata),
			MessageCount:  len(messages),
			CreatedAt:     projection.ConversationCreatedAt,
			UpdatedAt:     projection.ConversationUpdatedAt,
		},
		Phase:               string(projection.Phase),
		ExtractedInfo:       rawJSONArray(projection.ExtractedInfo),
		PendingInteractions: rawJSONArray(projection.PendingInteractions),
		InteractionHistory:  rawJSONArray(projection.InteractionHistory),
		ActiveTurnRunID:     activeTurnRunID,
		ActiveTurnEvents:    newThreadRuntimeEventWires(activeTurnEvents),
		Messages:            messages,
		ToolCalls:           newProjectedToolCallWires(toolCalls),
		CreatedAt:           projection.SessionCreatedAt,
		UpdatedAt:           projection.SessionUpdatedAt,
		EndedAt:             projection.EndedAt,
	}
	body, err := strictOpenAPIConvert[openapiv1.ConsultationThreadResponse]("ConsultationThreadResponse", wire)
	if err != nil {
		return nil, err
	}
	return openapiv1.GetConsultationThread200JSONResponse(body), nil
}

type consultationThreadConversationWire struct {
	ID            uuid.UUID       `json:"id"`
	Status        string          `json:"status"`
	DefaultModel  *string         `json:"default_model"`
	LastMessageAt *time.Time      `json:"last_message_at"`
	Metadata      json.RawMessage `json:"metadata"`
	MessageCount  int             `json:"message_count"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type consultationThreadWire struct {
	ConversationID      uuid.UUID                          `json:"conversation_id"`
	Conversation        consultationThreadConversationWire `json:"conversation"`
	Phase               string                             `json:"phase"`
	ExtractedInfo       json.RawMessage                    `json:"extracted_info"`
	PendingInteractions json.RawMessage                    `json:"pending_interactions"`
	InteractionHistory  json.RawMessage                    `json:"interaction_history"`
	ActiveTurnRunID     *uuid.UUID                         `json:"active_turn_run_id"`
	ActiveTurnEvents    []threadRuntimeEventWire           `json:"active_turn_events"`
	Messages            []model.ThreadProjectionMessage    `json:"messages"`
	ToolCalls           []projectedToolCallWire            `json:"tool_calls"`
	CreatedAt           time.Time                          `json:"created_at"`
	UpdatedAt           time.Time                          `json:"updated_at"`
	EndedAt             *time.Time                         `json:"ended_at"`
}

type threadRuntimeEventWire struct {
	Version   int             `json:"version"`
	Seq       int             `json:"seq"`
	Channel   string          `json:"channel"`
	Type      string          `json:"type"`
	IDs       json.RawMessage `json:"ids"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

func newThreadRuntimeEventWires(events []model.RuntimeEvent) []threadRuntimeEventWire {
	items := make([]threadRuntimeEventWire, 0, len(events))
	for _, event := range events {
		items = append(items, threadRuntimeEventWire{
			Version: 1, Seq: event.Seq, Channel: event.Channel, Type: event.Type,
			IDs: rawJSONObject(event.IDs), Payload: rawJSONObject(event.Payload), CreatedAt: event.CreatedAt,
		})
	}
	return items
}

type projectedToolCallWire struct {
	ToolCallID     string          `json:"tool_call_id"`
	ConversationID uuid.UUID       `json:"conversation_id"`
	RunID          uuid.UUID       `json:"run_id"`
	MessageID      *uuid.UUID      `json:"message_id"`
	ToolName       string          `json:"tool_name"`
	Arguments      json.RawMessage `json:"arguments"`
	Status         string          `json:"status"`
	Result         any             `json:"result"`
	Error          any             `json:"error"`
	CreatedAt      time.Time       `json:"created_at"`
	StartedAt      time.Time       `json:"started_at"`
	FinishedAt     *time.Time      `json:"finished_at"`
	Metadata       json.RawMessage `json:"metadata"`
}

func newProjectedToolCallWires(items []model.ThreadProjectionToolCall) []projectedToolCallWire {
	result := make([]projectedToolCallWire, 0, len(items))
	for _, item := range items {
		result = append(result, projectedToolCallWire{
			ToolCallID: item.ToolCallID, ConversationID: item.ConversationID, RunID: item.RunID,
			MessageID: item.MessageID, ToolName: item.ToolName, Arguments: rawJSONObject(item.Arguments),
			Status: item.Status, Result: rawJSONValue(item.Result), Error: rawJSONValue(item.Error),
			CreatedAt: item.CreatedAt, StartedAt: item.StartedAt, FinishedAt: item.FinishedAt,
			Metadata: rawJSONObject(item.Metadata),
		})
	}
	return result
}

func rawJSONArray(value []byte) json.RawMessage {
	if len(strings.TrimSpace(string(value))) == 0 || string(value) == "null" {
		return json.RawMessage(`[]`)
	}
	return json.RawMessage(value)
}

func rawJSONObject(value []byte) json.RawMessage {
	if len(strings.TrimSpace(string(value))) == 0 || string(value) == "null" {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(value)
}

func rawJSONValue(value []byte) any {
	if len(strings.TrimSpace(string(value))) == 0 || string(value) == "null" {
		return nil
	}
	var decoded any
	if json.Unmarshal(value, &decoded) != nil {
		return nil
	}
	return decoded
}

func nullableString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}
