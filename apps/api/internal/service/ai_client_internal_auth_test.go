package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The Go API must present the shared internal token on every ai-service
// call once AI_SERVICE_INTERNAL_TOKEN is configured, and must send no
// token header while it is unset — the ai-service's backward-compatible
// unconfigured mode (R2-02/#625).

const testInternalToken = "unit-test-internal-token"

func tokenCaptureServer(t *testing.T, seen *string, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = r.Header.Get(internalTokenHeader)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func TestAIClientSendsInternalTokenOnJSONCall(t *testing.T) {
	t.Setenv(aiServiceInternalTokenEnv, testInternalToken)

	var seen string
	server := tokenCaptureServer(t, &seen, `{"knowledge_sources":1,"knowledge_segments":2,"knowledge_units":3,"knowledge_clips":4}`)
	defer server.Close()

	client := newAIClient(server.URL, 5*time.Second)
	if _, err := client.KnowledgeStats(context.Background()); err != nil {
		t.Fatalf("KnowledgeStats: %v", err)
	}
	if seen != testInternalToken {
		t.Fatalf("internal token header = %q, want the configured token", seen)
	}
}

func TestAIClientSendsInternalTokenOnCallJSON(t *testing.T) {
	t.Setenv(aiServiceInternalTokenEnv, testInternalToken)

	var seen string
	server := tokenCaptureServer(t, &seen, `{}`)
	defer server.Close()

	client := newAIClient(server.URL, 5*time.Second)
	_, err := client.AnalyzeDiagnosis(context.Background(), DiagnosisRequest{
		ConfigurationID:   "diagnosis-test",
		BodyStateRevision: 1,
		BodyState:         []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("AnalyzeDiagnosis: %v", err)
	}
	if seen != testInternalToken {
		t.Fatalf("internal token header = %q, want the configured token", seen)
	}
}

func TestAIClientSendsInternalTokenOnStream(t *testing.T) {
	t.Setenv(aiServiceInternalTokenEnv, testInternalToken)

	var seen string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get(internalTokenHeader)
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"version":1,"seq":"1","ids":{"conversation_id":"` + testRuntimeConversationID + `","run_id":"` + testRuntimeRunID + `"},"text_delta":{"delta":"hello"}}` + "\n"))
	}))
	defer server.Close()

	client := newAIClient(server.URL, 5*time.Second)
	events, err := client.StartConsultationTurn(
		context.Background(),
		testRuntimeThreadID,
		validStartConsultationTurnRequest(),
	)
	if err != nil {
		t.Fatalf("StartConsultationTurn: %v", err)
	}
	for range events {
	}
	if seen != testInternalToken {
		t.Fatalf("internal token header = %q, want the configured token", seen)
	}
}

func TestAIClientOmitsInternalTokenWhenUnconfigured(t *testing.T) {
	t.Setenv(aiServiceInternalTokenEnv, "")

	var seen string
	var present bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header[http.CanonicalHeaderKey(internalTokenHeader)]
		seen = r.Header.Get(internalTokenHeader)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"knowledge_sources":0,"knowledge_segments":0,"knowledge_units":0,"knowledge_clips":0}`))
	}))
	defer server.Close()

	client := newAIClient(server.URL, 5*time.Second)
	if _, err := client.KnowledgeStats(context.Background()); err != nil {
		t.Fatalf("KnowledgeStats: %v", err)
	}
	if present || seen != "" {
		t.Fatalf("internal token header sent while unconfigured: %q", seen)
	}
}
