package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	ctxbuilder "github.com/bodysense/api/internal/context"
	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/stream"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const (
	// maxSSEEvents caps events processed from the AI stream.
	maxSSEEvents = 10000
	// sseTimeout is the maximum duration for the SSE streaming loop.
	sseTimeout = 5 * time.Minute
)

// HTTPError is returned before an SSE stream has been established.
type HTTPError struct {
	Status  int
	Code    string
	Message string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Runtime owns the full chat-turn Module behind the HTTP handler seam.
type Runtime struct {
	conversationService *service.ConversationService
	messageService      *service.MessageService
	runService          *service.RunService
	consultationService *service.ConsultationService
	aiClient            *service.AIClient
	agentToolService    *service.AgentToolService
	interactionService  *service.AgentInteractionService
	outputReviewService *service.OutputReviewService
	contextBuilder      ctxbuilder.Builder
	streamRuntime       *stream.Runtime
}

// NewRuntime creates a chat Runtime.
func NewRuntime(
	conversationService *service.ConversationService,
	messageService *service.MessageService,
	runService *service.RunService,
	consultationService *service.ConsultationService,
	aiClient *service.AIClient,
	contextBuilder ctxbuilder.Builder,
	agentToolService *service.AgentToolService,
	interactionService *service.AgentInteractionService,
	outputReviewService *service.OutputReviewService,
) *Runtime {
	return &Runtime{
		conversationService: conversationService,
		messageService:      messageService,
		runService:          runService,
		consultationService: consultationService,
		aiClient:            aiClient,
		agentToolService:    agentToolService,
		interactionService:  interactionService,
		outputReviewService: outputReviewService,
		contextBuilder:      contextBuilder,
		streamRuntime:       stream.NewRuntime(),
	}
}

// SendMessage executes one chat turn and streams its events to the response writer.
func (r *Runtime) SendMessage(ctx context.Context, w http.ResponseWriter, uid uuid.UUID, req dto.SendMessageRequest) *HTTPError {
	existing, found, err := r.runService.CheckIdempotency(ctx, uid, req.RequestID)
	if err != nil {
		log.Printf("idempotency check failed for user %s requestID %s: %v", uid, req.RequestID, err)
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check idempotency")
	}
	if found {
		if existing.Status == "running" {
			return httpErr(http.StatusConflict, "RUN_IN_PROGRESS", "a run with this request ID is already in progress")
		}
		r.replayCompletedRun(ctx, w, existing)
		return nil
	}

	conversationID, isDraft, setupErr := r.prepareConversation(ctx, uid, req)
	if setupErr != nil {
		return setupErr
	}

	userSeq, err := r.messageService.GetNextSeq(ctx, conversationID)
	if err != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get message sequence")
	}
	assistantSeq := userSeq + 1
	turnID := uuid.New()

	partsJSON, err := json.Marshal(req.Message.Parts)
	if err != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to marshal message parts")
	}

	var userMetadata datatypes.JSON
	if len(req.Message.Metadata) > 0 {
		userMetadata = datatypes.JSON(req.Message.Metadata)
	} else {
		userMetadata = datatypes.JSON("{}")
	}

	userMsg, err := r.messageService.CreateMessage(ctx, conversationID, turnID, "user", datatypes.JSON(partsJSON), userSeq, "completed", userMetadata)
	if err != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create user message")
	}

	assistantMsg, err := r.messageService.CreateMessage(ctx, conversationID, turnID, "assistant", datatypes.JSON("[]"), assistantSeq, "streaming", datatypes.JSON("{}"))
	if err != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create assistant message")
	}

	run, err := r.runService.CreateRun(ctx, conversationID, turnID, req.RequestID, uid, "")
	if err != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create run")
	}

	runIDStr := run.ID.String()
	if updateErr := r.conversationService.UpdateActiveRunID(ctx, conversationID, uid, &run.ID, runIDStr); updateErr != nil {
		log.Printf("failed to set active_run_id for conversation %s: %v", conversationID, updateErr)
	}

	baseIDs := dto.StreamEventIDs{
		ConversationID: conversationID.String(),
		RunID:          run.ID.String(),
		TurnID:         turnID.String(),
	}
	sw := r.streamRuntime.NewWriter(w, baseIDs)

	r.emitTurnStarted(ctx, sw, baseIDs, req, isDraft, userMsg, assistantMsg)

	chatReq, _, err := r.contextBuilder.BuildChatContext(ctx, ctxbuilder.BuildChatContextInput{
		ConversationID: conversationID,
		TurnID:         turnID,
		UserID:         uid,
		ContextDTO:     req.Context,
		MessageParts:   req.Message.Parts,
		IsDraft:        isDraft,
	})
	if err != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to build chat context")
	}

	phase := chatReq.Phase
	streamCtx, cancel := context.WithTimeout(ctx, sseTimeout)
	defer cancel()

	events, err := r.aiClient.ChatStream(streamCtx, *chatReq)
	if err != nil {
		_ = sw.SendNew(ctx, "message", "message.failed", dto.StreamEventIDs{
			ConversationID: conversationID.String(),
			RunID:          run.ID.String(),
			TurnID:         turnID.String(),
			MessageID:      assistantMsg.ID.String(),
		}, map[string]any{"status": "failed", "error": map[string]any{"message": "AI service unavailable"}}, assistantMsg.ID.String())
		_ = sw.SendNew(ctx, "stream", "stream.done", baseIDs, map[string]any{}, "")
		_ = r.runService.FailRun(ctx, run.ID, uid, map[string]any{"message": "AI service connection failed"})
		return nil
	}

	result, stopped := r.streamAIEvents(ctx, sw, events, streamState{
		UID:             uid,
		ConversationID:  conversationID,
		TurnID:          turnID,
		Run:             run,
		AssistantMsg:    assistantMsg,
		BaseIDs:         baseIDs,
		CurrentPhase:    phase,
		RequestDone:     ctx.Done(),
		AssistantMsgID:  assistantMsg.ID.String(),
		ConversationStr: conversationID.String(),
	})
	if stopped {
		return nil
	}

	r.persistCompletedTurn(ctx, uid, conversationID, run, assistantMsg, result)
	_ = sw.SendNew(ctx, "message", "message.completed", dto.StreamEventIDs{
		ConversationID: conversationID.String(),
		RunID:          run.ID.String(),
		TurnID:         turnID.String(),
		MessageID:      assistantMsg.ID.String(),
	}, map[string]any{"status": "completed", "finish_reason": "stop", "usage": result.Usage}, assistantMsg.ID.String())
	_ = sw.SendNew(ctx, "stream", "stream.done", baseIDs, map[string]any{}, "")

	r.maybeGenerateTitle(ctx, conversationID, uid)
	return nil
}

func (r *Runtime) prepareConversation(ctx context.Context, uid uuid.UUID, req dto.SendMessageRequest) (uuid.UUID, bool, *HTTPError) {
	if req.ConversationID != nil {
		conversationID, err := uuid.Parse(*req.ConversationID)
		if err != nil {
			return uuid.Nil, false, httpErr(http.StatusBadRequest, "INVALID_CONVERSATION_ID", "invalid conversation id format")
		}
		return conversationID, false, nil
	}

	conv, err := r.conversationService.CreateConversation(ctx, uid, "")
	if err != nil {
		return uuid.Nil, false, httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create conversation")
	}
	if req.Context != nil && req.Context.Entry == "consultation" {
		if csErr := r.consultationService.CreateConsultation(ctx, conv.ID, uid); csErr != nil {
			log.Printf("failed to create consultation session for conversation %s: %v", conv.ID, csErr)
		}
	}
	return conv.ID, true, nil
}

func (r *Runtime) emitTurnStarted(ctx context.Context, sw *stream.StreamWriter, baseIDs dto.StreamEventIDs, req dto.SendMessageRequest, isDraft bool, userMsg, assistantMsg *model.Message) {
	if isDraft {
		draftID := ""
		if req.ClientDraftID != "" {
			draftID = req.ClientDraftID
		}
		if err := sw.SendNew(ctx, "conversation", "conversation.created", baseIDs, map[string]any{"replaces_draft_id": draftID}, ""); err != nil {
			log.Printf("SSE write error (conversation.created): %v", err)
		}
	}

	if err := sw.SendNew(ctx, "message", "message.persisted", dto.StreamEventIDs{
		ConversationID: userMsg.ConversationID.String(),
		TurnID:         userMsg.TurnID.String(),
		MessageID:      userMsg.ID.String(),
	}, map[string]any{"client_message_id": req.ClientMessageID, "role": "user"}, userMsg.ID.String()); err != nil {
		log.Printf("SSE write error (message.persisted): %v", err)
	}

	if err := sw.SendNew(ctx, "message", "message.created", dto.StreamEventIDs{
		ConversationID: assistantMsg.ConversationID.String(),
		TurnID:         assistantMsg.TurnID.String(),
		MessageID:      assistantMsg.ID.String(),
	}, map[string]any{"role": "assistant", "status": "streaming"}, assistantMsg.ID.String()); err != nil {
		log.Printf("SSE write error (message.created): %v", err)
	}
}

type streamState struct {
	UID             uuid.UUID
	ConversationID  uuid.UUID
	TurnID          uuid.UUID
	Run             *model.Run
	AssistantMsg    *model.Message
	BaseIDs         dto.StreamEventIDs
	CurrentPhase    string
	RequestDone     <-chan struct{}
	AssistantMsgID  string
	ConversationStr string
}

type streamResult struct {
	AssistantParts     []map[string]any
	Usage              any
	ProviderResponseID string
	GovernanceResult   datatypes.JSON
}

func (r *Runtime) streamAIEvents(ctx context.Context, sw *stream.StreamWriter, events <-chan dto.StreamEvent, state streamState) (streamResult, bool) {
	result := streamResult{}
	eventCount := 0
	phase := state.CurrentPhase

	for {
		select {
		case <-state.RequestDone:
			log.Printf("client disconnected during stream for conversation %s", state.ConversationID)
			_ = r.runService.FailRun(context.Background(), state.Run.ID, state.UID, map[string]any{"message": "client disconnected"})
			_ = r.messageService.UpdateMessageStatus(context.Background(), state.AssistantMsg.ID, state.ConversationID, "aborted")
			r.clearActiveRun(context.Background(), state.ConversationID, state.UID)
			return result, true

		case event, ok := <-events:
			if !ok {
				return result, false
			}

			eventCount++
			if eventCount > maxSSEEvents {
				log.Printf("event count exceeded limit for conversation %s", state.ConversationID)
				_ = sw.SendNew(ctx, "message", "message.failed", dto.StreamEventIDs{
					ConversationID: state.ConversationID.String(),
					RunID:          state.Run.ID.String(),
					TurnID:         state.TurnID.String(),
					MessageID:      state.AssistantMsg.ID.String(),
				}, map[string]any{"status": "failed", "error": map[string]any{"message": "stream exceeded maximum event count"}}, state.AssistantMsgID)
				_ = sw.SendNew(ctx, "stream", "stream.done", state.BaseIDs, map[string]any{}, "")
				_ = r.runService.FailRun(ctx, state.Run.ID, state.UID, map[string]any{"message": "event count limit exceeded"})
				r.clearActiveRun(ctx, state.ConversationID, state.UID)
				return result, true
			}

			if r.handleAIEvent(ctx, sw, event, state, &result, &phase) {
				return result, true
			}
		}
	}
}

func (r *Runtime) handleAIEvent(ctx context.Context, sw *stream.StreamWriter, event dto.StreamEvent, state streamState, result *streamResult, phase *string) bool {
	switch event.Type {
	case "message.text.delta":
		var payload struct {
			Delta string `json:"delta"`
		}
		_ = event.PayloadAs(&payload)
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, "text.delta")
		result.AssistantParts = append(result.AssistantParts, map[string]any{"type": "text", "text": payload.Delta})

	case "tool.call":
		var payload struct {
			Tool string          `json:"tool"`
			Args json.RawMessage `json:"args"`
		}
		_ = event.PayloadAs(&payload)
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, "tool.call")
		result.AssistantParts = append(result.AssistantParts, map[string]any{"type": "tool_call", "tool": payload.Tool, "args": payload.Args})
		msgID := state.AssistantMsg.ID
		r.agentToolService.RecordToolCall(ctx, state.Run.ID, state.ConversationID, &msgID, event.IDs.ToolCallID, payload.Tool, datatypes.JSON(payload.Args))

	case "tool.result":
		var payload struct {
			Tool   string          `json:"tool"`
			Result json.RawMessage `json:"result"`
		}
		_ = event.PayloadAs(&payload)
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, "tool.result")
		result.AssistantParts = append(result.AssistantParts, map[string]any{"type": "tool_result", "tool": payload.Tool, "result": payload.Result})
		r.agentToolService.RecordToolResult(ctx, state.Run.ID, event.IDs.ToolCallID, datatypes.JSON(payload.Result), toolResultIsError(payload.Result))

	case "state.extracted_info.upsert", "state.diagnosis.ready", "state.treatment.ready", "source.citation.added", "safety.red_flag.detected":
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, event.Type)

	case "state.interaction.required":
		return r.handleInteractionRequired(ctx, sw, event, state, result)

	case "state.phase.changed":
		var payload struct {
			From   string `json:"from,omitempty"`
			To     string `json:"to"`
			Reason string `json:"reason"`
		}
		_ = event.PayloadAs(&payload)
		if payload.From == "" {
			payload.From = *phase
		}
		if patched, err := json.Marshal(payload); err == nil {
			event.Payload = patched
		}
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, "phase_change")
		*phase = payload.To
		if err := r.consultationService.UpdatePhase(ctx, state.ConversationID, state.UID, payload.To); err != nil {
			log.Printf("failed to update phase for conversation %s: %v", state.ConversationID, err)
		}

	case "usage.reported":
		var payload struct {
			Usage json.RawMessage `json:"usage"`
		}
		_ = event.PayloadAs(&payload)
		result.Usage = payload.Usage
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, "usage.reported")

	case "stream.done":
		var payload struct {
			ResponseID string          `json:"response_id"`
			Usage      json.RawMessage `json:"usage"`
			Governance json.RawMessage `json:"governance"`
		}
		_ = event.PayloadAs(&payload)
		if payload.ResponseID != "" {
			result.ProviderResponseID = payload.ResponseID
		}
		if len(payload.Usage) > 0 {
			result.Usage = payload.Usage
		}
		if len(payload.Governance) > 0 {
			result.GovernanceResult = datatypes.JSON(payload.Governance)
		}

	case "stream.error":
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, "stream.error")
	}
	return false
}

func (r *Runtime) handleInteractionRequired(ctx context.Context, sw *stream.StreamWriter, event dto.StreamEvent, state streamState, result *streamResult) bool {
	var payload struct {
		InteractionID string          `json:"interaction_id"`
		Question      json.RawMessage `json:"question"`
	}
	_ = event.PayloadAs(&payload)

	interaction, err := r.interactionService.CreatePendingInteraction(ctx, state.Run.ID, state.ConversationID, event.IDs.ToolCallID, datatypes.JSON(payload.Question))
	if err != nil {
		log.Printf("failed to create pending interaction for conversation %s: %v", state.ConversationID, err)
		_ = sw.SendNew(ctx, "stream", "stream.error", state.BaseIDs, map[string]any{"message": "failed to persist user interaction"}, "")
		r.clearActiveRun(ctx, state.ConversationID, state.UID)
		return true
	}

	interactionID := interaction.ID.String()
	event.IDs.InteractionID = interactionID
	payload.InteractionID = interactionID
	if patched, err := json.Marshal(payload); err == nil {
		event.Payload = patched
	}
	r.sendEvent(ctx, sw, event, state.AssistantMsgID, "interaction.required")

	// Save accumulated parts and keep status as completed
	finalPartsJSON, _ := json.Marshal(result.AssistantParts)
	if err := r.messageService.UpdateMessageCompleted(ctx, state.AssistantMsg.ID, state.ConversationID, datatypes.JSON(finalPartsJSON), nil, nil); err != nil {
		log.Printf("failed to update assistant message %s: %v", state.AssistantMsg.ID, err)
	}

	_ = sw.SendNew(ctx, "stream", "stream.done", state.BaseIDs, map[string]any{}, "")
	r.clearActiveRun(ctx, state.ConversationID, state.UID)
	return true
}

func (r *Runtime) persistCompletedTurn(ctx context.Context, uid uuid.UUID, conversationID uuid.UUID, run *model.Run, assistantMsg *model.Message, result streamResult) {
	finalPartsJSON, _ := json.Marshal(result.AssistantParts)
	if err := r.messageService.UpdateMessageCompleted(ctx, assistantMsg.ID, conversationID, datatypes.JSON(finalPartsJSON), nil, nil); err != nil {
		log.Printf("failed to update assistant message %s: %v", assistantMsg.ID, err)
	}
	if err := r.conversationService.UpdateLastMessageAt(ctx, conversationID, uid); err != nil {
		log.Printf("failed to update last_message_at for conversation %s: %v", conversationID, err)
	}
	if err := r.runService.CompleteRun(ctx, run.ID, uid, result.Usage, result.ProviderResponseID); err != nil {
		log.Printf("failed to complete run %s: %v", run.ID, err)
	}
	r.recordGovernance(ctx, uid, conversationID, run.ID, result.GovernanceResult)
	r.clearActiveRun(ctx, conversationID, uid)
}

func (r *Runtime) recordGovernance(ctx context.Context, uid uuid.UUID, conversationID uuid.UUID, runID uuid.UUID, governanceResult datatypes.JSON) {
	if len(governanceResult) == 0 {
		return
	}
	var govStatus string
	var govIssues datatypes.JSON
	var govValidatedOutput datatypes.JSON
	var govPayload struct {
		Status          string          `json:"status"`
		Issues          json.RawMessage `json:"issues"`
		ValidatedOutput json.RawMessage `json:"validated_output"`
	}
	if err := json.Unmarshal(governanceResult, &govPayload); err == nil {
		govStatus = govPayload.Status
		govIssues = datatypes.JSON(govPayload.Issues)
		if len(govPayload.ValidatedOutput) > 0 {
			govValidatedOutput = datatypes.JSON(govPayload.ValidatedOutput)
		}
	} else {
		govStatus = "unknown"
		govIssues = datatypes.JSON("[]")
	}
	r.outputReviewService.RecordReview(ctx, "consultation_reply", govStatus, &uid, &runID, nil, &conversationID, govIssues, govValidatedOutput, governanceResult)
}

func (r *Runtime) replayCompletedRun(ctx context.Context, w http.ResponseWriter, run *model.Run) {
	baseIDs := dto.StreamEventIDs{
		ConversationID: run.ConversationID.String(),
		RunID:          run.ID.String(),
		TurnID:         run.TurnID.String(),
	}
	sw := r.streamRuntime.NewWriter(w, baseIDs)
	_ = sw.SendNew(ctx, "stream", "stream.error", baseIDs, map[string]any{"message": "this request has already been processed", "status": run.Status}, "")
	_ = sw.SendNew(ctx, "stream", "stream.done", baseIDs, map[string]any{}, "")
}

func (r *Runtime) clearActiveRun(ctx context.Context, conversationID, userID uuid.UUID) {
	if err := r.conversationService.UpdateActiveRunID(ctx, conversationID, userID, nil, ""); err != nil {
		log.Printf("failed to clear active_run_id for conversation %s: %v", conversationID, err)
	}
}

func (r *Runtime) maybeGenerateTitle(ctx context.Context, conversationID, userID uuid.UUID) {
	conv, err := r.conversationService.GetConversationByID(ctx, conversationID, userID)
	if err != nil || conv == nil {
		return
	}
	if conv.TitleStatus == "pending" && conv.Title == "" {
		if err := r.conversationService.GenerateTitle(ctx, conversationID, userID); err != nil {
			log.Printf("failed to trigger title generation for conversation %s: %v", conversationID, err)
		}
	}
}

func (r *Runtime) sendEvent(ctx context.Context, sw *stream.StreamWriter, event dto.StreamEvent, messageID string, label string) {
	if err := sw.Send(ctx, event, messageID); err != nil {
		log.Printf("SSE write error (%s): %v", label, err)
	}
}

func toolResultIsError(result json.RawMessage) bool {
	var parsed struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return false
	}
	return parsed.Status == "error" || parsed.Error != ""
}

func httpErr(status int, code string, message string) *HTTPError {
	return &HTTPError{Status: status, Code: code, Message: message}
}
