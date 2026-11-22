package consultation

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/stream"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func (r *Runtime) handleExecutionContextDone(ctx context.Context, sw *stream.StreamWriter, state streamState) {
	terminalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	run, err := r.runService.GetRunForUser(terminalCtx, state.Run.ID, state.UID)
	if err != nil {
		slog.Default().Error("load_terminal_run_failed", "run_id", state.Run.ID, "error", err)
		return
	}
	if run != nil && run.Status == model.RunStatusCancelled {
		if state.AssistantMsg != nil {
			parts, _ := json.Marshal([]map[string]any{})
			_ = r.messageService.UpdateMessageCompletedWithStatus(
				terminalCtx, state.AssistantMsg.ID, state.ConversationID,
				datatypes.JSON(parts), "aborted",
			)
		}
		r.clearActiveRun(terminalCtx, state.ConversationID, state.UID)
		r.refreshThreadProjection(terminalCtx, state.ConversationID, state.UID)
		return
	}

	// Deadline/host-side cancellation is operational failure, distinct from an
	// explicit user cancellation command.
	r.failActiveStream(terminalCtx, sw, state, "consultation execution deadline exceeded")
}

func (r *Runtime) reattachActiveRun(ctx context.Context, w http.ResponseWriter, run *model.Run) {
	baseIDs := dto.StreamEventIDs{
		ConversationID: run.ConversationID.String(),
		RunID:          run.ID.String(),
		TurnID:         run.TurnID.String(),
	}
	sw := r.streamRuntime.NewWriter(w, baseIDs)

	if r.runtimeEventService == nil {
		r.sendNewEvent(ctx, sw, "stream", "stream.error", baseIDs, map[string]any{"message": "runtime event log unavailable", "status": run.Status}, "", "stream.error")
		r.sendNewEvent(ctx, sw, "stream", "stream.done", baseIDs, map[string]any{}, "", "stream.done")
		return
	}

	const batchSize = 200
	const pollInterval = 250 * time.Millisecond
	afterSeq := 0
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		events, hasMore, err := r.runtimeEventService.ListRunEvents(ctx, run.ConversationID, run.ID, afterSeq, batchSize)
		if err != nil {
			slog.Default().Error("consultation_transport_reattach_list_failed", "run_id", run.ID, "after_seq", afterSeq, "error", err)
			return
		}

		for _, stored := range events {
			event, err := storedRuntimeEventToStreamEvent(stored)
			if err != nil {
				slog.Default().Warn("consultation_transport_reattach_decode_failed", "run_id", run.ID, "seq", stored.Seq, "error", err)
				continue
			}
			if err := sw.WriteEvent(ctx, event); err != nil {
				slog.Default().Warn("consultation_transport_reattach_disconnected", "run_id", run.ID, "seq", stored.Seq, "error", err)
				return
			}
			if stored.Seq > afterSeq {
				afterSeq = stored.Seq
			}
			if stored.Type == "stream.done" {
				slog.Default().Info("consultation_transport_reattach_completed", "run_id", run.ID, "last_seq", afterSeq)
				return
			}
		}

		if hasMore {
			continue
		}

		select {
		case <-ctx.Done():
			slog.Default().Info("consultation_transport_reattach_request_closed", "run_id", run.ID, "last_seq", afterSeq, "error", ctx.Err())
			return
		case <-ticker.C:
		}
	}
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
			slog.Default().Error("replay_runtime_events_failed", "run_id", run.ID, "error", err)
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
		event, err := storedRuntimeEventToStreamEvent(stored)
		if err != nil {
			slog.Default().Error("decode_runtime_event_ids_failed", "run_id", run.ID, "seq", stored.Seq, "error", err)
			continue
		}
		if stored.Seq > maxSeq {
			maxSeq = stored.Seq
		}
		if stored.Type == "stream.done" {
			sawStreamDone = true
		}

		if err := sw.WriteEvent(ctx, event); err != nil {
			slog.Default().Error("write_replayed_event_failed", "run_id", run.ID, "seq", stored.Seq, "error", err)
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
		slog.Default().Error("write_replay_done_failed", "run_id", run.ID, "error", err)
	}
}

func (r *Runtime) clearActiveRun(ctx context.Context, conversationID, userID uuid.UUID) {
	if r.conversationService == nil {
		return
	}
	if err := r.conversationService.UpdateActiveRunID(ctx, conversationID, userID, nil, ""); err != nil {
		slog.Default().Error("clear_active_run_id_failed", "conversation_id", conversationID, "error", err)
	}
	// Every path that ends a Run (completed, failed, cancelled, parked on Ask
	// User) clears it here after its messages reached their final status, so
	// this is the retention settle trigger (PC-RET-001).
	if r.chatRetention != nil {
		r.chatRetention.OnRunSettled(ctx, userID)
	}
}

func (r *Runtime) refreshThreadProjection(ctx context.Context, conversationID, userID uuid.UUID) {
	if r.threadProjectionService == nil {
		return
	}

	if _, _, _, _, _, err := r.threadProjectionService.RefreshAndGetThread(ctx, conversationID, userID); err != nil {
		slog.Default().Error("refresh_thread_projection_failed", "conversation_id", conversationID, "error", err)
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
	r.failRunAndEmit(ctx, sw, streamState{
		UID:            uid,
		ConversationID: conversationID,
		TurnID:         run.TurnID,
		Run:            run,
		AssistantMsg:   assistantMsg,
		AssistantMsgID: assistantMsg.ID.String(),
		BaseIDs: dto.StreamEventIDs{
			ConversationID: conversationID.String(),
			RunID:          run.ID.String(),
			TurnID:         run.TurnID.String(),
		},
	}, message)
}

func (r *Runtime) failActiveStream(
	ctx context.Context,
	sw *stream.StreamWriter,
	state streamState,
	message string,
) {
	r.failRunAndEmit(ctx, sw, state, message)
}

// failRunAndEmit makes Run.status=failed and run.failed one atomic durable
// decision. Only the transaction winner may emit the matching SSE/message
// terminal projection; a concurrent cancel/completion winner is never
// overwritten by a late failure path.
func (r *Runtime) failRunAndEmit(
	ctx context.Context,
	sw *stream.StreamWriter,
	state streamState,
	message string,
) {
	if strings.TrimSpace(message) == "" {
		message = "stream failed"
	}
	if r.runService == nil || state.Run == nil || state.AssistantMsg == nil {
		slog.Default().Error("fail_consultation_run_missing_lifecycle_dependencies")
		return
	}

	runFailedEvent, err := sw.NewEvent(
		"run",
		"run.failed",
		dto.StreamEventIDs{
			ConversationID: state.ConversationID.String(),
			RunID:          state.Run.ID.String(),
			TurnID:         state.TurnID.String(),
		},
		map[string]any{"status": "failed", "error": map[string]any{"message": message}},
		"",
	)
	if err != nil {
		slog.Default().Error("build_run_failed_event_failed", "run_id", state.Run.ID, "error", err)
		return
	}

	committed, err := r.runService.FailRunWithEvent(
		ctx,
		state.Run,
		state.UID,
		map[string]any{"message": message},
		runFailedEvent,
	)
	if err != nil {
		slog.Default().Error("atomically_fail_run_failed", "run_id", state.Run.ID, "error", err)
		return
	}
	if !committed {
		// Another terminal transition won. Never manufacture a contradictory
		// run.failed/message.failed sequence after that durable decision.
		r.clearActiveRun(ctx, state.ConversationID, state.UID)
		r.refreshThreadProjection(ctx, state.ConversationID, state.UID)
		return
	}

	if err := sw.WriteEvent(ctx, runFailedEvent); err != nil {
		slog.Default().Warn("sse_write_error", "event_type", "run.failed", "run_id", state.Run.ID, "conversation_id", state.ConversationID, "seq", runFailedEvent.Seq, "error", err)
	}
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
	if r.messageService != nil {
		_ = r.messageService.UpdateMessageStatus(ctx, state.AssistantMsg.ID, state.ConversationID, "failed")
	}
	r.clearActiveRun(ctx, state.ConversationID, state.UID)
	r.refreshThreadProjection(ctx, state.ConversationID, state.UID)
}
