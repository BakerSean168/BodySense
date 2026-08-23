package consultation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

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
func (r *fakeConsultationRunRepo) UpdateStatus(context.Context, uuid.UUID, string) error { return nil }
func (r *fakeConsultationRunRepo) CompleteRun(context.Context, uuid.UUID, uuid.UUID, any, string) error {
	return nil
}
func (r *fakeConsultationRunRepo) TryCompleteRun(context.Context, uuid.UUID, uuid.UUID, any, string) (bool, error) {
	return true, nil
}
func (r *fakeConsultationRunRepo) CancelRun(context.Context, uuid.UUID, uuid.UUID, any) (bool, error) {
	return true, nil
}
func (r *fakeConsultationRunRepo) FailRun(context.Context, uuid.UUID, uuid.UUID, any) error {
	return nil
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

func validConsultationHandshake(t *testing.T, state streamState) dto.StreamEvent {
	t.Helper()
	event, err := dto.NewStreamEvent(
		1,
		"runtime",
		"runtime.agent_configuration",
		state.BaseIDs,
		map[string]any{
			"agent_configuration": map[string]any{
				"id":                       "consult-config-2bd9b46735dd693c",
				"role":                     "consultation",
				"decision_policy_revision": service.ConsultationDecisionPolicyV1,
				"logical_model":            "bodysense-consultation",
			},
			"execution_provenance": map[string]any{
				"runtime":       "langgraph",
				"logical_model": "bodysense-consultation",
			},
		},
	)
	if err != nil {
		t.Fatalf("build handshake: %v", err)
	}
	return event
}

func TestRuntimeNoLongerOwnsPerRunPendingAgentConfiguration(t *testing.T) {
	typeOfRuntime := reflect.TypeOf(Runtime{})
	for _, fieldName := range []string{
		"pendingAgentConfigurationID",
		"pendingAgentConfiguration",
		"pendingExecutionProvenance",
	} {
		if _, found := typeOfRuntime.FieldByName(fieldName); found {
			t.Fatalf("service-global per-run field %s must not exist on Runtime", fieldName)
		}
	}
}

func TestValidateConsultationExecutionIdentity(t *testing.T) {
	state := testStreamState()
	state.ExpectedConfigurationID = "consult-config-2bd9b46735dd693c"
	event := validConsultationHandshake(t, state)

	identity, err := validateConsultationExecutionIdentity(event, state.ExpectedConfigurationID)
	if err != nil {
		t.Fatalf("valid handshake rejected: %v", err)
	}
	if identity.ConfigurationID != state.ExpectedConfigurationID {
		t.Fatalf("unexpected configuration id: %s", identity.ConfigurationID)
	}
}

func TestValidateConsultationExecutionIdentityRejectsMismatch(t *testing.T) {
	state := testStreamState()
	state.ExpectedConfigurationID = "consult-config-2bd9b46735dd693c"

	tests := []struct {
		name       string
		mutate     func(map[string]any, map[string]any)
		wantErrSub string
	}{
		{
			name: "configuration id",
			mutate: func(config, provenance map[string]any) {
				config["id"] = "consult-config-wrong"
			},
			wantErrSub: "configuration id",
		},
		{
			name: "role",
			mutate: func(config, provenance map[string]any) {
				config["role"] = "diagnosis"
			},
			wantErrSub: "role",
		},
		{
			name: "decision policy",
			mutate: func(config, provenance map[string]any) {
				config["decision_policy_revision"] = "wrong-policy"
			},
			wantErrSub: "decision policy",
		},
		{
			name: "configuration logical model",
			mutate: func(config, provenance map[string]any) {
				config["logical_model"] = "wrong-model"
			},
			wantErrSub: "logical model",
		},
		{
			name: "execution logical model",
			mutate: func(config, provenance map[string]any) {
				provenance["logical_model"] = "wrong-model"
			},
			wantErrSub: "execution logical model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configuration := map[string]any{
				"id":                       state.ExpectedConfigurationID,
				"role":                     "consultation",
				"decision_policy_revision": service.ConsultationDecisionPolicyV1,
				"logical_model":            "bodysense-consultation",
			}
			provenance := map[string]any{
				"runtime":       "langgraph",
				"logical_model": "bodysense-consultation",
			}
			tt.mutate(configuration, provenance)
			event, _ := dto.NewStreamEvent(1, "runtime", "runtime.agent_configuration", state.BaseIDs, map[string]any{
				"agent_configuration":  configuration,
				"execution_provenance": provenance,
			})
			_, err := validateConsultationExecutionIdentity(event, state.ExpectedConfigurationID)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Fatalf("expected %q error, got %v", tt.wantErrSub, err)
			}
		})
	}
}

func TestStreamAIEventsFailsClosedBeforeFirstSemanticEventWithoutHandshake(t *testing.T) {
	runtime := &Runtime{streamRuntime: stream.NewRuntime()}
	state := testStreamState()
	state.ExpectedConfigurationID = "consult-config-2bd9b46735dd693c"
	recorder := httptest.NewRecorder()
	sw := runtime.streamRuntime.NewWriter(recorder, state.BaseIDs)
	events := make(chan dto.StreamEvent, 1)
	textEvent, _ := dto.NewStreamEvent(1, "message", "message.text.delta", state.BaseIDs, map[string]any{"delta": "must not leak"})
	events <- textEvent
	close(events)

	_, stopped := runtime.streamAIEvents(context.Background(), sw, events, state)
	if !stopped {
		t.Fatal("stream without first identity handshake must fail closed")
	}
	body := recorder.Body.String()
	if strings.Contains(body, "must not leak") || strings.Contains(body, "message.text.delta") {
		t.Fatalf("semantic output escaped before identity validation: %s", body)
	}
	if !strings.Contains(body, "run.failed") {
		t.Fatalf("expected durable failed-run event, got %s", body)
	}
}

func TestHandleAgentConfigurationPersistsIdentityImmediately(t *testing.T) {
	repo := &fakeConsultationRunRepo{}
	runtime := &Runtime{
		runService:    service.NewRunService(repo),
		streamRuntime: stream.NewRuntime(),
	}
	state := testStreamState()
	state.ExpectedConfigurationID = "consult-config-2bd9b46735dd693c"
	recorder := httptest.NewRecorder()
	sw := runtime.streamRuntime.NewWriter(recorder, state.BaseIDs)
	phase := "collecting"
	result := streamResult{}

	if stopped := runtime.handleAIEvent(
		context.Background(), sw, validConsultationHandshake(t, state), state, &result, &phase,
	); stopped {
		t.Fatal("valid identity handshake unexpectedly stopped stream")
	}
	if repo.updatedConfigurationID != state.ExpectedConfigurationID {
		t.Fatalf("identity was not persisted at handshake: got %q", repo.updatedConfigurationID)
	}
	if result.ExecutionIdentity.ConfigurationID != state.ExpectedConfigurationID {
		t.Fatalf("identity was not retained run-locally: %+v", result.ExecutionIdentity)
	}
}

func TestExecutionIdentityValidationIsConcurrentAndRunLocal(t *testing.T) {
	state := testStreamState()
	state.ExpectedConfigurationID = "consult-config-2bd9b46735dd693c"
	const workers = 24
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(marker int) {
			defer wg.Done()
			event, _ := dto.NewStreamEvent(1, "runtime", "runtime.agent_configuration", state.BaseIDs, map[string]any{
				"agent_configuration": map[string]any{
					"id":                       state.ExpectedConfigurationID,
					"role":                     "consultation",
					"decision_policy_revision": service.ConsultationDecisionPolicyV1,
					"logical_model":            "bodysense-consultation",
				},
				"execution_provenance": map[string]any{
					"runtime":        "langgraph",
					"logical_model":  "bodysense-consultation",
					"request_marker": marker,
				},
			})
			identity, err := validateConsultationExecutionIdentity(event, state.ExpectedConfigurationID)
			if err != nil {
				errs <- err
				return
			}
			var provenance map[string]any
			if err := json.Unmarshal(identity.ExecutionProvenance, &provenance); err != nil {
				errs <- err
				return
			}
			if got := int(provenance["request_marker"].(float64)); got != marker {
				errs <- errors.New("execution provenance crossed run-local validation boundary")
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestDurableExecutionContextSurvivesTransportDisconnect(t *testing.T) {
	parent, disconnect := context.WithCancel(context.Background())
	executionCtx, cancelExecution := durableExecutionContext(parent)
	defer cancelExecution()
	disconnect()

	select {
	case <-executionCtx.Done():
		t.Fatalf("transport disconnect must not cancel durable run: %v", executionCtx.Err())
	case <-time.After(20 * time.Millisecond):
	}
}

func TestRegisteredRunCancellationIsExplicit(t *testing.T) {
	runtime := &Runtime{runCancels: make(map[uuid.UUID]context.CancelFunc)}
	runID := uuid.New()
	executionCtx, cancelExecution := context.WithCancel(context.Background())
	defer cancelExecution()
	unregister := runtime.registerRunCancellation(runID, cancelExecution)
	defer unregister()

	if !runtime.cancelRegisteredRun(runID) {
		t.Fatal("explicit cancellation should find registered run")
	}
	select {
	case <-executionCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("registered cancellation did not cancel execution context")
	}
}

func TestStreamAIEventsPrefersExplicitCancellationOverReadySemanticEvent(t *testing.T) {
	state := testStreamState()
	state.AssistantMsg = nil
	repo := &fakeConsultationRunRepo{run: &model.Run{
		ID: state.Run.ID, UserID: state.UID, ConversationID: state.ConversationID,
		TurnID: state.TurnID, Status: "cancelled",
	}}
	runtime := &Runtime{
		runService:    service.NewRunService(repo),
		streamRuntime: stream.NewRuntime(),
	}
	state.ExpectedConfigurationID = "consult-config-2bd9b46735dd693c"
	recorder := httptest.NewRecorder()
	sw := runtime.streamRuntime.NewWriter(recorder, state.BaseIDs)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	events := make(chan dto.StreamEvent, 1)
	textEvent, _ := dto.NewStreamEvent(
		1, "message", "message.text.delta", state.BaseIDs,
		map[string]any{"delta": "must-not-leak-after-cancel"},
	)
	events <- textEvent
	close(events)

	_, stopped := runtime.streamAIEvents(ctx, sw, events, state)
	if !stopped {
		t.Fatal("cancelled execution must stop even when a producer event is ready")
	}
	if strings.Contains(recorder.Body.String(), "must-not-leak-after-cancel") {
		t.Fatalf("semantic event leaked after cancellation: %s", recorder.Body.String())
	}
}

type fakeKnowledgeObservationService struct {
	inputs []service.RecordKnowledgePublicationObservationInput
}

func (f *fakeKnowledgeObservationService) Record(
	_ context.Context,
	input service.RecordKnowledgePublicationObservationInput,
) error {
	f.inputs = append(f.inputs, input)
	return nil
}

func publishedCitationPartForRuntimeObservation() map[string]any {
	return map[string]any{
		"type":       "source",
		"title":      "疼痛与伤害感受 · 一句话定义",
		"sourceType": "document",
		"providerMetadata": map[string]any{
			"bodysense": map[string]any{
				"source_type":           "thought_forest_note",
				"lifecycle_status":      "published",
				"publication_id":        "11111111-1111-1111-1111-111111111111",
				"publication_key":       "pain-definition-v3",
				"publication_batch_key": "thought-forest-reviewed-health-pilot",
				"published_version":     3,
				"unit_key":              "tfu-example",
				"claim_id":              "tfc-example",
				"claim_review_id":       "claim-review-example",
				"source_locator": map[string]any{
					"locator_type": "markdown_lines",
					"repository":   "thought-forest",
					"git_commit":   "abc123",
					"path":         "z/pain-and-nociception.md",
					"line_start":   20,
					"line_end":     23,
				},
			},
		},
	}
}

func answerAttributionPartForRuntimeObservation() map[string]any {
	ref := "published:11111111-1111-1111-1111-111111111111:v3:tfu-example"
	return map[string]any{
		"type": "data",
		"name": "answer_attribution",
		"data": map[string]any{
			"attribution": map[string]any{
				"attribution_id":   "tool-1:0",
				"policy_revision":  service.ConsultationAnswerAttributionPolicyV1,
				"claim_text":       "疼痛与伤害感受不是同一现象",
				"evidence_refs":    []string{ref},
				"grounding_status": "supported",
				"reason_codes":     []string{"lexical_support_sufficient"},
				"bindings": []map[string]any{{
					"evidence_ref":          ref,
					"publication_id":        "11111111-1111-1111-1111-111111111111",
					"publication_key":       "pain-definition-v3",
					"publication_batch_key": "thought-forest-reviewed-health-pilot",
					"published_version":     3,
					"unit_key":              "tfu-example",
					"claim_id":              "tfc-example",
					"claim_review_id":       "claim-review-example",
					"claim_kind":            "definition",
					"grounding_status":      "supported",
					"reason_codes":          []string{"lexical_support_sufficient"},
					"source_locator": map[string]any{
						"locator_type": "markdown_lines",
						"repository":   "thought-forest",
						"git_commit":   "abc123",
						"path":         "z/pain-and-nociception.md",
						"line_start":   20,
						"line_end":     23,
					},
				}},
			},
		},
	}
}

func TestRecordKnowledgeRuntimeObservationsStoresExactAttributedPublication(t *testing.T) {
	observer := &fakeKnowledgeObservationService{}
	runtime := &Runtime{knowledgeObservationService: observer}
	runID := uuid.New()
	messageID := uuid.New()

	runtime.recordKnowledgeRuntimeObservations(
		context.Background(),
		runID,
		messageID,
		[]map[string]any{
			publishedCitationPartForRuntimeObservation(),
			answerAttributionPartForRuntimeObservation(),
		},
	)

	if len(observer.inputs) != 1 {
		t.Fatalf("recorded %d observations, want 1", len(observer.inputs))
	}
	input := observer.inputs[0]
	if input.ObservationKind != "runtime_answer" || input.GroundingStatus != "supported" {
		t.Fatalf("unexpected runtime observation: %+v", input)
	}
	if input.PublicationKey != "pain-definition-v3" || input.ExpectedPublishedVersion != 3 {
		t.Fatalf("publication identity not pinned: %+v", input)
	}
}

func TestRecordKnowledgeRuntimeObservationsHoldsWhenPublishedCitationHasNoAttribution(t *testing.T) {
	observer := &fakeKnowledgeObservationService{}
	runtime := &Runtime{knowledgeObservationService: observer}

	runtime.recordKnowledgeRuntimeObservations(
		context.Background(),
		uuid.New(),
		uuid.New(),
		[]map[string]any{publishedCitationPartForRuntimeObservation()},
	)

	if len(observer.inputs) != 1 {
		t.Fatalf("recorded %d observations, want 1", len(observer.inputs))
	}
	input := observer.inputs[0]
	if input.GroundingStatus != "degraded" {
		t.Fatalf("grounding status = %q, want degraded", input.GroundingStatus)
	}
	if !strings.Contains(string(input.Metadata), "missing_answer_attribution") {
		t.Fatalf("missing attribution reason not preserved: %s", input.Metadata)
	}
}

func TestRecordKnowledgeRuntimeObservationsRejectsAttributionWithoutPersistedCitation(t *testing.T) {
	observer := &fakeKnowledgeObservationService{}
	runtime := &Runtime{knowledgeObservationService: observer}

	runtime.recordKnowledgeRuntimeObservations(
		context.Background(),
		uuid.New(),
		uuid.New(),
		[]map[string]any{answerAttributionPartForRuntimeObservation()},
	)

	if len(observer.inputs) != 1 {
		t.Fatalf("recorded %d observations, want 1", len(observer.inputs))
	}
	input := observer.inputs[0]
	if input.CitationStatus != "invalid" {
		t.Fatalf("citation status = %q, want invalid", input.CitationStatus)
	}
	if !strings.Contains(string(input.Metadata), "attribution_without_persisted_citation") {
		t.Fatalf("citation loss reason not preserved: %s", input.Metadata)
	}
}

func TestRecordKnowledgeRuntimeObservationsRejectsCitationAttributionIdentityDrift(t *testing.T) {
	observer := &fakeKnowledgeObservationService{}
	runtime := &Runtime{knowledgeObservationService: observer}
	citation := publishedCitationPartForRuntimeObservation()
	provider := citation["providerMetadata"].(map[string]any)
	metadata := provider["bodysense"].(map[string]any)
	metadata["claim_review_id"] = "different-review"

	runtime.recordKnowledgeRuntimeObservations(
		context.Background(),
		uuid.New(),
		uuid.New(),
		[]map[string]any{citation, answerAttributionPartForRuntimeObservation()},
	)

	if len(observer.inputs) != 1 {
		t.Fatalf("recorded %d observations, want 1", len(observer.inputs))
	}
	input := observer.inputs[0]
	if input.CitationStatus != "invalid" {
		t.Fatalf("citation status = %q, want invalid", input.CitationStatus)
	}
	if !strings.Contains(string(input.Metadata), "citation_attribution_identity_mismatch") {
		t.Fatalf("identity drift reason not preserved: %s", input.Metadata)
	}
}
