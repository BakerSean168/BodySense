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
				ConsultationSnapshot: ConsultationSnapshot{
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

func TestGenerateTreatmentSendsConfirmedDiagnosis(t *testing.T) {
	var captured map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/diagnosis/treatment" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"treatment_plan":{"goal":"test"}}`))
	}))
	defer server.Close()

	client := &AIClient{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	_, err := client.GenerateTreatment(context.Background(), TreatmentRequest{
		ConfirmedDiagnosis: json.RawMessage(`{"name":"头前伸"}`),
		ExtractedInfo:      json.RawMessage(`[]`),
		UseCase:            "llm.json",
	})
	if err != nil {
		t.Fatalf("GenerateTreatment returned error: %v", err)
	}

	if _, ok := captured["confirmed_diagnosis"]; !ok {
		t.Fatalf("missing confirmed_diagnosis in request: %#v", captured)
	}
	if captured["use_case"] != "llm.json" {
		t.Fatalf("expected use_case llm.json, got %#v", captured["use_case"])
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
		_, _ = w.Write([]byte(`{"diagnoses":[{"name":"头前伸倾向","confidence":"中","severity":"轻度","basis":"久坐后颈肩酸胀","typical_symptoms":"颈肩酸胀"}],"governance":{"verdict":"accepted","kind":"diagnosis","reasons":[],"issues":[]}}`))
	}))
	defer server.Close()

	client := &AIClient{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	result, err := client.AnalyzeDiagnosis(context.Background(), DiagnosisRequest{
		ExtractedInfo:       json.RawMessage(`[{"body_part":"颈椎","symptom_type":"酸胀"}]`),
		Profile:             json.RawMessage(`{"age":30,"occupation":"程序员"}`),
		ConversationSummary: "久坐后颈肩酸胀",
		RAGContext:          "## 知识库\n头前伸相关资料",
		RAGResults:          json.RawMessage(`[{"title":"头前伸自测"}]`),
		UseCase:             "llm.json",
	})
	if err != nil {
		t.Fatalf("AnalyzeDiagnosis returned error: %v", err)
	}

	if captured["use_case"] != "llm.json" {
		t.Fatalf("expected use_case llm.json, got %#v", captured["use_case"])
	}
	if captured["conversation_summary"] != "久坐后颈肩酸胀" {
		t.Fatalf("unexpected conversation_summary: %#v", captured["conversation_summary"])
	}
	if captured["rag_context"] != "## 知识库\n头前伸相关资料" {
		t.Fatalf("unexpected rag_context: %#v", captured["rag_context"])
	}

	for _, key := range []string{"extracted_info", "profile", "rag_results"} {
		if _, ok := captured[key]; !ok {
			t.Fatalf("missing %s in request: %#v", key, captured)
		}
	}

	var response map[string]any
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("AnalyzeDiagnosis returned invalid JSON: %v", err)
	}
	if _, ok := response["diagnoses"]; !ok {
		t.Fatalf("missing diagnoses in response: %#v", response)
	}
	if governance, ok := response["governance"].(map[string]any); !ok || governance["verdict"] != "accepted" {
		t.Fatalf("expected accepted governance response, got %#v", response["governance"])
	}
}
