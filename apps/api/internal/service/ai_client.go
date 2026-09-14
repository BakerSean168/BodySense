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
	"strings"
	"time"
)

type DiagnosisRequest struct {
	UserID          string `json:"user_id,omitempty"`
	ConfigurationID string `json:"configuration_id"`
	// New ADR 0004 boundary: Diagnosis pins exact durable BodyState input.
	BodyStateRevision int64           `json:"body_state_revision"`
	BodyState         json.RawMessage `json:"body_state"`
	RelevantHistory   json.RawMessage `json:"relevant_history,omitempty"`
	Profile           json.RawMessage `json:"profile,omitempty"`
}

// TreatmentRecommendationRequest pins exact durable identities and returns a
// proposal that still requires explicit user acceptance in Go.
type TreatmentRecommendationRequest struct {
	UserID               string          `json:"user_id,omitempty"`
	ConfigurationID      string          `json:"configuration_id"`
	BodyStateRevision    int64           `json:"body_state_revision"`
	BodyState            json.RawMessage `json:"body_state"`
	DiagnosisAnalysis    json.RawMessage `json:"diagnosis_analysis"`
	CandidateAssessments json.RawMessage `json:"candidate_assessments,omitempty"`
	Profile              json.RawMessage `json:"profile,omitempty"`
	UserConstraints      json.RawMessage `json:"user_constraints,omitempty"`
	Evidence             json.RawMessage `json:"evidence,omitempty"`
}

type AssessmentGenerationRequest struct {
	ConfigurationID  string          `json:"configuration_id"`
	Profile          json.RawMessage `json:"profile"`
	BodyState        json.RawMessage `json:"body_state"`
	ReportIndicators json.RawMessage `json:"report_indicators,omitempty"`
	RAGContext       string          `json:"rag_context,omitempty"`
	Images           []string        `json:"images,omitempty"`
	PostureAnalysis  json.RawMessage `json:"posture_analysis,omitempty"`
	// ReviewedReportEvidence carries user-confirmed/corrected health-document
	// indicators derived from the durable review projection. It is never
	// assembled from mutating OCRResult/evidence_admissibility.
	ReviewedReportEvidence json.RawMessage `json:"reviewed_report_evidence,omitempty"`
}

type AIClient struct {
	httpClient *http.Client
	baseURL    string
}

// AIServiceHTTPError preserves upstream status semantics so application services
// can distinguish an invalid model output from transport/infrastructure failure.
type AIServiceHTTPError struct {
	StatusCode int
	Body       string
}

func (e *AIServiceHTTPError) Error() string {
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("AI service returned status %d", e.StatusCode)
	}
	return fmt.Sprintf("AI service returned status %d: %s", e.StatusCode, e.Body)
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

type ConsultationSpatialContext struct {
	BodyRegionID    string `json:"body_region_id,omitempty"`
	BodyRegionLabel string `json:"body_region_label,omitempty"`
	AnatomyID       string `json:"anatomy_id,omitempty"`
	AnatomyName     string `json:"anatomy_name,omitempty"`
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
	SpatialContext   *ConsultationSpatialContext     `json:"spatial_context,omitempty"`
	// PostureAnalysis is the user's completed three-view analysis summary,
	// prefetched by Go so the consultation Agent tool can read it without a
	// Python→Go round trip.
	PostureAnalysis json.RawMessage `json:"posture_analysis,omitempty"`
}

type StartConsultationTurnRequest struct {
	RunID           string                      `json:"run_id"`
	ConversationID  string                      `json:"conversation_id"`
	UserID          string                      `json:"user_id"`
	ConfigurationID string                      `json:"configuration_id"`
	Input           ConsultationUserInput       `json:"input"`
	BusinessContext ConsultationBusinessContext `json:"business_context"`
}

type ResumeConsultationInterruptRequest struct {
	RunID           string                      `json:"run_id"`
	ConversationID  string                      `json:"conversation_id"`
	UserID          string                      `json:"user_id"`
	ConfigurationID string                      `json:"configuration_id"`
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
) (<-chan ConsultationRuntimeEvent, error) {
	body, err := marshalStartTurnCommand(threadID, req)
	if err != nil {
		return nil, err
	}
	return c.streamNDJSON(ctx, "/runtime/threads/"+threadID+"/turns", body)
}

func (c *AIClient) ResumeConsultationInterrupt(
	ctx context.Context,
	threadID string,
	interruptID string,
	req ResumeConsultationInterruptRequest,
) (<-chan ConsultationRuntimeEvent, error) {
	body, err := marshalResumeInterruptCommand(threadID, interruptID, req)
	if err != nil {
		return nil, err
	}
	return c.streamNDJSON(
		ctx,
		"/runtime/threads/"+threadID+"/interrupts/"+interruptID+"/resume",
		body,
	)
}

func (c *AIClient) streamNDJSON(
	ctx context.Context,
	path string,
	body []byte,
) (<-chan ConsultationRuntimeEvent, error) {

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

	events := make(chan ConsultationRuntimeEvent, 32)
	go func() {
		defer resp.Body.Close()
		defer close(events)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}

			event, err := decodeConsultationRuntimeProtoEvent(line)
			if err != nil {
				sendConsultationProtocolError(ctx, events, fmt.Sprintf("invalid AI runtime Proto event: %v", err))
				return
			}

			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			sendConsultationProtocolError(ctx, events, fmt.Sprintf("AI runtime stream read failed: %v", err))
		}
	}()

	return events, nil
}

func sendConsultationProtocolError(ctx context.Context, events chan<- ConsultationRuntimeEvent, message string) {
	select {
	case events <- consultationProtocolError(message):
	case <-ctx.Done():
	}
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
	Messages        []map[string]any `json:"messages"`
	ConfigurationID string           `json:"configuration_id"`
}

// TitleGenerateResponse is the response body from /api/title/generate.
type TitleGenerateResponse struct {
	Title               string         `json:"title"`
	AgentConfiguration  map[string]any `json:"agent_configuration"`
	ExecutionProvenance map[string]any `json:"execution_provenance"`
}

// GenerateTitle calls /api/title/generate with the exact Go-selected immutable configuration.
func (c *AIClient) GenerateTitle(ctx context.Context, messages []map[string]any, configurationID string) (*TitleGenerateResponse, error) {
	req := TitleGenerateRequest{Messages: messages, ConfigurationID: configurationID}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/title/generate", bytes.NewReader(body))
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

	var result TitleGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
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
		return nil, &AIServiceHTTPError{StatusCode: resp.StatusCode, Body: string(bodyBytes)}
	}

	var result json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}
