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

type DiagnosisRequest struct {
	UserID string `json:"user_id,omitempty"`
	// New ADR 0004 boundary: Diagnosis pins exact durable BodyState input.
	BodyStateRevision int64           `json:"body_state_revision"`
	BodyState         json.RawMessage `json:"body_state"`
	RelevantHistory   json.RawMessage `json:"relevant_history,omitempty"`
	Profile           json.RawMessage `json:"profile,omitempty"`
	UseCase           string          `json:"use_case,omitempty"`
}

// TreatmentRecommendationRequest pins exact durable identities and returns a
// proposal that still requires explicit user acceptance in Go.
type TreatmentRecommendationRequest struct {
	UserID               string          `json:"user_id,omitempty"`
	BodyStateRevision    int64           `json:"body_state_revision"`
	BodyState            json.RawMessage `json:"body_state"`
	DiagnosisAnalysis    json.RawMessage `json:"diagnosis_analysis"`
	CandidateAssessments json.RawMessage `json:"candidate_assessments,omitempty"`
	Profile              json.RawMessage `json:"profile,omitempty"`
	UserConstraints      json.RawMessage `json:"user_constraints,omitempty"`
	Evidence             json.RawMessage `json:"evidence,omitempty"`
	UseCase              string          `json:"use_case,omitempty"`
}

type AssessmentGenerationRequest struct {
	Profile         json.RawMessage `json:"profile"`
	RAGContext      string          `json:"rag_context,omitempty"`
	Images          []string        `json:"images,omitempty"`
	PostureAnalysis json.RawMessage `json:"posture_analysis,omitempty"`
	UseCase         string          `json:"use_case,omitempty"`
}

type AIClient struct {
	httpClient *http.Client
	baseURL    string
}

// ConsultationImageRef is a single user-attached image for multimodal turns.
// DataURL is a data: URL resolved server-side from an owned upload; never trust
// client-supplied raw URLs for model input.
type ConsultationImageRef struct {
	UploadID string `json:"upload_id,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	DataURL  string `json:"data_url"`
}

type ConsultationUserInput struct {
	Type   string                 `json:"type"`
	Text   string                 `json:"text"`
	Images []ConsultationImageRef `json:"images,omitempty"`
}

type ConsultationRuntimeState struct {
	Phase         string          `json:"phase"`
	ExtractedInfo json.RawMessage `json:"extracted_info"`
}

type ConsultationBusinessContext struct {
	Profile json.RawMessage `json:"profile"`
	// BodyState is durable user-level health truth. RuntimeState carries only
	// transient consultation orchestration state such as extraction and phase.
	BodyState    json.RawMessage          `json:"body_state,omitempty"`
	RuntimeState ConsultationRuntimeState `json:"runtime_state"`
	// RelevantHistory is a small quoted retrieval result from older messages.
	// It is never health truth and current BodyState always has precedence.
	RelevantHistory  []ConsultationHistoricalMessage `json:"relevant_history,omitempty"`
	CurrentDiagnosis json.RawMessage                 `json:"current_diagnosis,omitempty"`
	CurrentTreatment json.RawMessage                 `json:"current_treatment,omitempty"`
	RecentOutcomes   json.RawMessage                 `json:"recent_outcomes,omitempty"`
	// PostureAnalysis is the user's completed three-view analysis summary,
	// prefetched by Go so the consultation Agent tool can read it without a
	// Python→Go round trip.
	PostureAnalysis json.RawMessage `json:"posture_analysis,omitempty"`
}

type StartConsultationTurnRequest struct {
	RunID           string                      `json:"run_id"`
	ConversationID  string                      `json:"conversation_id"`
	UserID          string                      `json:"user_id"`
	Input           ConsultationUserInput       `json:"input"`
	BusinessContext ConsultationBusinessContext `json:"business_context"`
}

type ResumeConsultationInterruptRequest struct {
	RunID           string                      `json:"run_id"`
	ConversationID  string                      `json:"conversation_id"`
	UserID          string                      `json:"user_id"`
	InterruptID     string                      `json:"interrupt_id"`
	Answer          json.RawMessage             `json:"answer"`
	BusinessContext ConsultationBusinessContext `json:"business_context"`
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

func (c *AIClient) StartConsultationTurn(
	ctx context.Context,
	threadID string,
	req StartConsultationTurnRequest,
) (<-chan dto.StreamEvent, error) {
	return c.streamNDJSON(ctx, "/runtime/threads/"+threadID+"/turns", req)
}

func (c *AIClient) ResumeConsultationInterrupt(
	ctx context.Context,
	threadID string,
	interruptID string,
	req ResumeConsultationInterruptRequest,
) (<-chan dto.StreamEvent, error) {
	return c.streamNDJSON(
		ctx,
		"/runtime/threads/"+threadID+"/interrupts/"+interruptID+"/resume",
		req,
	)
}

func (c *AIClient) streamNDJSON(
	ctx context.Context,
	path string,
	req any,
) (<-chan dto.StreamEvent, error) {
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

// GenerateAssessment calls the typed observation-only Assessment Agent.
func (c *AIClient) GenerateAssessment(ctx context.Context, req AssessmentGenerationRequest) (json.RawMessage, error) {
	return c.callJSON(ctx, "/api/assessment/generate", req)
}

// AnalyzeDiagnosis calls /api/diagnosis/analyze.
func (c *AIClient) AnalyzeDiagnosis(ctx context.Context, req DiagnosisRequest) (json.RawMessage, error) {
	return c.callJSON(ctx, "/api/diagnosis/analyze", req)
}

// RecommendTreatment calls the typed Treatment Agent. The result is a proposal;
// only Go can accept it into the durable current Treatment aggregate.
func (c *AIClient) RecommendTreatment(ctx context.Context, req TreatmentRecommendationRequest) (json.RawMessage, error) {
	return c.callJSON(ctx, "/api/treatment/recommend", req)
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
