package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/bodysense/api/internal/dto"
)

type ChatStreamRequest struct {
	SessionID     string          `json:"session_id"`
	UserID        string          `json:"user_id"`
	Content       string          `json:"content"`
	UseCase       string          `json:"use_case,omitempty"`
	Profile       json.RawMessage `json:"profile,omitempty"`
	Messages      []ChatMessage   `json:"messages,omitempty"`
	ExtractedInfo json.RawMessage `json:"extracted_info,omitempty"`
	RAGResults    json.RawMessage `json:"rag_results,omitempty"`
	Phase         string          `json:"phase,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ToolDefinition struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Parameters  any    `json:"parameters"`
	} `json:"function"`
}

type DiagnosisRequest struct {
	ExtractedInfo       json.RawMessage `json:"extracted_info"`
	Profile             json.RawMessage `json:"profile,omitempty"`
	ConversationSummary string          `json:"conversation_summary,omitempty"`
	RAGContext          string          `json:"rag_context,omitempty"`
	RAGResults          json.RawMessage `json:"rag_results,omitempty"`
	UseCase             string          `json:"use_case,omitempty"`
}

type TreatmentRequest struct {
	ConfirmedDiagnosis json.RawMessage `json:"confirmed_diagnosis"`
	ExtractedInfo      json.RawMessage `json:"extracted_info"`
	Profile            json.RawMessage `json:"profile,omitempty"`
	RAGContext         string          `json:"rag_context,omitempty"`
	RAGResults         json.RawMessage `json:"rag_results,omitempty"`
	UseCase            string          `json:"use_case,omitempty"`
}

type AIClient struct {
	httpClient *http.Client
	baseURL    string
}

func NewAIClient() *AIClient {
	baseURL := os.Getenv("AI_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8100"
	}
	return &AIClient{
		httpClient: &http.Client{Timeout: 120 * time.Second},
		baseURL:    baseURL,
	}
}

// BaseURL returns the AI service base URL.
func (c *AIClient) BaseURL() string {
	return c.baseURL
}

// ChatStream calls Python /api/chat/stream and returns an NDJSON event channel.
func (c *AIClient) ChatStream(ctx context.Context, req ChatStreamRequest) (<-chan dto.StreamEvent, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat/stream", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call AI service: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("AI service returned status %d", resp.StatusCode)
	}

	events := make(chan dto.StreamEvent, 32)
	go func() {
		defer resp.Body.Close()
		defer close(events)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var event dto.StreamEvent
			if err := json.Unmarshal(line, &event); err != nil {
				continue // skip malformed lines
			}

			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	return events, nil
}

// AnalyzeDiagnosis calls /api/diagnosis/analyze.
func (c *AIClient) AnalyzeDiagnosis(ctx context.Context, req DiagnosisRequest) (json.RawMessage, error) {
	return c.callJSON(ctx, "/api/diagnosis/analyze", req)
}

// GenerateTreatment calls /api/diagnosis/treatment.
func (c *AIClient) GenerateTreatment(ctx context.Context, req TreatmentRequest) (json.RawMessage, error) {
	return c.callJSON(ctx, "/api/diagnosis/treatment", req)
}

// TitleGenerateRequest is the request body for /api/title/generate.
type TitleGenerateRequest struct {
	Messages []map[string]any `json:"messages"`
}

// TitleGenerateResponse is the response body from /api/title/generate.
type TitleGenerateResponse struct {
	Title string `json:"title"`
}

// GenerateTitle calls /api/title/generate to produce a concise Chinese title.
func (c *AIClient) GenerateTitle(ctx context.Context, messages []map[string]any) (string, error) {
	req := TitleGenerateRequest{Messages: messages}
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/title/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("call AI service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("AI service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result TitleGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return result.Title, nil
}

func (c *AIClient) callJSON(ctx context.Context, path string, req any) (json.RawMessage, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call AI service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("AI service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}
