package consultation

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/episodicrecall"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/stream"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

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
		if errors.Is(err, model.ErrInteractionResumeConflict) {
			return nil, uuid.Nil, nil, nil, nil, dto.StreamEventIDs{}, false, httpErr(http.StatusConflict, "REQUEST_ID_CONFLICT", "request ID already belongs to an accepted answer")
		}
		if errors.Is(err, model.ErrConversationRunInProgress) {
			return nil, uuid.Nil, nil, nil, nil, dto.StreamEventIDs{}, false, httpErr(
				http.StatusConflict,
				"RUN_IN_PROGRESS",
				"this conversation already has an active run",
			)
		}
		return nil, uuid.Nil, nil, nil, nil, dto.StreamEventIDs{}, false, httpErr(
			http.StatusInternalServerError,
			ConsultationErrorInternal,
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

	// Only the allowlisted identity context reaches the Agent; display name,
	// avatar and preferences stay presentation data.
	data, err := json.Marshal(profile.AgentContext())
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
	spatialContext *service.ConsultationSpatialContext,
) service.ConsultationBusinessContext {
	bc := service.ConsultationBusinessContext{
		Profile:        profileJSON,
		SpatialContext: spatialContext,
		RuntimeState: service.ConsultationRuntimeState{
			Phase:         string(session.Phase),
			ExtractedInfo: json.RawMessage(session.ExtractedInfo),
		},
	}
	// The native V2 agent view (facts, observations, episodes, user
	// hypotheses, Safety digest, usable equipment) pinned to one revision. A
	// read failure is explicit (records_available=false) so the turn cannot
	// invent revision zero, claim capture, or present symptoms as screened.
	var bodyContext *service.AgentBodyContext
	if r.bodyStateService != nil {
		if view, err := r.bodyStateService.GetAgentBodyContext(ctx, uid); err != nil {
			slog.Default().Warn("agent_body_context_load_failed", "user_id", uid, "error", err)
		} else {
			bodyContext = view
		}
		bc.BodyState = consultationBodyStatePayload(bodyContext, r.bodyFacts != nil)
	}
	bc.ContextWindow = r.consultationContextWindow(ctx, uid, conversationID)
	if r.contextRetrievalService != nil {
		if history, err := r.contextRetrievalService.Retrieve(
			ctx, uid, conversationID, queryText, bodyContext,
		); err != nil {
			slog.Default().Warn("historical_context_retrieval_failed", "conversation_id", conversationID, "error", err)
		} else {
			bc.RelevantHistory = history
		}
	}
	// S4: real-time episodic recall, non-clinical context only. Recall already
	// bounds itself by the 100ms budget and degrades to an empty slice, so this
	// block can never fail, block or delay the clinical paths below.
	if r.episodicRecall != nil {
		r.attachRecalledEpisodicNotes(ctx, uid, conversationID, queryText, &bc)
	} else {
		r.logEpisodicRecall(uid, conversationID, episodicrecall.RecallResult{Outcome: episodicrecall.OutcomeDisabled})
	}
	if r.diagnosisReader != nil {
		if diagnosis, err := r.diagnosisReader.Read(ctx, uid); err != nil {
			slog.Default().Warn("current_diagnosis_context_load_failed", "user_id", uid, "error", err)
		} else if diagnosis != nil && diagnosis.CurrentRevision > 0 {
			if raw, marshalErr := json.Marshal(diagnosis); marshalErr == nil {
				bc.CurrentDiagnosis = raw
			}
		}
	}
	if r.outcomeService != nil {
		if outcomes, err := r.outcomeService.ListOutcomes(ctx, uid, 12); err == nil {
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
		slog.Default().Warn("posture_analysis_prefetch_failed", "user_id", uid, "error", err)
		return bc
	}
	if raw, err := json.Marshal(summary); err == nil {
		bc.PostureAnalysis = raw
	}
	return bc
}

// attachRecalledEpisodicNotes adds the served notes to the non-clinical
// business context and always emits one content-free structured record. The
// record makes the serving boundary observable: a "served" outcome with an
// empty runtime payload means a later projection dropped the notes.
func (r *Runtime) attachRecalledEpisodicNotes(
	ctx context.Context,
	uid uuid.UUID,
	conversationID uuid.UUID,
	queryText string,
	bc *service.ConsultationBusinessContext,
) {
	result := r.episodicRecall.RecallTurn(ctx, uid, queryText)
	r.logEpisodicRecall(uid, conversationID, result)
	if len(result.Notes) == 0 {
		return
	}
	bc.EpisodicNotes = make([]service.ConsultationEpisodicNote, 0, len(result.Notes))
	for _, note := range result.Notes {
		sourceIDs := make([]string, len(note.SourceMessageIDs))
		for i, id := range note.SourceMessageIDs {
			sourceIDs[i] = id.String()
		}
		bc.EpisodicNotes = append(bc.EpisodicNotes, service.ConsultationEpisodicNote{
			NoteID:           note.ID.String(),
			FromSeq:          note.FromSeq,
			ToSeq:            note.ToSeq,
			SourceMessageIDs: sourceIDs,
			CreatedAt:        note.CreatedAt,
			Text:             note.Text,
			Authority:        service.ConsultationEpisodicNoteAuthority,
		})
	}
}

// logEpisodicRecall emits the S4 serving outcome. It carries ids and counts
// only: never note text and never the user's message.
func (r *Runtime) logEpisodicRecall(uid, conversationID uuid.UUID, result episodicrecall.RecallResult) {
	level := slog.LevelInfo
	switch result.Outcome {
	case episodicrecall.OutcomeTimeout, episodicrecall.OutcomeStoreError, episodicrecall.OutcomeInvalidProvenance:
		level = slog.LevelWarn
	}
	noteIDs := make([]string, len(result.Notes))
	for i, note := range result.Notes {
		noteIDs[i] = note.ID.String()
	}
	attrs := []any{
		"outcome", string(result.Outcome),
		"served", len(result.Notes),
		"candidates", result.Candidates,
		"elapsed_ms", result.Elapsed.Milliseconds(),
		"user_id", uid.String(),
		"conversation_id", conversationID.String(),
	}
	if len(noteIDs) > 0 {
		attrs = append(attrs, "note_ids", noteIDs)
	}
	if result.Err != nil {
		attrs = append(attrs, "error", result.Err.Error())
	}
	slog.Default().Log(context.Background(), level, "episodic_recall", attrs...)
}

// consultationContextWindow reads Go's retention context cursor for the turn.
// A read failure sends no window, and the runtime falls back to its own
// bounded history rather than to the whole thread.
func (r *Runtime) consultationContextWindow(ctx context.Context, uid, conversationID uuid.UUID) *service.ConsultationContextWindow {
	if r.chatRetention == nil {
		return nil
	}
	window, err := r.chatRetention.ContextWindow(ctx, uid, conversationID)
	if err != nil {
		slog.Default().Warn("chat_context_window_read_failed", "conversation_id", conversationID, "error", err)
		return nil
	}
	out := &service.ConsultationContextWindow{ContextStartSeq: window.ContextStartSeq, RunIDs: make([]string, len(window.RunIDs))}
	for i, id := range window.RunIDs {
		out.RunIDs[i] = id.String()
	}
	return out
}

// consultationBodyStatePayload encodes the agent view for the turn. Capture is
// available only when the view loaded and the run can commit BodyFacts.
func consultationBodyStatePayload(view *service.AgentBodyContext, captureAvailable bool) json.RawMessage {
	if view == nil {
		raw, _ := json.Marshal(map[string]any{"schema": service.AgentBodyContextSchemaV1, "records_available": false})
		return raw
	}
	payload := *view
	payload.RecordsAvailable = view.RecordsAvailable && captureAvailable
	raw, err := json.Marshal(payload)
	if err != nil {
		raw, _ = json.Marshal(map[string]any{"schema": service.AgentBodyContextSchemaV1, "records_available": false})
	}
	return raw
}

// AttachLongitudinalContextServices adds optional read-only business context
// collaborators without expanding the stable runtime constructor used by tests.
func (r *Runtime) AttachLongitudinalContextServices(
	contextRetrieval *service.ContextRetrievalService,
	diagnosis runtimeDiagnosisReader,
	outcomes *service.OutcomeService,
) {
	r.contextRetrievalService = contextRetrieval
	r.diagnosisReader = diagnosis
	r.outcomeService = outcomes
}

// AttachEpisodicRecall wires the default-off S4 real-time recall path. A nil
// recaller is the off state: buildBusinessContext performs no query and emits
// no episodic_notes field, so the payload is byte-identical to today.
func (r *Runtime) AttachEpisodicRecall(recaller *episodicrecall.Recaller) {
	r.episodicRecall = recaller
}

// runtimeDiagnosisReader is the native Diagnosis projection; the consultation
// context carries only Go-committed Conditions, never an Agent proposal.
type runtimeDiagnosisReader interface {
	Read(ctx context.Context, owner uuid.UUID) (*service.NativeDiagnosisView, error)
}

// runtimeChatRetention is the PC-RET-001 seam: the settle trigger that moves
// the processed and context cursors, and the context window read.
type runtimeChatRetention interface {
	OnRunSettled(ctx context.Context, owner uuid.UUID)
	ContextWindow(ctx context.Context, owner, conversationID uuid.UUID) (*model.ChatContextWindow, error)
}

// AttachChatRetention wires the chat retention settle trigger.
func (r *Runtime) AttachChatRetention(retention runtimeChatRetention) {
	r.chatRetention = retention
}
