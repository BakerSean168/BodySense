package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// Retry classification pinned for R2-13/#636: idempotent reads and
// dedup-keyed writes retry on connection errors and 502/503/504 only;
// unkeyed writes and streaming turns never retry; 429 and business 4xx
// never retry.

const testKnowledgeStatsBody = `{"knowledge_sources":1,"knowledge_segments":2,"knowledge_units":3,"knowledge_clips":4}`

func fastRetryClient(baseURL string) *AIClient {
	client := newAIClient(baseURL, 5*time.Second)
	client.retryPolicy = aiRetryPolicy{
		maxAttempts: 3,
		baseDelay:   time.Millisecond,
		maxDelay:    5 * time.Millisecond,
		budget:      10 * time.Second,
	}
	return client
}

// scriptedServer answers the i-th request with statuses[i-1] (the last
// status repeats) and counts the requests it received.
func scriptedServer(attempts *atomic.Int32, statuses []int, okBody string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(attempts.Add(1))
		status := statuses[len(statuses)-1]
		if n <= len(statuses) {
			status = statuses[n-1]
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okBody))
	}))
}

func TestKnowledgeStatsRetriesTransient503ThenSucceeds(t *testing.T) {
	var attempts atomic.Int32
	server := scriptedServer(&attempts, []int{http.StatusServiceUnavailable, http.StatusOK}, testKnowledgeStatsBody)
	defer server.Close()

	client := fastRetryClient(server.URL)
	stats, err := client.KnowledgeStats(context.Background())
	if err != nil {
		t.Fatalf("KnowledgeStats: %v", err)
	}
	if stats.KnowledgeUnits != 3 {
		t.Fatalf("stats = %+v, want units 3", stats)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2 (one 503, one success)", got)
	}
}

func TestKnowledgeStatsStopsAtAttemptCap(t *testing.T) {
	var attempts atomic.Int32
	server := scriptedServer(&attempts, []int{http.StatusServiceUnavailable}, testKnowledgeStatsBody)
	defer server.Close()

	client := fastRetryClient(server.URL)
	if _, err := client.KnowledgeStats(context.Background()); err == nil {
		t.Fatal("KnowledgeStats succeeded against a persistent 503")
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("attempts = %d, want the 3-attempt cap", got)
	}
}

func TestKnowledgeStatsDoesNotRetryBusinessErrors(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusTooManyRequests} {
		var attempts atomic.Int32
		server := scriptedServer(&attempts, []int{status}, testKnowledgeStatsBody)

		client := fastRetryClient(server.URL)
		if _, err := client.KnowledgeStats(context.Background()); err == nil {
			t.Fatalf("status %d: KnowledgeStats succeeded", status)
		}
		if got := attempts.Load(); got != 1 {
			t.Fatalf("status %d: attempts = %d, want 1 (never retried)", status, got)
		}
		server.Close()
	}
}

func TestKnowledgeSearchRetriesTransient503(t *testing.T) {
	var attempts atomic.Int32
	server := scriptedServer(&attempts, []int{http.StatusServiceUnavailable, http.StatusOK}, `{"results":[],"total":0}`)
	defer server.Close()

	client := fastRetryClient(server.URL)
	if _, err := client.SearchKnowledge(context.Background(), KnowledgeSearchRequest{}); err != nil {
		t.Fatalf("SearchKnowledge: %v", err)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestConnectionErrorIsRetried(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("test server does not support hijacking")
				return
			}
			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			_ = conn.Close()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(testKnowledgeStatsBody))
	}))
	defer server.Close()

	client := fastRetryClient(server.URL)
	if _, err := client.KnowledgeStats(context.Background()); err != nil {
		t.Fatalf("KnowledgeStats after a dropped connection: %v", err)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestUnkeyedGenerationWriteIsNotRetried(t *testing.T) {
	var attempts atomic.Int32
	server := scriptedServer(&attempts, []int{http.StatusServiceUnavailable}, `{}`)
	defer server.Close()

	client := fastRetryClient(server.URL)
	_, err := client.AnalyzeDiagnosis(context.Background(), DiagnosisRequest{
		ConfigurationID:   "diagnosis-test",
		BodyStateRevision: 1,
		BodyState:         []byte(`{}`),
	})
	var httpErr *AIServiceHTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("AnalyzeDiagnosis error = %v, want AIServiceHTTPError 503", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1 (unkeyed writes never retry)", got)
	}
}

func TestKeyedErasureWriteIsRetried(t *testing.T) {
	var attempts atomic.Int32
	assetID := uuid.New()
	server := scriptedServer(
		&attempts,
		[]int{http.StatusServiceUnavailable, http.StatusOK},
		`{"status":"erased","subject_id":"`+assetID.String()+`","threads":[]}`,
	)
	defer server.Close()

	client := fastRetryClient(server.URL)
	receipt, err := client.EraseAssetRuntimeCopies(
		context.Background(),
		uuid.New(),
		assetID,
		model.AssetRuntimeScope{ContentSHA256: "abc123"},
	)
	if err != nil {
		t.Fatalf("EraseAssetRuntimeCopies: %v", err)
	}
	if receipt.Status != "erased" {
		t.Fatalf("receipt status = %q, want erased", receipt.Status)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestVideoIngestionIsRetried(t *testing.T) {
	var attempts atomic.Int32
	server := scriptedServer(&attempts, []int{http.StatusBadGateway, http.StatusOK}, `{"status":"ingested"}`)
	defer server.Close()

	client := fastRetryClient(server.URL)
	_, status, err := client.IngestKnowledgeVideo(context.Background(), map[string]any{"source_key": "s1"})
	if err != nil {
		t.Fatalf("IngestKnowledgeVideo: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestStreamingTurnIsNotRetried(t *testing.T) {
	var attempts atomic.Int32
	server := scriptedServer(&attempts, []int{http.StatusServiceUnavailable}, "")
	defer server.Close()

	client := fastRetryClient(server.URL)
	_, err := client.StartConsultationTurn(
		context.Background(),
		testRuntimeThreadID,
		validStartConsultationTurnRequest(),
	)
	if err == nil {
		t.Fatal("StartConsultationTurn succeeded against a 503")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1 (streams never retry)", got)
	}
}

func TestRetryStopsWhenContextIsCancelled(t *testing.T) {
	var attempts atomic.Int32
	server := scriptedServer(&attempts, []int{http.StatusServiceUnavailable}, testKnowledgeStatsBody)
	defer server.Close()

	client := newAIClient(server.URL, 5*time.Second)
	client.retryPolicy = aiRetryPolicy{
		maxAttempts: 3,
		baseDelay:   30 * time.Second,
		maxDelay:    30 * time.Second,
		budget:      time.Minute,
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	started := time.Now()
	if _, err := client.KnowledgeStats(ctx); err == nil {
		t.Fatal("KnowledgeStats succeeded despite cancellation")
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("cancellation took %v, want a prompt stop", elapsed)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1 (cancelled during backoff)", got)
	}
}
