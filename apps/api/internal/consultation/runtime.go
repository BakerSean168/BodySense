package consultation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bodysense/api/internal/config"
	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/episodicrecall"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/stream"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const maxSSEEvents = 10000

// durableExecutionContext is the total time budget for one consultation
// execution. It applies to every serial model call (diagnosis, structuring,
// knowledge tool rounds, final composition) plus all persistence, so a slow
// provider doing multi-turn work can legitimately exceed the previous fixed
// five minutes. The budget arrives from the composition root (see
// config.Consultation); a zero-value Runtime keeps the safe default.
func (r *Runtime) durableExecutionContext(parent context.Context) (context.Context, context.CancelFunc) {
	budget := r.executionTimeout
	if budget <= 0 {
		budget = config.DefaultConsultationExecutionTimeout
	}
	return context.WithTimeout(context.WithoutCancel(parent), budget)
}

type ConsultationErrorCode string

const (
	ConsultationErrorInteractionClosed       ConsultationErrorCode = "INTERACTION_CLOSED"
	ConsultationErrorInteractionExpired      ConsultationErrorCode = "INTERACTION_EXPIRED"
	ConsultationErrorInteractionNotResumable ConsultationErrorCode = "INTERACTION_NOT_RESUMABLE"
	ConsultationErrorInternal                ConsultationErrorCode = "INTERNAL_ERROR"
	ConsultationErrorInvalidID               ConsultationErrorCode = "INVALID_ID"
	ConsultationErrorInvalidRequest          ConsultationErrorCode = "INVALID_REQUEST"
	ConsultationErrorInvalidSpatialContext   ConsultationErrorCode = "INVALID_SPATIAL_CONTEXT"
	ConsultationErrorNotFound                ConsultationErrorCode = "NOT_FOUND"
	ConsultationErrorRunTerminal             ConsultationErrorCode = "RUN_TERMINAL"
)

type HTTPError struct {
	Status  int
	Code    ConsultationErrorCode
	Message string
}

type runtimeBodyStateService interface {
	GetAgentBodyContext(ctx context.Context, userID uuid.UUID) (*service.AgentBodyContext, error)
}

type runtimeKnowledgeObservationService interface {
	Record(ctx context.Context, input service.RecordKnowledgePublicationObservationInput) error
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type Runtime struct {
	conversationService         *service.ConversationService
	consultationService         *service.ConsultationService
	profileService              *service.ProfileService
	messageService              *service.MessageService
	runService                  *service.RunService
	aiClient                    *service.AIClient
	agentToolService            *service.AgentToolService
	interactionService          *service.AgentInteractionService
	outputReviewService         *service.OutputReviewService
	threadProjectionService     *service.ThreadProjectionService
	runtimeEventService         *service.RuntimeEventService
	uploadService               *service.UploadService
	bodyStateService            runtimeBodyStateService
	bodyFacts                   runtimeBodyFactService
	interactionResumes          *service.InteractionResumeService
	suggestionSources           service.BodyStateSuggestionSourceReader
	contextRetrievalService     *service.ContextRetrievalService
	episodicRecall              *episodicrecall.Recaller
	diagnosisReader             runtimeDiagnosisReader
	outcomeService              *service.OutcomeService
	streamRuntime               *stream.Runtime
	deployment                  *service.AgentDeploymentPolicy
	rolloutService              *service.ConsultationRolloutService
	knowledgeObservationService runtimeKnowledgeObservationService
	chatRetention               runtimeChatRetention
	executionTimeout            time.Duration

	cancelMu   sync.Mutex
	runCancels map[uuid.UUID]context.CancelFunc
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
	uploadService *service.UploadService,
	deployment *service.AgentDeploymentPolicy,
	cfg config.Consultation,
	bodyStateServices ...runtimeBodyStateService,
) *Runtime {
	var bodyStateService runtimeBodyStateService
	if len(bodyStateServices) > 0 {
		bodyStateService = bodyStateServices[0]
	}
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
		uploadService:           uploadService,
		deployment:              deployment,
		bodyStateService:        bodyStateService,
		executionTimeout:        cfg.ExecutionTimeout,
		streamRuntime:           stream.NewRuntime(),
		runCancels:              make(map[uuid.UUID]context.CancelFunc),
	}
}

func (r *Runtime) registerRunCancellation(runID uuid.UUID, cancel context.CancelFunc) func() {
	r.cancelMu.Lock()
	r.runCancels[runID] = cancel
	r.cancelMu.Unlock()
	return func() {
		r.cancelMu.Lock()
		delete(r.runCancels, runID)
		r.cancelMu.Unlock()
	}
}

func (r *Runtime) cancelRegisteredRun(runID uuid.UUID) bool {
	r.cancelMu.Lock()
	cancel := r.runCancels[runID]
	r.cancelMu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

// CancelRun is an explicit business command. HTTP transport disconnects do not
// call this method and therefore do not cancel durable execution.
func (r *Runtime) CancelRun(ctx context.Context, uid, runID uuid.UUID, reason string) *HTTPError {
	run, transitioned, err := r.runService.CancelRunWithEvent(ctx, runID, uid, reason)
	if err != nil {
		if errors.Is(err, service.ErrRunTerminal) {
			return httpErr(http.StatusConflict, ConsultationErrorRunTerminal, "run is already terminal")
		}
		return httpErr(http.StatusInternalServerError, ConsultationErrorInternal, "failed to cancel run")
	}
	if run == nil {
		return httpErr(http.StatusNotFound, ConsultationErrorNotFound, "run not found")
	}

	// Pending HITL belongs to the cancelled run and must not remain answerable.
	pending, pendingErr := r.interactionService.GetPendingInteractions(ctx, run.ConversationID)
	if pendingErr != nil {
		return httpErr(http.StatusInternalServerError, ConsultationErrorInternal, "failed to load pending interactions")
	}
	for i := range pending {
		if pending[i].RunID == runID {
			if cancelErr := r.interactionService.CancelInteraction(ctx, pending[i].ID); cancelErr != nil && !errors.Is(cancelErr, service.ErrInteractionClosed) {
				return httpErr(http.StatusInternalServerError, ConsultationErrorInternal, "failed to cancel pending interaction")
			}
		}
	}

	// run.cancelled was committed in the same transaction as Run.status. If a
	// live execution exists, cancel it now; the browser's durable watcher owns
	// recovery of that already-persisted terminal event after the SSE closes.
	active := r.cancelRegisteredRun(runID)
	if transitioned && !active {
		r.clearActiveRun(ctx, run.ConversationID, uid)
		r.refreshThreadProjection(ctx, run.ConversationID, uid)
	}
	return nil
}

// StartRun handles a unified consultation run request.
// If req.ConversationID is nil, a new conversation is created atomically before streaming begins.
// This eliminates the two-step create-then-send flow and ensures last_message_at is set before the SSE stream starts.
func (r *Runtime) StartRun(
	ctx context.Context,
	w http.ResponseWriter,
	uid uuid.UUID,
	req StartRunInput,
) *HTTPError {
	// --- 1. Idempotency check (before any side effects) ---
	existing, found, err := r.runService.CheckIdempotency(ctx, uid, req.RequestID)
	if err != nil {
		return httpErr(http.StatusInternalServerError, ConsultationErrorInternal, "failed to check idempotency")
	}
	if found {
		if existing.Status == "running" || existing.Status == "waiting_user" {
			// Same request_id is an idempotent transport re-attachment, not a
			// second business command. This is important when a browser/proxy
			// rebuilds an SSE POST after the original socket disappears.
			slog.Default().Info("consultation_transport_reattach", "request_id", req.RequestID, "run_id", existing.ID, "conversation_id", existing.ConversationID, "status", existing.Status)
			r.reattachActiveRun(ctx, w, existing)
			return nil
		}
		r.replayCompletedRun(ctx, w, existing)
		return nil
	}

	// --- 2. Resolve requested conversation identity ---
	var requestedConversationID *uuid.UUID
	conversationIDMissing := req.ConversationID == nil || *req.ConversationID == ""
	if !conversationIDMissing {
		parsed, err := uuid.Parse(*req.ConversationID)
		if err != nil {
			return httpErr(http.StatusBadRequest, ConsultationErrorInvalidID, "invalid conversation id")
		}
		requestedConversationID = &parsed
	}

	// --- 3. Validate message ---
	userText := messagePartsToText(req.Message.Parts)
	imageIDs := messagePartsToImageUploadIDs(req.Message.Parts)
	if strings.TrimSpace(userText) == "" && len(imageIDs) == 0 {
		return httpErr(http.StatusBadRequest, ConsultationErrorInvalidRequest, "message text or image is required")
	}
	if strings.TrimSpace(userText) == "" {
		userText = "请结合我附上的照片，分析与体态/不适相关的可见信息，并给出谨慎建议。"
	}

	userPartsJSON, err := marshalDurableMessageParts(req.Message.Parts)
	if err != nil {
		return httpErr(http.StatusBadRequest, ConsultationErrorInvalidRequest, err.Error())
	}
	userMetadata, spatialContext, metadataErr := normalizeSpatialContextMetadata(req.Message.Metadata)
	if metadataErr != nil {
		return httpErr(http.StatusBadRequest, ConsultationErrorInvalidSpatialContext, "invalid Body Explorer context")
	}

	// --- 4. Create turn envelope (conversation/session + run + messages) ---
	session, turn, run, userMsg, assistantMsg, baseIDs, conversationCreated, setupErr := r.createTurnEnvelope(
		ctx, uid, requestedConversationID, req.RequestID,
		datatypes.JSON(userPartsJSON), userMetadata,
	)
	if setupErr != nil {
		return setupErr
	}
	conversationID := session.ConversationID
	isNewConversation := conversationCreated
	executionCtx, cancelExecution := r.durableExecutionContext(ctx)
	defer cancelExecution()
	unregisterCancellation := r.registerRunCancellation(run.ID, cancelExecution)
	defer unregisterCancellation()
	stopLeaseHeartbeat := r.runService.StartLeaseHeartbeat(executionCtx, run.ID, uid)
	defer stopLeaseHeartbeat()

	sw := r.streamRuntime.NewWriter(w, baseIDs)
	r.sendNewEvent(
		executionCtx,
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
				"status":          "active",
				"last_message_at": userMsg.CreatedAt.UTC().Format(time.RFC3339),
				"created_at":      userMsg.CreatedAt.UTC().Format(time.RFC3339),
			},
			"",
			"conversation.created",
		)
	}

	// --- 7. Emit message.persisted + message.created ---
	r.emitUserTurnStarted(executionCtx, sw, req.ClientMessageID, userMsg, assistantMsg)
	r.refreshThreadProjection(executionCtx, conversationID, uid)

	// --- 8. Stream AI response ---
	result, stopped := r.executeRunFlow(executionCtx, sw, uid, conversationID, turn, run, assistantMsg, baseIDs, userText, userMsg.ID, req.Message.Parts, session, spatialContext)
	if stopped {
		return nil
	}

	// --- 9. Persist completed turn. Explicit cancellation can win the race
	// after the AI stream closes but before the terminal DB transition.
	if executionCtx.Err() != nil {
		r.handleExecutionContextDone(executionCtx, sw, streamState{
			UID: uid, ConversationID: conversationID, TurnID: turn, Run: run, AssistantMsg: assistantMsg, BaseIDs: baseIDs, AssistantMsgID: assistantMsg.ID.String(),
		})
		return nil
	}
	if !r.finishTurn(executionCtx, sw, uid, conversationID, run, assistantMsg, turn, result, baseIDs) {
		r.handleExecutionContextDone(executionCtx, sw, streamState{
			UID: uid, ConversationID: conversationID, TurnID: turn, Run: run, AssistantMsg: assistantMsg, BaseIDs: baseIDs, AssistantMsgID: assistantMsg.ID.String(),
		})
		return nil
	}

	// --- 10. Emit stream.done ---
	r.sendNewEvent(executionCtx, sw, "stream", "stream.done", baseIDs, map[string]any{}, "", "stream.done")
	return nil
}

// executeRunFlow is the shared turn execution path used by StartRun and ResumeInteraction.
// It loads the profile, starts the AI stream, and processes events. The caller is responsible for
// setup (idempotency, turn envelope, initial SSE events) and teardown (persist, stream.done).
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
	userMessageID uuid.UUID,
	parts []PartInput,
	session *model.ConsultationSession,
	spatialContext *service.ConsultationSpatialContext,
) (streamResult, bool) {
	profileJSON, profileErr := r.loadProfileJSON(ctx, uid)
	if profileErr != nil {
		r.failBeforeStreaming(ctx, sw, run, assistantMsg, uid, conversationID, "failed to load profile")
		return streamResult{}, true
	}

	images := r.resolveConsultationImages(ctx, uid, parts)

	events, err := r.aiClient.StartConsultationTurn(
		ctx,
		conversationID.String(),
		service.StartConsultationTurnRequest{
			RunID:           run.ID.String(),
			ConversationID:  conversationID.String(),
			UserID:          uid.String(),
			ConfigurationID: r.deployment.ConsultationConfigurationID(),
			Input: service.ConsultationUserInput{
				Type:      "user_message",
				Text:      userText,
				Images:    images,
				MessageID: userMessageID.String(),
			},
			BusinessContext: r.buildBusinessContext(ctx, uid, conversationID, userText, profileJSON, session, spatialContext),
		},
	)
	if err != nil {
		r.failBeforeStreaming(ctx, sw, run, assistantMsg, uid, conversationID, "AI service unavailable")
		return streamResult{}, true
	}

	return r.streamAIEvents(ctx, sw, events, streamState{
		UID:                     uid,
		ConversationID:          conversationID,
		TurnID:                  turn,
		Run:                     run,
		AssistantMsg:            assistantMsg,
		BaseIDs:                 baseIDs,
		CurrentPhase:            string(session.Phase),
		AssistantMsgID:          assistantMsg.ID.String(),
		ConversationStr:         conversationID.String(),
		ExpectedConfigurationID: r.deployment.ConsultationConfigurationID(),
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
) bool {
	terminalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	runCompletedEvent, err := sw.NewEvent(
		"run", "run.completed",
		dto.StreamEventIDs{ConversationID: conversationID.String(), RunID: run.ID.String(), TurnID: turn.String()},
		map[string]any{"status": "completed", "usage": result.Usage}, "",
	)
	if err != nil {
		slog.Default().Error("build_run_completed_event_failed", "run_id", run.ID, "error", err)
		return false
	}
	completed, err := r.runService.TryCompleteRunWithEvent(
		terminalCtx, run, uid, result.Usage, result.ProviderResponseID, runCompletedEvent,
	)
	if err != nil {
		slog.Default().Error("complete_run_atomically_failed", "run_id", run.ID, "error", err)
		return false
	}
	if !completed {
		return false
	}

	finalPartsJSON, _ := json.Marshal(result.AssistantParts)
	messagePersisted := true
	if err := r.messageService.UpdateMessageCompleted(
		terminalCtx,
		assistantMsg.ID,
		conversationID,
		datatypes.JSON(finalPartsJSON),
		nil,
		nil,
	); err != nil {
		messagePersisted = false
		slog.Default().Error("update_assistant_message_failed", "message_id", assistantMsg.ID, "error", err)
	}
	if messagePersisted {
		r.recordKnowledgeRuntimeObservations(terminalCtx, run.ID, assistantMsg.ID, result.AssistantParts)
	}
	if err := r.conversationService.UpdateLastMessageAt(terminalCtx, conversationID, uid); err != nil {
		slog.Default().Error("update_last_message_at_failed", "conversation_id", conversationID, "error", err)
	}

	if result.ExecutionIdentity.ConfigurationID != "" {
		if err := r.runService.UpdateAgentConfiguration(
			terminalCtx,
			run.ID,
			result.ExecutionIdentity.ConfigurationID,
			datatypes.JSON(result.ExecutionIdentity.AgentConfiguration),
			datatypes.JSON(result.ExecutionIdentity.ExecutionProvenance),
		); err != nil {
			slog.Default().Error("persist_run_agent_configuration_failed", "run_id", run.ID, "error", err)
		}
	}
	if r.rolloutService != nil && result.ExecutionIdentity.ConfigurationID != "" {
		decision := &service.ConsultationRunDecision{
			RunID:                      run.ID.String(),
			SourceConfigurationID:      r.deployment.ConsultationConfigurationID(),
			PersistedConfigurationID:   result.ExecutionIdentity.ConfigurationID,
			ConfigurationIdentityMatch: result.ExecutionIdentity.ConfigurationID == r.deployment.ConsultationConfigurationID(),
			ReplayInputFrozen:          len(run.ReplayInput) > 2,
			ExecutionProvenance:        datatypes.JSON(result.ExecutionIdentity.ExecutionProvenance),
		}
		if err := r.rolloutService.ObserveRun(terminalCtx, run, decision); err != nil {
			slog.Default().Error("record_consultation_rollout_observation_failed", "run_id", run.ID, "error", err)
		}
	}
	r.recordGovernance(terminalCtx, uid, conversationID, run.ID, result.GovernanceResult)
	r.clearActiveRun(terminalCtx, conversationID, uid)
	r.refreshThreadProjection(terminalCtx, conversationID, uid)
	if err := sw.WriteEvent(terminalCtx, runCompletedEvent); err != nil {
		slog.Default().Warn("sse_write_error", "event_type", "run.completed", "run_id", run.ID, "conversation_id", conversationID, "seq", runCompletedEvent.Seq, "error", err)
	}
	r.sendNewEvent(
		terminalCtx, sw, "message", "message.completed",
		dto.StreamEventIDs{ConversationID: conversationID.String(), RunID: run.ID.String(), TurnID: turn.String(), MessageID: assistantMsg.ID.String()},
		map[string]any{"status": "completed", "finish_reason": "stop", "usage": result.Usage},
		assistantMsg.ID.String(), "message.completed",
	)
	return true
}
