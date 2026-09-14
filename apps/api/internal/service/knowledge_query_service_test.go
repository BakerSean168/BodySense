package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestKnowledgeQueryServiceUsesTypedSearchAndStatsContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/knowledge/search":
			var req KnowledgeSearchRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req.Query != "neck" || req.TopK != 5 {
				t.Fatalf("search request=%#v", req)
			}
			_ = json.NewEncoder(w).Encode(KnowledgeSearchResponse{Results: []KnowledgeSearchResult{{
				ID: 1, ProblemSlug: "neck", Category: "education", UnitType: "concept", Title: "T", Summary: "S", BodyMarkdown: "B",
				Similarity: .9, SourceTitle: "Source", SourceAuthor: "Author", SourceTimestamp: "00:01", Tags: []string{"neck"},
				Clips: []KnowledgeSearchClip{{ID: 2, ClipKey: "clip-2", ClipType: "video", Title: "Clip", FilePath: "/private/data/clip.mp4", SourceTimestamp: "00:01"}},
			}}, Total: 1})
		case "/api/knowledge/stats":
			_ = json.NewEncoder(w).Encode(KnowledgeStats{KnowledgeSources: 1, KnowledgeSegments: 2, KnowledgeUnits: 3, KnowledgeClips: 4})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := NewKnowledgeQueryService(server.URL)
	search, err := svc.Search(context.Background(), KnowledgeSearchRequest{Query: " neck "})
	if err != nil || search.Total != 1 || len(search.Results) != 1 || len(search.Results[0].Clips) != 1 {
		t.Fatalf("search=%#v err=%v", search, err)
	}
	stats, err := svc.Stats(context.Background())
	if err != nil || stats.KnowledgeUnits != 3 || stats.KnowledgeClips != 4 {
		t.Fatalf("stats=%#v err=%v", stats, err)
	}
}

func TestKnowledgeQueryServiceFailsClosedOnUpstreamSchemaDrift(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"knowledge_sources":1,"knowledge_segments":2,"knowledge_units":3,"knowledge_clips":4,"unexpected":true}`))
	}))
	defer server.Close()

	_, err := NewKnowledgeQueryService(server.URL).Stats(context.Background())
	if err == nil {
		t.Fatal("expected strict upstream schema failure")
	}
}
