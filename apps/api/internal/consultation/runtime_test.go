package consultation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/stream"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeRuntimeEventRepo struct {
	events []model.RuntimeEvent
}

func (r *fakeRuntimeEventRepo) Create(ctx context.Context, event *model.RuntimeEvent) error {
	r.events = append(r.events, *event)
	return nil
}

func (r *fakeRuntimeEventRepo) CreateBatch(ctx context.Context, events []*model.RuntimeEvent) error {
	for _, event := range events {
		if err := r.Create(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (r *fakeRuntimeEventRepo) ListByRunID(ctx context.Context, conversationID, runID uuid.UUID, afterSeq, limit int) ([]model.RuntimeEvent, bool, error) {
	out := make([]model.RuntimeEvent, 0, len(r.events))
	for _, event := range r.events {
		if event.ConversationID == conversationID && event.RunID == runID && event.Seq > afterSeq {
			out = append(out, event)
		}
	}
	if len(out) > limit {
		return out[:limit], true, nil
	}
	return out, false, nil
}

func (r *fakeRuntimeEventRepo) ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.RuntimeEvent, error) {
	out := make([]model.RuntimeEvent, 0, len(r.events))
	for _, event := range r.events {
		if event.ConversationID == conversationID {
			out = append(out, event)
		}
	}
	return out, nil
}

type failingResponseWriter struct {
	header http.Header
}

func (w *failingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingResponseWriter) WriteHeader(int) {}
func (w *failingResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("socket closed")
}
func (w *failingResponseWriter) Flush() {}

type fakeRuntimeBodyState struct {
	extractedErr   error
	safetyErr      error
	interactionErr error
}

func (f *fakeRuntimeBodyState) GetSnapshot(context.Context, uuid.UUID, int) (*service.BodyStateSnapshot, error) {
	return &service.BodyStateSnapshot{}, nil
}

func (f *fakeRuntimeBodyState) UpsertExtractedSymptom(context.Context, uuid.UUID, uuid.UUID, json.RawMessage) error {
	return f.extractedErr
}

func (f *fakeRuntimeBodyState) RecordSafetyEvent(context.Context, uuid.UUID, json.RawMessage) error {
	return f.safetyErr
}

func (f *fakeRuntimeBodyState) RecordInteractionAnswer(context.Context, uuid.UUID, uuid.UUID, datatypes.JSON, json.RawMessage) error {
	return f.interactionErr
}

func testStreamState() streamState {
	conversationID := uuid.New()
	turnID := uuid.New()
	runID := uuid.New()
	messageID := uuid.New()
	return streamState{
		UID: uuid.New(), ConversationID: conversationID, TurnID: turnID,
		Run:          &model.Run{ID: runID, ConversationID: conversationID, TurnID: turnID},
		AssistantMsg: &model.Message{ID: messageID, ConversationID: conversationID, TurnID: turnID},
		BaseIDs: dto.StreamEventIDs{
			ConversationID: conversationID.String(), RunID: runID.String(), TurnID: turnID.String(),
		},
		AssistantMsgID: messageID.String(),
	}
}

func TestHandleAIEventFailsClosedWhenExtractedBodyStateWriteFails(t *testing.T) {
	runtime := &Runtime{
		bodyStateService: &fakeRuntimeBodyState{extractedErr: errors.New("db unavailable")},
		streamRuntime:    stream.NewRuntime(),
	}
	state := testStreamState()
	recorder := httptest.NewRecorder()
	sw := runtime.streamRuntime.NewWriter(recorder, state.BaseIDs)
	phase := "collecting"
	result := streamResult{}
	event, _ := dto.NewStreamEvent(1, "state", "state.extracted_info.upsert", state.BaseIDs, map[string]any{
		"info": map[string]any{"body_part": "颈肩", "symptom_type": "酸胀"},
	})

	if stopped := runtime.handleAIEvent(context.Background(), sw, event, state, &result, &phase); !stopped {
		t.Fatal("durable BodyState failure must stop the active stream")
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "run.failed") || !strings.Contains(body, "message.failed") {
		t.Fatalf("expected fail-closed runtime events, got %s", body)
	}
	if strings.Contains(body, "state.extracted_info.upsert") {
		t.Fatalf("uncommitted extracted state must not be emitted as successful: %s", body)
	}
}

func TestHandleAIEventFailsClosedWhenSafetyWriteFails(t *testing.T) {
	runtime := &Runtime{
		bodyStateService: &fakeRuntimeBodyState{safetyErr: errors.New("db unavailable")},
		streamRuntime:    stream.NewRuntime(),
	}
	state := testStreamState()
	recorder := httptest.NewRecorder()
	sw := runtime.streamRuntime.NewWriter(recorder, state.BaseIDs)
	phase := "collecting"
	result := streamResult{}
	event, _ := dto.NewStreamEvent(1, "safety", "safety.red_flag.detected", state.BaseIDs, map[string]any{
		"has_red_flags": true,
	})

	if stopped := runtime.handleAIEvent(context.Background(), sw, event, state, &result, &phase); !stopped {
		t.Fatal("safety persistence failure must stop the active stream")
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "failed to persist safety state") {
		t.Fatalf("expected explicit safety persistence failure, got %s", body)
	}
	if strings.Contains(body, "safety.red_flag.detected") {
		t.Fatalf("uncommitted safety signal must not be emitted as successful: %s", body)
	}
}

func TestPersistInteractionAnswerPropagatesBodyStateFailure(t *testing.T) {
	runtime := &Runtime{bodyStateService: &fakeRuntimeBodyState{interactionErr: errors.New("db unavailable")}}
	err := runtime.persistInteractionAnswer(
		context.Background(), uuid.New(), uuid.New(), datatypes.JSON(`{"text":"是否麻木"}`), json.RawMessage(`"是"`),
	)
	if err == nil {
		t.Fatal("interaction answer persistence failure must be returned before interaction resume")
	}
}

func TestDurableExecutionContextSurvivesRequestCancellation(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	cancelParent()

	executionCtx, cancelExecution := durableExecutionContext(parent)
	defer cancelExecution()
	select {
	case <-executionCtx.Done():
		t.Fatalf("durable execution inherited request cancellation: %v", executionCtx.Err())
	default:
	}
}

func TestSendNewEventPersistsBeforeSocketWrite(t *testing.T) {
	repo := &fakeRuntimeEventRepo{}
	runtime := &Runtime{
		runtimeEventService: service.NewRuntimeEventService(repo),
		streamRuntime:       stream.NewRuntime(),
	}
	conversationID := uuid.New()
	runID := uuid.New()
	baseIDs := dto.StreamEventIDs{
		ConversationID: conversationID.String(),
		RunID:          runID.String(),
	}
	writer := runtime.streamRuntime.NewWriter(&failingResponseWriter{}, baseIDs)

	runtime.sendNewEvent(
		context.Background(), writer, "run", "run.started", baseIDs,
		map[string]any{"status": "running"}, "", "run.started",
	)

	if len(repo.events) != 1 || repo.events[0].Type != "run.started" {
		t.Fatalf("socket failure must not drop durable event: %#v", repo.events)
	}
}

func TestReplayCompletedRunReplaysRuntimeEventsWithoutSyntheticError(t *testing.T) {
	conversationID := uuid.New()
	runID := uuid.New()
	turnID := uuid.New()
	ids := datatypes.JSON([]byte(`{"conversation_id":"` + conversationID.String() + `","run_id":"` + runID.String() + `","turn_id":"` + turnID.String() + `"}`))

	runtime := &Runtime{
		runtimeEventService: service.NewRuntimeEventService(&fakeRuntimeEventRepo{events: []model.RuntimeEvent{
			{ConversationID: conversationID, RunID: runID, TurnID: &turnID, Seq: 3, Channel: "message", Type: "message.completed", IDs: ids, Payload: datatypes.JSON([]byte(`{"status":"completed"}`))},
			{ConversationID: conversationID, RunID: runID, TurnID: &turnID, Seq: 4, Channel: "run", Type: "run.completed", IDs: ids, Payload: datatypes.JSON([]byte(`{"status":"completed"}`))},
		}}),
		streamRuntime: stream.NewRuntime(),
	}

	recorder := httptest.NewRecorder()
	runtime.replayCompletedRun(context.Background(), recorder, &model.Run{
		ID:             runID,
		ConversationID: conversationID,
		TurnID:         turnID,
		Status:         "completed",
	})

	body := recorder.Body.String()
	if strings.Contains(body, "stream.error") {
		t.Fatalf("replay should not emit stream.error: %s", body)
	}
	if !strings.Contains(body, "event: message.completed") || !strings.Contains(body, "event: run.completed") {
		t.Fatalf("replay did not include stored events: %s", body)
	}
	if !strings.Contains(body, `"type":"stream.done"`) || !strings.Contains(body, `"seq":5`) {
		t.Fatalf("replay done should follow stored seqs: %s", body)
	}
}

func TestReplayCompletedRunDoesNotDuplicateStoredStreamDone(t *testing.T) {
	conversationID := uuid.New()
	runID := uuid.New()
	turnID := uuid.New()
	ids := datatypes.JSON([]byte(`{"conversation_id":"` + conversationID.String() + `","run_id":"` + runID.String() + `","turn_id":"` + turnID.String() + `"}`))
	runtime := &Runtime{
		runtimeEventService: service.NewRuntimeEventService(&fakeRuntimeEventRepo{events: []model.RuntimeEvent{
			{ConversationID: conversationID, RunID: runID, TurnID: &turnID, Seq: 3, Channel: "message", Type: "message.completed", IDs: ids, Payload: datatypes.JSON(`{"status":"completed"}`)},
			{ConversationID: conversationID, RunID: runID, TurnID: &turnID, Seq: 4, Channel: "stream", Type: "stream.done", IDs: ids, Payload: datatypes.JSON(`{}`)},
		}}),
		streamRuntime: stream.NewRuntime(),
	}
	recorder := httptest.NewRecorder()
	runtime.replayCompletedRun(context.Background(), recorder, &model.Run{
		ID: runID, ConversationID: conversationID, TurnID: turnID, Status: "completed",
	})
	if count := strings.Count(recorder.Body.String(), "event: stream.done"); count != 1 {
		t.Fatalf("expected one stored stream.done, got %d: %s", count, recorder.Body.String())
	}
}
