package consultation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
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

func durableExecutionContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), sseTimeout)
}

type HTTPError struct {
	Status  int
	Code    string
	Message string
}

type runtimeBodyStateService interface {
	GetSnapshot(ctx context.Context, userID uuid.UUID, historyLimit int) (*service.BodyStateSnapshot, error)
	UpsertExtractedSymptom(ctx context.Context, userID, runID uuid.UUID, info json.RawMessage) error
	RecordSafetyEvent(ctx context.Context, userID uuid.UUID, payload json.RawMessage) error
	RecordInteractionAnswer(ctx context.Context, userID, interactionID uuid.UUID, question datatypes.JSON, answer json.RawMessage) error
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
	contextRetrievalService     *service.ContextRetrievalService
	diagnosisAnalysisService    *service.DiagnosisAnalysisService
	diagnosisFreshnessService   *service.DiagnosisFreshnessService
	treatmentService            *service.TreatmentService
	streamRuntime               *stream.Runtime
	deployment                  *service.AgentDeploymentPolicy
	rolloutService              *service.ConsultationRolloutService
	knowledgeObservationService runtimeKnowledgeObservationService

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
		streamRuntime:           stream.NewRuntime(),
		runCancels:              make(map[uuid.UUID]context.CancelFunc),
	}
}

// AttachLongitudinalContextServices adds optional read-only business context
// collaborators without expanding the stable runtime constructor used by tests.
func (r *Runtime) AttachLongitudinalContextServices(
	contextRetrieval *service.ContextRetrievalService,
	diagnosis *service.DiagnosisAnalysisService,
	freshness *service.DiagnosisFreshnessService,
	treatment *service.TreatmentService,
) {
	r.contextRetrievalService = contextRetrieval
	r.diagnosisAnalysisService = diagnosis
	r.diagnosisFreshnessService = freshness
	r.treatmentService = treatment
}

// AttachRolloutService attaches the anonymous rollout observer (North-Star).
func (r *Runtime) AttachRolloutService(rollout *service.ConsultationRolloutService) {
	r.rolloutService = rollout
}

// AttachKnowledgeObservationService records publication-version answer observations after message completion.
func (r *Runtime) AttachKnowledgeObservationService(observer runtimeKnowledgeObservationService) {
	r.knowledgeObservationService = observer
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
	run, transitioned, err := r.runService.CancelRun(ctx, runID, uid, reason)
	if err != nil {
		if errors.Is(err, service.ErrRunTerminal) {
			return httpErr(http.StatusConflict, "RUN_TERMINAL", "run is already terminal")
		}
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to cancel run")
	}
	if run == nil {
		return httpErr(http.StatusNotFound, "NOT_FOUND", "run not found")
	}

	// Pending HITL belongs to the cancelled run and must not remain answerable.
	pending, pendingErr := r.interactionService.GetPendingInteractions(ctx, run.ConversationID)
	if pendingErr != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load pending interactions")
	}
	for i := range pending {
		if pending[i].RunID == runID {
			if cancelErr := r.interactionService.CancelInteraction(ctx, pending[i].ID); cancelErr != nil && !errors.Is(cancelErr, service.ErrInteractionClosed) {
				return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to cancel pending interaction")
			}
		}
	}

	// If an execution context is registered, it owns the live StreamWriter and
	// will emit run.cancelled using the same per-run sequence. Waiting runs have
	// no writer, so the endpoint appends the terminal event out-of-band.
	active := r.cancelRegisteredRun(runID)
	if transitioned && !active && r.runtimeEventService != nil {
		if err := r.runtimeEventService.RecordRunCancelled(ctx, run, reason); err != nil {
			return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to persist cancellation event")
		}
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
	conversationIDMissing := req.ConversationID == nil || *req.ConversationID == ""
	if !conversationIDMissing {
		parsed, err := uuid.Parse(*req.ConversationID)
		if err != nil {
			return httpErr(http.StatusBadRequest, "INVALID_ID", "invalid conversation id")
		}
		requestedConversationID = &parsed
	}

	// --- 3. Validate message ---
	userText := messagePartsToText(req.Message.Parts)
	imageIDs := messagePartsToImageUploadIDs(req.Message.Parts)
	if strings.TrimSpace(userText) == "" && len(imageIDs) == 0 {
		return httpErr(http.StatusBadRequest, "INVALID_REQUEST", "message text or image is required")
	}
	if strings.TrimSpace(userText) == "" {
		userText = "请结合我附上的照片，分析与体态/不适相关的可见信息，并给出谨慎建议。"
	}

	userPartsJSON, err := json.Marshal(req.Message.Parts)
	if err != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to marshal message parts")
	}

	// --- 4. Create turn envelope (conversation/session + run + messages) ---
	session, turn, run, userMsg, assistantMsg, baseIDs, conversationCreated, setupErr := r.createTurnEnvelope(
		ctx, uid, requestedConversationID, req.RequestID,
		datatypes.JSON(userPartsJSON), datatypes.JSON("{}"),
	)
	if setupErr != nil {
		return setupErr
	}
	conversationID := session.ConversationID
	isNewConversation := conversationCreated
	executionCtx, cancelExecution := durableExecutionContext(ctx)
	defer cancelExecution()
	unregisterCancellation := r.registerRunCancellation(run.ID, cancelExecution)
	defer unregisterCancellation()

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
	r.emitUserTurnStarted(executionCtx, sw, req.ClientMessageID, userMsg, assistantMsg)
	r.refreshThreadProjection(executionCtx, conversationID, uid)

	// --- 8. Stream AI response ---
	result, stopped := r.executeRunFlow(executionCtx, sw, uid, conversationID, turn, run, assistantMsg, baseIDs, userText, req.Message.Parts, session)
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

	// --- 10. Generate title for new conversations (non-blocking) ---
	if isNewConversation {
		r.generateTitleAndNotify(executionCtx, conversationID, uid, sw, baseIDs)
	}

	// --- 11. Emit stream.done ---
	r.sendNewEvent(executionCtx, sw, "stream", "stream.done", baseIDs, map[string]any{}, "", "stream.done")
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
	parts []dto.PartDTO,
	session *model.ConsultationSession,
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
				Type:   "user_message",
				Text:   userText,
				Images: images,
			},
			BusinessContext: r.buildBusinessContext(ctx, uid, conversationID, userText, profileJSON, session),
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
		CurrentPhase:            session.Phase,
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

	completed, err := r.runService.TryCompleteRun(terminalCtx, run.ID, uid, result.Usage, result.ProviderResponseID)
	if err != nil {
		log.Printf("failed to complete run %s: %v", run.ID, err)
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
		log.Printf("failed to update assistant message %s: %v", assistantMsg.ID, err)
	}
	if messagePersisted {
		r.recordKnowledgeRuntimeObservations(terminalCtx, run.ID, assistantMsg.ID, result.AssistantParts)
	}
	if err := r.conversationService.UpdateLastMessageAt(terminalCtx, conversationID, uid); err != nil {
		log.Printf("failed to update last_message_at for conversation %s: %v", conversationID, err)
	}

	if result.ExecutionIdentity.ConfigurationID != "" {
		if err := r.runService.UpdateAgentConfiguration(
			terminalCtx,
			run.ID,
			result.ExecutionIdentity.ConfigurationID,
			datatypes.JSON(result.ExecutionIdentity.AgentConfiguration),
			datatypes.JSON(result.ExecutionIdentity.ExecutionProvenance),
		); err != nil {
			log.Printf("failed to persist run %s agent configuration: %v", run.ID, err)
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
			log.Printf("failed to record consultation rollout observation for run %s: %v", run.ID, err)
		}
	}
	r.recordGovernance(terminalCtx, uid, conversationID, run.ID, result.GovernanceResult)
	r.clearActiveRun(terminalCtx, conversationID, uid)
	r.refreshThreadProjection(terminalCtx, conversationID, uid)
	r.sendNewEvent(
		terminalCtx, sw, "run", "run.completed",
		dto.StreamEventIDs{ConversationID: conversationID.String(), RunID: run.ID.String(), TurnID: turn.String()},
		map[string]any{"status": "completed", "usage": result.Usage}, "", "run.completed",
	)
	r.sendNewEvent(
		terminalCtx, sw, "message", "message.completed",
		dto.StreamEventIDs{ConversationID: conversationID.String(), RunID: run.ID.String(), TurnID: turn.String(), MessageID: assistantMsg.ID.String()},
		map[string]any{"status": "completed", "finish_reason": "stop", "usage": result.Usage},
		assistantMsg.ID.String(), "message.completed",
	)
	return true
}

func (r *Runtime) handleExecutionContextDone(ctx context.Context, sw *stream.StreamWriter, state streamState) {
	terminalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	run, err := r.runService.GetRunForUser(terminalCtx, state.Run.ID, state.UID)
	if err != nil {
		log.Printf("load terminal run %s after context cancellation: %v", state.Run.ID, err)
		return
	}
	if run != nil && run.Status == "cancelled" {
		if state.AssistantMsg != nil {
			parts, _ := json.Marshal([]map[string]any{})
			_ = r.messageService.UpdateMessageCompletedWithStatus(
				terminalCtx, state.AssistantMsg.ID, state.ConversationID,
				datatypes.JSON(parts), "aborted",
			)
		}
		r.sendNewEvent(
			terminalCtx, sw, "run", "run.cancelled",
			dto.StreamEventIDs{ConversationID: state.ConversationID.String(), RunID: state.Run.ID.String(), TurnID: state.TurnID.String()},
			map[string]any{"status": "cancelled", "reason": "cancelled_by_user"}, "", "run.cancelled",
		)
		r.sendNewEvent(terminalCtx, sw, "stream", "stream.done", state.BaseIDs, map[string]any{}, "", "stream.done")
		r.clearActiveRun(terminalCtx, state.ConversationID, state.UID)
		r.refreshThreadProjection(terminalCtx, state.ConversationID, state.UID)
		return
	}

	// Deadline/host-side cancellation is operational failure, distinct from an
	// explicit user cancellation command.
	r.failActiveStream(terminalCtx, sw, state, "consultation execution deadline exceeded")
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

	// Resume is continuation of the exact logical Agent thread. Pin it to the
	// immutable configuration recorded on the interrupted source run instead of
	// silently switching to the current Champion while the user was waiting.
	sourceRun, err := r.runService.GetRunForUser(ctx, interaction.RunID, uid)
	if err != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load interrupted run")
	}
	if sourceRun == nil || sourceRun.ConversationID != conversationID {
		return httpErr(http.StatusNotFound, "NOT_FOUND", "interrupted run not found")
	}
	pinnedConfigurationID := strings.TrimSpace(sourceRun.AgentConfigurationID)
	if pinnedConfigurationID == "" {
		return httpErr(
			http.StatusConflict,
			"INTERACTION_NOT_RESUMABLE",
			"interrupted run predates durable Agent configuration identity; start a new run",
		)
	}
	if _, err := service.ConsultationDecisionPolicyRevisionForConfiguration(pinnedConfigurationID); err != nil {
		return httpErr(http.StatusConflict, "INTERACTION_NOT_RESUMABLE", "interrupted run configuration is no longer repository-authorized")
	}

	// The interaction answer is a durable health input, not merely a chat message.
	// Commit it before closing the interaction so a persistence failure remains
	// retryable and never resumes the Agent from an unrecorded health answer.
	if err := r.persistInteractionAnswer(ctx, uid, interactionID, interaction.Question, req.Answer); err != nil {
		return httpErr(http.StatusInternalServerError, "BODY_STATE_PERSISTENCE_FAILED", "failed to persist interaction answer")
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
		case err == service.ErrInteractionExpired:
			return httpErr(http.StatusConflict, "INTERACTION_EXPIRED", "interaction has expired; start a new question")
		default:
			return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
	}

	closedSourceRun, err := r.runService.TryCompleteRun(ctx, interaction.RunID, uid, nil, "")
	if err != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to close interrupted run")
	}
	if !closedSourceRun {
		return httpErr(http.StatusConflict, "RUN_TERMINAL", "interrupted run was cancelled or already closed")
	}

	answerPartsJSON, answerMetadata := interactionAnswerParts(req.Answer, interactionID.String())
	_, turn, run, _, assistantMsg, baseIDs, _, setupErr := r.createTurnEnvelope(
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

	executionCtx, cancelExecution := durableExecutionContext(ctx)
	defer cancelExecution()
	unregisterCancellation := r.registerRunCancellation(run.ID, cancelExecution)
	defer unregisterCancellation()

	profileJSON, profileErr := r.loadProfileJSON(executionCtx, uid)
	if profileErr != nil {
		return httpErr(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load profile")
	}

	events, err := r.aiClient.ResumeConsultationInterrupt(
		executionCtx,
		conversationID.String(),
		interactionID.String(),
		service.ResumeConsultationInterruptRequest{
			RunID:           run.ID.String(),
			ConversationID:  conversationID.String(),
			UserID:          uid.String(),
			ConfigurationID: pinnedConfigurationID,
			InterruptID:     interactionID.String(),
			Answer:          json.RawMessage(req.Answer),
			BusinessContext: r.buildBusinessContext(executionCtx, uid, conversationID, extractAnswerText(req.Answer), profileJSON, session),
		},
	)
	if err != nil {
		r.failBeforeStreaming(executionCtx, sw, run, assistantMsg, uid, conversationID, "AI service unavailable")
		return nil
	}

	result, stopped := r.streamAIEvents(executionCtx, sw, events, streamState{
		UID:                     uid,
		ConversationID:          conversationID,
		TurnID:                  turn,
		Run:                     run,
		AssistantMsg:            assistantMsg,
		BaseIDs:                 baseIDs,
		CurrentPhase:            session.Phase,
		AssistantMsgID:          assistantMsg.ID.String(),
		ConversationStr:         conversationID.String(),
		ExpectedConfigurationID: pinnedConfigurationID,
	})
	if stopped {
		return nil
	}

	if executionCtx.Err() != nil || !r.finishTurn(executionCtx, sw, uid, conversationID, run, assistantMsg, turn, result, baseIDs) {
		r.handleExecutionContextDone(executionCtx, sw, streamState{
			UID: uid, ConversationID: conversationID, TurnID: turn, Run: run, AssistantMsg: assistantMsg, BaseIDs: baseIDs, AssistantMsgID: assistantMsg.ID.String(),
		})
		return nil
	}
	r.sendNewEvent(executionCtx, sw, "stream", "stream.done", baseIDs, map[string]any{}, "", "stream.done")
	r.maybeGenerateTitle(executionCtx, conversationID, uid)
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
	bool,
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
		if errors.Is(err, model.ErrConversationRunInProgress) {
			return nil, uuid.Nil, nil, nil, nil, dto.StreamEventIDs{}, false, httpErr(
				http.StatusConflict,
				"RUN_IN_PROGRESS",
				"this conversation already has an active run",
			)
		}
		return nil, uuid.Nil, nil, nil, nil, dto.StreamEventIDs{}, false, httpErr(
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"failed to create run envelope",
		)
	}
	if envelope.Existed {
		return nil, uuid.Nil, nil, nil, nil, dto.StreamEventIDs{}, false, httpErr(
			http.StatusConflict,
			"RUN_IN_PROGRESS",
			"a run with this request ID already exists",
		)
	}

	return envelope.Session, envelope.TurnID, envelope.Run, envelope.UserMessage, envelope.AssistantMessage, dto.StreamEventIDs{
		ConversationID: envelope.Session.ConversationID.String(),
		RunID:          envelope.Run.ID.String(),
		TurnID:         envelope.TurnID.String(),
	}, envelope.ConversationCreated, nil
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

// buildBusinessContext assembles the payload Python's consultation runtime
// receives for a turn. Prefetches completed posture analysis so the Agent
// tool can read analysis_result without a reverse HTTP call.
func (r *Runtime) buildBusinessContext(
	ctx context.Context,
	uid uuid.UUID,
	conversationID uuid.UUID,
	queryText string,
	profileJSON json.RawMessage,
	session *model.ConsultationSession,
) service.ConsultationBusinessContext {
	bc := service.ConsultationBusinessContext{
		Profile: profileJSON,
		RuntimeState: service.ConsultationRuntimeState{
			Phase:         session.Phase,
			ExtractedInfo: json.RawMessage(session.ExtractedInfo),
		},
	}
	var bodyStateSnapshot *service.BodyStateSnapshot
	if r.bodyStateService != nil {
		if snapshot, err := r.bodyStateService.GetSnapshot(ctx, uid, 20); err != nil {
			log.Printf("body state context load failed for user %s: %v", uid, err)
		} else {
			bodyStateSnapshot = snapshot
			if raw, marshalErr := json.Marshal(snapshot); marshalErr == nil {
				bc.BodyState = raw
			}
		}
	}
	if r.contextRetrievalService != nil {
		if history, err := r.contextRetrievalService.Retrieve(
			ctx, uid, conversationID, queryText, bodyStateSnapshot,
		); err != nil {
			log.Printf("historical context retrieval failed for conversation %s: %v", conversationID, err)
		} else {
			bc.RelevantHistory = history
		}
	}
	if r.diagnosisAnalysisService != nil {
		if analysis, err := r.diagnosisAnalysisService.GetLatest(ctx, uid); err != nil {
			log.Printf("current diagnosis context load failed for user %s: %v", uid, err)
		} else if analysis != nil {
			payload := r.diagnosisAnalysisService.PublicPayload(analysis)
			if r.diagnosisFreshnessService != nil {
				if freshness, freshErr := r.diagnosisFreshnessService.Preview(ctx, uid, analysis); freshErr == nil {
					payload["freshness"] = freshness
				}
			}
			if raw, marshalErr := json.Marshal(payload); marshalErr == nil {
				bc.CurrentDiagnosis = raw
			}
		}
	}
	if r.treatmentService != nil {
		if treatment, err := r.treatmentService.PreviewCurrentReview(ctx, uid); err != nil {
			log.Printf("current treatment context load failed for user %s: %v", uid, err)
		} else if treatment != nil {
			if raw, marshalErr := json.Marshal(treatment); marshalErr == nil {
				bc.CurrentTreatment = raw
			}
		}
		if outcomes, err := r.treatmentService.ListOutcomes(ctx, uid, 12); err == nil {
			if raw, marshalErr := json.Marshal(outcomes); marshalErr == nil {
				bc.RecentOutcomes = raw
			}
		}
	}
	if r.uploadService == nil {
		return bc
	}
	summary, err := r.uploadService.GetPostureAnalysisSummary(ctx, uid)
	if err != nil {
		log.Printf("posture analysis prefetch failed for user %s: %v", uid, err)
		return bc
	}
	if raw, err := json.Marshal(summary); err == nil {
		bc.PostureAnalysis = raw
	}
	return bc
}

type streamState struct {
	UID                     uuid.UUID
	ConversationID          uuid.UUID
	TurnID                  uuid.UUID
	Run                     *model.Run
	AssistantMsg            *model.Message
	BaseIDs                 dto.StreamEventIDs
	CurrentPhase            string
	AssistantMsgID          string
	ConversationStr         string
	ExpectedConfigurationID string
}

type ConsultationExecutionIdentity struct {
	ConfigurationID     string
	AgentConfiguration  json.RawMessage
	ExecutionProvenance json.RawMessage
}

type streamResult struct {
	AssistantParts     []map[string]any
	Usage              any
	ProviderResponseID string
	GovernanceResult   datatypes.JSON
	ExecutionIdentity  ConsultationExecutionIdentity
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
	handshakeAccepted := false

	for {
		select {
		case <-ctx.Done():
			r.handleExecutionContextDone(ctx, sw, state)
			return result, true
		case event, ok := <-events:
			// If explicit cancellation raced with a ready producer event, prefer
			// the business terminal state over accepting one more semantic event.
			// A select may choose either ready case, so re-check the context here.
			if ctx.Err() != nil {
				r.handleExecutionContextDone(ctx, sw, state)
				return result, true
			}
			if !ok {
				if !handshakeAccepted {
					r.failActiveStream(ctx, sw, state, "missing runtime Agent configuration handshake")
					return result, true
				}
				return result, false
			}

			eventCount++
			if eventCount > maxSSEEvents {
				log.Printf("event count exceeded limit for conversation %s", state.ConversationID)
				r.failActiveStream(ctx, sw, state, "stream exceeded maximum event count")
				return result, true
			}

			// The immutable execution identity is a control-plane handshake, not
			// ordinary semantic output. Nothing else is trusted until it is the
			// first event and has been validated/persisted by Go.
			if !handshakeAccepted && event.Type != "runtime.agent_configuration" {
				r.failActiveStream(ctx, sw, state, "runtime Agent configuration handshake must be the first event")
				return result, true
			}
			if handshakeAccepted && event.Type == "runtime.agent_configuration" {
				r.failActiveStream(ctx, sw, state, "duplicate runtime Agent configuration handshake")
				return result, true
			}

			if r.handleAIEvent(ctx, sw, event, state, &result, &phase) {
				return result, true
			}
			if event.Type == "runtime.agent_configuration" {
				handshakeAccepted = result.ExecutionIdentity.ConfigurationID != ""
				if !handshakeAccepted {
					r.failActiveStream(ctx, sw, state, "invalid runtime Agent configuration handshake")
					return result, true
				}
			}
		}
	}
}

type consultationConfigurationEnvelope struct {
	ID                     string `json:"id"`
	Role                   string `json:"role"`
	DecisionPolicyRevision string `json:"decision_policy_revision"`
	LogicalModel           string `json:"logical_model"`
}

type consultationExecutionProvenanceEnvelope struct {
	LogicalModel string `json:"logical_model"`
}

func validateConsultationExecutionIdentity(
	event dto.StreamEvent,
	expectedConfigurationID string,
) (ConsultationExecutionIdentity, error) {
	if event.Type != "runtime.agent_configuration" || event.Channel != "runtime" {
		return ConsultationExecutionIdentity{}, fmt.Errorf("unexpected handshake event %s/%s", event.Channel, event.Type)
	}
	expectedConfigurationID = strings.TrimSpace(expectedConfigurationID)
	if expectedConfigurationID == "" {
		return ConsultationExecutionIdentity{}, errors.New("missing expected Consultation configuration id")
	}
	expectedPolicy, err := service.ConsultationDecisionPolicyRevisionForConfiguration(expectedConfigurationID)
	if err != nil {
		return ConsultationExecutionIdentity{}, err
	}
	expectedLogicalModel, err := service.ConsultationLogicalModelForConfiguration(expectedConfigurationID)
	if err != nil {
		return ConsultationExecutionIdentity{}, err
	}

	var payload struct {
		AgentConfiguration  json.RawMessage `json:"agent_configuration"`
		ExecutionProvenance json.RawMessage `json:"execution_provenance"`
	}
	if err := event.PayloadAs(&payload); err != nil {
		return ConsultationExecutionIdentity{}, fmt.Errorf("decode handshake payload: %w", err)
	}
	if len(payload.AgentConfiguration) == 0 || len(payload.ExecutionProvenance) == 0 {
		return ConsultationExecutionIdentity{}, errors.New("handshake is missing configuration or execution provenance")
	}

	var configuration consultationConfigurationEnvelope
	if err := json.Unmarshal(payload.AgentConfiguration, &configuration); err != nil {
		return ConsultationExecutionIdentity{}, fmt.Errorf("decode Agent configuration: %w", err)
	}
	var provenance consultationExecutionProvenanceEnvelope
	if err := json.Unmarshal(payload.ExecutionProvenance, &provenance); err != nil {
		return ConsultationExecutionIdentity{}, fmt.Errorf("decode execution provenance: %w", err)
	}

	if configuration.ID != expectedConfigurationID {
		return ConsultationExecutionIdentity{}, fmt.Errorf("configuration id %q != expected %q", configuration.ID, expectedConfigurationID)
	}
	if configuration.Role != "consultation" {
		return ConsultationExecutionIdentity{}, fmt.Errorf("role %q != consultation", configuration.Role)
	}
	if configuration.DecisionPolicyRevision != expectedPolicy {
		return ConsultationExecutionIdentity{}, fmt.Errorf("decision policy %q != expected %q", configuration.DecisionPolicyRevision, expectedPolicy)
	}
	if configuration.LogicalModel != expectedLogicalModel {
		return ConsultationExecutionIdentity{}, fmt.Errorf("logical model %q != expected %q", configuration.LogicalModel, expectedLogicalModel)
	}
	if provenance.LogicalModel != expectedLogicalModel {
		return ConsultationExecutionIdentity{}, fmt.Errorf("execution logical model %q != expected %q", provenance.LogicalModel, expectedLogicalModel)
	}

	return ConsultationExecutionIdentity{
		ConfigurationID:     configuration.ID,
		AgentConfiguration:  append(json.RawMessage(nil), payload.AgentConfiguration...),
		ExecutionProvenance: append(json.RawMessage(nil), payload.ExecutionProvenance...),
	}, nil
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
		if err := event.PayloadAs(&payload); err != nil {
			r.failActiveStream(ctx, sw, state, "invalid message delta payload")
			return true
		}
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
		if err := event.PayloadAs(&payload); err != nil {
			r.failActiveStream(ctx, sw, state, "invalid tool.call payload")
			return true
		}
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, "tool.call")
		result.AssistantParts = append(
			result.AssistantParts,
			map[string]any{
				"type":       "tool-call",
				"toolName":   payload.Tool,
				"args":       payload.Args,
				"toolCallId": event.IDs.ToolCallID,
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
		if err := event.PayloadAs(&payload); err != nil {
			r.failActiveStream(ctx, sw, state, "invalid tool.result payload")
			return true
		}
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, "tool.result")
		result.AssistantParts = append(
			result.AssistantParts,
			map[string]any{
				"type":       "tool-result",
				"toolName":   payload.Tool,
				"result":     payload.Result,
				"toolCallId": event.IDs.ToolCallID,
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
		var payload struct {
			Info json.RawMessage `json:"info"`
		}
		if err := event.PayloadAs(&payload); err != nil {
			r.failActiveStream(ctx, sw, state, "invalid extracted-info payload")
			return true
		}
		if err := r.persistExtractedSymptom(ctx, state.UID, state.Run.ID, payload.Info); err != nil {
			log.Printf("failed to persist extracted symptom in BodyState for run %s: %v", state.Run.ID, err)
			r.failActiveStream(ctx, sw, state, "failed to persist durable health state")
			return true
		}
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, event.Type)

	case "source.citation.added":
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, event.Type)
		result.AssistantParts = append(result.AssistantParts, citationPart(event.Payload))

	case "source.answer_attribution.added":
		if _, err := service.ParseConsultationAnswerAttributionPayload(event.Payload); err != nil {
			r.failActiveStream(ctx, sw, state, "invalid answer attribution payload")
			return true
		}
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, event.Type)
		result.AssistantParts = append(result.AssistantParts, dataPart("answer_attribution", event.Payload))

	case "source.knowledge_gap":
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, event.Type)
		result.AssistantParts = append(result.AssistantParts, dataPart("knowledge_gap", event.Payload))

	case "safety.red_flag.detected":
		if err := r.persistSafetyEvent(ctx, state.UID, event.Payload); err != nil {
			log.Printf("failed to persist BodyState safety event for run %s: %v", state.Run.ID, err)
			r.failActiveStream(ctx, sw, state, "failed to persist safety state")
			return true
		}
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, event.Type)
		result.AssistantParts = append(result.AssistantParts, dataPart("red_flag", event.Payload))

	case "safety.output_reviewed", "safety.output_rejected":
		r.handleSafetyOutputEvent(ctx, sw, event, state, result)

	case "state.interaction.required":
		return r.handleInteractionRequired(ctx, sw, event, state, result)

	case "state.phase.changed":
		var payload struct {
			From   string `json:"from,omitempty"`
			To     string `json:"to"`
			Reason string `json:"reason"`
		}
		if err := event.PayloadAs(&payload); err != nil {
			r.failActiveStream(ctx, sw, state, "invalid phase-change payload")
			return true
		}
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
		if err := event.PayloadAs(&payload); err != nil {
			r.failActiveStream(ctx, sw, state, "invalid usage payload")
			return true
		}
		result.Usage = payload.Usage
		r.sendEvent(ctx, sw, event, state.AssistantMsgID, "usage.reported")

	case "runtime.agent_configuration":
		identity, err := validateConsultationExecutionIdentity(event, state.ExpectedConfigurationID)
		if err != nil {
			log.Printf("rejected Consultation runtime identity for run %s: %v", state.Run.ID, err)
			r.failActiveStream(ctx, sw, state, "runtime Agent configuration identity mismatch")
			return true
		}
		if r.runService == nil {
			r.failActiveStream(ctx, sw, state, "runtime Agent configuration persistence unavailable")
			return true
		}
		if err := r.runService.UpdateAgentConfiguration(
			ctx,
			state.Run.ID,
			identity.ConfigurationID,
			datatypes.JSON(identity.AgentConfiguration),
			datatypes.JSON(identity.ExecutionProvenance),
		); err != nil {
			log.Printf("failed to persist Consultation runtime identity for run %s: %v", state.Run.ID, err)
			r.failActiveStream(ctx, sw, state, "failed to persist runtime Agent configuration identity")
			return true
		}
		result.ExecutionIdentity = identity

	case "stream.done":
		var payload struct {
			ResponseID string          `json:"response_id"`
			Usage      json.RawMessage `json:"usage"`
			Governance json.RawMessage `json:"governance"`
		}
		if err := event.PayloadAs(&payload); err != nil {
			r.failActiveStream(ctx, sw, state, "invalid stream.done payload")
			return true
		}
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
		if err := event.PayloadAs(&payload); err != nil {
			payload.Message = "AI runtime protocol error"
		}
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
		CreatedAt     string          `json:"created_at,omitempty"`
	}
	if err := event.PayloadAs(&payload); err != nil {
		r.failActiveStream(ctx, sw, state, "invalid interaction-required payload")
		return true
	}

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
	payload.CreatedAt = interaction.CreatedAt.UTC().Format(time.RFC3339Nano)
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

func (r *Runtime) handleSafetyOutputEvent(
	ctx context.Context,
	sw *stream.StreamWriter,
	event dto.StreamEvent,
	state streamState,
	result *streamResult,
) {
	r.sendEvent(ctx, sw, event, state.AssistantMsgID, event.Type)

	var payload struct {
		Kind           string          `json:"kind"`
		Verdict        string          `json:"verdict"`
		Reasons        []string        `json:"reasons"`
		Issues         json.RawMessage `json:"issues"`
		SafetyFallback string          `json:"safety_fallback"`
	}
	if err := event.PayloadAs(&payload); err != nil {
		log.Printf("invalid safety output payload for run %s: %v", state.Run.ID, err)
		return
	}

	partName := "output_reviewed"
	if event.Type == "safety.output_rejected" {
		partName = "output_rejected"
	}
	result.AssistantParts = append(result.AssistantParts, dataPart(partName, event.Payload))

	if r.outputReviewService == nil {
		return
	}

	outputType := payload.Kind
	if outputType == "" {
		outputType = "structured_output"
	}
	verdict := payload.Verdict
	if verdict == "" {
		if event.Type == "safety.output_rejected" {
			verdict = "rejected"
		} else {
			verdict = "unknown"
		}
	}

	issues := datatypes.JSON("[]")
	if len(payload.Issues) > 0 {
		issues = datatypes.JSON(payload.Issues)
	} else if len(payload.Reasons) > 0 {
		if b, err := json.Marshal(payload.Reasons); err == nil {
			issues = datatypes.JSON(b)
		}
	}

	runID := state.Run.ID
	convID := state.ConversationID
	uid := state.UID
	r.outputReviewService.RecordReview(
		ctx,
		outputType,
		verdict,
		&uid,
		&runID,
		nil,
		&convID,
		issues,
		nil,
		datatypes.JSON(event.Payload),
	)
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
		// Prefer P2 "verdict"; keep "status" for older stream.done payloads.
		Verdict         string          `json:"verdict"`
		Status          string          `json:"status"`
		Issues          json.RawMessage `json:"issues"`
		ValidatedOutput json.RawMessage `json:"validated_output"`
	}
	if err := json.Unmarshal(governanceResult, &govPayload); err == nil {
		govStatus = govPayload.Verdict
		if govStatus == "" {
			govStatus = govPayload.Status
		}
		govIssues = datatypes.JSON(govPayload.Issues)
		if len(govPayload.ValidatedOutput) > 0 {
			govValidatedOutput = datatypes.JSON(govPayload.ValidatedOutput)
		}
	} else {
		govStatus = "unknown"
		govIssues = datatypes.JSON("[]")
	}
	if govStatus == "" {
		govStatus = "unknown"
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
	sawStreamDone := false
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
		if stored.Type == "stream.done" {
			sawStreamDone = true
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

	if sawStreamDone {
		return
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

func (r *Runtime) persistExtractedSymptom(
	ctx context.Context,
	userID, runID uuid.UUID,
	info json.RawMessage,
) error {
	if r.bodyStateService == nil {
		return errors.New("BodyState service is not configured")
	}
	return r.bodyStateService.UpsertExtractedSymptom(ctx, userID, runID, info)
}

func (r *Runtime) persistSafetyEvent(
	ctx context.Context,
	userID uuid.UUID,
	payload json.RawMessage,
) error {
	if r.bodyStateService == nil {
		return errors.New("BodyState service is not configured")
	}
	return r.bodyStateService.RecordSafetyEvent(ctx, userID, payload)
}

func (r *Runtime) persistInteractionAnswer(
	ctx context.Context,
	userID, interactionID uuid.UUID,
	question datatypes.JSON,
	answer json.RawMessage,
) error {
	if r.bodyStateService == nil {
		return errors.New("BodyState service is not configured")
	}
	return r.bodyStateService.RecordInteractionAnswer(ctx, userID, interactionID, question, answer)
}

func (r *Runtime) clearActiveRun(ctx context.Context, conversationID, userID uuid.UUID) {
	if r.conversationService == nil {
		return
	}
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
	if r.runService != nil {
		_ = r.runService.FailRun(ctx, state.Run.ID, state.UID, map[string]any{"message": message})
	}
	if r.messageService != nil {
		_ = r.messageService.UpdateMessageStatus(ctx, state.AssistantMsg.ID, state.ConversationID, "failed")
	}
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
	r.recordPublicEvent(ctx, event)
	if err := sw.WriteEvent(ctx, event); err != nil {
		log.Printf("SSE write error (%s): %v", label, err)
	}
}

func (r *Runtime) sendEvent(
	ctx context.Context,
	sw *stream.StreamWriter,
	event dto.StreamEvent,
	messageID string,
	label string,
) {
	enriched := sw.EnrichEvent(event, messageID)
	r.recordPublicEvent(ctx, enriched)
	if err := sw.WriteEvent(ctx, enriched); err != nil {
		log.Printf("SSE write error (%s): %v", label, err)
	}
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

func messagePartsToImageUploadIDs(parts []dto.PartDTO) []string {
	ids := make([]string, 0)
	seen := map[string]bool{}
	for _, part := range parts {
		if part.Type != "image" {
			continue
		}
		id := strings.TrimSpace(part.UploadID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

// resolveConsultationImages turns image parts into data-URLs owned by uid.
// Failures are logged and skipped so a bad attachment does not kill the turn.
func (r *Runtime) resolveConsultationImages(
	ctx context.Context,
	uid uuid.UUID,
	parts []dto.PartDTO,
) []service.ConsultationImageRef {
	if r.uploadService == nil {
		return nil
	}
	ids := messagePartsToImageUploadIDs(parts)
	if len(ids) == 0 {
		return nil
	}
	const maxImages = 3
	if len(ids) > maxImages {
		ids = ids[:maxImages]
	}
	out := make([]service.ConsultationImageRef, 0, len(ids))
	for _, rawID := range ids {
		uploadID, err := uuid.Parse(rawID)
		if err != nil {
			log.Printf("consultation image upload_id invalid %q: %v", rawID, err)
			continue
		}
		dataURL, mime, err := r.uploadService.ReadImageDataURL(ctx, uid, uploadID)
		if err != nil {
			log.Printf("consultation image resolve %s: %v", uploadID, err)
			continue
		}
		out = append(out, service.ConsultationImageRef{
			UploadID: uploadID.String(),
			MimeType: mime,
			DataURL:  dataURL,
		})
	}
	return out
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
	metadata := map[string]any{
		"is_interaction_answer": true,
		"interaction_id":        interactionID,
	}
	// Preserve structured multi-field answers for runtime audit and analytics.
	var raw map[string]any
	if err := json.Unmarshal(answer, &raw); err == nil {
		if fields, ok := raw["fields"].(map[string]any); ok && len(fields) > 0 {
			metadata["fields"] = fields
		}
		if selected, ok := raw["selected"]; ok {
			metadata["selected"] = selected
		}
	}
	metadataJSON, _ := json.Marshal(metadata)
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
	// Multi-field form answers: { fields: { key: value, ... } }
	if fields, ok := rawAnswer["fields"].(map[string]any); ok && len(fields) > 0 {
		parts := make([]string, 0, len(fields))
		for key, value := range fields {
			parts = append(parts, fmt.Sprintf("%s: %v", key, value))
		}
		// Stable order is not critical for LLM context readability.
		sort.Strings(parts)
		return strings.Join(parts, "；")
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

func runtimeKnowledgeObservationIdentity(runID, messageID uuid.UUID, seed string) (string, string) {
	digest := sha256.Sum256([]byte(runID.String() + "|" + messageID.String() + "|" + seed))
	suffix := fmt.Sprintf("%x", digest[:12])
	return "runtime-answer:" + runID.String() + ":" + suffix, "runtime-" + suffix
}

func (r *Runtime) recordKnowledgeRuntimeObservations(
	ctx context.Context,
	runID, messageID uuid.UUID,
	parts []map[string]any,
) {
	if r.knowledgeObservationService == nil {
		return
	}

	publishedCitations := map[string]map[string]any{}
	for _, part := range parts {
		if part["type"] != "source" {
			continue
		}
		providerMetadata, ok := part["providerMetadata"].(map[string]any)
		if !ok {
			continue
		}
		citation, ok := providerMetadata["bodysense"].(map[string]any)
		if !ok || fmt.Sprint(citation["source_type"]) != "thought_forest_note" ||
			fmt.Sprint(citation["lifecycle_status"]) != "published" {
			continue
		}
		publicationID := fmt.Sprint(citation["publication_id"])
		unitKey := fmt.Sprint(citation["unit_key"])
		version, ok := intFromAny(citation["published_version"])
		if !ok || publicationID == "" || unitKey == "" {
			continue
		}
		evidenceRef := fmt.Sprintf("published:%s:v%d:%s", publicationID, version, unitKey)
		publishedCitations[evidenceRef] = citation
	}

	attributedRefs := map[string]struct{}{}
	for _, part := range parts {
		if part["type"] != "data" || part["name"] != "answer_attribution" {
			continue
		}
		data, ok := part["data"].(map[string]any)
		if !ok {
			continue
		}
		raw, err := json.Marshal(data)
		if err != nil {
			continue
		}
		parsed, err := service.ParseConsultationAnswerAttributionPayload(raw)
		if err != nil {
			log.Printf("skip invalid completed answer attribution for run %s: %v", runID, err)
			continue
		}
		attribution := parsed.Attribution
		for _, binding := range attribution.Bindings {
			attributedRefs[binding.EvidenceRef] = struct{}{}
			citationStatus := "invalid"
			reasonOverride := "attribution_without_persisted_citation"
			if citation, ok := publishedCitations[binding.EvidenceRef]; ok {
				if publishedCitationMatchesBinding(citation, binding) {
					citationStatus = "valid"
					reasonOverride = ""
				} else {
					reasonOverride = "citation_attribution_identity_mismatch"
				}
			}
			r.recordKnowledgeRuntimeObservation(
				ctx, runID, messageID, attribution, binding, citationStatus, reasonOverride,
			)
		}
	}

	for evidenceRef, citation := range publishedCitations {
		if _, attributed := attributedRefs[evidenceRef]; attributed {
			continue
		}
		publicationID := fmt.Sprint(citation["publication_id"])
		unitKey := fmt.Sprint(citation["unit_key"])
		version, ok := intFromAny(citation["published_version"])
		if !ok {
			continue
		}
		binding := service.PublishedAnswerEvidenceBinding{
			EvidenceRef:         evidenceRef,
			PublicationID:       publicationID,
			PublicationKey:      fmt.Sprint(citation["publication_key"]),
			PublicationBatchKey: fmt.Sprint(citation["publication_batch_key"]),
			PublishedVersion:    version,
			UnitKey:             unitKey,
			ClaimID:             fmt.Sprint(citation["claim_id"]),
			ClaimReviewID:       fmt.Sprint(citation["claim_review_id"]),
			GroundingStatus:     "degraded",
			ReasonCodes:         []string{"missing_answer_attribution"},
		}
		attribution := service.ConsultationAnswerAttribution{
			AttributionID:   "missing:" + evidenceRef,
			PolicyRevision:  service.ConsultationAnswerAttributionPolicyV1,
			EvidenceRefs:    []string{evidenceRef},
			GroundingStatus: "degraded",
			ReasonCodes:     []string{"missing_answer_attribution"},
			Bindings:        []service.PublishedAnswerEvidenceBinding{binding},
		}
		r.recordKnowledgeRuntimeObservation(
			ctx, runID, messageID, attribution, binding, "valid", "missing_answer_attribution",
		)
	}
}

func publishedCitationMatchesBinding(
	citation map[string]any,
	binding service.PublishedAnswerEvidenceBinding,
) bool {
	if fmt.Sprint(citation["publication_id"]) != binding.PublicationID ||
		fmt.Sprint(citation["publication_key"]) != binding.PublicationKey ||
		fmt.Sprint(citation["publication_batch_key"]) != binding.PublicationBatchKey ||
		fmt.Sprint(citation["unit_key"]) != binding.UnitKey ||
		fmt.Sprint(citation["claim_id"]) != binding.ClaimID ||
		fmt.Sprint(citation["claim_review_id"]) != binding.ClaimReviewID {
		return false
	}
	version, ok := intFromAny(citation["published_version"])
	if !ok || version != binding.PublishedVersion {
		return false
	}
	locator, ok := citation["source_locator"].(map[string]any)
	if !ok {
		return false
	}
	return fmt.Sprint(locator["locator_type"]) == binding.SourceLocator.LocatorType &&
		fmt.Sprint(locator["repository"]) == binding.SourceLocator.Repository &&
		fmt.Sprint(locator["git_commit"]) == binding.SourceLocator.GitCommit &&
		fmt.Sprint(locator["path"]) == binding.SourceLocator.Path &&
		intValueEquals(locator["line_start"], binding.SourceLocator.LineStart) &&
		intValueEquals(locator["line_end"], binding.SourceLocator.LineEnd)
}

func intValueEquals(value any, expected int) bool {
	actual, ok := intFromAny(value)
	return ok && actual == expected
}

func intFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if typed == float64(int(typed)) {
			return int(typed), true
		}
	}
	return 0, false
}

func (r *Runtime) recordKnowledgeRuntimeObservation(
	ctx context.Context,
	runID, messageID uuid.UUID,
	attribution service.ConsultationAnswerAttribution,
	binding service.PublishedAnswerEvidenceBinding,
	citationStatus string,
	reasonOverride string,
) {
	publicationID, err := uuid.Parse(binding.PublicationID)
	if err != nil {
		log.Printf("skip runtime knowledge observation with invalid publication id: %v", err)
		return
	}
	seed := attribution.AttributionID + "|" + binding.EvidenceRef
	observationKey, caseID := runtimeKnowledgeObservationIdentity(runID, messageID, seed)
	reasons := binding.ReasonCodes
	if reasonOverride != "" {
		reasons = []string{reasonOverride}
	}
	observationSource := "source.answer_attribution.added"
	if reasonOverride == "missing_answer_attribution" {
		observationSource = "source.citation.added"
	}
	metadata, _ := json.Marshal(map[string]any{
		"run_id":             runID.String(),
		"message_id":         messageID.String(),
		"attribution_id":     attribution.AttributionID,
		"claim_text":         attribution.ClaimText,
		"evidence_ref":       binding.EvidenceRef,
		"unit_key":           binding.UnitKey,
		"claim_id":           binding.ClaimID,
		"claim_review_id":    binding.ClaimReviewID,
		"reason_codes":       reasons,
		"observation_source": observationSource,
	})
	if err := r.knowledgeObservationService.Record(ctx, service.RecordKnowledgePublicationObservationInput{
		PublicationKey:           binding.PublicationKey,
		ExpectedPublicationID:    publicationID,
		ExpectedPublishedVersion: binding.PublishedVersion,
		ObservationKey:           observationKey,
		ObservationKind:          "runtime_answer",
		EvaluatorRevision:        service.ConsultationAnswerAttributionPolicyV1,
		CaseID:                   caseID,
		RetrievalStatus:          "hit",
		CitationStatus:           citationStatus,
		GroundingStatus:          binding.GroundingStatus,
		IdentityStatus:           "match",
		ProvenanceStatus:         "valid",
		Metadata:                 datatypes.JSON(metadata),
	}); err != nil {
		log.Printf("record runtime knowledge observation for run %s: %v", runID, err)
	}
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
