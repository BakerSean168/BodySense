package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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

type KnowledgeQueryService struct {
	baseURL string
	client  *http.Client
}

func NewKnowledgeQueryService(baseURL string) *KnowledgeQueryService {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = "http://localhost:8100"
	}
	return &KnowledgeQueryService{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
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
	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode knowledge search request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/knowledge/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	var response KnowledgeSearchResponse
	if err := s.doJSON(req, &response); err != nil {
		return nil, err
	}
	if response.Results == nil {
		response.Results = []KnowledgeSearchResult{}
	}
	return &response, nil
}

func (s *KnowledgeQueryService) Stats(ctx context.Context) (*KnowledgeStats, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/api/knowledge/stats", nil)
	if err != nil {
		return nil, err
	}
	var response KnowledgeStats
	if err := s.doJSON(req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (s *KnowledgeQueryService) doJSON(req *http.Request, target any) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("%w: client unavailable", ErrKnowledgeQueryUpstream)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrKnowledgeQueryUpstream, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("%w: status %d", ErrKnowledgeQueryUpstream, resp.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 8<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: invalid response: %v", ErrKnowledgeQueryUpstream, err)
	}
	return nil
}
