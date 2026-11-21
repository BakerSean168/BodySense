package consultation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func (r *Runtime) ResumeInteraction(
	ctx context.Context,
	w http.ResponseWriter,
	uid uuid.UUID,
	conversationID uuid.UUID,
	interactionID uuid.UUID,
	req ResumeInteractionInput,
) *HTTPError {
	session, err := r.consultationService.GetConsultation(ctx, conversationID, uid)
	if errors.Is(err, service.ErrConsultationNotFound) {
		return httpErr(http.StatusNotFound, ConsultationErrorNotFound, "consultation not found")
	}
	if err != nil {
		return httpErr(http.StatusInternalServerError, ConsultationErrorInternal, "failed to load consultation")
	}
	if session == nil {
		return httpErr(http.StatusNotFound, ConsultationErrorNotFound, "consultation not found")
	}

	interaction, err := r.interactionService.GetInteractionByID(ctx, interactionID)
	if err != nil {
		return httpErr(http.StatusInternalServerError, ConsultationErrorInternal, "failed to load interaction")
	}
	if interaction == nil || interaction.ConversationID != conversationID {
		return httpErr(http.StatusNotFound, ConsultationErrorNotFound, "interaction not found")
	}

	// Resume is continuation of the exact logical Agent thread. Pin it to the
	// immutable configuration recorded on the interrupted source run instead of
	// silently switching to the current Champion while the user was waiting.
	sourceRun, err := r.runService.GetRunForUser(ctx, interaction.RunID, uid)
	if err != nil {
		return httpErr(http.StatusInternalServerError, ConsultationErrorInternal, "failed to load interrupted run")
	}
	if sourceRun == nil || sourceRun.ConversationID != conversationID {
		return httpErr(http.StatusNotFound, ConsultationErrorNotFound, "interrupted run not found")
	}
	pinnedConfigurationID := strings.TrimSpace(sourceRun.AgentConfigurationID)
	if pinnedConfigurationID == "" {
		return httpErr(
			http.StatusConflict,
			ConsultationErrorInteractionNotResumable,
			"interrupted run predates durable Agent configuration identity; start a new run",
		)
	}
	if _, err := service.ConsultationDecisionPolicyRevisionForConfiguration(pinnedConfigurationID); err != nil {
		return httpErr(http.StatusConflict, ConsultationErrorInteractionNotResumable, "interrupted run configuration is no longer repository-authorized")
	}

	policy, _ := service.ConsultationDecisionPolicyRevisionForConfiguration(pinnedConfigurationID)
	if policy == service.ConsultationDecisionPolicyV3 {
		return r.resumeBodyFactInteraction(ctx, w, uid, session, interaction, req)
	}

	existing, found, err := r.runService.CheckIdempotency(ctx, uid, req.RequestID)
	if err != nil {
		return httpErr(
			http.StatusInternalServerError,
			ConsultationErrorInternal,
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

	// Legacy (pre-v3) runs have no source-bound resume envelope, so their
	// answers stay chat messages only and never become BodyFacts. Without a
	// recorded intake the Safety coverage stays incomplete (fail-closed).

	if err := r.interactionService.ResumeInteractionWithEvent(ctx, interactionID, datatypes.JSON(req.Answer)); err != nil {
		switch {
		case err == service.ErrInteractionNotFound:
			return httpErr(http.StatusNotFound, ConsultationErrorNotFound, "interaction not found")
		case err == service.ErrInteractionConflict:
			return httpErr(
				http.StatusConflict,
				"INTERACTION_CONFLICT",
				"interaction was already answered differently",
			)
		case err == service.ErrInteractionClosed:
			return httpErr(http.StatusConflict, ConsultationErrorInteractionClosed, err.Error())
		case err == service.ErrInteractionExpired:
			return httpErr(http.StatusConflict, ConsultationErrorInteractionExpired, "interaction has expired; start a new question")
		default:
			return httpErr(http.StatusInternalServerError, ConsultationErrorInternal, err.Error())
		}
	}

	closedSourceRun, err := r.runService.CompleteRunOutOfBandWithEvent(ctx, sourceRun, uid)
	if err != nil {
		return httpErr(http.StatusInternalServerError, ConsultationErrorInternal, "failed to close interrupted run")
	}
	if !closedSourceRun {
		return httpErr(http.StatusConflict, ConsultationErrorRunTerminal, "interrupted run was cancelled or already closed")
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

	executionCtx, cancelExecution := r.durableExecutionContext(ctx)
	defer cancelExecution()
	unregisterCancellation := r.registerRunCancellation(run.ID, cancelExecution)
	defer unregisterCancellation()
	stopLeaseHeartbeat := r.runService.StartLeaseHeartbeat(executionCtx, run.ID, uid)
	defer stopLeaseHeartbeat()

	profileJSON, profileErr := r.loadProfileJSON(executionCtx, uid)
	if profileErr != nil {
		return httpErr(http.StatusInternalServerError, ConsultationErrorInternal, "failed to load profile")
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
			BusinessContext: r.buildBusinessContext(executionCtx, uid, conversationID, extractAnswerText(req.Answer), profileJSON, session, nil),
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
		CurrentPhase:            string(session.Phase),
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
	return nil
}
