package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

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

// ---------------------------------------------------------------------------
// Asset and knowledge endpoints. Every outbound AI-service call lives on this
// adapter so URL, client construction and response handling stay in one place.
// ---------------------------------------------------------------------------

// ExtractHealthDocument streams one already-authorized document to the bounded
// health-document runtime. Go remains the blob authority; Python receives only
// the request body, never OSS credentials or an object-store location.
func (c *AIClient) ExtractHealthDocument(reader io.Reader, mimeType, configurationID string) ([]byte, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="file"; filename="upload"`},
		"Content-Type":        {mimeType},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create form part: %w", err)
	}
	if _, err = io.Copy(part, reader); err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}
	if err := writer.WriteField("configuration_id", configurationID); err != nil {
		return nil, fmt.Errorf("failed to write OCR configuration id: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize OCR multipart body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/ocr/extract", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to health-document service: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OCR response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OCR service returned status %d", resp.StatusCode)
	}
	return respBody, nil
}

// AnalyzePosture sends a photo to the posture endpoint together with its view
// and returns the raw response body.
func (c *AIClient) AnalyzePosture(reader io.Reader, mimeType, view, configurationID string) ([]byte, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("view", view); err != nil {
		return nil, fmt.Errorf("failed to write view field: %w", err)
	}
	if strings.TrimSpace(configurationID) == "" {
		return nil, fmt.Errorf("posture configuration id is required")
	}
	if err := writer.WriteField("configuration_id", configurationID); err != nil {
		return nil, fmt.Errorf("failed to write configuration_id field: %w", err)
	}
	part, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="file"; filename="upload"`},
		"Content-Type":        {mimeType},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create form part: %w", err)
	}
	if _, err = io.Copy(part, reader); err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize posture multipart body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/posture/analyze", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to AI service: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read posture response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("posture service returned status %d", resp.StatusCode)
	}
	return respBody, nil
}

// SearchKnowledge posts one typed search and decodes the strict response
// contract. Unknown upstream fields fail closed.
func (c *AIClient) SearchKnowledge(ctx context.Context, input KnowledgeSearchRequest) (*KnowledgeSearchResponse, error) {
	if c == nil || c.httpClient == nil {
		return nil, fmt.Errorf("%w: client unavailable", ErrKnowledgeQueryUpstream)
	}
	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode knowledge search request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/knowledge/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	var response KnowledgeSearchResponse
	if err := c.decodeKnowledgeJSON(req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// KnowledgeStats reads the knowledge corpus counters.
func (c *AIClient) KnowledgeStats(ctx context.Context) (*KnowledgeStats, error) {
	if c == nil || c.httpClient == nil {
		return nil, fmt.Errorf("%w: client unavailable", ErrKnowledgeQueryUpstream)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/knowledge/stats", nil)
	if err != nil {
		return nil, err
	}
	var response KnowledgeStats
	if err := c.decodeKnowledgeJSON(req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *AIClient) decodeKnowledgeJSON(req *http.Request, target any) error {
	resp, err := c.httpClient.Do(req)
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

// IngestKnowledgeVideo posts one pinned video-ingestion command and returns the
// upstream status with the bounded raw body for typed decoding by the caller.
func (c *AIClient) IngestKnowledgeVideo(ctx context.Context, payload any) ([]byte, int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("encode knowledge ingestion request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/knowledge/ingestions/video", bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("AI knowledge ingestion returned status %d", resp.StatusCode)
	}
	return responseBody, resp.StatusCode, nil
}
