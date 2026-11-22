package consultation

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/stream"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeRuntimeEventRepo struct {
	events []model.RuntimeEvent
}

type immediateRunTransaction struct{}

func (immediateRunTransaction) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func testRunService(repo *fakeConsultationRunRepo) *service.RunService {
	events := service.NewRuntimeEventService(&fakeRuntimeEventRepo{})
	return service.NewRunService(repo).WithLifecycleEvents(events, immediateRunTransaction{})
}

type fakeConsultationRunRepo struct {
	mu                     sync.Mutex
	run                    *model.Run
	updatedConfigurationID string
	updatedConfiguration   datatypes.JSON
	updatedProvenance      datatypes.JSON
}

func (r *fakeConsultationRunRepo) Create(context.Context, *model.Run) error { return nil }
func (r *fakeConsultationRunRepo) CreateWithIdempotency(context.Context, *model.Run) (*model.Run, bool, error) {
	return nil, false, nil
}
func (r *fakeConsultationRunRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Run, error) {
	if r.run != nil && r.run.ID == id {
		clone := *r.run
		return &clone, nil
	}
	return nil, nil
}
func (r *fakeConsultationRunRepo) GetByRequestID(context.Context, uuid.UUID, string) (*model.Run, error) {
	return nil, nil
}
func (r *fakeConsultationRunRepo) ListByConversationID(context.Context, uuid.UUID) ([]model.Run, error) {
	return nil, nil
}
func (r *fakeConsultationRunRepo) MarkWaitingUser(context.Context, uuid.UUID) (bool, error) {
	return true, nil
}
func (r *fakeConsultationRunRepo) ResumeRunning(context.Context, uuid.UUID, string, time.Time) (bool, error) {
	return true, nil
}
func (r *fakeConsultationRunRepo) FailWaitingUser(context.Context, uuid.UUID, any) (bool, error) {
	return true, nil
}
func (r *fakeConsultationRunRepo) ListExpiredRuns(context.Context, time.Time, int) ([]model.Run, error) {
	return nil, nil
}
func (r *fakeConsultationRunRepo) FailExpiredRun(context.Context, uuid.UUID, time.Time, any) (bool, error) {
	return false, nil
}
func (r *fakeConsultationRunRepo) CompleteRun(context.Context, uuid.UUID, uuid.UUID, any, string) error {
	return nil
}
func (r *fakeConsultationRunRepo) TryCompleteRun(context.Context, uuid.UUID, uuid.UUID, any, string) (bool, error) {
	return true, nil
}
func (r *fakeConsultationRunRepo) CancelRun(context.Context, uuid.UUID, uuid.UUID, any) (bool, error) {
	return true, nil
}
func (r *fakeConsultationRunRepo) FailRun(context.Context, uuid.UUID, uuid.UUID, any) (bool, error) {
	return true, nil
}
func (r *fakeConsultationRunRepo) UpdateAgentConfiguration(
	_ context.Context,
	_ uuid.UUID,
	configurationID string,
	configuration datatypes.JSON,
	provenance datatypes.JSON,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updatedConfigurationID = configurationID
	r.updatedConfiguration = append(datatypes.JSON(nil), configuration...)
	r.updatedProvenance = append(datatypes.JSON(nil), provenance...)
	return nil
}

func (r *fakeConsultationRunRepo) RenewLease(context.Context, uuid.UUID, uuid.UUID, string, time.Time, time.Time) (bool, error) {
	return true, nil
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

func (r *fakeRuntimeEventRepo) CreateWithNextSequence(ctx context.Context, event *model.RuntimeEvent) error {
	maxSeq := 0
	for i := range r.events {
		if r.events[i].RunID == event.RunID && r.events[i].Seq > maxSeq {
			maxSeq = r.events[i].Seq
		}
	}
	clone := *event
	clone.Seq = maxSeq + 1
	event.Seq = clone.Seq
	r.events = append(r.events, clone)
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

type fakeRuntimeBodyState struct{}

func (f *fakeRuntimeBodyState) GetAgentBodyContext(_ context.Context, owner uuid.UUID) (*service.AgentBodyContext, error) {
	return &service.AgentBodyContext{Schema: service.AgentBodyContextSchemaV1, UserID: owner, RecordsAvailable: true}, nil
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

func privateRuntimeEvent(
	t *testing.T,
	seq int,
	kind service.ConsultationRuntimeEventKind,
	state streamState,
	payload any,
) service.ConsultationRuntimeEvent {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal private runtime event payload: %v", err)
	}
	var typed service.ConsultationRuntimeEventPayload
	switch kind {
	case service.ConsultationRuntimeAgentConfiguration:
		var value struct {
			AgentConfiguration  json.RawMessage `json:"agent_configuration"`
			ExecutionProvenance json.RawMessage `json:"execution_provenance"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatal(err)
		}
		typed = service.ConsultationRuntimeAgentConfigurationPayload{AgentConfiguration: value.AgentConfiguration, ExecutionProvenance: value.ExecutionProvenance}
	case service.ConsultationRuntimeTextDelta:
		var value service.ConsultationRuntimeTextDeltaPayload
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatal(err)
		}
		typed = value
	case service.ConsultationRuntimeExtractedInfo:
		var value struct {
			Info json.RawMessage `json:"info"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatal(err)
		}
		typed = service.ConsultationRuntimeExtractedInfoPayload{Info: value.Info}
	case service.ConsultationRuntimeLifestyleContext:
		var value struct {
			Context json.RawMessage `json:"context"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatal(err)
		}
		typed = service.ConsultationRuntimeLifestyleContextPayload{Context: value.Context}
	case service.ConsultationRuntimeRedFlagDetected:
		var value struct {
			HasRedFlags bool              `json:"has_red_flags"`
			Flags       []json.RawMessage `json:"flags"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatal(err)
		}
		if value.Flags == nil {
			value.Flags = []json.RawMessage{}
		}
		typed = service.ConsultationRuntimeRedFlagDetectedPayload{HasRedFlags: value.HasRedFlags, Flags: value.Flags}
	case service.ConsultationRuntimeInteraction:
		var value struct {
			InteractionID string          `json:"interaction_id"`
			Question      json.RawMessage `json:"question"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatal(err)
		}
		typed = service.ConsultationRuntimeInteractionPayload{InteractionID: value.InteractionID, Question: value.Question}
	default:
		t.Fatalf("unsupported private runtime test event kind %q", kind)
	}
	return service.ConsultationRuntimeEvent{
		Seq: seq,
		IDs: service.ConsultationRuntimeEventIDs{
			ConversationID: state.BaseIDs.ConversationID,
			RunID:          state.BaseIDs.RunID,
			ToolCallID:     state.BaseIDs.ToolCallID,
		},
		Payload: typed,
	}
}

func TestHandleAIEventForwardsExtractedCandidatesWithoutPersisting(t *testing.T) {
	for _, tc := range []struct {
		kind    service.ConsultationRuntimeEventKind
		payload map[string]any
	}{
		{service.ConsultationRuntimeExtractedInfo, map[string]any{"info": map[string]any{"body_part": "颈肩", "symptom_type": "酸胀"}}},
		{service.ConsultationRuntimeLifestyleContext, map[string]any{"context": map[string]any{"section": "sleep", "summary": "每晚睡五小时"}}},
	} {
		state := testStreamState()
		// No BodyState service is configured: a model-extracted candidate must
		// never need persistence, so it cannot fail the stream either.
		runtime := &Runtime{streamRuntime: stream.NewRuntime()}
		recorder := httptest.NewRecorder()
		sw := runtime.streamRuntime.NewWriter(recorder, state.BaseIDs)
		phase := "collecting"
		result := streamResult{}
		event := privateRuntimeEvent(t, 1, tc.kind, state, tc.payload)
		if stopped := runtime.handleAIEvent(context.Background(), sw, event, state, &result, &phase); stopped {
			t.Fatalf("%s candidate must be forwarded as ephemeral state: %s", tc.kind, recorder.Body.String())
		}
		if body := recorder.Body.String(); strings.Contains(body, "run.failed") {
			t.Fatalf("%s candidate failed the stream: %s", tc.kind, body)
		}
	}
}

func TestHandleAIWarningHasNoSafetyWriteAuthority(t *testing.T) {
	state := testStreamState()
	repo := &fakeConsultationRunRepo{run: state.Run}
	runtime := &Runtime{
		bodyStateService: &fakeRuntimeBodyState{},
		runService:       testRunService(repo),
		streamRuntime:    stream.NewRuntime(),
	}
	recorder := httptest.NewRecorder()
	sw := runtime.streamRuntime.NewWriter(recorder, state.BaseIDs)
	phase := "collecting"
	result := streamResult{}
	event := privateRuntimeEvent(t, 1, service.ConsultationRuntimeRedFlagDetected, state, map[string]any{
		"has_red_flags": true,
		"flags":         []any{},
	})

	if stopped := runtime.handleAIEvent(context.Background(), sw, event, state, &result, &phase); stopped {
		t.Fatal("untrusted model warning must not write Safety or stop the stream")
	}
	body := recorder.Body.String()
	if strings.Contains(body, "failed to persist safety state") {
		t.Fatalf("expected explicit safety persistence failure, got %s", body)
	}
	if !strings.Contains(body, "safety.red_flag.detected") {
		t.Fatalf("uncommitted safety signal must not be emitted as successful: %s", body)
	}
}

func TestDurableExecutionContextSurvivesRequestCancellation(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	cancelParent()

	executionCtx, cancelExecution := (&Runtime{}).durableExecutionContext(parent)
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

func TestReattachActiveRunReplaysDurableEventsThroughStoredStreamDone(t *testing.T) {
	conversationID := uuid.New()
	runID := uuid.New()
	turnID := uuid.New()
	ids := datatypes.JSON([]byte(`{"conversation_id":"` + conversationID.String() + `","run_id":"` + runID.String() + `","turn_id":"` + turnID.String() + `"}`))
	runtime := &Runtime{
		runtimeEventService: service.NewRuntimeEventService(&fakeRuntimeEventRepo{events: []model.RuntimeEvent{
			{ConversationID: conversationID, RunID: runID, TurnID: &turnID, Seq: 1, Channel: "run", Type: "run.started", IDs: ids, Payload: datatypes.JSON([]byte(`{"status":"running"}`))},
			{ConversationID: conversationID, RunID: runID, TurnID: &turnID, Seq: 2, Channel: "message", Type: "message.text.delta", IDs: ids, Payload: datatypes.JSON([]byte(`{"delta":"hello"}`))},
			{ConversationID: conversationID, RunID: runID, TurnID: &turnID, Seq: 3, Channel: "stream", Type: "stream.done", IDs: ids, Payload: datatypes.JSON([]byte(`{}`))},
		}}),
		streamRuntime: stream.NewRuntime(),
	}

	recorder := httptest.NewRecorder()
	runtime.reattachActiveRun(context.Background(), recorder, &model.Run{
		ID: runID, ConversationID: conversationID, TurnID: turnID, Status: "running",
	})

	body := recorder.Body.String()
	if !strings.Contains(body, "event: run.started") || !strings.Contains(body, "event: message.text.delta") {
		t.Fatalf("reattach did not replay active durable events: %s", body)
	}
	if count := strings.Count(body, "event: stream.done"); count != 1 {
		t.Fatalf("reattach should stop at one durable stream.done, got %d: %s", count, body)
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

func validConsultationHandshake(t *testing.T, state streamState) service.ConsultationRuntimeEvent {
	t.Helper()
	return privateRuntimeEvent(t, 1, service.ConsultationRuntimeAgentConfiguration, state, map[string]any{
		"agent_configuration": map[string]any{
			"id":                       "consult-config-7feb8ca2d5bfad5a",
			"role":                     "consultation",
			"decision_policy_revision": service.ConsultationDecisionPolicyV2,
			"logical_model":            "bodysense-consultation",
		},
		"execution_provenance": map[string]any{
			"runtime":       "langgraph",
			"logical_model": "bodysense-consultation",
		},
	})
}
