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
		if r.URL.Path != "/api/chat/stream" {
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

	events, err := client.ChatStream(context.Background(), ChatStreamRequest{
		SessionID:     "s1",
		UserID:        "u1",
		Content:       "hello",
		UseCase:       "consultation.reply",
		Profile:       json.RawMessage(`{"age":30}`),
		Messages:      []ChatMessage{{Role: "user", Content: "hello"}},
		ExtractedInfo: json.RawMessage(`[]`),
		Phase:         "collecting",
	})
	if err != nil {
		t.Fatalf("ChatStream returned error: %v", err)
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

	if _, ok := captured["context"]; ok {
		t.Fatal("chat request must not contain nested context")
	}
	for _, key := range []string{"session_id", "user_id", "content", "messages", "profile", "extracted_info", "phase", "use_case"} {
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
