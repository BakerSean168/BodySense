package consultation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/stream"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const (
	maxSSEEvents = 10000
	sseTimeout   = 5 * time.Minute
)

type HTTPError struct {
	Status  int
	Code    string
	Message string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type Runtime struct {
	conversationService     *service.ConversationService
	consultationService     *service.ConsultationService
	profileService          *service.ProfileService
	messageService          *service.MessageService
	runService              *service.RunService
	aiClient                *service.AIClient
	agentToolService        *service.AgentToolService
	interactionService      *service.AgentInteractionService
	outputReviewService     *service.OutputReviewService
	threadProjectionService *service.ThreadProjectionService
	runtimeEventService     *service.RuntimeEventService
	streamRuntime           *stream.Runtime
}

func NewRuntime(
	conversationService *service.ConversationService,
	consultationService *service.ConsultationService,
	profileService *service.ProfileService,
	messageService *service.MessageService,
	runService *service.RunService,
	aiClient *service.AIClient,
	agentToolService *service.AgentToolService,
	interactionService *service.AgentInteractionService,
	outputReviewService *service.OutputReviewService,
	threadProjectionService *service.ThreadProjectionService,
	runtimeEventService *service.RuntimeEventService,
) *Runtime {
	return &Runtime{
		conversationService:     conversationService,
		consultationService:     consultationService,
		profileService:          profileService,
		messageService:          messageService,
		runService:              runService,
		aiClient:                aiClient,
		agentToolService:        agentToolService,
		interactionService:      interactionService,
		outputReviewService:     outputReviewService,
		threadProjectionService: threadProjectionService,
		runtimeEventService:     runtimeEventService,
		streamRuntime:           stream.NewRuntime(),
	}
}

// StartRun handles a unified consultation run request.
// If req.ConversationID is nil, a new conversation is created atomically before streaming begins.
// This eliminates the two-step create-then-send flow and ensures last_message_at is set before the SSE stream starts.
func (r *Runtime) StartRun(
	ctx context.Context,
	w http.ResponseWriter,
	uid uuid.UUID,
	req dto.StartConsultationRunRequest,
) *HTTPError {
	// --- 1. Idempotency check (before any side effects) ---
	existing, found, err := r.runService.CheckIdempotency(ctx, uid, req.RequestID)
	if err != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check idempotency")
	}
	if found {
		if existing.Status == "running" || existing.Status == "waiting_user" {
			return httpErr(http.StatusConflict, "RUN_IN_PROGRESS", "a run with this request ID is already in progress")
		}
		r.replayCompletedRun(ctx, w, existing)
		return nil
	}

	// --- 2. Resolve requested conversation identity ---
	var requestedConversationID *uuid.UUID
	isNewConversation := req.ConversationID == nil || *req.ConversationID == ""
	if !isNewConversation {
		parsed, err := uuid.Parse(*req.ConversationID)
		if err != nil {
			return httpErr(http.StatusBadRequest, "INVALID_ID", "invalid conversation id")
		}
		requestedConversationID = &parsed
	}

	// --- 3. Validate message ---
	userText := messagePartsToText(req.Message.Parts)
	if strings.TrimSpace(userText) == "" {
		return httpErr(http.StatusBadRequest, "INVALID_REQUEST", "message text is required")
	}

	userPartsJSON, err := json.Marshal(req.Message.Parts)
	if err != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to marshal message parts")
	}

	// --- 4. Create turn envelope (conversation/session + run + messages) ---
	session, turn, run, userMsg, assistantMsg, baseIDs, setupErr := r.createTurnEnvelope(
		ctx, uid, requestedConversationID, req.RequestID,
		datatypes.JSON(userPartsJSON), datatypes.JSON("{}"),
	)
	if setupErr != nil {
		return setupErr
	}
	conversationID := session.ConversationID

	sw := r.streamRuntime.NewWriter(w, baseIDs)
	r.sendNewEvent(
		ctx,
		sw,
		"run",
		"run.started",
		baseIDs,
		map[string]any{"status": "running", "source": "start_turn"},
		"",
		"run.started",
	)

	// --- 6. Emit conversation.created for new conversations ---
	if isNewConversation {
		r.sendNewEvent(
			ctx,
			sw,
			"conversation",
			"conversation.created",
			dto.StreamEventIDs{
				ConversationID: conversationID.String(),
				RunID:          run.ID.String(),
				TurnID:         turn.String(),
			},
			map[string]any{
				"title":           "",
				"title_status":    "pending",
				"status":          "active",
				"last_message_at": userMsg.CreatedAt.UTC().Format(time.RFC3339),
				"created_at":      userMsg.CreatedAt.UTC().Format(time.RFC3339),
			},
			"",
			"conversation.created",
		)
	}

	// --- 7. Emit message.persisted + message.created ---
	r.emitUserTurnStarted(ctx, sw, req.ClientMessageID, userMsg, assistantMsg)
	r.refreshThreadProjection(ctx, conversationID, uid)

	// --- 8. Stream AI response ---
	result, stopped := r.executeRunFlow(ctx, sw, uid, conversationID, turn, run, assistantMsg, baseIDs, userText, session)
	if stopped {
		return nil
	}

	// --- 9. Persist completed turn ---
	r.finishTurn(ctx, sw, uid, conversationID, run, assistantMsg, turn, result, baseIDs)

	// --- 10. Generate title for new conversations (non-blocking) ---
	if isNewConversation {
		r.generateTitleAndNotify(ctx, conversationID, uid, sw, baseIDs)
	}

	// --- 11. Emit stream.done ---
	r.sendNewEvent(ctx, sw, "stream", "stream.done", baseIDs, map[string]any{}, "", "stream.done")
	return nil
}

// generateTitleAndNotify generates a title synchronously and emits a title.generated SSE event.
func (r *Runtime) generateTitleAndNotify(
	ctx context.Context,
	conversationID uuid.UUID,
	userID uuid.UUID,
	sw *stream.StreamWriter,
	baseIDs dto.StreamEventIDs,
) {
	title, err := r.conversationService.GenerateTitleSync(ctx, conversationID, userID)
	if err != nil {
		log.Printf("title generation failed for conversation %s: %v", conversationID, err)
		return
	}

	r.sendNewEvent(
		ctx,
		sw,
		"title",
		"title.generated",
		baseIDs,
		map[string]any{"title": title},
		"",
		"title.generated",
	)
	r.refreshThreadProjection(ctx, conversationID, userID)
}

// executeRunFlow is the shared turn execution path used by StartRun and ResumeInteraction.
// It loads the profile, starts the AI stream, and processes events. The caller is responsible for
// setup (idempotency, turn envelope, initial SSE events) and teardown (persist, stream.done, title).
func (r *Runtime) executeRunFlow(
	ctx context.Context,
	sw *stream.StreamWriter,
	uid uuid.UUID,
	conversationID uuid.UUID,
	turn uuid.UUID,
	run *model.Run,
	assistantMsg *model.Message,
	baseIDs dto.StreamEventIDs,
	userText string,
	session *model.ConsultationSession,
) (streamResult, bool) {
	streamCtx, cancel := context.WithTimeout(ctx, sseTimeout)
	defer cancel()

	profileJSON, profileErr := r.loadProfileJSON(ctx, uid)
	if profileErr != nil {
		r.failBeforeStreaming(ctx, sw, run, assistantMsg, uid, conversationID, "failed to load profile")
		return streamResult{}, true
	}

	events, err := r.aiClient.StartConsultationTurn(
		streamCtx,
		conversationID.String(),
		service.StartConsultationTurnRequest{
			RunID:          run.ID.String(),
			ConversationID: conversationID.String(),
			UserID:         uid.String(),
			Input: service.ConsultationUserInput{
				Type: "user_message",
				Text: userText,
			},
			BusinessContext: service.ConsultationBusinessContext{
				Profile: profileJSON,
				ConsultationSnapshot: service.ConsultationSnapshot{
					Phase:         session.Phase,
					ExtractedInfo: json.RawMessage(session.ExtractedInfo),
				},
			},
		},
	)
	if err != nil {
		r.failBeforeStreaming(ctx, sw, run, assistantMsg, uid, conversationID, "AI service unavailable")
		return streamResult{}, true
	}

	return r.streamAIEvents(ctx, sw, events, streamState{
		UID:             uid,
		ConversationID:  conversationID,
		TurnID:          turn,
		Run:             run,
		AssistantMsg:    assistantMsg,
		BaseIDs:         baseIDs,
		CurrentPhase:    session.Phase,
		RequestDone:     ctx.Done(),
		AssistantMsgID:  assistantMsg.ID.String(),
		ConversationStr: conversationID.String(),
	})
}

// finishTurn persists the completed turn and emits the message.completed SSE event.
func (r *Runtime) finishTurn(
	ctx context.Context,
	sw *stream.StreamWriter,
	uid uuid.UUID,
	conversationID uuid.UUID,
	run *model.Run,
	assistantMsg *model.Message,
	turn uuid.UUID,
	result streamResult,
	baseIDs dto.StreamEventIDs,
) {
	r.persistCompletedTurn(ctx, uid, conversationID, run, assistantMsg, result)
	r.refreshThreadProjection(ctx, conversationID, uid)
	r.sendNewEvent(
		ctx,
		sw,
		"run",
		"run.completed",
		dto.StreamEventIDs{
			ConversationID: conversationID.String(),
			RunID:          run.ID.String(),
			TurnID:         turn.String(),
		},
		map[string]any{"status": "completed", "usage": result.Usage},
		"",
		"run.completed",
	)
	r.sendNewEvent(
		ctx,
		sw,
		"message",
		"message.completed",
		dto.StreamEventIDs{
			ConversationID: conversationID.String(),
			RunID:          run.ID.String(),
			TurnID:         turn.String(),
			MessageID:      assistantMsg.ID.String(),
		},
		map[string]any{"status": "completed", "finish_reason": "stop", "usage": result.Usage},
		assistantMsg.ID.String(),
		"message.completed",
	)
}

func (r *Runtime) ResumeInteraction(
	ctx context.Context,
	w http.ResponseWriter,
	uid uuid.UUID,
	conversationID uuid.UUID,
	interactionID uuid.UUID,
	req dto.ResumeConsultationInteractionRequest,
) *HTTPError {
	existing, found, err := r.runService.CheckIdempotency(ctx, uid, req.RequestID)
	if err != nil {
		return httpErr(
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"failed to check idempotency",
		)
	}
	if found {
		if existing.Status == "running" || existing.Status == "waiting_user" {
			return httpErr(
				http.StatusConflict,
				"RUN_IN_PROGRESS",
				"a run with this request ID is already in progress",
			)
		}
		r.replayCompletedRun(ctx, w, existing)
		return nil
	}

	session, err := r.consultationService.GetConsultation(ctx, conversationID, uid)
	if err != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load consultation")
	}
	if session == nil {
		return httpErr(http.StatusNotFound, "NOT_FOUND", "consultation not found")
	}

	interaction, err := r.interactionService.GetInteractionByID(ctx, interactionID)
	if err != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load interaction")
	}
	if interaction == nil || interaction.ConversationID != conversationID {
		return httpErr(http.StatusNotFound, "NOT_FOUND", "interaction not found")
	}

	if err := r.interactionService.ResumeInteraction(ctx, interactionID, datatypes.JSON(req.Answer)); err != nil {
		switch {
		case err == service.ErrInteractionNotFound:
			return httpErr(http.StatusNotFound, "NOT_FOUND", "interaction not found")
		case err == service.ErrInteractionConflict:
			return httpErr(
				http.StatusConflict,
				"INTERACTION_CONFLICT",
				"interaction was already answered differently",
			)
		case err == service.ErrInteractionClosed:
			return httpErr(http.StatusConflict, "INTERACTION_CLOSED", err.Error())
		default:
			return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
	}

	answerPartsJSON, answerMetadata := interactionAnswerParts(req.Answer, interactionID.String())
	_, turn, run, _, assistantMsg, baseIDs, setupErr := r.createTurnEnvelope(
		ctx,
		uid,
		&conversationID,
		req.RequestID,
		answerPartsJSON,
		answerMetadata,
	)
	if setupErr != nil {
		return setupErr
	}

	sw := r.streamRuntime.NewWriter(w, baseIDs)
	r.sendNewEvent(
		ctx,
		sw,
		"state",
		"state.interaction.answered",
		dto.StreamEventIDs{
			ConversationID: conversationID.String(),
			RunID:          run.ID.String(),
			TurnID:         turn.String(),
			InteractionID:  interactionID.String(),
		},
		map[string]any{
			"interaction_id": interactionID.String(),
			"answer":         json.RawMessage(req.Answer),
		},
		"",
		"state.interaction.answered",
	)
	r.sendNewEvent(
		ctx,
		sw,
		"run",
		"run.resumed",
		dto.StreamEventIDs{
			ConversationID: conversationID.String(),
			RunID:          run.ID.String(),
			TurnID:         turn.String(),
			InteractionID:  interactionID.String(),
		},
		map[string]any{"status": "running", "interaction_id": interactionID.String()},
		"",
		"run.resumed",
	)
	r.sendNewEvent(
		ctx,
		sw,
		"message",
		"message.created",
		dto.StreamEventIDs{
			ConversationID: assistantMsg.ConversationID.String(),
			TurnID:         assistantMsg.TurnID.String(),
			MessageID:      assistantMsg.ID.String(),
		},
		map[string]any{"role": "assistant", "status": "streaming"},
		assistantMsg.ID.String(),
		"message.created",
	)
	r.refreshThreadProjection(ctx, conversationID, uid)

	streamCtx, cancel := context.WithTimeout(ctx, sseTimeout)
	defer cancel()

	profileJSON, profileErr := r.loadProfileJSON(ctx, uid)
	if profileErr != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load profile")
	}

	events, err := r.aiClient.ResumeConsultationInterrupt(
		streamCtx,
		conversationID.String(),
		interactionID.String(),
		service.ResumeConsultationInterruptRequest{
			RunID:          run.ID.String(),
			ConversationID: conversationID.String(),
			UserID:         uid.String(),
			InterruptID:    interactionID.String(),
			Answer:         json.RawMessage(req.Answer),
			BusinessContext: service.ConsultationBusinessContext{
				Profile: profileJSON,
				ConsultationSnapshot: service.ConsultationSnapshot{
					Phase:         session.Phase,
					ExtractedInfo: json.RawMessage(session.ExtractedInfo),
				},
			},
		},
	)
	if err != nil {
		r.failBeforeStreaming(ctx, sw, run, assistantMsg, uid, conversationID, "AI service unavailable")
		return nil
	}

	result, stopped := r.streamAIEvents(ctx, sw, events, streamState{
		UID:             uid,
		ConversationID:  conversationID,
		TurnID:          turn,
		Run:             run,
		AssistantMsg:    assistantMsg,
		BaseIDs:         baseIDs,
		CurrentPhase:    session.Phase,
		RequestDone:     ctx.Done(),
		AssistantMsgID:  assistantMsg.ID.String(),
		ConversationStr: conversationID.String(),
	})
	if stopped {
		return nil
	}

	r.finishTurn(ctx, sw, uid, conversationID, run, assistantMsg, turn, result, baseIDs)
	r.sendNewEvent(ctx, sw, "stream", "stream.done", baseIDs, map[string]any{}, "", "stream.done")
	r.maybeGenerateTitle(ctx, conversationID, uid)
	return nil
}

func (r *Runtime) createTurnEnvelope(
	ctx context.Context,
	uid uuid.UUID,
	conversationID *uuid.UUID,
	requestID string,
	userParts datatypes.JSON,
	userMetadata datatypes.JSON,
) (
	*model.ConsultationSession,
	uuid.UUID,
	*model.Run,
	*model.Message,
	*model.Message,
	dto.StreamEventIDs,
	*HTTPError,
) {
	envelope, err := r.consultationService.CreateRunEnvelope(
		ctx,
		uid,
		conversationID,
		requestID,
		userParts,
		userMetadata,
		"consultation-thread",
	)
	if err != nil {
		return nil, uuid.Nil, nil, nil, nil, dto.StreamEventIDs{}, httpErr(
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"failed to create run envelope",
		)
	}
	if envelope.Existed {
		return nil, uuid.Nil, nil, nil, nil, dto.StreamEventIDs{}, httpErr(
			http.StatusConflict,
			"RUN_IN_PROGRESS",
			"a run with this request ID already exists",
		)
	}

	return envelope.Session, envelope.TurnID, envelope.Run, envelope.UserMessage, envelope.AssistantMessage, dto.StreamEventIDs{
		ConversationID: envelope.Session.ConversationID.String(),
		RunID:          envelope.Run.ID.String(),
		TurnID:         envelope.TurnID.String(),
	}, nil
}
func (r *Runtime) emitUserTurnStarted(
	ctx context.Context,
	sw *stream.StreamWriter,
	clientMessageID string,
	userMsg *model.Message,
	assistantMsg *model.Message,
) {
	r.sendNewEvent(
		ctx,
		sw,
		"message",
		"message.persisted",
		dto.StreamEventIDs{
			ConversationID: userMsg.ConversationID.String(),
			TurnID:         userMsg.TurnID.String(),
			MessageID:      userMsg.ID.String(),
		},
		map[string]any{"client_message_id": clientMessageID, "role": "user"},
		userMsg.ID.String(),
		"message.persisted",
	)

	r.sendNewEvent(
		ctx,
		sw,
		"message",
		"message.created",
		dto.StreamEventIDs{
			ConversationID: assistantMsg.ConversationID.String(),
			TurnID:         assistantMsg.TurnID.String(),
			MessageID:      assistantMsg.ID.String(),
		},
		map[string]any{"role": "assistant", "status": "streaming"},
		assistantMsg.ID.String(),
		"message.created",
	)
}

func (r *Runtime) loadProfileJSON(ctx context.Context, uid uuid.UUID) (json.RawMessage, error) {
	profile, err := r.profileService.GetProfile(ctx, uid)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return json.RawMessage(`{}`), nil
	}

	data, err := json.Marshal(profile)
	if err != nil {
		return nil, err
	}
	return data, nil
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

func (r *Runtime) streamAIEvents(
	ctx context.Context,
	sw *stream.StreamWriter,
	events <-chan dto.StreamEvent,
	state streamState,
) (streamResult, bool) {
	result := streamResult{}
	eventCount := 0
	phase := state.CurrentPhase

	for {
		select {
		case <-state.RequestDone:
			log.Printf("client disconnected during stream for conversation %s", state.ConversationID)
			bg := context.Background()
			_ = r.runService.FailRun(
				bg,
				state.Run.ID,
				state.UID,
				map[string]any{"message": "client disconnected"},
			)
			_ = r.messageService.UpdateMessageStatus(
				bg,
				state.AssistantMsg.ID,
				state.ConversationID,
				"aborted",
			)
			r.clearActiveRun(bg, state.ConversationID, state.UID)
			r.refreshThreadProjection(bg, state.ConversationID, state.UID)
			return result, true

		case event, ok := <-events:
			if !ok {
				return result, false
			}

			eventCount++
			if eventCount > maxSSEEvents {
				log.Printf("event count exceeded limit for conversation %s", state.ConversationID)
				r.failActiveStream(ctx, sw, state, "stream exceeded maximum event count")
				return result, true
			}

			if r.handleAIEvent(ctx, sw, event, state, &result, &phase) {
				return result, true
			}
		}
	}
}

func (r *Runtime) handleAIEvent(
	ctx context.Context,
	sw *stream.StreamWriter,
	event dto.StreamEvent,
	state streamState,
	result *streamResult,
	phase *string,
) bool {
	switch event.Type {
	case "message.text.delta":
		var payload struct {
			Delta string `json:"delta"`
		}
		_ = event.PayloadAs(&payload)
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, "text.delta")
		result.AssistantParts = append(
			result.AssistantParts,
			map[string]any{"type": "text", "text": payload.Delta},
		)

	case "tool.call":
		var payload struct {
			Tool string          `json:"tool"`
			Args json.RawMessage `json:"args"`
		}
		_ = event.PayloadAs(&payload)
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, "tool.call")
		result.AssistantParts = append(
			result.AssistantParts,
			map[string]any{
				"type":         "tool_call",
				"tool":         payload.Tool,
				"args":         payload.Args,
				"tool_call_id": event.IDs.ToolCallID,
			},
		)
		msgID := state.AssistantMsg.ID
		r.agentToolService.RecordToolCall(
			ctx,
			state.Run.ID,
			state.ConversationID,
			&msgID,
			event.IDs.ToolCallID,
			payload.Tool,
			datatypes.JSON(payload.Args),
		)

	case "tool.result":
		var payload struct {
			Tool   string          `json:"tool"`
			Result json.RawMessage `json:"result"`
		}
		_ = event.PayloadAs(&payload)
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, "tool.result")
		result.AssistantParts = append(
			result.AssistantParts,
			map[string]any{
				"type":         "tool_result",
				"tool":         payload.Tool,
				"result":       payload.Result,
				"tool_call_id": event.IDs.ToolCallID,
			},
		)
		r.agentToolService.RecordToolResult(
			ctx,
			state.Run.ID,
			event.IDs.ToolCallID,
			datatypes.JSON(payload.Result),
			toolResultIsError(payload.Result),
		)

	case "state.extracted_info.upsert":
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, event.Type)

	case "source.citation.added":
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, event.Type)
		result.AssistantParts = append(result.AssistantParts, citationPart(event.Payload))

	case "source.knowledge_gap":
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, event.Type)
		result.AssistantParts = append(result.AssistantParts, dataPart("knowledge_gap", event.Payload))

	case "safety.red_flag.detected":
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, event.Type)
		result.AssistantParts = append(result.AssistantParts, dataPart("red_flag", event.Payload))

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
		var payload struct {
			Message string `json:"message"`
		}
		_ = event.PayloadAs(&payload)
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, "stream.error")
		r.failActiveStream(ctx, sw, state, payload.Message)
		return true
	}

	return false
}

func (r *Runtime) handleInteractionRequired(
	ctx context.Context,
	sw *stream.StreamWriter,
	event dto.StreamEvent,
	state streamState,
	result *streamResult,
) bool {
	var payload struct {
		InteractionID string          `json:"interaction_id"`
		Question      json.RawMessage `json:"question"`
	}
	_ = event.PayloadAs(&payload)

	interaction, err := r.interactionService.CreatePendingInteraction(
		ctx,
		state.Run.ID,
		state.ConversationID,
		event.IDs.ToolCallID,
		datatypes.JSON(payload.Question),
	)
	if err != nil {
		log.Printf("failed to create pending interaction for conversation %s: %v", state.ConversationID, err)
		r.sendNewEvent(
			ctx,
			sw,
			"stream",
			"stream.error",
			state.BaseIDs,
			map[string]any{"message": "failed to persist user interaction"},
			"",
			"stream.error",
		)
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
	r.sendNewEvent(
		ctx,
		sw,
		"run",
		"run.interrupted",
		dto.StreamEventIDs{
			ConversationID: state.ConversationID.String(),
			RunID:          state.Run.ID.String(),
			TurnID:         state.TurnID.String(),
			InteractionID:  interactionID,
		},
		map[string]any{"status": "waiting_user", "interaction_id": interactionID},
		"",
		"run.interrupted",
	)

	// Atomically set both parts and aborted status in a single query.
	// The previous two-step approach (UpdateMessageCompleted then UpdateMessageStatus)
	// could leave the message in an inconsistent state on crash between the two calls.
	finalPartsJSON, _ := json.Marshal(result.AssistantParts)
	if err := r.messageService.UpdateMessageCompletedWithStatus(
		ctx,
		state.AssistantMsg.ID,
		state.ConversationID,
		datatypes.JSON(finalPartsJSON),
		"aborted",
	); err != nil {
		log.Printf("failed to finalize interrupted assistant message %s: %v", state.AssistantMsg.ID, err)
	}

	r.sendNewEvent(ctx, sw, "stream", "stream.done", state.BaseIDs, map[string]any{}, "", "stream.done")
	r.clearActiveRun(ctx, state.ConversationID, state.UID)
	r.refreshThreadProjection(ctx, state.ConversationID, state.UID)
	return true
}

func (r *Runtime) persistCompletedTurn(
	ctx context.Context,
	uid uuid.UUID,
	conversationID uuid.UUID,
	run *model.Run,
	assistantMsg *model.Message,
	result streamResult,
) {
	finalPartsJSON, _ := json.Marshal(result.AssistantParts)
	if err := r.messageService.UpdateMessageCompleted(
		ctx,
		assistantMsg.ID,
		conversationID,
		datatypes.JSON(finalPartsJSON),
		nil,
		nil,
	); err != nil {
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

func (r *Runtime) recordGovernance(
	ctx context.Context,
	uid uuid.UUID,
	conversationID uuid.UUID,
	runID uuid.UUID,
	governanceResult datatypes.JSON,
) {
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

	r.outputReviewService.RecordReview(
		ctx,
		"consultation_reply",
		govStatus,
		&uid,
		&runID,
		nil,
		&conversationID,
		govIssues,
		govValidatedOutput,
		governanceResult,
	)
}

func (r *Runtime) replayCompletedRun(ctx context.Context, w http.ResponseWriter, run *model.Run) {
	baseIDs := dto.StreamEventIDs{
		ConversationID: run.ConversationID.String(),
		RunID:          run.ID.String(),
		TurnID:         run.TurnID.String(),
	}
	sw := r.streamRuntime.NewWriter(w, baseIDs)

	if r.runtimeEventService == nil {
		r.sendNewEvent(
			ctx,
			sw,
			"stream",
			"stream.error",
			baseIDs,
			map[string]any{"message": "runtime event log unavailable", "status": run.Status},
			"",
			"stream.error",
		)
		r.sendNewEvent(ctx, sw, "stream", "stream.done", baseIDs, map[string]any{}, "", "stream.done")
		return
	}

	events, err := r.runtimeEventService.ListAllRunEvents(ctx, run.ConversationID, run.ID)
	if err != nil || len(events) == 0 {
		if err != nil {
			log.Printf("failed to replay runtime events for run %s: %v", run.ID, err)
		}
		r.sendNewEvent(
			ctx,
			sw,
			"stream",
			"stream.error",
			baseIDs,
			map[string]any{"message": "runtime event log unavailable", "status": run.Status},
			"",
			"stream.error",
		)
		r.sendNewEvent(ctx, sw, "stream", "stream.done", baseIDs, map[string]any{}, "", "stream.done")
		return
	}

	maxSeq := 0
	for _, stored := range events {
		var ids dto.StreamEventIDs
		if len(stored.IDs) > 0 {
			if err := json.Unmarshal(stored.IDs, &ids); err != nil {
				log.Printf("failed to decode runtime event ids for run %s seq %d: %v", run.ID, stored.Seq, err)
				continue
			}
		}

		payload := json.RawMessage(stored.Payload)
		if len(payload) == 0 {
			payload = json.RawMessage(`{}`)
		}
		if stored.Seq > maxSeq {
			maxSeq = stored.Seq
		}

		if err := sw.WriteEvent(ctx, dto.StreamEvent{
			Version: 1,
			Seq:     stored.Seq,
			Channel: stored.Channel,
			Type:    stored.Type,
			IDs:     ids,
			Payload: payload,
		}); err != nil {
			log.Printf("failed to write replayed event for run %s seq %d: %v", run.ID, stored.Seq, err)
			return
		}
	}

	if err := sw.WriteEvent(ctx, dto.StreamEvent{
		Version: 1,
		Seq:     maxSeq + 1,
		Channel: "stream",
		Type:    "stream.done",
		IDs:     baseIDs,
		Payload: json.RawMessage(`{}`),
	}); err != nil {
		log.Printf("failed to write replay done for run %s: %v", run.ID, err)
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

func (r *Runtime) clearActiveRun(ctx context.Context, conversationID, userID uuid.UUID) {
	if err := r.conversationService.UpdateActiveRunID(ctx, conversationID, userID, nil, ""); err != nil {
		log.Printf("failed to clear active_run_id for conversation %s: %v", conversationID, err)
	}
}

func (r *Runtime) refreshThreadProjection(ctx context.Context, conversationID, userID uuid.UUID) {
	if r.threadProjectionService == nil {
		return
	}

	if _, _, _, _, _, err := r.threadProjectionService.RefreshAndGetThread(ctx, conversationID, userID); err != nil {
		log.Printf("failed to refresh thread projection for conversation %s: %v", conversationID, err)
	}
}

func (r *Runtime) failBeforeStreaming(
	ctx context.Context,
	sw *stream.StreamWriter,
	run *model.Run,
	assistantMsg *model.Message,
	uid uuid.UUID,
	conversationID uuid.UUID,
	message string,
) {
	r.sendNewEvent(
		ctx,
		sw,
		"run",
		"run.failed",
		dto.StreamEventIDs{
			ConversationID: conversationID.String(),
			RunID:          run.ID.String(),
			TurnID:         run.TurnID.String(),
		},
		map[string]any{"status": "failed", "error": map[string]any{"message": message}},
		"",
		"run.failed",
	)
	r.sendNewEvent(
		ctx,
		sw,
		"message",
		"message.failed",
		dto.StreamEventIDs{
			ConversationID: conversationID.String(),
			RunID:          run.ID.String(),
			TurnID:         run.TurnID.String(),
			MessageID:      assistantMsg.ID.String(),
		},
		map[string]any{"status": "failed", "error": map[string]any{"message": message}},
		assistantMsg.ID.String(),
		"message.failed",
	)
	r.sendNewEvent(
		ctx,
		sw,
		"stream",
		"stream.done",
		dto.StreamEventIDs{
			ConversationID: conversationID.String(),
			RunID:          run.ID.String(),
			TurnID:         run.TurnID.String(),
		},
		map[string]any{},
		"",
		"stream.done",
	)
	_ = r.runService.FailRun(ctx, run.ID, uid, map[string]any{"message": message})
	r.clearActiveRun(ctx, conversationID, uid)
	r.refreshThreadProjection(ctx, conversationID, uid)
}

func (r *Runtime) failActiveStream(
	ctx context.Context,
	sw *stream.StreamWriter,
	state streamState,
	message string,
) {
	if strings.TrimSpace(message) == "" {
		message = "stream failed"
	}

	r.sendNewEvent(
		ctx,
		sw,
		"run",
		"run.failed",
		dto.StreamEventIDs{
			ConversationID: state.ConversationID.String(),
			RunID:          state.Run.ID.String(),
			TurnID:         state.TurnID.String(),
		},
		map[string]any{"status": "failed", "error": map[string]any{"message": message}},
		"",
		"run.failed",
	)
	r.sendNewEvent(
		ctx,
		sw,
		"message",
		"message.failed",
		dto.StreamEventIDs{
			ConversationID: state.ConversationID.String(),
			RunID:          state.Run.ID.String(),
			TurnID:         state.TurnID.String(),
			MessageID:      state.AssistantMsg.ID.String(),
		},
		map[string]any{"status": "failed", "error": map[string]any{"message": message}},
		state.AssistantMsgID,
		"message.failed",
	)
	r.sendNewEvent(ctx, sw, "stream", "stream.done", state.BaseIDs, map[string]any{}, "", "stream.done")
	_ = r.runService.FailRun(ctx, state.Run.ID, state.UID, map[string]any{"message": message})
	_ = r.messageService.UpdateMessageStatus(ctx, state.AssistantMsg.ID, state.ConversationID, "failed")
	r.clearActiveRun(ctx, state.ConversationID, state.UID)
	r.refreshThreadProjection(ctx, state.ConversationID, state.UID)
}
func (r *Runtime) sendNewEvent(
	ctx context.Context,
	sw *stream.StreamWriter,
	channel string,
	eventType string,
	ids dto.StreamEventIDs,
	payload any,
	messageID string,
	label string,
) {
	event, err := sw.NewEvent(channel, eventType, ids, payload, messageID)
	if err != nil {
		log.Printf("SSE build error (%s): %v", label, err)
		return
	}
	if err := sw.WriteEvent(ctx, event); err != nil {
		log.Printf("SSE write error (%s): %v", label, err)
		return
	}
	r.recordPublicEvent(ctx, event)
}

func (r *Runtime) sendEvent(
	ctx context.Context,
	sw *stream.StreamWriter,
	event dto.StreamEvent,
	messageID string,
	label string,
) {
	enriched := sw.EnrichEvent(event, messageID)
	if err := sw.WriteEvent(ctx, enriched); err != nil {
		log.Printf("SSE write error (%s): %v", label, err)
		return
	}
	r.recordPublicEvent(ctx, enriched)
}

func (r *Runtime) recordPublicEvent(ctx context.Context, event dto.StreamEvent) {
	if r.runtimeEventService == nil {
		return
	}

	conversationID, err := uuid.Parse(event.IDs.ConversationID)
	if err != nil {
		return
	}

	runID, err := uuid.Parse(event.IDs.RunID)
	if err != nil {
		return
	}

	var turnID *uuid.UUID
	if event.IDs.TurnID != "" {
		if parsedTurnID, parseErr := uuid.Parse(event.IDs.TurnID); parseErr == nil {
			turnID = &parsedTurnID
		}
	}

	if err := r.runtimeEventService.RecordPublicEvent(ctx, conversationID, runID, turnID, event); err != nil {
		log.Printf("failed to persist runtime event %s for run %s: %v", event.Type, runID, err)
	}
}

func messagePartsToText(parts []dto.PartDTO) string {
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
			texts = append(texts, part.Text)
		}
	}
	return strings.TrimSpace(strings.Join(texts, "\n"))
}

func interactionAnswerParts(answer json.RawMessage, interactionID string) (datatypes.JSON, datatypes.JSON) {
	text := extractAnswerText(answer)
	partsJSON, _ := json.Marshal([]map[string]any{{"type": "text", "text": text}})
	metadataJSON, _ := json.Marshal(map[string]any{
		"is_interaction_answer": true,
		"interaction_id":        interactionID,
	})
	return datatypes.JSON(partsJSON), datatypes.JSON(metadataJSON)
}

func extractAnswerText(answer json.RawMessage) string {
	var rawAnswer map[string]any
	if err := json.Unmarshal(answer, &rawAnswer); err != nil {
		return string(answer)
	}
	if text, ok := rawAnswer["text"].(string); ok {
		return text
	}
	if selected, ok := rawAnswer["selected"].([]any); ok {
		parts := make([]string, 0, len(selected))
		for _, item := range selected {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, ", ")
	}
	if value, ok := rawAnswer["value"]; ok {
		return fmt.Sprintf("%v", value)
	}
	// Fallback: serialize the entire answer as text
	if b, err := json.Marshal(rawAnswer); err == nil {
		return string(b)
	}
	return ""
}

func citationPart(payload json.RawMessage) map[string]any {
	var citation map[string]any
	_ = json.Unmarshal(payload, &struct {
		Citation *map[string]any `json:"citation"`
	}{Citation: &citation})

	title, _ := citation["title"].(string)
	return map[string]any{
		"type":       "source",
		"title":      title,
		"sourceType": "document",
		"providerMetadata": map[string]any{
			"bodysense": citation,
		},
	}
}

func dataPart(name string, payload json.RawMessage) map[string]any {
	var data map[string]any
	_ = json.Unmarshal(payload, &data)
	return map[string]any{
		"type": "data",
		"name": name,
		"data": data,
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
