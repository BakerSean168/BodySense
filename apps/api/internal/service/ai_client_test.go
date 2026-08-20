package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestChatStreamSendsFlatPythonRequestAndParsesStreamEvent(t *testing.T) {
	var captured map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/runtime/threads/thread-1/turns" {
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
		"thread-1",
		StartConsultationTurnRequest{
			RunID:          "run-1",
			ConversationID: "conv-1",
			UserID:         "u1",
			Input: ConsultationUserInput{
				Type: "user_message",
				Text: "hello",
			},
			BusinessContext: ConsultationBusinessContext{
				Profile: json.RawMessage(`{"age":30}`),
				RuntimeState: ConsultationRuntimeState{
					Phase:         "collecting",
					ExtractedInfo: json.RawMessage(`[]`),
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

	for _, key := range []string{"run_id", "conversation_id", "user_id", "input", "business_context"} {
		if _, ok := captured[key]; !ok {
			t.Fatalf("missing top-level key %q in request: %#v", key, captured)
		}
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
		BodyStateRevision: 12,
		BodyState:         json.RawMessage(`{"current_revision":12,"facts":[{"id":"fact-1","kind":"discomfort","value":"颈肩酸胀"}],"observations":[]}`),
		RelevantHistory:   json.RawMessage(`[{"revision":11,"change_type":"fact.temporal_changed"}]`),
		Profile:           json.RawMessage(`{"age":30,"occupation":"程序员"}`),
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

	for _, key := range []string{"user_id", "body_state", "relevant_history", "profile"} {
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
