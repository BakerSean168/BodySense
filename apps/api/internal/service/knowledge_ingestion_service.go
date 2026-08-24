package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	KnowledgeIngestVideoJobType = "knowledge.ingest_video"
	knowledgeIngestMaxAttempts  = 3
)

var (
	ErrKnowledgeIngestionSourceMismatch = errors.New("knowledge ingestion source identity mismatch")
	ErrKnowledgeIngestionNotFound       = errors.New("knowledge ingestion job not found")
)

type knowledgeIngestionDeployment interface {
	KnowledgeCuratorConfigurationID() string
	KnowledgeCuratorDecisionPolicyRevision() string
	KnowledgeCuratorLogicalModel() string
	KnowledgeSplitterConfigurationID() string
	KnowledgeSplitterDecisionPolicyRevision() string
	KnowledgeSplitterLogicalModel() string
}

// KnowledgeVideoIngestionRequest contains operator-selectable execution knobs.
// Source identity and Agent configuration are resolved server-side.
type KnowledgeVideoIngestionRequest struct {
	SourceKey          string
	VideoPath          string
	TranscriptProvider string
	TranscriptModel    string
	WhisperModel       string
	ForceTranscribe    bool
	ExportClips        bool
	SplitterProvider   string
	AIRefine           bool
}

type knowledgeVideoJobInput struct {
	SourceID           int64  `json:"source_id"`
	SourceKey          string `json:"source_key"`
	SourceVersion      string `json:"source_version"`
	ContentHash        string `json:"content_hash"`
	SourceType         string `json:"source_type"`
	Title              string `json:"title"`
	Author             string `json:"author"`
	ProblemSlug        string `json:"problem_slug"`
	ProblemDisplayName string `json:"problem_display_name"`
	OriginalFilePath   string `json:"original_file_path"`
	Language           string `json:"language"`
	OperatorID         string `json:"operator_id"`

	TranscriptProvider string `json:"transcript_provider"`
	TranscriptModel    string `json:"transcript_model,omitempty"`
	WhisperModel       string `json:"whisper_model"`
	ForceTranscribe    bool   `json:"force_transcribe"`
	ExportClips        bool   `json:"export_clips"`
	SplitterProvider   string `json:"splitter_provider"`
	AIRefine           bool   `json:"ai_refine"`

	SplitterConfigurationID string `json:"splitter_configuration_id,omitempty"`
	SplitterDecisionPolicy  string `json:"splitter_decision_policy_revision,omitempty"`
	SplitterLogicalModel    string `json:"splitter_logical_model,omitempty"`
	CuratorConfigurationID  string `json:"curator_configuration_id,omitempty"`
	CuratorDecisionPolicy   string `json:"curator_decision_policy_revision,omitempty"`
	CuratorLogicalModel     string `json:"curator_logical_model,omitempty"`
}

type knowledgeIngestResponse struct {
	SourceID           *int64                    `json:"source_id"`
	SourceKey          string                    `json:"source_key"`
	Status             string                    `json:"status"`
	ArtifactDir        string                    `json:"artifact_dir"`
	TranscriptSegments int                       `json:"transcript_segments"`
	KnowledgeUnits     int                       `json:"knowledge_units"`
	Clips              int                       `json:"clips"`
	AgentExecution     map[string]agentExecution `json:"agent_execution"`
}

type agentExecution struct {
	AgentConfiguration  map[string]any `json:"agent_configuration"`
	ExecutionProvenance map[string]any `json:"execution_provenance"`
}

type knowledgeJobRuntime interface {
	CreateJobWithIdempotencyAttempts(context.Context, uuid.UUID, string, datatypes.JSON, string, int, *uuid.UUID, *uuid.UUID) (*model.Job, bool, error)
	GetJob(context.Context, uuid.UUID) (*model.Job, error)
	ListRecoverable(context.Context, string, time.Duration, int) ([]model.Job, error)
	ClaimPending(context.Context, uuid.UUID) (*model.Job, bool, error)
	UpdateProgress(context.Context, uuid.UUID, any) error
	TransitionTo(context.Context, uuid.UUID, string, any, any) error
}

type KnowledgeIngestionService struct {
	registry     *KnowledgeSourceRegistry
	jobs         knowledgeJobRuntime
	deployment   knowledgeIngestionDeployment
	aiServiceURL string
	httpClient   *http.Client
}

func NewKnowledgeIngestionService(
	registry *KnowledgeSourceRegistry,
	jobs knowledgeJobRuntime,
	deployment knowledgeIngestionDeployment,
	aiServiceURL string,
) *KnowledgeIngestionService {
	if strings.TrimSpace(aiServiceURL) == "" {
		aiServiceURL = "http://localhost:8100"
	}
	return &KnowledgeIngestionService{
		registry:     registry,
		jobs:         jobs,
		deployment:   deployment,
		aiServiceURL: strings.TrimRight(aiServiceURL, "/"),
		httpClient:   &http.Client{Timeout: 12 * time.Minute},
	}
}

func (s *KnowledgeIngestionService) EnqueueVideo(
	ctx context.Context,
	operatorID uuid.UUID,
	req KnowledgeVideoIngestionRequest,
) (*model.Job, bool, error) {
	if s == nil || s.registry == nil || s.jobs == nil || s.deployment == nil {
		return nil, false, errors.New("knowledge ingestion service is unavailable")
	}
	source, err := s.registry.FindIngestible(ctx, strings.TrimSpace(req.SourceKey))
	if err != nil {
		return nil, false, err
	}
	if source.SourceType != "video" {
		return nil, false, fmt.Errorf("%w: registered source is not video", ErrKnowledgeIngestionSourceMismatch)
	}
	if strings.TrimSpace(req.VideoPath) != "" && cleanKnowledgePath(req.VideoPath) != cleanKnowledgePath(source.OriginalFilePath) {
		return nil, false, fmt.Errorf("%w: video path differs from registered source", ErrKnowledgeIngestionSourceMismatch)
	}
	if source.ContentHash == nil || source.RegisteredBy == nil || source.RegisteredAt == nil {
		return nil, false, ErrKnowledgeSourceNotReady
	}

	input := knowledgeVideoJobInput{
		SourceID:           source.ID,
		SourceKey:          source.SourceKey,
		SourceVersion:      source.SourceVersion,
		ContentHash:        strings.ToLower(*source.ContentHash),
		SourceType:         source.SourceType,
		Title:              source.Title,
		Author:             source.Author,
		ProblemSlug:        source.ProblemSlug,
		ProblemDisplayName: source.ProblemDisplayName,
		OriginalFilePath:   source.OriginalFilePath,
		Language:           source.Language,
		OperatorID:         operatorID.String(),
		TranscriptProvider: defaultString(req.TranscriptProvider, "whisper.cpp"),
		TranscriptModel:    strings.TrimSpace(req.TranscriptModel),
		WhisperModel:       defaultString(req.WhisperModel, "ggml-base.bin"),
		ForceTranscribe:    req.ForceTranscribe,
		ExportClips:        req.ExportClips,
		SplitterProvider:   defaultString(req.SplitterProvider, "heuristic"),
		AIRefine:           req.AIRefine,
	}
	if input.SplitterProvider != "heuristic" && input.SplitterProvider != "llm" {
		return nil, false, fmt.Errorf("unsupported splitter provider %q", input.SplitterProvider)
	}
	if input.SplitterProvider == "llm" {
		input.SplitterConfigurationID = s.deployment.KnowledgeSplitterConfigurationID()
		input.SplitterDecisionPolicy = s.deployment.KnowledgeSplitterDecisionPolicyRevision()
		input.SplitterLogicalModel = s.deployment.KnowledgeSplitterLogicalModel()
	}
	if input.AIRefine {
		input.CuratorConfigurationID = s.deployment.KnowledgeCuratorConfigurationID()
		input.CuratorDecisionPolicy = s.deployment.KnowledgeCuratorDecisionPolicyRevision()
		input.CuratorLogicalModel = s.deployment.KnowledgeCuratorLogicalModel()
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, false, fmt.Errorf("encode knowledge ingestion input: %w", err)
	}
	// Idempotency belongs to the immutable source+pipeline execution, not to the
	// operator who happened to request it. The first creator remains the audit actor.
	fingerprintInput := input
	fingerprintInput.OperatorID = ""
	fingerprintJSON, err := json.Marshal(fingerprintInput)
	if err != nil {
		return nil, false, fmt.Errorf("encode knowledge ingestion fingerprint: %w", err)
	}
	fingerprint := sha256.Sum256(fingerprintJSON)
	idempotencyKey := fmt.Sprintf("%s:%d:%s", KnowledgeIngestVideoJobType, source.ID, hex.EncodeToString(fingerprint[:]))
	return s.jobs.CreateJobWithIdempotencyAttempts(
		ctx,
		operatorID,
		KnowledgeIngestVideoJobType,
		inputJSON,
		idempotencyKey,
		knowledgeIngestMaxAttempts,
		nil,
		nil,
	)
}

func (s *KnowledgeIngestionService) GetJob(ctx context.Context, jobID uuid.UUID) (*model.Job, error) {
	job, err := s.jobs.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job == nil || job.JobType != KnowledgeIngestVideoJobType {
		return nil, ErrKnowledgeIngestionNotFound
	}
	return job, nil
}

func (s *KnowledgeIngestionService) StartWorker(ctx context.Context, interval, staleRunningAfter time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if staleRunningAfter <= 0 {
		staleRunningAfter = 15 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if _, err := s.RecoverJobs(ctx, 10, staleRunningAfter); err != nil {
				log.Printf("knowledge ingestion recovery failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *KnowledgeIngestionService) RecoverJobs(ctx context.Context, limit int, staleRunningAfter time.Duration) (int, error) {
	jobs, err := s.jobs.ListRecoverable(ctx, KnowledgeIngestVideoJobType, staleRunningAfter, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, job := range jobs {
		switch job.Status {
		case "pending":
			if err := s.processPending(ctx, job.ID); err != nil {
				log.Printf("knowledge ingestion job %s processing failed: %v", job.ID, err)
			}
			processed++
		case "running":
			if job.Attempts < job.MaxAttempts {
				_ = s.jobs.UpdateProgress(ctx, job.ID, map[string]any{
					"stage": "retry_pending", "reason": "stale_execution", "attempt": job.Attempts,
				})
				if err := s.jobs.TransitionTo(ctx, job.ID, "pending", nil, nil); err != nil {
					return processed, err
				}
			} else if err := s.jobs.TransitionTo(ctx, job.ID, "timed_out", nil, map[string]any{
				"code": "stale_execution", "attempts": job.Attempts,
			}); err != nil {
				return processed, err
			}
			processed++
		}
	}
	return processed, nil
}

func (s *KnowledgeIngestionService) processPending(ctx context.Context, jobID uuid.UUID) error {
	job, claimed, err := s.jobs.ClaimPending(ctx, jobID)
	if err != nil || !claimed {
		return err
	}
	var input knowledgeVideoJobInput
	if err := json.Unmarshal(job.Input, &input); err != nil {
		return s.failJob(ctx, job, "invalid_job_input", false, err)
	}
	if input.OperatorID != job.UserID.String() {
		return s.failJob(ctx, job, "operator_identity_mismatch", false, ErrKnowledgeIngestionSourceMismatch)
	}
	if err := s.validatePinnedSource(ctx, input); err != nil {
		return s.failJob(ctx, job, "source_identity_mismatch", false, err)
	}
	_ = s.jobs.UpdateProgress(ctx, job.ID, map[string]any{
		"stage": "ingesting", "percent": 10, "attempt": job.Attempts,
	})

	response, status, callErr := s.executeVideo(ctx, input)
	if callErr != nil {
		retriable := status == 0 || status >= http.StatusInternalServerError
		return s.failJob(ctx, job, "ai_ingestion_failed", retriable, callErr)
	}
	if err := validateKnowledgeAgentExecution(response.AgentExecution, input); err != nil {
		return s.failJob(ctx, job, "agent_identity_mismatch", false, err)
	}
	if response.SourceKey != input.SourceKey || (response.SourceID != nil && *response.SourceID != input.SourceID) {
		return s.failJob(ctx, job, "source_result_mismatch", false, ErrKnowledgeIngestionSourceMismatch)
	}
	result, _ := json.Marshal(response)
	_ = s.jobs.UpdateProgress(ctx, job.ID, map[string]any{
		"stage": "ingested", "percent": 100, "attempt": job.Attempts,
	})
	if err := s.jobs.TransitionTo(ctx, job.ID, "completed", json.RawMessage(result), nil); err != nil {
		return err
	}
	return nil
}

func (s *KnowledgeIngestionService) validatePinnedSource(ctx context.Context, input knowledgeVideoJobInput) error {
	source, err := s.registry.FindByKey(ctx, input.SourceKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrKnowledgeIngestionSourceMismatch
		}
		return err
	}
	if source.ID != input.SourceID || source.SourceVersion != input.SourceVersion || source.ContentHash == nil ||
		!strings.EqualFold(*source.ContentHash, input.ContentHash) || source.SourceType != input.SourceType ||
		source.OriginalFilePath != input.OriginalFilePath || source.Title != input.Title || source.Author != input.Author ||
		source.ProblemSlug != input.ProblemSlug || source.ProblemDisplayName != input.ProblemDisplayName ||
		source.Language != input.Language || (source.IngestStatus != "registered" && source.IngestStatus != "ingested") {
		return ErrKnowledgeIngestionSourceMismatch
	}
	return nil
}

func (s *KnowledgeIngestionService) executeVideo(ctx context.Context, input knowledgeVideoJobInput) (knowledgeIngestResponse, int, error) {
	payload := map[string]any{
		"source_key": input.SourceKey, "expected_content_hash": input.ContentHash,
		"video_path": input.OriginalFilePath, "problem_slug": input.ProblemSlug,
		"problem_display_name": input.ProblemDisplayName, "author": input.Author,
		"source_title": input.Title, "language": input.Language,
		"transcript_provider": input.TranscriptProvider, "whisper_model": input.WhisperModel,
		"force_transcribe": input.ForceTranscribe, "export_clips": input.ExportClips,
		"overwrite_source": false, "splitter_provider": input.SplitterProvider,
		"ai_refine": input.AIRefine,
	}
	if input.TranscriptModel != "" {
		payload["transcript_model"] = input.TranscriptModel
	}
	if input.SplitterConfigurationID != "" {
		payload["splitter_configuration_id"] = input.SplitterConfigurationID
	}
	if input.CuratorConfigurationID != "" {
		payload["curator_configuration_id"] = input.CuratorConfigurationID
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.aiServiceURL+"/api/knowledge/ingestions/video", bytes.NewReader(body))
	if err != nil {
		return knowledgeIngestResponse{}, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return knowledgeIngestResponse{}, 0, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return knowledgeIngestResponse{}, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return knowledgeIngestResponse{}, resp.StatusCode, fmt.Errorf("AI knowledge ingestion returned status %d", resp.StatusCode)
	}
	var decoded knowledgeIngestResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return knowledgeIngestResponse{}, resp.StatusCode, fmt.Errorf("decode AI knowledge ingestion response: %w", err)
	}
	return decoded, resp.StatusCode, nil
}

func (s *KnowledgeIngestionService) failJob(ctx context.Context, job *model.Job, code string, retriable bool, cause error) error {
	if retriable && job.Attempts < job.MaxAttempts {
		_ = s.jobs.UpdateProgress(ctx, job.ID, map[string]any{
			"stage": "retry_pending", "code": code, "attempt": job.Attempts,
		})
		if err := s.jobs.TransitionTo(ctx, job.ID, "pending", nil, nil); err != nil {
			return err
		}
		return cause
	}
	if err := s.jobs.TransitionTo(ctx, job.ID, "failed", nil, map[string]any{
		"code": code, "attempts": job.Attempts,
	}); err != nil {
		return err
	}
	return cause
}

func validateKnowledgeAgentExecution(execution map[string]agentExecution, input knowledgeVideoJobInput) error {
	check := func(key, expectedID, expectedRole, expectedPolicy, expectedModel string) error {
		record, ok := execution[key]
		if !ok {
			return fmt.Errorf("missing %s execution record", key)
		}
		id, _ := record.AgentConfiguration["id"].(string)
		role, _ := record.AgentConfiguration["role"].(string)
		policy, _ := record.AgentConfiguration["decision_policy_revision"].(string)
		logicalModel, _ := record.AgentConfiguration["logical_model"].(string)
		executionStatus, _ := record.ExecutionProvenance["status"].(string)
		executionModel, _ := record.ExecutionProvenance["logical_model"].(string)
		if id != expectedID || role != expectedRole || policy != expectedPolicy || logicalModel != expectedModel {
			return fmt.Errorf("%s immutable configuration mismatch", key)
		}
		if (executionStatus != "executed" && executionStatus != "degraded") || executionModel != expectedModel {
			return fmt.Errorf("%s execution provenance mismatch", key)
		}
		return nil
	}
	if input.SplitterProvider == "llm" {
		if err := check("knowledge_splitter", input.SplitterConfigurationID, "knowledge_splitter", input.SplitterDecisionPolicy, input.SplitterLogicalModel); err != nil {
			return err
		}
	}
	if input.AIRefine {
		if err := check("knowledge_curator", input.CuratorConfigurationID, "knowledge_curator", input.CuratorDecisionPolicy, input.CuratorLogicalModel); err != nil {
			return err
		}
	}
	return nil
}

func defaultString(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func cleanKnowledgePath(value string) string {
	return strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "./")
}
