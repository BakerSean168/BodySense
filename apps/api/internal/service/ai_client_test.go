package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bodysense/api/internal/dto"
)

const (
	testRuntimeThreadID       = "11111111-1111-4111-8111-111111111111"
	testRuntimeRunID          = "22222222-2222-4222-8222-222222222222"
	testRuntimeConversationID = "33333333-3333-4333-8333-333333333333"
	testRuntimeUserID         = "44444444-4444-4444-8444-444444444444"
	testRuntimeInterruptID    = "55555555-5555-4555-8555-555555555555"
)

func validStartConsultationTurnRequest() StartConsultationTurnRequest {
	return StartConsultationTurnRequest{
		RunID:           testRuntimeRunID,
		ConversationID:  testRuntimeConversationID,
		UserID:          testRuntimeUserID,
		ConfigurationID: defaultConsultationConfigurationID,
		Input: ConsultationUserInput{
			Type: "user_message",
			Text: "hello",
		},
		BusinessContext: ConsultationBusinessContext{
			Profile: json.RawMessage(`{}`),
			RuntimeState: ConsultationRuntimeState{
				Phase:         "collecting",
				ExtractedInfo: json.RawMessage(`[]`),
			},
		},
	}
}

func TestChatStreamSendsFlatPythonRequestAndParsesStreamEvent(t *testing.T) {
	var captured map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/runtime/threads/"+testRuntimeThreadID+"/turns" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"version":1,"seq":1,"channel":"message","type":"message.text.delta","ids":{"conversation_id":"s1"},"payload":{"delta":"hello"}}` + "\n"))
	}))
	defer server.Close()

	client := &AIClient{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	events, err := client.StartConsultationTurn(
		context.Background(),
		testRuntimeThreadID,
		StartConsultationTurnRequest{
			RunID:           testRuntimeRunID,
			ConversationID:  testRuntimeConversationID,
			UserID:          testRuntimeUserID,
			ConfigurationID: defaultConsultationConfigurationID,
			Input: ConsultationUserInput{
				Type: "user_message",
				Text: "hello",
			},
			BusinessContext: ConsultationBusinessContext{
				Profile: json.RawMessage(`{"gender":"female","birth_date":"1996-08-27","age_years":30}`),
				RuntimeState: ConsultationRuntimeState{
					Phase:         "collecting",
					ExtractedInfo: json.RawMessage(`[]`),
				},
				SpatialContext: &ConsultationSpatialContext{
					BodyRegionID:    "shoulder.right",
					BodyRegionLabel: "右肩",
					AnatomyID:       "appendicular-skeleton-clavicle-right",
					AnatomyName:     "Right clavicle",
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("StartConsultationTurn returned error: %v", err)
	}

	select {
	case event := <-events:
		if event.Type != "message.text.delta" {
			t.Fatalf("expected message.text.delta event, got %s", event.Type)
		}
		if string(event.Payload) != `{"delta":"hello"}` {
			t.Fatalf("unexpected payload: %s", event.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stream event")
	}

	for _, key := range []string{"thread_id", "run_id", "conversation_id", "user_id", "configuration_id", "input", "business_context"} {
		if _, ok := captured[key]; !ok {
			t.Fatalf("missing top-level key %q in request: %#v", key, captured)
		}
	}
	businessContext, ok := captured["business_context"].(map[string]any)
	if !ok {
		t.Fatalf("business_context is not an object: %#v", captured["business_context"])
	}
	spatialContext, ok := businessContext["spatial_context"].(map[string]any)
	if !ok || spatialContext["body_region_id"] != "shoulder.right" || spatialContext["anatomy_id"] != "appendicular-skeleton-clavicle-right" {
		t.Fatalf("unexpected spatial_context payload: %#v", businessContext["spatial_context"])
	}
}

func TestConsultationStartRejectsInvalidProtoCommandBeforeHTTP(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer server.Close()
	client := &AIClient{httpClient: server.Client(), baseURL: server.URL}

	_, err := client.StartConsultationTurn(context.Background(), "not-a-uuid", validStartConsultationTurnRequest())
	if err == nil {
		t.Fatal("expected invalid runtime command to fail before HTTP")
	}
	if called {
		t.Fatal("invalid runtime command reached AI HTTP boundary")
	}
}

func TestConsultationResumeSendsValidatedThreadAndInterruptIdentity(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantPath := "/runtime/threads/" + testRuntimeThreadID + "/interrupts/" + testRuntimeInterruptID + "/resume"
		if r.URL.Path != wantPath {
			t.Fatalf("path = %q, want %q", r.URL.Path, wantPath)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"version":1,"seq":1,"channel":"stream","type":"stream.done","ids":{"conversation_id":"` + testRuntimeConversationID + `","run_id":"` + testRuntimeRunID + `"},"payload":{}}` + "\n"))
	}))
	defer server.Close()
	client := &AIClient{httpClient: server.Client(), baseURL: server.URL}

	req := ResumeConsultationInterruptRequest{
		RunID:           testRuntimeRunID,
		ConversationID:  testRuntimeConversationID,
		UserID:          testRuntimeUserID,
		ConfigurationID: defaultConsultationConfigurationID,
		InterruptID:     testRuntimeInterruptID,
		Answer:          json.RawMessage(`{"value":"yes"}`),
		BusinessContext: validStartConsultationTurnRequest().BusinessContext,
	}
	events, err := client.ResumeConsultationInterrupt(context.Background(), testRuntimeThreadID, testRuntimeInterruptID, req)
	if err != nil {
		t.Fatalf("ResumeConsultationInterrupt: %v", err)
	}
	if event := <-events; event.Type != "stream.done" {
		t.Fatalf("event type = %q, want stream.done", event.Type)
	}
	if captured["thread_id"] != testRuntimeThreadID || captured["interrupt_id"] != testRuntimeInterruptID {
		t.Fatalf("runtime command lost path identities: %#v", captured)
	}
}

func TestAnalyzeDiagnosisSendsPythonContract(t *testing.T) {
	var captured map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/diagnosis/analyze" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"name":"头前伸倾向","confidence":"中","severity":"轻度","basis":"久坐后颈肩酸胀","typical_symptoms":"颈肩酸胀"}],"governance":{"verdict":"accepted","kind":"diagnosis","reasons":[],"issues":[]}}`))
	}))
	defer server.Close()

	client := &AIClient{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	result, err := client.AnalyzeDiagnosis(context.Background(), DiagnosisRequest{
		UserID:            "user-1",
		ConfigurationID:   defaultDiagnosisConfigurationID,
		BodyStateRevision: 12,
		BodyState:         json.RawMessage(`{"current_revision":12,"facts":[{"id":"fact-1","kind":"discomfort","value":"颈肩酸胀"}],"observations":[]}`),
		RelevantHistory:   json.RawMessage(`[{"revision":11,"change_type":"fact.temporal_changed"}]`),
		Profile:           json.RawMessage(`{"gender":"male","birth_date":"1996-08-27","age_years":30}`),
	})
	if err != nil {
		t.Fatalf("AnalyzeDiagnosis returned error: %v", err)
	}

	if _, exists := captured["use_case"]; exists {
		t.Fatalf("Diagnosis contract must not expose provider/model routing intent: %#v", captured)
	}
	if captured["body_state_revision"] != float64(12) {
		t.Fatalf("unexpected body_state_revision: %#v", captured["body_state_revision"])
	}

	for _, key := range []string{"user_id", "configuration_id", "body_state", "relevant_history", "profile"} {
		if _, ok := captured[key]; !ok {
			t.Fatalf("missing %s in request: %#v", key, captured)
		}
	}

	var response map[string]any
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("AnalyzeDiagnosis returned invalid JSON: %v", err)
	}
	if _, ok := response["candidates"]; !ok {
		t.Fatalf("missing candidates in response: %#v", response)
	}
	if governance, ok := response["governance"].(map[string]any); !ok || governance["verdict"] != "accepted" {
		t.Fatalf("expected accepted governance response, got %#v", response["governance"])
	}
}

func TestConsultationStreamConvertsMalformedNDJSONToProtocolError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte("{not-json}\n"))
	}))
	defer server.Close()
	client := &AIClient{httpClient: server.Client(), baseURL: server.URL}
	events, err := client.StartConsultationTurn(context.Background(), testRuntimeThreadID, validStartConsultationTurnRequest())
	if err != nil {
		t.Fatal(err)
	}
	event := <-events
	if event.Type != "stream.error" || event.Channel != "stream" {
		t.Fatalf("expected protocol stream.error, got %#v", event)
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := event.PayloadAs(&payload); err != nil || payload.Message == "" {
		t.Fatalf("expected sanitized protocol error payload, got %s (%v)", event.Payload, err)
	}
}

func TestConsultationStreamRejectsUnknownInternalEventType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"version":1,"seq":1,"channel":"state","type":"state.unknown","ids":{},"payload":{}}` + "\n"))
	}))
	defer server.Close()
	client := &AIClient{httpClient: server.Client(), baseURL: server.URL}
	events, err := client.StartConsultationTurn(context.Background(), testRuntimeThreadID, validStartConsultationTurnRequest())
	if err != nil {
		t.Fatal(err)
	}
	event := <-events
	if event.Type != "stream.error" {
		t.Fatalf("expected protocol stream.error, got %#v", event)
	}
}

func TestValidateConsultationInternalEventRejectsMalformedRedFlag(t *testing.T) {
	event, _ := dto.NewStreamEvent(1, "safety", "safety.red_flag.detected", dto.StreamEventIDs{}, map[string]any{
		"has_red_flags": "yes", "flags": []any{},
	})
	if err := validateConsultationInternalEvent(event); err == nil {
		t.Fatal("malformed authority payload must be rejected")
	}
}

func TestValidateConsultationInternalEventRequiresPublishedThoughtForestCitationIdentity(t *testing.T) {
	validCitation := map[string]any{
		"source_type":           "thought_forest_note",
		"unit_key":              "tfu-pain",
		"source_key":            "thought-forest:z/pain.md",
		"lifecycle_status":      "published",
		"publication_id":        "11111111-1111-1111-1111-111111111111",
		"published_version":     3,
		"publication_key":       "pain-v3",
		"publication_batch_key": "pain-batch",
		"claim_id":              "claim-pain",
		"claim_review_id":       "review-pain",
		"source_locator": map[string]any{
			"locator_type": "markdown_lines",
			"git_commit":   "abc123",
			"path":         "z/pain.md",
			"line_start":   20,
			"line_end":     23,
		},
	}
	event, _ := dto.NewStreamEvent(
		1,
		"source",
		"source.citation.added",
		dto.StreamEventIDs{},
		map[string]any{"citation": validCitation},
	)
	if err := validateConsultationInternalEvent(event); err != nil {
		t.Fatalf("valid published Thought Forest citation rejected: %v", err)
	}

	delete(validCitation, "publication_id")
	invalid, _ := dto.NewStreamEvent(
		1,
		"source",
		"source.citation.added",
		dto.StreamEventIDs{},
		map[string]any{"citation": validCitation},
	)
	if err := validateConsultationInternalEvent(invalid); err == nil {
		t.Fatal("published Thought Forest citation without publication identity must be rejected")
	}
}

func TestValidateConsultationInternalEventKeepsLegacyVideoCitationCompatible(t *testing.T) {
	event, _ := dto.NewStreamEvent(
		1,
		"source",
		"source.citation.added",
		dto.StreamEventIDs{},
		map[string]any{"citation": map[string]any{
			"title":       "Legacy video citation",
			"source_type": "video",
		}},
	)
	if err := validateConsultationInternalEvent(event); err != nil {
		t.Fatalf("legacy video citation must remain compatible: %v", err)
	}
}

func TestValidateConsultationInternalEventAcceptsValidatedAnswerAttribution(t *testing.T) {
	var payload map[string]any
	if err := json.Unmarshal(validAnswerAttributionPayload(), &payload); err != nil {
		t.Fatal(err)
	}
	event, _ := dto.NewStreamEvent(
		1,
		"source",
		"source.answer_attribution.added",
		dto.StreamEventIDs{},
		payload,
	)
	if err := validateConsultationInternalEvent(event); err != nil {
		t.Fatalf("valid answer attribution rejected: %v", err)
	}

	attribution := payload["attribution"].(map[string]any)
	bindings := attribution["bindings"].([]any)
	bindings[0].(map[string]any)["publication_id"] = "not-a-uuid"
	invalid, _ := dto.NewStreamEvent(
		1,
		"source",
		"source.answer_attribution.added",
		dto.StreamEventIDs{},
		payload,
	)
	if err := validateConsultationInternalEvent(invalid); err == nil {
		t.Fatal("invalid answer attribution publication identity must be rejected")
	}
}
