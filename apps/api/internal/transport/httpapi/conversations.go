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

const (
	conversationDeletedMessage    = "conversation deleted"
	conversationPinUpdatedMessage = "conversation pin updated"
	conversationUpdatedMessage    = "conversation updated"
	conversationUnsharedMessage   = "conversation unshared"
	conversationTitleUpdatedMsg   = "title updated"
	conversationTitleStartedMsg   = "title generation started"
)

type conversationApplication interface {
	ListConversations(ctx context.Context, userID uuid.UUID, cursor *time.Time, limit int) ([]model.Conversation, bool, error)
	GetConversation(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, []model.Message, error)
	GetConversationByID(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error)
	DeleteConversation(ctx context.Context, id, userID uuid.UUID) error
	PinConversation(ctx context.Context, id, userID uuid.UUID, pinned bool) error
	RenameTitle(ctx context.Context, id, userID uuid.UUID, title string) error
	GenerateTitle(ctx context.Context, id, userID uuid.UUID) error
	UpdateConversationStatus(ctx context.Context, id, userID uuid.UUID, status string) error
	ListRuns(ctx context.Context, conversationID, userID uuid.UUID) ([]model.Run, error)
}

type conversationShareApplication interface {
	ShareConversation(ctx context.Context, conversationID, userID uuid.UUID) (*model.ConversationShare, string, error)
	UnshareConversation(ctx context.Context, conversationID, userID uuid.UUID) error
	GetSharedConversation(ctx context.Context, token string) (*model.ConversationShare, error)
}

type runtimeEventApplication interface {
	ListRunEvents(ctx context.Context, conversationID, runID uuid.UUID, afterSeq, limit int) ([]model.RuntimeEvent, bool, error)
}

func (s *PublicServer) WithConversations(
	conversations conversationApplication,
	shares conversationShareApplication,
	runtimeEvents runtimeEventApplication,
) *PublicServer {
	s.conversations = conversations
	s.shares = shares
	s.runtimeEvents = runtimeEvents
	return s
}

func (s *PublicServer) ListConversations(
	ctx context.Context,
	request openapiv1.ListConversationsRequestObject,
) (openapiv1.ListConversationsResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return listConversationsError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}

	limit := 20
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	var cursor *time.Time
	if request.Params.Cursor != nil && *request.Params.Cursor != "" {
		parsed, parseErr := time.Parse(time.RFC3339, *request.Params.Cursor)
		if parseErr != nil {
			return listConversationsError(http.StatusBadRequest, "INVALID_CURSOR", "cursor must be RFC3339 timestamp"), nil
		}
		cursor = &parsed
	}

	conversations, hasMore, listErr := s.conversations.ListConversations(ctx, userID, cursor, limit)
	if listErr != nil {
		return listConversationsError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list conversations"), nil
	}

	items := make([]conversationWire, 0, len(conversations))
	for _, conversation := range conversations {
		items = append(items, newConversationWire(conversation))
	}
	wire := conversationListWire{Conversations: items, HasMore: hasMore}
	if hasMore && len(conversations) > 0 {
		next := conversations[len(conversations)-1].UpdatedAt.Format(time.RFC3339)
		wire.NextCursor = &next
	}
	body, err := strictOpenAPIConvert[openapiv1.ConversationListResponse]("ConversationListResponse", wire)
	if err != nil {
		return listConversationsError(http.StatusInternalServerError, "INTERNAL_ERROR", "conversation page violates the public contract"), nil
	}
	return openapiv1.ListConversations200JSONResponse(body), nil
}

func (s *PublicServer) GetConversation(
	ctx context.Context,
	request openapiv1.GetConversationRequestObject,
) (openapiv1.GetConversationResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return getConversationError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}

	conversation, messages, getErr := s.conversations.GetConversation(ctx, request.Id, userID)
	if getErr != nil {
		return getConversationError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get conversation"), nil
	}
	if conversation == nil {
		return getConversationError(http.StatusNotFound, "NOT_FOUND", "conversation not found"), nil
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

func (s *PublicServer) UpdateConversation(
	ctx context.Context,
	request openapiv1.UpdateConversationRequestObject,
) (openapiv1.UpdateConversationResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return updateConversationError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if request.Body == nil {
		return updateConversationError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}

	if request.Body.Status != nil {
		if err := s.conversations.UpdateConversationStatus(ctx, request.Id, userID, string(*request.Body.Status)); err != nil {
			return updateConversationError(http.StatusNotFound, "NOT_FOUND", "conversation not found"), nil
		}
	}
	return openapiv1.UpdateConversation200JSONResponse(openapiv1.ConversationMutationResponse{Message: conversationUpdatedMessage}), nil
}

func (s *PublicServer) DeleteConversation(
	ctx context.Context,
	request openapiv1.DeleteConversationRequestObject,
) (openapiv1.DeleteConversationResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return deleteConversationError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}

	if err := s.conversations.DeleteConversation(ctx, request.Id, userID); err != nil {
		return deleteConversationError(http.StatusNotFound, "NOT_FOUND", "conversation not found"), nil
	}
	return openapiv1.DeleteConversation200JSONResponse(openapiv1.ConversationMutationResponse{Message: conversationDeletedMessage}), nil
}

func (s *PublicServer) PinConversation(
	ctx context.Context,
	request openapiv1.PinConversationRequestObject,
) (openapiv1.PinConversationResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return pinConversationError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if request.Body == nil {
		return pinConversationError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}

	pinned := request.Body.Pinned != nil && *request.Body.Pinned
	if err := s.conversations.PinConversation(ctx, request.Id, userID, pinned); err != nil {
		return pinConversationError(http.StatusNotFound, "NOT_FOUND", "conversation not found"), nil
	}
	return openapiv1.PinConversation200JSONResponse(openapiv1.ConversationMutationResponse{Message: conversationPinUpdatedMessage}), nil
}

func (s *PublicServer) RenameConversationTitle(
	ctx context.Context,
	request openapiv1.RenameConversationTitleRequestObject,
) (openapiv1.RenameConversationTitleResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return renameConversationTitleError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if request.Body == nil {
		return renameConversationTitleError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}

	if err := s.conversations.RenameTitle(ctx, request.Id, userID, request.Body.Title); err != nil {
		return renameConversationTitleError(http.StatusNotFound, "NOT_FOUND", "conversation not found"), nil
	}
	return openapiv1.RenameConversationTitle200JSONResponse(openapiv1.ConversationMutationResponse{Message: conversationTitleUpdatedMsg}), nil
}

func (s *PublicServer) GenerateConversationTitle(
	ctx context.Context,
	request openapiv1.GenerateConversationTitleRequestObject,
) (openapiv1.GenerateConversationTitleResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return generateConversationTitleError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}

	if err := s.conversations.GenerateTitle(ctx, request.Id, userID); err != nil {
		return generateConversationTitleError(http.StatusNotFound, "NOT_FOUND", "conversation not found"), nil
	}
	return openapiv1.GenerateConversationTitle202JSONResponse(openapiv1.ConversationMutationResponse{Message: conversationTitleStartedMsg}), nil
}

func (s *PublicServer) ShareConversation(
	ctx context.Context,
	request openapiv1.ShareConversationRequestObject,
) (openapiv1.ShareConversationResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return shareConversationError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}

	share, shareURL, err := s.shares.ShareConversation(ctx, request.Id, userID)
	if err != nil || share == nil {
		return shareConversationError(http.StatusNotFound, "NOT_FOUND", "conversation not found"), nil
	}
	return openapiv1.ShareConversation201JSONResponse(openapiv1.ShareConversationResponse{ShareToken: share.ShareToken, ShareUrl: shareURL}), nil
}

func (s *PublicServer) UnshareConversation(
	ctx context.Context,
	request openapiv1.UnshareConversationRequestObject,
) (openapiv1.UnshareConversationResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return unshareConversationError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}

	if err := s.shares.UnshareConversation(ctx, request.Id, userID); err != nil {
		return unshareConversationError(http.StatusNotFound, "NOT_FOUND", "conversation not found"), nil
	}
	return openapiv1.UnshareConversation200JSONResponse(openapiv1.ConversationMutationResponse{Message: conversationUnsharedMessage}), nil
}

func (s *PublicServer) GetSharedConversation(
	ctx context.Context,
	request openapiv1.GetSharedConversationRequestObject,
) (openapiv1.GetSharedConversationResponseObject, error) {
	share, err := s.shares.GetSharedConversation(ctx, request.Token)
	if err != nil {
		return getSharedConversationError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get shared conversation"), nil
	}
	if share == nil {
		return getSharedConversationError(http.StatusNotFound, "NOT_FOUND", "shared conversation not found"), nil
	}

	body, err := strictOpenAPIConvert[openapiv1.SharedConversationResponse](
		"SharedConversationResponse",
		sharedConversationWire{Title: share.SnapshotTitle, Messages: share.SnapshotMessages},
	)
	if err != nil {
		return getSharedConversationError(http.StatusInternalServerError, "INTERNAL_ERROR", "shared conversation violates the public contract"), nil
	}
	return openapiv1.GetSharedConversation200JSONResponse(body), nil
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
	Title         string         `json:"title,omitempty"`
	TitleStatus   string         `json:"title_status"`
	Status        string         `json:"status"`
	Pinned        bool           `json:"pinned"`
	PinnedAt      *time.Time     `json:"pinned_at,omitempty"`
	DefaultModel  string         `json:"default_model,omitempty"`
	LastMessageAt *time.Time     `json:"last_message_at,omitempty"`
	Metadata      datatypes.JSON `json:"metadata"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

func newConversationWire(conversation model.Conversation) conversationWire {
	return conversationWire{
		ID:            conversation.ID,
		Title:         conversation.Title,
		TitleStatus:   conversation.TitleStatus,
		Status:        conversation.Status,
		Pinned:        conversation.Pinned,
		PinnedAt:      conversation.PinnedAt,
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
			Status:         run.Status,
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

type conversationListWire struct {
	Conversations []conversationWire `json:"conversations"`
	HasMore       bool               `json:"hasMore"`
	NextCursor    *string            `json:"nextCursor,omitempty"`
}

type conversationDetailWire struct {
	Conversation conversationWire          `json:"conversation"`
	Messages     []conversationMessageWire `json:"messages"`
}

type conversationRunListWire struct {
	Runs []conversationRunWire `json:"runs"`
}

type sharedConversationWire struct {
	Title    string         `json:"title"`
	Messages datatypes.JSON `json:"messages"`
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

func listConversationsError(status int, code, message string) openapiv1.ListConversationsResponseObject {
	switch status {
	case http.StatusBadRequest:
		return openapiv1.ListConversations400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(errorEnvelope(code, message))}
	case http.StatusUnauthorized:
		return openapiv1.ListConversations401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message))}
	default:
		return openapiv1.ListConversations500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope(code, message))}
	}
}

func getConversationError(status int, code, message string) openapiv1.GetConversationResponseObject {
	switch status {
	case http.StatusBadRequest:
		return openapiv1.GetConversation400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(errorEnvelope(code, message))}
	case http.StatusUnauthorized:
		return openapiv1.GetConversation401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message))}
	case http.StatusNotFound:
		return openapiv1.GetConversation404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(errorEnvelope(code, message))}
	default:
		return openapiv1.GetConversation500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope(code, message))}
	}
}

func updateConversationError(status int, code, message string) openapiv1.UpdateConversationResponseObject {
	switch status {
	case http.StatusBadRequest:
		return openapiv1.UpdateConversation400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(errorEnvelope(code, message))}
	case http.StatusUnauthorized:
		return openapiv1.UpdateConversation401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message))}
	case http.StatusNotFound:
		return openapiv1.UpdateConversation404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(errorEnvelope(code, message))}
	default:
		return openapiv1.UpdateConversation500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope(code, message))}
	}
}

func deleteConversationError(status int, code, message string) openapiv1.DeleteConversationResponseObject {
	switch status {
	case http.StatusBadRequest:
		return openapiv1.DeleteConversation400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(errorEnvelope(code, message))}
	case http.StatusUnauthorized:
		return openapiv1.DeleteConversation401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message))}
	case http.StatusNotFound:
		return openapiv1.DeleteConversation404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(errorEnvelope(code, message))}
	default:
		return openapiv1.DeleteConversation500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope(code, message))}
	}
}

func pinConversationError(status int, code, message string) openapiv1.PinConversationResponseObject {
	switch status {
	case http.StatusBadRequest:
		return openapiv1.PinConversation400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(errorEnvelope(code, message))}
	case http.StatusUnauthorized:
		return openapiv1.PinConversation401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message))}
	case http.StatusNotFound:
		return openapiv1.PinConversation404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(errorEnvelope(code, message))}
	default:
		return openapiv1.PinConversation500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope(code, message))}
	}
}

func renameConversationTitleError(status int, code, message string) openapiv1.RenameConversationTitleResponseObject {
	switch status {
	case http.StatusBadRequest:
		return openapiv1.RenameConversationTitle400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(errorEnvelope(code, message))}
	case http.StatusUnauthorized:
		return openapiv1.RenameConversationTitle401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message))}
	case http.StatusNotFound:
		return openapiv1.RenameConversationTitle404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(errorEnvelope(code, message))}
	default:
		return openapiv1.RenameConversationTitle500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope(code, message))}
	}
}

func generateConversationTitleError(status int, code, message string) openapiv1.GenerateConversationTitleResponseObject {
	switch status {
	case http.StatusBadRequest:
		return openapiv1.GenerateConversationTitle400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(errorEnvelope(code, message))}
	case http.StatusUnauthorized:
		return openapiv1.GenerateConversationTitle401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message))}
	case http.StatusNotFound:
		return openapiv1.GenerateConversationTitle404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(errorEnvelope(code, message))}
	default:
		return openapiv1.GenerateConversationTitle500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope(code, message))}
	}
}

func shareConversationError(status int, code, message string) openapiv1.ShareConversationResponseObject {
	switch status {
	case http.StatusBadRequest:
		return openapiv1.ShareConversation400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(errorEnvelope(code, message))}
	case http.StatusUnauthorized:
		return openapiv1.ShareConversation401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message))}
	case http.StatusNotFound:
		return openapiv1.ShareConversation404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(errorEnvelope(code, message))}
	default:
		return openapiv1.ShareConversation500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope(code, message))}
	}
}

func unshareConversationError(status int, code, message string) openapiv1.UnshareConversationResponseObject {
	switch status {
	case http.StatusBadRequest:
		return openapiv1.UnshareConversation400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(errorEnvelope(code, message))}
	case http.StatusUnauthorized:
		return openapiv1.UnshareConversation401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message))}
	case http.StatusNotFound:
		return openapiv1.UnshareConversation404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(errorEnvelope(code, message))}
	default:
		return openapiv1.UnshareConversation500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope(code, message))}
	}
}

func getSharedConversationError(status int, code, message string) openapiv1.GetSharedConversationResponseObject {
	switch status {
	case http.StatusBadRequest:
		return openapiv1.GetSharedConversation400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(errorEnvelope(code, message))}
	case http.StatusNotFound:
		return openapiv1.GetSharedConversation404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(errorEnvelope(code, message))}
	default:
		return openapiv1.GetSharedConversation500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope(code, message))}
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
