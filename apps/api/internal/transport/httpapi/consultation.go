package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	consultationruntime "github.com/bodysense/api/internal/consultation"
	"github.com/bodysense/api/internal/dto"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type consultationRuntimeApplication interface {
	StartRun(context.Context, http.ResponseWriter, uuid.UUID, consultationruntime.StartRunInput) *consultationruntime.HTTPError
	CancelRun(context.Context, uuid.UUID, uuid.UUID, string) *consultationruntime.HTTPError
	ResumeInteraction(context.Context, http.ResponseWriter, uuid.UUID, uuid.UUID, uuid.UUID, consultationruntime.ResumeInteractionInput) *consultationruntime.HTTPError
}

type consultationSessionApplication interface {
	GetConsultation(context.Context, uuid.UUID, uuid.UUID) (*model.ConsultationSession, error)
}

type consultationInteractionApplication interface {
	GetPendingInteractions(context.Context, uuid.UUID) ([]model.AgentInteraction, error)
	GetInteractionMetrics(context.Context, uuid.UUID, *uuid.UUID) (service.InteractionMetrics, error)
}

type consultationReplayApplication interface {
	HistoricalReplay(context.Context, uuid.UUID, uuid.UUID) (*service.ConsultationRunDecision, error)
	CounterfactualReplay(context.Context, uuid.UUID, uuid.UUID, string) (*service.ConsultationRunDecision, error)
}

type consultationThreadApplication interface {
	RefreshAndGetThread(context.Context, uuid.UUID, uuid.UUID) (*model.ThreadProjection, []model.ThreadProjectionMessage, []model.ThreadProjectionToolCall, *uuid.UUID, []model.RuntimeEvent, error)
}

type consultationBodyStateApplication interface {
	GetSnapshot(context.Context, uuid.UUID, int) (*service.BodyStateSnapshot, error)
}

func (s *PublicServer) WithConsultation(
	runtime consultationRuntimeApplication,
	sessions consultationSessionApplication,
	interactions consultationInteractionApplication,
	replay consultationReplayApplication,
	threads consultationThreadApplication,
	bodyState consultationBodyStateApplication,
) *PublicServer {
	s.consultationRuntime = runtime
	s.consultationSessions = sessions
	s.consultationInteractions = interactions
	s.consultationReplay = replay
	s.consultationThreads = threads
	s.consultationBodyState = bodyState
	return s
}

// consultationStreamedResponse is returned only after Runtime has already
// written/flushed the SSE response directly to Gin's real ResponseWriter.
type consultationStreamedResponse struct{}

func (consultationStreamedResponse) VisitStartConsultationRunResponse(http.ResponseWriter) error {
	return nil
}
func (consultationStreamedResponse) VisitResumeConsultationInteractionResponse(http.ResponseWriter) error {
	return nil
}

type consultationHTTPErrorResponse struct {
	status  int
	code    string
	message string
}

func (r consultationHTTPErrorResponse) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	return json.NewEncoder(w).Encode(dto.NewErrorResponse(r.code, r.message))
}

func (r consultationHTTPErrorResponse) VisitStartConsultationRunResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r consultationHTTPErrorResponse) VisitCancelConsultationRunResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r consultationHTTPErrorResponse) VisitReplayConsultationRunResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r consultationHTTPErrorResponse) VisitReplayConsultationRunCounterfactualResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r consultationHTTPErrorResponse) VisitGetConsultationResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r consultationHTTPErrorResponse) VisitGetConsultationThreadResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r consultationHTTPErrorResponse) VisitResumeConsultationInteractionResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r consultationHTTPErrorResponse) VisitGetConsultationInteractionMetricsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func runtimeHTTPError(err *consultationruntime.HTTPError) consultationHTTPErrorResponse {
	return consultationHTTPErrorResponse{status: err.Status, code: string(err.Code), message: err.Message}
}

func internalConsultationError(message string) consultationHTTPErrorResponse {
	return consultationHTTPErrorResponse{status: http.StatusInternalServerError, code: "INTERNAL_ERROR", message: message}
}

func (s *PublicServer) StartConsultationRun(
	ctx context.Context,
	request openapiv1.StartConsultationRunRequestObject,
) (openapiv1.StartConsultationRunResponseObject, error) {
	uid, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return consultationHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "missing authenticated user"}, nil
	}
	if s.consultationRuntime == nil || request.Body == nil {
		return consultationHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "CONSULTATION_RUNTIME_UNAVAILABLE", message: "consultation runtime is not configured"}, nil
	}
	input, err := mapStartConsultationRunInput(*request.Body)
	if err != nil {
		return consultationHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: err.Error()}, nil
	}
	ginCtx, ginOK := ctx.(*gin.Context)
	if !ginOK {
		return internalConsultationError("missing HTTP streaming context"), nil
	}
	if runErr := s.consultationRuntime.StartRun(ginCtx.Request.Context(), ginCtx.Writer, uid, input); runErr != nil {
		return runtimeHTTPError(runErr), nil
	}
	return consultationStreamedResponse{}, nil
}

func mapStartConsultationRunInput(body openapiv1.StartConsultationRunRequest) (consultationruntime.StartRunInput, error) {
	var conversationID *string
	if body.ConversationId != nil {
		value := body.ConversationId.String()
		conversationID = &value
	}
	parts := make([]consultationruntime.PartInput, 0, len(body.Message.Parts))
	for _, raw := range body.Message.Parts {
		value, err := raw.ValueByDiscriminator()
		if err != nil {
			return consultationruntime.StartRunInput{}, err
		}
		switch part := value.(type) {
		case openapiv1.ConsultationTextPart:
			parts = append(parts, consultationruntime.PartInput{Type: "text", Text: part.Text})
		case openapiv1.ConsultationImagePart:
			parts = append(parts, consultationruntime.PartInput{
				Type:     "image",
				UploadID: part.UploadId.String(),
				MimeType: stringValue(part.MimeType),
				ImageURL: stringValue(part.ImageUrl),
			})
		default:
			return consultationruntime.StartRunInput{}, errors.New("unsupported consultation message part")
		}
	}
	metadata := json.RawMessage(`{}`)
	if body.Message.Metadata != nil {
		encoded, err := json.Marshal(*body.Message.Metadata)
		if err != nil {
			return consultationruntime.StartRunInput{}, err
		}
		metadata = encoded
	}
	return consultationruntime.StartRunInput{
		ConversationID:  conversationID,
		ClientMessageID: body.ClientMessageId,
		RequestID:       body.RequestId,
		Message: consultationruntime.MessageInput{
			Role:     string(body.Message.Role),
			Parts:    parts,
			Metadata: metadata,
		},
	}, nil
}

func (s *PublicServer) CancelConsultationRun(
	ctx context.Context,
	request openapiv1.CancelConsultationRunRequestObject,
) (openapiv1.CancelConsultationRunResponseObject, error) {
	uid, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return consultationHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "missing authenticated user"}, nil
	}
	if s.consultationRuntime == nil {
		return consultationHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "CONSULTATION_RUNTIME_UNAVAILABLE", message: "consultation runtime is not configured"}, nil
	}
	reason := ""
	if request.Body != nil && request.Body.Reason != nil {
		reason = *request.Body.Reason
	}
	if runErr := s.consultationRuntime.CancelRun(ctx, uid, request.Id, reason); runErr != nil {
		return runtimeHTTPError(runErr), nil
	}
	return openapiv1.CancelConsultationRun200JSONResponse{
		Status: openapiv1.CancelConsultationRunResponseStatus("cancelled"),
		RunId:  request.Id,
	}, nil
}

func (s *PublicServer) GetConsultation(
	ctx context.Context,
	request openapiv1.GetConsultationRequestObject,
) (openapiv1.GetConsultationResponseObject, error) {
	uid, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return consultationHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "missing authenticated user"}, nil
	}
	if s.consultationSessions == nil || s.consultationInteractions == nil {
		return consultationHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "CONSULTATION_UNAVAILABLE", message: "consultation services are not configured"}, nil
	}
	session, err := s.consultationSessions.GetConsultation(ctx, request.Id, uid)
	if err != nil {
		return internalConsultationError("failed to get consultation"), nil
	}
	if session == nil {
		return consultationHTTPErrorResponse{status: http.StatusNotFound, code: "NOT_FOUND", message: "consultation not found"}, nil
	}
	pending, err := s.consultationInteractions.GetPendingInteractions(ctx, request.Id)
	if err != nil {
		return internalConsultationError("failed to get pending interactions"), nil
	}
	wire := consultationSessionWire{
		ConversationID:      session.ConversationID,
		Phase:               string(session.Phase),
		ExtractedInfo:       rawJSONArray(session.ExtractedInfo),
		PendingInteractions: pending,
		CreatedAt:           session.CreatedAt,
		UpdatedAt:           session.UpdatedAt,
		EndedAt:             session.EndedAt,
	}
	body, err := strictOpenAPIConvert[openapiv1.ConsultationSessionResponse]("ConsultationSessionResponse", wire)
	if err != nil {
		return nil, err
	}
	return openapiv1.GetConsultation200JSONResponse(body), nil
}

type consultationSessionWire struct {
	ConversationID      uuid.UUID                `json:"conversation_id"`
	Phase               string                   `json:"phase"`
	ExtractedInfo       json.RawMessage          `json:"extracted_info"`
	PendingInteractions []model.AgentInteraction `json:"pending_interactions"`
	CreatedAt           time.Time                `json:"created_at"`
	UpdatedAt           time.Time                `json:"updated_at"`
	EndedAt             *time.Time               `json:"ended_at"`
}

func (s *PublicServer) ResumeConsultationInteraction(
	ctx context.Context,
	request openapiv1.ResumeConsultationInteractionRequestObject,
) (openapiv1.ResumeConsultationInteractionResponseObject, error) {
	uid, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return consultationHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "missing authenticated user"}, nil
	}
	if s.consultationRuntime == nil || request.Body == nil {
		return consultationHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "CONSULTATION_RUNTIME_UNAVAILABLE", message: "consultation runtime is not configured"}, nil
	}
	answer, err := json.Marshal(request.Body.Answer)
	if err != nil {
		return consultationHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "invalid interaction answer"}, nil
	}
	ginCtx, ginOK := ctx.(*gin.Context)
	if !ginOK {
		return internalConsultationError("missing HTTP streaming context"), nil
	}
	if runErr := s.consultationRuntime.ResumeInteraction(
		ginCtx.Request.Context(), ginCtx.Writer, uid, request.Id, request.InteractionId,
		consultationruntime.ResumeInteractionInput{RequestID: request.Body.RequestId, Answer: answer},
	); runErr != nil {
		return runtimeHTTPError(runErr), nil
	}
	return consultationStreamedResponse{}, nil
}

func (s *PublicServer) GetConsultationInteractionMetrics(
	ctx context.Context,
	request openapiv1.GetConsultationInteractionMetricsRequestObject,
) (openapiv1.GetConsultationInteractionMetricsResponseObject, error) {
	uid, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return consultationHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "missing authenticated user"}, nil
	}
	if s.consultationInteractions == nil {
		return consultationHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "CONSULTATION_UNAVAILABLE", message: "interaction service is not configured"}, nil
	}
	metrics, err := s.consultationInteractions.GetInteractionMetrics(ctx, uid, &request.Id)
	if err != nil {
		if errors.Is(err, service.ErrConversationNotFound) {
			return consultationHTTPErrorResponse{status: http.StatusNotFound, code: "NOT_FOUND", message: "consultation not found or access denied"}, nil
		}
		return internalConsultationError("failed to compute interaction metrics"), nil
	}
	return openapiv1.GetConsultationInteractionMetrics200JSONResponse{
		Total:          metrics.Total,
		Answered:       metrics.Answered,
		Expired:        metrics.Expired,
		Pending:        metrics.Pending,
		AnswerRate:     metrics.AnswerRate,
		ExpireRate:     metrics.ExpireRate,
		AvgWaitSeconds: metrics.AvgWaitSeconds,
	}, nil
}

func (s *PublicServer) ReplayConsultationRun(
	ctx context.Context,
	request openapiv1.ReplayConsultationRunRequestObject,
) (openapiv1.ReplayConsultationRunResponseObject, error) {
	uid, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return consultationHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "missing authenticated user"}, nil
	}
	if s.consultationReplay == nil {
		return consultationHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "REPLAY_UNAVAILABLE", message: "replay service not configured"}, nil
	}
	decision, err := s.consultationReplay.HistoricalReplay(ctx, uid, request.Id)
	if err != nil {
		if errors.Is(err, service.ErrConsultationReplayUnavailable) {
			return consultationHTTPErrorResponse{status: http.StatusUnprocessableEntity, code: "REPLAY_UNAVAILABLE", message: "run predates North-Star provenance"}, nil
		}
		return consultationHTTPErrorResponse{status: http.StatusInternalServerError, code: "REPLAY_FAILED", message: err.Error()}, nil
	}
	return replayConsultationDecisionResponse(decision, false)
}

func (s *PublicServer) ReplayConsultationRunCounterfactual(
	ctx context.Context,
	request openapiv1.ReplayConsultationRunCounterfactualRequestObject,
) (openapiv1.ReplayConsultationRunCounterfactualResponseObject, error) {
	uid, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return consultationHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "missing authenticated user"}, nil
	}
	if s.consultationReplay == nil || request.Body == nil {
		return consultationHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "REPLAY_UNAVAILABLE", message: "replay service not configured"}, nil
	}
	decision, err := s.consultationReplay.CounterfactualReplay(ctx, uid, request.Id, request.Body.ConfigurationId)
	if err != nil {
		if errors.Is(err, service.ErrConsultationReplayUnavailable) {
			return consultationHTTPErrorResponse{status: http.StatusUnprocessableEntity, code: "REPLAY_UNAVAILABLE", message: "run predates North-Star provenance"}, nil
		}
		return consultationHTTPErrorResponse{status: http.StatusInternalServerError, code: "REPLAY_FAILED", message: err.Error()}, nil
	}
	body, convertErr := strictOpenAPIConvert[openapiv1.ConsultationRunDecision]("ConsultationRunDecision", decision)
	if convertErr != nil {
		return nil, convertErr
	}
	return openapiv1.ReplayConsultationRunCounterfactual200JSONResponse(body), nil
}

func replayConsultationDecisionResponse(
	decision *service.ConsultationRunDecision,
	_ bool,
) (openapiv1.ReplayConsultationRunResponseObject, error) {
	body, err := strictOpenAPIConvert[openapiv1.ConsultationRunDecision]("ConsultationRunDecision", decision)
	if err != nil {
		return nil, err
	}
	return openapiv1.ReplayConsultationRun200JSONResponse(body), nil
}

func (s *PublicServer) GetConsultationThread(
	ctx context.Context,
	request openapiv1.GetConsultationThreadRequestObject,
) (openapiv1.GetConsultationThreadResponseObject, error) {
	uid, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return consultationHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "missing authenticated user"}, nil
	}
	if s.consultationThreads == nil || s.consultationBodyState == nil {
		return consultationHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "CONSULTATION_UNAVAILABLE", message: "consultation thread service is not configured"}, nil
	}
	projection, messages, toolCalls, activeTurnRunID, activeTurnEvents, err := s.consultationThreads.RefreshAndGetThread(ctx, request.Id, uid)
	if err != nil {
		return internalConsultationError("failed to get consultation thread"), nil
	}
	if projection == nil {
		return consultationHTTPErrorResponse{status: http.StatusNotFound, code: "NOT_FOUND", message: "consultation thread not found"}, nil
	}
	bodyState, _ := s.consultationBodyState.GetSnapshot(ctx, uid, 20)
	wire := consultationThreadWire{
		ConversationID: projection.ConversationID,
		Conversation: consultationThreadConversationWire{
			ID:            projection.ConversationID,
			Title:         projection.Title,
			TitleStatus:   projection.TitleStatus,
			Status:        projection.Status,
			Pinned:        projection.Pinned,
			PinnedAt:      projection.PinnedAt,
			DefaultModel:  nullableString(projection.DefaultModel),
			LastMessageAt: projection.LastMessageAt,
			Metadata:      rawJSONObject(projection.Metadata),
			MessageCount:  len(messages),
			CreatedAt:     projection.ConversationCreatedAt,
			UpdatedAt:     projection.ConversationUpdatedAt,
		},
		Phase:               string(projection.Phase),
		ExtractedInfo:       rawJSONArray(projection.ExtractedInfo),
		BodyState:           bodyState,
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
	Title         string          `json:"title"`
	TitleStatus   string          `json:"title_status"`
	Status        string          `json:"status"`
	Pinned        bool            `json:"pinned"`
	PinnedAt      *time.Time      `json:"pinned_at"`
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
	BodyState           *service.BodyStateSnapshot         `json:"body_state"`
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
