package consultation

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func (r *Runtime) AttachInteractionResumeService(resumes *service.InteractionResumeService) {
	r.interactionResumes = resumes
}

// AttachBodyStateSuggestionSources lets the runtime bind source-cited
// BodyState suggestions; without it every suggestion is stripped.
func (r *Runtime) AttachBodyStateSuggestionSources(reader service.BodyStateSuggestionSourceReader) {
	r.suggestionSources = reader
}

func (r *Runtime) resumeBodyFactInteraction(ctx context.Context, w http.ResponseWriter, uid uuid.UUID, session *model.ConsultationSession, interaction *model.AgentInteraction, req ResumeInteractionInput) *HTTPError {
	if r.interactionResumes == nil {
		return httpErr(http.StatusServiceUnavailable, ConsultationErrorInteractionNotResumable, "interaction resume is unavailable")
	}
	claimed, err := r.interactionResumes.Claim(ctx, model.ClaimInteractionResume{UserID: uid, ConversationID: session.ConversationID, InteractionID: interaction.ID, RequestID: req.RequestID, Answer: datatypes.JSON(req.Answer)})
	if err != nil {
		return interactionResumeHTTPError(err)
	}
	envelope := claimed.Envelope
	run, assistant := envelope.Run, envelope.AssistantMessage
	if envelope.Existed {
		if run.Status == model.RunStatusRunning && assistant.Status == "streaming" {
			return httpErr(http.StatusConflict, "RUN_IN_PROGRESS", "this answer is already executing")
		}
		return r.replayInteractionResume(ctx, w, envelope)
	}
	nativeID, err := service.NativeInteractionInterrupt(envelope.Interaction)
	if err != nil {
		return interactionResumeHTTPError(err)
	}
	lastSeq := 0
	for _, event := range claimed.Events {
		if event.Seq > lastSeq {
			lastSeq = event.Seq
		}
	}
	base := dto.StreamEventIDs{ConversationID: run.ConversationID.String(), RunID: run.ID.String(), TurnID: run.TurnID.String()}
	// Install cancellation before exposing resume milestones or dispatching AI.
	// A concurrent cancellation that won before registration is visible here;
	// replay its durable result rather than restarting the stopped Run.
	executionCtx, cancel := durableExecutionContext(ctx)
	defer cancel()
	unregister := r.registerRunCancellation(run.ID, cancel)
	defer unregister()
	stopHeartbeat := r.runService.StartLeaseHeartbeat(executionCtx, run.ID, uid)
	defer stopHeartbeat()
	current, readErr := r.runService.GetRunForUser(executionCtx, run.ID, uid)
	if readErr != nil || current == nil {
		return httpErr(http.StatusServiceUnavailable, ConsultationErrorInteractionNotResumable, "answer is retained, but the Run cannot currently be verified")
	}
	if current.Status != model.RunStatusRunning {
		return r.replayInteractionResume(ctx, w, envelope)
	}
	sw := r.streamRuntime.NewWriterAfter(w, base, lastSeq)
	for _, event := range claimed.Events {
		if err := sw.WriteEvent(ctx, event); err != nil {
			log.Printf("resume event write failed run=%s seq=%d", run.ID, event.Seq)
		}
	}
	state := streamState{UID: uid, ConversationID: run.ConversationID, TurnID: run.TurnID, Run: run, AssistantMsg: assistant, BaseIDs: base, CurrentPhase: string(session.Phase), AssistantMsgID: assistant.ID.String(), ConversationStr: run.ConversationID.String(), ExpectedConfigurationID: run.AgentConfigurationID,
		AnswerSource: &model.AnswerFactSource{RunID: run.ID, InteractionID: envelope.Interaction.ID, MessageID: envelope.UserMessage.ID}}
	r.refreshThreadProjection(executionCtx, run.ConversationID, uid)
	profile, err := r.loadProfileJSON(executionCtx, uid)
	if err != nil {
		r.failActiveStream(executionCtx, sw, state, "个人资料暂时无法读取，未继续处理回答。")
		return nil
	}
	_, answerText, err := model.ValidateInteractionAnswer(envelope.Interaction.Question, envelope.Interaction.Answer)
	if err != nil {
		r.failActiveStream(executionCtx, sw, state, "已保存的回答未通过核验，未继续处理。")
		return nil
	}
	events, err := r.aiClient.ResumeConsultationInterrupt(executionCtx, run.ConversationID.String(), nativeID, service.ResumeConsultationInterruptRequest{
		RunID: run.ID.String(), ConversationID: run.ConversationID.String(), UserID: uid.String(), ConfigurationID: run.AgentConfigurationID, InterruptID: nativeID, Answer: json.RawMessage(envelope.Interaction.Answer),
		BusinessContext: r.buildBusinessContext(executionCtx, uid, run.ConversationID, answerText, profile, session, nil),
	})
	if err != nil {
		r.failActiveStream(executionCtx, sw, state, "AI 服务暂时不可用；回答已保留，身体记录尚未确认保存。")
		return nil
	}
	result, stopped := r.streamAIEvents(executionCtx, sw, events, state)
	if stopped {
		return nil
	}
	if executionCtx.Err() != nil || !r.finishTurn(executionCtx, sw, uid, run.ConversationID, run, assistant, run.TurnID, result, base) {
		r.handleExecutionContextDone(executionCtx, sw, state)
		return nil
	}
	r.sendNewEvent(executionCtx, sw, "stream", "stream.done", base, map[string]any{}, "", "stream.done")
	return nil
}

// Replay only the accepted answer's segment. An earlier interruption's
// stream.done must not truncate this resumed response in the browser.
func (r *Runtime) replayInteractionResume(ctx context.Context, w http.ResponseWriter, envelope *model.InteractionResumeEnvelope) *HTTPError {
	events, err := r.runtimeEventService.ListAllRunEvents(ctx, envelope.Run.ConversationID, envelope.Run.ID)
	if err != nil {
		return httpErr(http.StatusInternalServerError, ConsultationErrorInternal, "failed to replay accepted answer")
	}
	base := dto.StreamEventIDs{ConversationID: envelope.Run.ConversationID.String(), RunID: envelope.Run.ID.String(), TurnID: envelope.Run.TurnID.String()}
	var replay []dto.StreamEvent
	started := false
	for _, event := range events {
		var ids dto.StreamEventIDs
		if json.Unmarshal(event.IDs, &ids) != nil {
			return httpErr(http.StatusInternalServerError, ConsultationErrorInternal, "invalid saved answer event")
		}
		if event.Type == "state.interaction.answered" && ids.InteractionID == envelope.Interaction.ID.String() {
			started = true
		}
		if !started {
			continue
		}
		replay = append(replay, dto.StreamEvent{Version: 1, Seq: event.Seq, Channel: event.Channel, Type: event.Type, IDs: ids, Payload: json.RawMessage(event.Payload)})
		if event.Type == "stream.done" {
			break
		}
	}
	if len(replay) == 0 {
		return httpErr(http.StatusConflict, ConsultationErrorInteractionNotResumable, "saved answer has no replayable execution yet")
	}
	sw := r.streamRuntime.NewWriter(w, base)
	for _, event := range replay {
		if err := sw.WriteEvent(ctx, event); err != nil {
			return nil
		}
	}
	return nil
}

func interactionResumeHTTPError(err error) *HTTPError {
	switch {
	case errors.Is(err, model.ErrInvalidInteractionAnswer):
		return httpErr(http.StatusBadRequest, ConsultationErrorInvalidRequest, "answer does not match the pending question")
	case errors.Is(err, model.ErrInteractionResumeNotFound):
		return httpErr(http.StatusNotFound, ConsultationErrorNotFound, "interaction not found")
	case errors.Is(err, model.ErrInteractionResumeExpired):
		return httpErr(http.StatusConflict, ConsultationErrorInteractionExpired, "interaction expired; old authorization is not reused")
	case errors.Is(err, model.ErrInteractionResumeConflict):
		return httpErr(http.StatusConflict, "INTERACTION_CONFLICT", "interaction was answered differently, stopped, or is no longer current")
	case errors.Is(err, model.ErrBodyStateSuggestionStale):
		return httpErr(http.StatusConflict, ConsultationErrorInteractionNotResumable, "body state changed since this suggestion; nothing was recorded")
	case errors.Is(err, model.ErrBodyStateSuggestionRejected):
		return httpErr(http.StatusConflict, ConsultationErrorInteractionNotResumable, "the cited statement is no longer available; nothing was recorded")
	case errors.Is(err, model.ErrInteractionResumeUnavailable):
		return httpErr(http.StatusConflict, ConsultationErrorInteractionNotResumable, "interaction is missing verified resume provenance")
	default:
		return httpErr(http.StatusInternalServerError, ConsultationErrorInternal, "answer was not committed; retry the same request")
	}
}
