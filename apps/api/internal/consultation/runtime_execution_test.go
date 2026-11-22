package consultation

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/stream"
	"github.com/google/uuid"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStreamAIEventsFailsClosedBeforeFirstSemanticEventWithoutHandshake(t *testing.T) {
	state := testStreamState()
	repo := &fakeConsultationRunRepo{run: state.Run}
	runtime := &Runtime{runService: testRunService(repo), streamRuntime: stream.NewRuntime()}
	state.ExpectedConfigurationID = "consult-config-7feb8ca2d5bfad5a"
	recorder := httptest.NewRecorder()
	sw := runtime.streamRuntime.NewWriter(recorder, state.BaseIDs)
	events := make(chan service.ConsultationRuntimeEvent, 1)
	events <- privateRuntimeEvent(t, 1, service.ConsultationRuntimeTextDelta, state, map[string]any{"delta": "must not leak"})
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
	state.ExpectedConfigurationID = "consult-config-7feb8ca2d5bfad5a"
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
	state.ExpectedConfigurationID = "consult-config-7feb8ca2d5bfad5a"
	const workers = 24
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(marker int) {
			defer wg.Done()
			event := privateRuntimeEvent(t, 1, service.ConsultationRuntimeAgentConfiguration, state, map[string]any{
				"agent_configuration": map[string]any{
					"id":                       state.ExpectedConfigurationID,
					"role":                     "consultation",
					"decision_policy_revision": service.ConsultationDecisionPolicyV2,
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
	executionCtx, cancelExecution := (&Runtime{}).durableExecutionContext(parent)
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
	state.ExpectedConfigurationID = "consult-config-7feb8ca2d5bfad5a"
	recorder := httptest.NewRecorder()
	sw := runtime.streamRuntime.NewWriter(recorder, state.BaseIDs)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	events := make(chan service.ConsultationRuntimeEvent, 1)
	events <- privateRuntimeEvent(
		t, 1, service.ConsultationRuntimeTextDelta, state,
		map[string]any{"delta": "must-not-leak-after-cancel"},
	)
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

func TestProjectConsultationRuntimeEventKeepsAgentConfigurationPrivate(t *testing.T) {
	state := testStreamState()
	event := privateRuntimeEvent(t, 1, service.ConsultationRuntimeAgentConfiguration, state, map[string]any{
		"agent_configuration":  map[string]any{"id": "consult-config-0123456789abcdef"},
		"execution_provenance": map[string]any{"status": "executed"},
	})
	if _, err := projectConsultationRuntimeEvent(event); err == nil {
		t.Fatal("private Agent configuration handshake must not have a public StreamEvent projection")
	}
}

func TestProjectConsultationRuntimeInteractionDoesNotExposeLangGraphInterruptID(t *testing.T) {
	state := testStreamState()
	event := privateRuntimeEvent(t, 7, service.ConsultationRuntimeInteraction, state, map[string]any{
		"interaction_id": "45fda8478b2ef754419799e10992af06",
		"question":       map[string]any{"type": "single_choice", "prompt": "Continue?"},
	})
	event.IDs.ToolCallID = "tool-ask"
	event.IDs.InteractionID = "45fda8478b2ef754419799e10992af06"

	publicEvent, err := projectConsultationRuntimeEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	if publicEvent.Type != "state.interaction.required" || publicEvent.Channel != "state" || publicEvent.Seq != 7 {
		t.Fatalf("unexpected public projection: %#v", publicEvent)
	}
	if publicEvent.IDs.InteractionID != "" {
		t.Fatalf("LangGraph private interrupt id leaked into public projection: %#v", publicEvent.IDs)
	}
	if publicEvent.IDs.ToolCallID != "tool-ask" {
		t.Fatalf("tool-call correlation was lost: %#v", publicEvent.IDs)
	}
}

func TestExecutionTimeoutDrivesDurableExecutionContext(t *testing.T) {
	cases := []struct {
		name    string
		timeout time.Duration
		want    time.Duration
	}{
		{name: "injected budget", timeout: 10 * time.Minute, want: 10 * time.Minute},
		{name: "zero value keeps the safe default", want: 5 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &Runtime{executionTimeout: tc.timeout}
			ctx, cancel := runtime.durableExecutionContext(context.Background())
			defer cancel()

			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("durable execution context has no deadline")
			}
			remaining := time.Until(deadline)
			if remaining < tc.want-time.Second || remaining > tc.want {
				t.Fatalf("durable execution deadline = %s, want about %s", remaining, tc.want)
			}
		})
	}
}
