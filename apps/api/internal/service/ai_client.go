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

	"github.com/bodysense/api/internal/dto"
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
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}

			var event dto.StreamEvent
			if err := json.Unmarshal(line, &event); err != nil {
				sendConsultationProtocolError(ctx, events, fmt.Sprintf("malformed AI runtime NDJSON: %v", err))
				return
			}
			if err := validateConsultationInternalEvent(event); err != nil {
				sendConsultationProtocolError(ctx, events, fmt.Sprintf("invalid AI runtime event: %v", err))
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

var consultationInternalEventChannels = map[string]string{
	"runtime.agent_configuration":     "runtime",
	"message.text.delta":              "message",
	"tool.call":                       "tool",
	"tool.result":                     "tool",
	"state.extracted_info.upsert":     "state",
	"state.lifestyle_context.upsert":  "state",
	"state.interaction.required":      "state",
	"state.phase.changed":             "state",
	"source.citation.added":           "source",
	"source.answer_attribution.added": "source",
	"source.knowledge_gap":            "source",
	"safety.red_flag.detected":        "safety",
	"safety.output_reviewed":          "safety",
	"safety.output_rejected":          "safety",
	"usage.reported":                  "usage",
	"stream.done":                     "stream",
	"stream.error":                    "stream",
}

func sendConsultationProtocolError(ctx context.Context, events chan<- dto.StreamEvent, message string) {
	event, _ := dto.NewStreamEvent(
		1,
		"stream",
		"stream.error",
		dto.StreamEventIDs{},
		map[string]any{"message": message},
	)
	select {
	case events <- event:
	case <-ctx.Done():
	}
}

func validateCitationPayload(raw json.RawMessage) error {
	var citation struct {
		SourceType          string `json:"source_type"`
		UnitKey             string `json:"unit_key"`
		SourceKey           string `json:"source_key"`
		LifecycleStatus     string `json:"lifecycle_status"`
		PublicationID       string `json:"publication_id"`
		PublicationKey      string `json:"publication_key"`
		PublicationBatchKey string `json:"publication_batch_key"`
		PublishedVersion    int    `json:"published_version"`
		ClaimID             string `json:"claim_id"`
		ClaimReviewID       string `json:"claim_review_id"`
		SourceLocator       struct {
			LocatorType string `json:"locator_type"`
			GitCommit   string `json:"git_commit"`
			Path        string `json:"path"`
			LineStart   int    `json:"line_start"`
			LineEnd     int    `json:"line_end"`
		} `json:"source_locator"`
	}
	if err := json.Unmarshal(raw, &citation); err != nil {
		return fmt.Errorf("citation payload is malformed")
	}
	if citation.SourceType != "thought_forest_note" {
		return nil
	}
	if strings.TrimSpace(citation.UnitKey) == "" || strings.TrimSpace(citation.SourceKey) == "" ||
		citation.LifecycleStatus != "published" || strings.TrimSpace(citation.PublicationID) == "" ||
		citation.PublishedVersion <= 0 || strings.TrimSpace(citation.PublicationKey) == "" ||
		strings.TrimSpace(citation.PublicationBatchKey) == "" || strings.TrimSpace(citation.ClaimID) == "" ||
		strings.TrimSpace(citation.ClaimReviewID) == "" {
		return fmt.Errorf("published Thought Forest citation identity is incomplete")
	}
	locator := citation.SourceLocator
	if locator.LocatorType != "markdown_lines" || strings.TrimSpace(locator.GitCommit) == "" ||
		strings.TrimSpace(locator.Path) == "" || locator.LineStart <= 0 || locator.LineEnd < locator.LineStart {
		return fmt.Errorf("published Thought Forest citation provenance is incomplete")
	}
	return nil
}

func validateConsultationInternalEvent(event dto.StreamEvent) error {
	if event.Version != 1 {
		return fmt.Errorf("unsupported version %d", event.Version)
	}
	if event.Seq < 1 {
		return fmt.Errorf("seq must be >= 1")
	}
	expectedChannel, ok := consultationInternalEventChannels[event.Type]
	if !ok {
		return fmt.Errorf("unsupported event type %q", event.Type)
	}
	if event.Channel != expectedChannel {
		return fmt.Errorf("event %q must use channel %q, got %q", event.Type, expectedChannel, event.Channel)
	}
	if len(event.Payload) == 0 || !json.Valid(event.Payload) {
		return fmt.Errorf("event %q has malformed payload", event.Type)
	}

	switch event.Type {
	case "runtime.agent_configuration":
		var payload struct {
			AgentConfiguration  json.RawMessage `json:"agent_configuration"`
			ExecutionProvenance json.RawMessage `json:"execution_provenance"`
		}
		if err := event.PayloadAs(&payload); err != nil || len(payload.AgentConfiguration) == 0 || len(payload.ExecutionProvenance) == 0 {
			return fmt.Errorf("runtime Agent configuration payload is malformed")
		}
	case "message.text.delta":
		var payload struct {
			Delta *string `json:"delta"`
		}
		if err := event.PayloadAs(&payload); err != nil || payload.Delta == nil {
			return fmt.Errorf("message delta payload is malformed")
		}
	case "tool.call":
		var payload struct {
			Tool string          `json:"tool"`
			Args json.RawMessage `json:"args"`
		}
		if err := event.PayloadAs(&payload); err != nil || strings.TrimSpace(payload.Tool) == "" || len(payload.Args) == 0 {
			return fmt.Errorf("tool.call payload is malformed")
		}
	case "tool.result":
		var payload struct {
			Tool   string          `json:"tool"`
			Result json.RawMessage `json:"result"`
		}
		if err := event.PayloadAs(&payload); err != nil || strings.TrimSpace(payload.Tool) == "" || len(payload.Result) == 0 {
			return fmt.Errorf("tool.result payload is malformed")
		}
	case "state.extracted_info.upsert":
		var payload struct {
			Info json.RawMessage `json:"info"`
		}
		if err := event.PayloadAs(&payload); err != nil || len(payload.Info) == 0 {
			return fmt.Errorf("extracted-info payload is malformed")
		}
	case "state.lifestyle_context.upsert":
		var payload struct {
			Context json.RawMessage `json:"context"`
		}
		if err := event.PayloadAs(&payload); err != nil || len(payload.Context) == 0 {
			return fmt.Errorf("lifestyle-context payload is malformed")
		}
	case "state.interaction.required":
		var payload struct {
			InteractionID string          `json:"interaction_id"`
			Question      json.RawMessage `json:"question"`
		}
		if err := event.PayloadAs(&payload); err != nil || strings.TrimSpace(payload.InteractionID) == "" || len(payload.Question) == 0 {
			return fmt.Errorf("interaction-required payload is malformed")
		}
	case "state.phase.changed":
		var payload struct {
			To     string `json:"to"`
			Reason string `json:"reason"`
		}
		if err := event.PayloadAs(&payload); err != nil || strings.TrimSpace(payload.To) == "" || strings.TrimSpace(payload.Reason) == "" {
			return fmt.Errorf("phase-change payload is malformed")
		}
	case "source.citation.added":
		var payload struct {
			Citation json.RawMessage `json:"citation"`
		}
		if err := event.PayloadAs(&payload); err != nil || len(payload.Citation) == 0 {
			return fmt.Errorf("citation payload is malformed")
		}
		if err := validateCitationPayload(payload.Citation); err != nil {
			return err
		}
	case "source.answer_attribution.added":
		if _, err := ParseConsultationAnswerAttributionPayload(event.Payload); err != nil {
			return err
		}
	case "source.knowledge_gap":
		var payload struct {
			Query   string `json:"query"`
			Message string `json:"message"`
		}
		if err := event.PayloadAs(&payload); err != nil || strings.TrimSpace(payload.Query) == "" || strings.TrimSpace(payload.Message) == "" {
			return fmt.Errorf("knowledge-gap payload is malformed")
		}
	case "safety.red_flag.detected":
		var raw map[string]json.RawMessage
		if err := event.PayloadAs(&raw); err != nil || raw["has_red_flags"] == nil || raw["flags"] == nil {
			return fmt.Errorf("red-flag payload is malformed")
		}
		var hasRedFlags bool
		var flags []any
		if err := json.Unmarshal(raw["has_red_flags"], &hasRedFlags); err != nil {
			return fmt.Errorf("red-flag has_red_flags must be boolean")
		}
		if err := json.Unmarshal(raw["flags"], &flags); err != nil {
			return fmt.Errorf("red-flag flags must be an array")
		}
	case "safety.output_reviewed", "safety.output_rejected":
		var payload struct {
			Kind    string `json:"kind"`
			Verdict string `json:"verdict"`
		}
		if err := event.PayloadAs(&payload); err != nil || strings.TrimSpace(payload.Kind) == "" || strings.TrimSpace(payload.Verdict) == "" {
			return fmt.Errorf("safety review payload is malformed")
		}
	case "usage.reported":
		var payload struct {
			Usage json.RawMessage `json:"usage"`
		}
		if err := event.PayloadAs(&payload); err != nil || len(payload.Usage) == 0 {
			return fmt.Errorf("usage payload is malformed")
		}
	case "stream.error":
		var payload struct {
			Message string `json:"message"`
		}
		if err := event.PayloadAs(&payload); err != nil || strings.TrimSpace(payload.Message) == "" {
			return fmt.Errorf("stream.error payload is malformed")
		}
	}
	return nil
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
