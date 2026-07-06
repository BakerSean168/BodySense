package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchKnowledgeMapsBodyMarkdownIntoContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/knowledge/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"title":            "什么是肘外翻",
					"summary":          "肘外翻是肘关节外偏过大。",
					"body_markdown":    "## 定义\n肘外翻是...",
					"category":         "posture.cubitus-valgus",
					"problem_slug":     "cubitus-valgus",
					"unit_type":        "definition",
					"source_title":     "肘外翻",
					"source_author":    "凯圣王",
					"source_timestamp": "00:00-00:18",
					"tags":             []string{"肘外翻"},
					"clips":            []map[string]any{},
				},
			},
		})
	}))
	defer server.Close()

	results, err := searchKnowledge(context.Background(), server.URL, "肘外翻是什么")
	if err != nil {
		t.Fatalf("searchKnowledge returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0]["content"] != "## 定义\n肘外翻是..." {
		t.Fatalf("expected body_markdown to be copied into content, got %v", results[0]["content"])
	}
}

func TestSortKnowledgeResultsPrefersExerciseForHowToQueries(t *testing.T) {
	t.Parallel()

	results := []knowledgeSearchResult{
		{
			Title:      "什么是肘外翻",
			UnitType:   "definition",
			Similarity: 0.91,
		},
		{
			Title:      "肘外翻怎么处理",
			UnitType:   "exercise",
			Similarity: 0.82,
		},
	}

	sortKnowledgeResults(results, "肘外翻怎么处理")

	if results[0].UnitType != "exercise" {
		t.Fatalf("expected exercise result first, got %s", results[0].UnitType)
	}
}
