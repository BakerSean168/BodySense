package handler

import (
	"testing"
)

func TestSortKnowledgeResultsPrefersSelfCheckForTestQueries(t *testing.T) {
	t.Parallel()

	results := []knowledgeSearchResult{
		{Title: "改善方法", UnitType: "exercise", Similarity: 0.9},
		{Title: "自测方法", UnitType: "self_check", Similarity: 0.8},
	}

	sortKnowledgeResults(results, "怎么自测")

	if results[0].UnitType != "self_check" {
		t.Fatalf("expected self_check first, got %s", results[0].UnitType)
	}
}

func TestSortKnowledgeResultsPrefersDefinitionForWhatIsQueries(t *testing.T) {
	t.Parallel()

	results := []knowledgeSearchResult{
		{Title: "改善动作", UnitType: "exercise", Similarity: 0.9},
		{Title: "定义说明", UnitType: "definition", Similarity: 0.85},
	}

	sortKnowledgeResults(results, "是什么")

	if results[0].UnitType != "definition" {
		t.Fatalf("expected definition first, got %s", results[0].UnitType)
	}
}

func TestSortKnowledgeResultsFallsBackToSimilarity(t *testing.T) {
	t.Parallel()

	results := []knowledgeSearchResult{
		{Title: "低分", UnitType: "exercise", Similarity: 0.7},
		{Title: "高分", UnitType: "exercise", Similarity: 0.95},
	}

	sortKnowledgeResults(results, "怎么处理")

	if results[0].Title != "高分" {
		t.Fatalf("expected higher similarity first, got %s", results[0].Title)
	}
}

func TestSortKnowledgeResultsSingleItem(t *testing.T) {
	t.Parallel()

	results := []knowledgeSearchResult{
		{Title: "唯一", UnitType: "definition", Similarity: 0.8},
	}

	sortKnowledgeResults(results, "怎么处理")

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestBuildKnowledgeContextEmpty(t *testing.T) {
	t.Parallel()

	result := buildKnowledgeContext([]map[string]any{})
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
}

func TestBuildKnowledgeContextFormatsTop3(t *testing.T) {
	t.Parallel()

	results := []map[string]any{
		{"title": "第一条", "summary": "摘要1", "body_markdown": "内容1", "category": "cat1"},
		{"title": "第二条", "summary": "摘要2", "body_markdown": "内容2", "category": "cat2"},
		{"title": "第三条", "summary": "摘要3", "body_markdown": "内容3", "category": "cat3"},
		{"title": "第四条", "summary": "摘要4", "body_markdown": "内容4", "category": "cat4"},
	}

	context := buildKnowledgeContext(results)

	if context == "" {
		t.Fatal("expected non-empty context")
	}
	// Should contain first 3
	for _, title := range []string{"第一条", "第二条", "第三条"} {
		if !containsString(context, title) {
			t.Fatalf("expected context to contain %q", title)
		}
	}
	// Should NOT contain 4th
	if containsString(context, "第四条") {
		t.Fatal("expected context to NOT contain 4th item")
	}
}

func TestBuildKnowledgeContextIncludesSource(t *testing.T) {
	t.Parallel()

	results := []map[string]any{
		{
			"title":            "测试标题",
			"summary":          "测试摘要",
			"body_markdown":    "测试内容",
			"category":         "test",
			"source_title":     "来源视频",
			"source_timestamp": "01:00-02:00",
		},
	}

	context := buildKnowledgeContext(results)

	if !containsString(context, "来源视频") {
		t.Fatal("expected context to contain source_title")
	}
	if !containsString(context, "01:00-02:00") {
		t.Fatal("expected context to contain source_timestamp")
	}
}

func TestBuildDiagnosisKnowledgeQuery(t *testing.T) {
	t.Parallel()

	extractedInfo := []any{
		map[string]any{
			"body_part":    "肩部",
			"symptom_type": "酸胀",
			"trigger":      "久坐后",
			"severity":     "轻度",
		},
	}

	query := buildDiagnosisKnowledgeQuery(extractedInfo)

	if query == "" {
		t.Fatal("expected non-empty query")
	}
	for _, keyword := range []string{"肩部", "酸胀", "久坐后", "轻度"} {
		if !containsString(query, keyword) {
			t.Fatalf("expected query to contain %q, got %q", keyword, query)
		}
	}
}

func TestBuildDiagnosisKnowledgeQuerySkipsInvalidItems(t *testing.T) {
	t.Parallel()

	extractedInfo := []any{
		"not a map",
		map[string]any{"body_part": "腰部"},
	}

	query := buildDiagnosisKnowledgeQuery(extractedInfo)

	if !containsString(query, "腰部") {
		t.Fatalf("expected query to contain 腰部, got %q", query)
	}
}

func TestBuildTreatmentKnowledgeQueryIncludesDiagnosisName(t *testing.T) {
	t.Parallel()

	diagnosis := map[string]any{
		"name":   "头前伸倾向",
		"basis":  "颈肩酸胀",
	}
	extractedInfo := []any{
		map[string]any{"body_part": "颈椎"},
	}

	query := buildTreatmentKnowledgeQuery(diagnosis, extractedInfo)

	if !containsString(query, "改善") {
		t.Fatal("expected query to contain 改善")
	}
	if !containsString(query, "头前伸倾向") {
		t.Fatal("expected query to contain diagnosis name")
	}
	if !containsString(query, "颈椎") {
		t.Fatal("expected query to contain body part from extracted info")
	}
}

func TestContainsAny(t *testing.T) {
	t.Parallel()

	if !containsAny("怎么处理这个问题", []string{"怎么", "如何"}) {
		t.Fatal("expected true for matching keyword")
	}
	if containsAny("没什么", []string{"怎么", "如何"}) {
		t.Fatal("expected false for non-matching text")
	}
}

func TestUnitTypeIntentScore(t *testing.T) {
	t.Parallel()

	preferred := []string{"exercise", "recommendation"}

	if score := unitTypeIntentScore("exercise", preferred); score != 2 {
		t.Fatalf("expected 2 for exercise, got %d", score)
	}
	if score := unitTypeIntentScore("recommendation", preferred); score != 1 {
		t.Fatalf("expected 1 for recommendation, got %d", score)
	}
	if score := unitTypeIntentScore("definition", preferred); score != 0 {
		t.Fatalf("expected 0 for definition, got %d", score)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
