package httpapi

import (
	"context"
	"net/http"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// conversationApplication is the singleton Conversation surface (PC-D4):
// get-or-create plus read-only Run history. There is no list, create, rename,
// pin, archive, delete or public share operation.
type conversationApplication interface {
	GetOrCreateConversation(ctx context.Context, userID uuid.UUID) (*model.Conversation, []model.Message, error)
	GetConversationByID(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error)
	ListRuns(ctx context.Context, conversationID, userID uuid.UUID) ([]model.Run, error)
}

type runtimeEventApplication interface {
	ListRunEvents(ctx context.Context, conversationID, runID uuid.UUID, afterSeq, limit int) ([]model.RuntimeEvent, bool, error)
}

func (s *PublicServer) WithConversations(
	conversations conversationApplication,
	runtimeEvents runtimeEventApplication,
) *PublicServer {
	s.conversations = conversations
	s.runtimeEvents = runtimeEvents
	return s
}

// GetConversation returns the authenticated user's single Conversation,
// lazily creating it on first use.
func (s *PublicServer) GetConversation(
	ctx context.Context,
	_ openapiv1.GetConversationRequestObject,
) (openapiv1.GetConversationResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return getConversationError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}

	conversation, messages, getErr := s.conversations.GetOrCreateConversation(ctx, userID)
	if getErr != nil || conversation == nil {
		return getConversationError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get conversation"), nil
	}

	body, err := strictOpenAPIConvert[openapiv1.ConversationDetailResponse](
		"ConversationDetailResponse",
		conversationDetailWire{Conversation: newConversationWire(*conversation), Messages: newMessageWires(messages)},
	)
	if err != nil {
		return getConversationError(http.StatusInternalServerError, "INTERNAL_ERROR", "conversation detail violates the public contract"), nil
	}
	return openapiv1.GetConversation200JSONResponse(body), nil
}

func (s *PublicServer) ListConversationRuns(
	ctx context.Context,
	request openapiv1.ListConversationRunsRequestObject,
) (openapiv1.ListConversationRunsResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return listConversationRunsError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}

	runs, err := s.conversations.ListRuns(ctx, request.Id, userID)
	if err != nil {
		return listConversationRunsError(http.StatusNotFound, "NOT_FOUND", "conversation not found"), nil
	}
	body, err := strictOpenAPIConvert[openapiv1.ConversationRunListResponse](
		"ConversationRunListResponse",
		conversationRunListWire{Runs: newRunWires(runs)},
	)
	if err != nil {
		return listConversationRunsError(http.StatusInternalServerError, "INTERNAL_ERROR", "conversation runs violate the public contract"), nil
	}
	return openapiv1.ListConversationRuns200JSONResponse(body), nil
}

func (s *PublicServer) ListRunEvents(
	ctx context.Context,
	request openapiv1.ListRunEventsRequestObject,
) (openapiv1.ListRunEventsResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return listRunEventsError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}

	conversation, err := s.conversations.GetConversationByID(ctx, request.Id, userID)
	if err != nil {
		return listRunEventsError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load conversation"), nil
	}
	if conversation == nil {
		return listRunEventsError(http.StatusNotFound, "NOT_FOUND", "conversation not found"), nil
	}

	afterSeq := 0
	if request.Params.AfterSeq != nil {
		afterSeq = *request.Params.AfterSeq
	}
	limit := 200
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}

	events, hasMore, err := s.runtimeEvents.ListRunEvents(ctx, request.Id, request.RunId, afterSeq, limit)
	if err != nil {
		return listRunEventsError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list runtime events"), nil
	}

	items := make([]runtimeEventWire, 0, len(events))
	var nextAfterSeq *int
	for _, event := range events {
		items = append(items, runtimeEventWire{
			Seq:       event.Seq,
			Channel:   event.Channel,
			Type:      event.Type,
			IDs:       event.IDs,
			Payload:   event.Payload,
			CreatedAt: event.CreatedAt,
		})
	}
	if len(events) > 0 {
		last := events[len(events)-1].Seq
		nextAfterSeq = &last
	}
	body, err := strictOpenAPIConvert[openapiv1.RuntimeEventListResponse](
		"RuntimeEventListResponse",
		runtimeEventListWire{Events: items, HasMore: hasMore, NextAfterSeq: nextAfterSeq},
	)
	if err != nil {
		return listRunEventsError(http.StatusInternalServerError, "INTERNAL_ERROR", "runtime events violate the public contract"), nil
	}
	return openapiv1.ListRunEvents200JSONResponse(body), nil
}

// Wire mirrors for strict schema validation of public projections. These types
// intentionally exclude persistence-only identity, provider/session internals,
// lease data and frozen agent provenance.

type conversationWire struct {
	ID            uuid.UUID      `json:"id"`
	Status        string         `json:"status"`
	DefaultModel  string         `json:"default_model,omitempty"`
	LastMessageAt *time.Time     `json:"last_message_at,omitempty"`
	Metadata      datatypes.JSON `json:"metadata"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

func newConversationWire(conversation model.Conversation) conversationWire {
	return conversationWire{
		ID:            conversation.ID,
		Status:        conversation.Status,
		DefaultModel:  conversation.DefaultModel,
		LastMessageAt: conversation.LastMessageAt,
		Metadata:      conversation.Metadata,
		CreatedAt:     conversation.CreatedAt,
		UpdatedAt:     conversation.UpdatedAt,
	}
}

type conversationMessageWire struct {
	ID                 uuid.UUID      `json:"id"`
	ConversationID     uuid.UUID      `json:"conversation_id"`
	TurnID             uuid.UUID      `json:"turn_id"`
	RunID              *uuid.UUID     `json:"run_id,omitempty"`
	ParentMessageID    *uuid.UUID     `json:"parent_message_id,omitempty"`
	Role               string         `json:"role"`
	Status             string         `json:"status"`
	Seq                int            `json:"seq"`
	Parts              datatypes.JSON `json:"parts"`
	ContentText        string         `json:"content_text,omitempty"`
	Model              string         `json:"model,omitempty"`
	Provider           string         `json:"provider,omitempty"`
	ProviderMessageID  string         `json:"provider_message_id,omitempty"`
	ProviderResponseID string         `json:"provider_response_id,omitempty"`
	InputTokens        *int           `json:"input_tokens,omitempty"`
	OutputTokens       *int           `json:"output_tokens,omitempty"`
	TotalTokens        *int           `json:"total_tokens,omitempty"`
	Error              datatypes.JSON `json:"error,omitempty"`
	Metadata           datatypes.JSON `json:"metadata"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

func newMessageWires(messages []model.Message) []conversationMessageWire {
	items := make([]conversationMessageWire, 0, len(messages))
	for _, message := range messages {
		items = append(items, conversationMessageWire{
			ID:                 message.ID,
			ConversationID:     message.ConversationID,
			TurnID:             message.TurnID,
			RunID:              message.RunID,
			ParentMessageID:    message.ParentMessageID,
			Role:               message.Role,
			Status:             message.Status,
			Seq:                message.Seq,
			Parts:              message.Parts,
			ContentText:        message.ContentText,
			Model:              message.Model,
			Provider:           message.Provider,
			ProviderMessageID:  message.ProviderMessageID,
			ProviderResponseID: message.ProviderResponseID,
			InputTokens:        message.InputTokens,
			OutputTokens:       message.OutputTokens,
			TotalTokens:        message.TotalTokens,
			Error:              message.Error,
			Metadata:           message.Metadata,
			CreatedAt:          message.CreatedAt,
			UpdatedAt:          message.UpdatedAt,
		})
	}
	return items
}

type conversationRunWire struct {
	ID             uuid.UUID      `json:"id"`
	ConversationID uuid.UUID      `json:"conversation_id"`
	TurnID         uuid.UUID      `json:"turn_id"`
	RequestID      string         `json:"request_id"`
	Status         string         `json:"status"`
	Model          string         `json:"model"`
	Provider       string         `json:"provider,omitempty"`
	StartedAt      time.Time      `json:"started_at"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
	Error          datatypes.JSON `json:"error,omitempty"`
	Usage          datatypes.JSON `json:"usage,omitempty"`
}

func newRunWires(runs []model.Run) []conversationRunWire {
	items := make([]conversationRunWire, 0, len(runs))
	for _, run := range runs {
		items = append(items, conversationRunWire{
			ID:             run.ID,
			ConversationID: run.ConversationID,
			TurnID:         run.TurnID,
			RequestID:      run.RequestID,
			Status:         string(run.Status),
			Model:          run.Model,
			Provider:       run.Provider,
			StartedAt:      run.StartedAt,
			CompletedAt:    run.CompletedAt,
			Error:          run.Error,
			Usage:          run.Usage,
		})
	}
	return items
}

type conversationDetailWire struct {
	Conversation conversationWire          `json:"conversation"`
	Messages     []conversationMessageWire `json:"messages"`
}

type conversationRunListWire struct {
	Runs []conversationRunWire `json:"runs"`
}

type runtimeEventWire struct {
	Seq       int            `json:"seq"`
	Channel   string         `json:"channel"`
	Type      string         `json:"type"`
	IDs       datatypes.JSON `json:"ids"`
	Payload   datatypes.JSON `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

type runtimeEventListWire struct {
	Events       []runtimeEventWire `json:"events"`
	HasMore      bool               `json:"hasMore"`
	NextAfterSeq *int               `json:"nextAfterSeq,omitempty"`
}

func getConversationError(status int, code, message string) openapiv1.GetConversationResponseObject {
	switch status {
	case http.StatusUnauthorized:
		return openapiv1.GetConversation401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message))}
	default:
		return openapiv1.GetConversation500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope(code, message))}
	}
}

func listConversationRunsError(status int, code, message string) openapiv1.ListConversationRunsResponseObject {
	switch status {
	case http.StatusBadRequest:
		return openapiv1.ListConversationRuns400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(errorEnvelope(code, message))}
	case http.StatusUnauthorized:
		return openapiv1.ListConversationRuns401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message))}
	case http.StatusNotFound:
		return openapiv1.ListConversationRuns404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(errorEnvelope(code, message))}
	default:
		return openapiv1.ListConversationRuns500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope(code, message))}
	}
}

func listRunEventsError(status int, code, message string) openapiv1.ListRunEventsResponseObject {
	switch status {
	case http.StatusBadRequest:
		return openapiv1.ListRunEvents400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(errorEnvelope(code, message))}
	case http.StatusUnauthorized:
		return openapiv1.ListRunEvents401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message))}
	case http.StatusNotFound:
		return openapiv1.ListRunEvents404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(errorEnvelope(code, message))}
	default:
		return openapiv1.ListRunEvents500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope(code, message))}
	}
}
