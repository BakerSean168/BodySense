package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// knowledgeServiceClient is a shared HTTP client with timeout for knowledge search calls.
var knowledgeServiceClient = &http.Client{Timeout: 30 * time.Second}

type knowledgeSearchResult struct {
	Title           string           `json:"title"`
	Summary         string           `json:"summary"`
	BodyMarkdown    string           `json:"body_markdown"`
	Category        string           `json:"category"`
	ProblemSlug     string           `json:"problem_slug"`
	UnitType        string           `json:"unit_type"`
	SourceTitle     string           `json:"source_title"`
	SourceAuthor    string           `json:"source_author"`
	SourceTimestamp string           `json:"source_timestamp"`
	Tags            []string         `json:"tags"`
	Clips           []map[string]any `json:"clips"`
	Similarity      float64          `json:"similarity"`
}

type knowledgeSearchResponse struct {
	Results []knowledgeSearchResult `json:"results"`
}

func (h *DiagnosisHandler) searchKnowledge(ctx context.Context, query string) ([]map[string]any, error) {
	return searchKnowledge(ctx, h.aiServiceURL, query)
}

func searchKnowledge(ctx context.Context, aiServiceURL string, query string) ([]map[string]any, error) {
	body, err := json.Marshal(SearchRequest{
		Query: query,
		TopK:  5,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		aiServiceURL+"/api/knowledge/search",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := knowledgeServiceClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("knowledge search failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var payload knowledgeSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	sortKnowledgeResults(payload.Results, query)

	results := make([]map[string]any, 0, len(payload.Results))
	for _, result := range payload.Results {
		results = append(results, map[string]any{
			"title":            result.Title,
			"summary":          result.Summary,
			"content":          result.BodyMarkdown,
			"body_markdown":    result.BodyMarkdown,
			"category":         result.Category,
			"problem_slug":     result.ProblemSlug,
			"unit_type":        result.UnitType,
			"source_title":     result.SourceTitle,
			"source_author":    result.SourceAuthor,
			"source_timestamp": result.SourceTimestamp,
			"tags":             result.Tags,
			"clips":            result.Clips,
		})
	}

	return results, nil
}

func buildKnowledgeContext(results []map[string]any) string {
	if len(results) == 0 {
		return ""
	}

	lines := []string{"## 相关知识库参考"}
	for idx, result := range results {
		if idx >= 3 {
			break
		}
		title, _ := result["title"].(string)
		summary, _ := result["summary"].(string)
		content, _ := result["body_markdown"].(string)
		if content == "" {
			content, _ = result["content"].(string)
		}
		category, _ := result["category"].(string)
		sourceTitle, _ := result["source_title"].(string)
		sourceTimestamp, _ := result["source_timestamp"].(string)

		lines = append(lines, fmt.Sprintf("\n### 参考%d：%s（分类：%s）", idx+1, title, category))
		if summary != "" {
			lines = append(lines, "摘要："+summary)
		}
		if sourceTitle != "" || sourceTimestamp != "" {
			lines = append(lines, "来源："+strings.TrimSpace(sourceTitle+" "+sourceTimestamp))
		}
		if len(content) > 800 {
			// Truncate at rune boundary to avoid splitting multi-byte UTF-8 characters
			content = string([]rune(content)[:800])
		}
		if content != "" {
			lines = append(lines, content)
		}
	}

	return strings.Join(lines, "\n")
}

func sortKnowledgeResults(results []knowledgeSearchResult, query string) {
	if len(results) < 2 {
		return
	}

	preferredUnitTypes := []string{}
	switch {
	case containsAny(query, []string{"自测", "测试", "判断", "检查"}):
		preferredUnitTypes = []string{"self_check"}
	case containsAny(query, []string{"是什么", "定义", "什么意思"}):
		preferredUnitTypes = []string{"definition"}
	case containsAny(query, []string{"怎么", "如何", "处理", "改善", "纠正", "矫正", "训练", "动作", "缓解"}):
		preferredUnitTypes = []string{"exercise", "recommendation"}
	}

	sort.SliceStable(results, func(i, j int) bool {
		leftScore := unitTypeIntentScore(results[i].UnitType, preferredUnitTypes)
		rightScore := unitTypeIntentScore(results[j].UnitType, preferredUnitTypes)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		return results[i].Similarity > results[j].Similarity
	})
}

func containsAny(query string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(query, keyword) {
			return true
		}
	}
	return false
}

func unitTypeIntentScore(unitType string, preferred []string) int {
	for idx, value := range preferred {
		if unitType == value {
			return len(preferred) - idx
		}
	}
	return 0
}

// buildDiagnosisKnowledgeQuery builds a search query from extracted info for diagnosis RAG.
func buildDiagnosisKnowledgeQuery(extractedInfo []any) string {
	parts := make([]string, 0, len(extractedInfo)*3)
	for _, item := range extractedInfo {
		info, ok := item.(map[string]any)
		if !ok {
			continue
		}
		appendIfString(&parts, info["body_part"])
		appendIfString(&parts, info["symptom_type"])
		appendIfString(&parts, info["trigger"])
		appendIfString(&parts, info["severity"])
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// buildTreatmentKnowledgeQuery builds a search query from confirmed diagnosis and extracted info for treatment RAG.
func buildTreatmentKnowledgeQuery(confirmedDiagnosis map[string]any, extractedInfo []any) string {
	parts := []string{"改善", "训练", "动作"}
	appendIfString(&parts, confirmedDiagnosis["name"])
	appendIfString(&parts, confirmedDiagnosis["basis"])
	if len(extractedInfo) > 0 {
		parts = append(parts, buildDiagnosisKnowledgeQuery(extractedInfo))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func appendIfString(parts *[]string, value any) {
	if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
		*parts = append(*parts, strings.TrimSpace(text))
	}
}
