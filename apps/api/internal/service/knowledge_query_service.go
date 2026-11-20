package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrKnowledgeQueryUpstream = errors.New("knowledge query upstream failed")

type KnowledgeSearchRequest struct {
	Query       string  `json:"query"`
	TopK        int     `json:"top_k"`
	ProblemSlug *string `json:"problem_slug,omitempty"`
	UnitType    *string `json:"unit_type,omitempty"`
}

type KnowledgeSearchClip struct {
	ID              int64  `json:"id"`
	ClipKey         string `json:"clip_key"`
	ClipType        string `json:"clip_type"`
	Title           string `json:"title"`
	FilePath        string `json:"file_path"`
	SourceTimestamp string `json:"source_timestamp"`
}

type KnowledgeSearchResult struct {
	ID              int64                 `json:"id"`
	ProblemSlug     string                `json:"problem_slug"`
	Category        string                `json:"category"`
	UnitType        string                `json:"unit_type"`
	Title           string                `json:"title"`
	Summary         string                `json:"summary"`
	BodyMarkdown    string                `json:"body_markdown"`
	Similarity      float64               `json:"similarity"`
	SourceTitle     string                `json:"source_title"`
	SourceAuthor    string                `json:"source_author"`
	SourceTimestamp string                `json:"source_timestamp"`
	Tags            []string              `json:"tags"`
	Clips           []KnowledgeSearchClip `json:"clips"`
}

type KnowledgeSearchResponse struct {
	Results []KnowledgeSearchResult `json:"results"`
	Total   int                     `json:"total"`
}

type KnowledgeStats struct {
	KnowledgeSources  int `json:"knowledge_sources"`
	KnowledgeSegments int `json:"knowledge_segments"`
	KnowledgeUnits    int `json:"knowledge_units"`
	KnowledgeClips    int `json:"knowledge_clips"`
}

// KnowledgeQueryService applies the knowledge query contract on top of the AI
// service adapter; the HTTP transport itself belongs to AIClient.
type KnowledgeQueryService struct {
	client *AIClient
}

func NewKnowledgeQueryService(baseURL string) *KnowledgeQueryService {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = "http://localhost:8100"
	}
	return &KnowledgeQueryService{
		client: newAIClient(strings.TrimRight(baseURL, "/"), knowledgeQueryTimeout),
	}
}

func (s *KnowledgeQueryService) Search(ctx context.Context, input KnowledgeSearchRequest) (*KnowledgeSearchResponse, error) {
	input.Query = strings.TrimSpace(input.Query)
	if input.Query == "" {
		return nil, fmt.Errorf("knowledge query is required")
	}
	if input.TopK == 0 {
		input.TopK = 5
	}
	if input.TopK < 1 || input.TopK > 20 {
		return nil, fmt.Errorf("knowledge top_k must be between 1 and 20")
	}
	response, err := s.client.SearchKnowledge(ctx, input)
	if err != nil {
		return nil, err
	}
	if response.Results == nil {
		response.Results = []KnowledgeSearchResult{}
	}
	return response, nil
}

func (s *KnowledgeQueryService) Stats(ctx context.Context) (*KnowledgeStats, error) {
	return s.client.KnowledgeStats(ctx)
}
