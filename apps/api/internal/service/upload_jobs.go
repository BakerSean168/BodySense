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
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// executeOCRCall streams an already-authorized asset to the bounded
// health-document runtime. Go remains the blob authority; Python receives only
// the request body, never OSS credentials or an object-store location.
func (s *UploadService) executeOCRCall(reader io.Reader, mimeType, configurationID string) ([]byte, error) {
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

	baseURL := s.healthDocumentServiceURL
	if baseURL == "" {
		baseURL = s.aiServiceURL
	}
	resp, err := http.Post(baseURL+"/api/ocr/extract", writer.FormDataContentType(), body)
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

type ocrJobInput struct {
	AssetID         string `json:"asset_id"`
	ConfigurationID string `json:"configuration_id,omitempty"`
}

// StartUploadWorker starts a background worker that recovers and processes
// both OCR and posture-analysis jobs. A single loop drives every
// asset-derived AI job type.
func (s *UploadService) StartUploadWorker(ctx context.Context, pollInterval, staleRunningAfter time.Duration) {
	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			if _, err := s.RecoverUploadJobs(ctx, 10, staleRunningAfter); err != nil {
				log.Printf("upload job recovery failed: %v", err)
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// uploadJobType describes how a recoverable asset job is processed and timed
// out. Adding a new asset-derived AI job type is a matter of registering one
// entry here.
type uploadJobType struct {
	name    string
	process func(ctx context.Context, job model.Job) error
	timeout func(ctx context.Context, job model.Job) error
}

func (s *UploadService) uploadJobTypes() []uploadJobType {
	return []uploadJobType{
		{name: ocrJobType, process: s.processOCRJob, timeout: s.timeoutOCRJob},
		{name: postureJobType, process: s.processPostureJob, timeout: s.timeoutPostureJob},
	}
}

// RecoverUploadJobs processes pending jobs and times out stale running jobs
// across every registered asset job type.
func (s *UploadService) RecoverUploadJobs(ctx context.Context, limit int, staleRunningAfter time.Duration) (int, error) {
	if s.jobRuntime == nil {
		return 0, nil
	}

	processed := 0
	for _, jt := range s.uploadJobTypes() {
		jobs, err := s.jobRuntime.ListRecoverable(ctx, jt.name, staleRunningAfter, limit)
		if err != nil {
			return processed, err
		}
		for _, job := range jobs {
			switch job.Status {
			case "pending":
				if err := jt.process(ctx, job); err != nil {
					log.Printf("failed to process %s job %s: %v", jt.name, job.ID, err)
				}
				processed++
			case "running":
				if err := jt.timeout(ctx, job); err != nil {
					log.Printf("failed to timeout stale %s job %s: %v", jt.name, job.ID, err)
				}
				processed++
			}
		}
	}
	return processed, nil
}

func (s *UploadService) enqueueOCRJob(ctx context.Context, assetID, userID uuid.UUID) (*model.Job, bool, error) {
	if s.jobRuntime == nil {
		return nil, false, errors.New("job runtime is not configured")
	}
	idempotencyKey := fmt.Sprintf("upload_ocr:%s", assetID.String())
	configurationID := s.resolveHealthDocumentConfigurationID()
	inputJSON, _ := json.Marshal(ocrJobInput{AssetID: assetID.String(), ConfigurationID: configurationID})
	job, existed, err := s.jobRuntime.CreateJobWithIdempotency(ctx, userID, ocrJobType, inputJSON, idempotencyKey, nil, nil)
	if err != nil {
		return nil, false, err
	}
	return job, existed, nil
}

// processOCRJob advances one derived extraction run pending -> processing ->
// completed, persisting the full extraction audit. The private original is
// streamed through the Go storage boundary and hashed on the fly; the result
// is accepted only after the health-document response contract validates.
func (s *UploadService) processOCRJob(ctx context.Context, job model.Job) error {
	input, err := parseOCRJobInput(job)
	if err != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, model.JobStatusFailed, nil, map[string]string{"error": err.Error()})
		return err
	}
	assetID, err := uuid.Parse(input.AssetID)
	if err != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, model.JobStatusFailed, nil, map[string]string{"error": "invalid asset_id"})
		return fmt.Errorf("invalid asset_id: %w", err)
	}
	asset, err := s.assets.GetOwnedByID(ctx, assetID, job.UserID)
	if err != nil {
		return fmt.Errorf("load OCR asset: %w", err)
	}
	if asset == nil || asset.UserID != job.UserID {
		err := errors.New("OCR asset is missing or not owned by job user")
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, model.JobStatusFailed, nil, map[string]string{"error": err.Error()})
		return err
	}
	if asset.Availability != model.UserAssetAvailable {
		return s.finalizeOCRFailure(ctx, job, assetID, model.JobStatusFailed, ErrUploadSourceUnavailable,
			json.RawMessage(`{"error":"upload source unavailable"}`))
	}
	run, err := s.extractionRuns.GetLatestOwnedByAsset(ctx, assetID, job.UserID)
	if err != nil {
		return fmt.Errorf("load extraction run: %w", err)
	}
	if run == nil {
		err := errors.New("report asset has no extraction run")
		return s.finalizeOCRFailure(ctx, job, assetID, model.JobStatusFailed, err,
			json.RawMessage(`{"error":"extraction run missing"}`))
	}

	if err := s.jobRuntime.TransitionTo(ctx, job.ID, model.JobStatusRunning, nil, nil); err != nil {
		return fmt.Errorf("start OCR job: %w", err)
	}
	_ = s.jobRuntime.UpdateProgress(ctx, job.ID, map[string]any{"stage": "ocr_processing", "percent": 10})
	started, err := s.extractionRuns.BeginProcessing(ctx, run.ID, job.UserID, job.ID)
	if err != nil {
		return fmt.Errorf("begin OCR extraction lifecycle: %w", err)
	}
	if !started {
		return fmt.Errorf("begin OCR extraction lifecycle: terminal or competing transition already won")
	}

	reader, _, err := openUploadObject(ctx, s.storage, asset)
	if err != nil {
		return s.finalizeOCRFailure(ctx, job, assetID, model.JobStatusFailed, err, json.RawMessage(`{"error":"upload object unavailable"}`))
	}
	documentHasher := sha256.New()
	respBody, callErr := s.executeOCRCall(io.TeeReader(reader, documentHasher), asset.MimeType, input.ConfigurationID)
	closeErr := reader.Close()
	if callErr == nil && closeErr != nil {
		callErr = closeErr
	}
	if callErr != nil {
		errPayload, _ := json.Marshal(map[string]string{"error": callErr.Error()})
		return s.finalizeOCRFailure(ctx, job, assetID, model.JobStatusFailed, callErr, errPayload)
	}
	validatedBody, err := validateHealthDocumentResponse(respBody, input.ConfigurationID)
	if err != nil {
		errPayload, _ := json.Marshal(map[string]string{"error": "health-document response validation failed"})
		return s.finalizeOCRFailure(ctx, job, assetID, model.JobStatusFailed, err, errPayload)
	}

	documentSHA256 := hex.EncodeToString(documentHasher.Sum(nil))
	completion, err := buildDocumentExtractionRun(
		validatedBody, documentSHA256, assetID, job.UserID, job.ID, input.ConfigurationID,
	)
	if err != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, model.JobStatusFailed, nil, map[string]string{"error": err.Error()})
		return err
	}
	completion.ID = run.ID
	completion.UserID = job.UserID
	completed, err := s.extractionRuns.CompleteRun(ctx, completion)
	if err != nil {
		cause := fmt.Errorf("persist document extraction audit run: %w", err)
		errPayload, _ := json.Marshal(map[string]string{"error": "persist document extraction audit run"})
		return s.finalizeOCRFailure(ctx, job, assetID, model.JobStatusFailed, cause, errPayload)
	}
	if !completed {
		cause := errors.New("persist document extraction audit run: competing terminal transition already won")
		errPayload, _ := json.Marshal(map[string]string{"error": "extraction run terminal transition lost"})
		return s.finalizeOCRFailure(ctx, job, assetID, model.JobStatusFailed, cause, errPayload)
	}

	_ = s.jobRuntime.UpdateProgress(ctx, job.ID, map[string]any{"stage": "ocr_completed", "percent": 100})
	if err := s.jobRuntime.TransitionTo(ctx, job.ID, model.JobStatusCompleted, json.RawMessage(validatedBody), nil); err != nil {
		return fmt.Errorf("complete OCR job lifecycle: %w", err)
	}
	return nil
}

func (s *UploadService) timeoutOCRJob(ctx context.Context, job model.Job) error {
	input, err := parseOCRJobInput(job)
	if err != nil {
		return s.jobRuntime.TransitionTo(ctx, job.ID, model.JobStatusTimedOut, nil, map[string]string{"error": err.Error()})
	}
	assetID, parseErr := uuid.Parse(input.AssetID)
	if parseErr != nil {
		return s.jobRuntime.TransitionTo(ctx, job.ID, model.JobStatusTimedOut, nil, map[string]string{"error": "stale OCR job timed out"})
	}
	return s.finalizeOCRFailure(ctx, job, assetID, model.JobStatusTimedOut, nil, json.RawMessage(`{"error":"stale OCR job timed out"}`))
}

// finalizeOCRFailure terminates the job and moves the derived extraction run
// of the asset to failed. Terminal runs are never reopened by late failures.
func (s *UploadService) finalizeOCRFailure(
	ctx context.Context,
	job model.Job,
	assetID uuid.UUID,
	jobStatus model.JobStatus,
	cause error,
	payload json.RawMessage,
) error {
	message := "OCR processing failed"
	if cause != nil {
		message = cause.Error()
	} else if jobStatus == model.JobStatusTimedOut {
		message = "stale OCR job timed out"
	}
	if err := s.jobRuntime.TransitionTo(ctx, job.ID, jobStatus, nil, map[string]string{"error": message}); err != nil {
		return fmt.Errorf("finalize OCR job lifecycle: %w", err)
	}
	if s.extractionRuns != nil {
		run, err := s.extractionRuns.GetLatestOwnedByAsset(ctx, assetID, job.UserID)
		if err != nil {
			return fmt.Errorf("fail OCR extraction lifecycle: %w", err)
		}
		if run != nil {
			updated, err := s.extractionRuns.FailRun(ctx, run.ID, job.UserID, payload)
			if err != nil {
				return fmt.Errorf("fail OCR extraction lifecycle: %w", err)
			}
			if !updated {
				return fmt.Errorf("fail OCR extraction lifecycle: competing terminal transition already won")
			}
		}
	}
	return cause
}

func parseOCRJobInput(job model.Job) (ocrJobInput, error) {
	var input ocrJobInput
	if err := json.Unmarshal(job.Input, &input); err != nil {
		return input, fmt.Errorf("parse OCR job input: %w", err)
	}
	if input.AssetID == "" {
		return input, fmt.Errorf("OCR job input missing asset_id")
	}
	if input.ConfigurationID == "" {
		// Jobs created before health-document configuration pinning retain the
		// exact Tesseract behavior they were enqueued under.
		input.ConfigurationID = tesseractChampionConfigurationID
	}
	if _, ok := knownHealthDocumentConfigurations[input.ConfigurationID]; !ok {
		return input, fmt.Errorf("unknown health-document configuration id %q", input.ConfigurationID)
	}
	return input, nil
}

// ---------------------------------------------------------------------------
// Posture analysis pipeline (mirrors the OCR pipeline above)
// ---------------------------------------------------------------------------

type postureJobInput struct {
	AssetID         string `json:"asset_id"`
	View            string `json:"view"` // "front" | "side" | "back"
	ConfigurationID string `json:"configuration_id,omitempty"`
}

// executePostureCall sends a photo to the AI service posture endpoint together
// with its view and returns the raw response body.
func (s *UploadService) executePostureCall(reader io.Reader, mimeType, view, configurationID string) ([]byte, error) {
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
	resp, err := http.Post(s.aiServiceURL+"/api/posture/analyze", writer.FormDataContentType(), body)
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

func (s *UploadService) enqueuePostureJob(ctx context.Context, assetID, userID uuid.UUID, fileType string) (*model.Job, bool, error) {
	kind, photoView, known := model.UserAssetKindFromFileType(fileType)
	if !known || kind != model.UserAssetKindPhoto || photoView == "" {
		return nil, false, fmt.Errorf("unsupported posture photo file_type: %s", fileType)
	}
	if s.jobRuntime == nil {
		return nil, false, errors.New("job runtime is not configured")
	}
	if s.deployment == nil {
		return nil, false, errors.New("posture deployment policy is not configured")
	}
	idempotencyKey := fmt.Sprintf("posture_analyze:%s", assetID.String())
	inputJSON, _ := json.Marshal(postureJobInput{
		AssetID:         assetID.String(),
		View:            string(photoView),
		ConfigurationID: s.deployment.PostureConfigurationID(),
	})
	job, existed, err := s.jobRuntime.CreateJobWithIdempotency(ctx, userID, postureJobType, inputJSON, idempotencyKey, nil, nil)
	if err != nil {
		return nil, false, err
	}
	return job, existed, nil
}

// processPostureJob advances the derived posture run of one photo asset
// pending -> processing -> completed, persisting the validated analysis
// payload and its immutable Agent configuration identity.
func (s *UploadService) processPostureJob(ctx context.Context, job model.Job) error {
	input, err := parsePostureJobInput(job)
	if err != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, model.JobStatusFailed, nil, map[string]string{"error": err.Error()})
		return err
	}
	assetID, err := uuid.Parse(input.AssetID)
	if err != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, model.JobStatusFailed, nil, map[string]string{"error": "invalid asset_id"})
		return fmt.Errorf("invalid asset_id: %w", err)
	}
	asset, err := s.assets.GetOwnedByID(ctx, assetID, job.UserID)
	if err != nil {
		return fmt.Errorf("load posture asset: %w", err)
	}
	if asset == nil || asset.UserID != job.UserID {
		err := errors.New("posture asset is missing or not owned by job user")
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, model.JobStatusFailed, nil, map[string]string{"error": err.Error()})
		return err
	}
	if asset.Availability != model.UserAssetAvailable {
		return s.finalizeAnalysisFailure(ctx, job, assetID, model.JobStatusFailed, ErrUploadSourceUnavailable,
			json.RawMessage(`{"error":"upload source unavailable"}`))
	}
	run, err := s.postureRuns.GetOwnedByAsset(ctx, assetID, job.UserID)
	if err != nil {
		return fmt.Errorf("load posture run: %w", err)
	}
	if run == nil {
		err := errors.New("photo asset has no posture run")
		return s.finalizeAnalysisFailure(ctx, job, assetID, model.JobStatusFailed, err,
			json.RawMessage(`{"error":"posture run missing"}`))
	}
	if err := s.jobRuntime.TransitionTo(ctx, job.ID, model.JobStatusRunning, nil, nil); err != nil {
		return fmt.Errorf("start posture job: %w", err)
	}
	_ = s.jobRuntime.UpdateProgress(ctx, job.ID, map[string]any{"stage": "posture_analyzing", "percent": 10})
	started, err := s.postureRuns.BeginProcessing(ctx, run.ID, job.UserID, job.ID)
	if err != nil {
		return fmt.Errorf("begin posture run lifecycle: %w", err)
	}
	if !started {
		return fmt.Errorf("begin posture run lifecycle: terminal or competing transition already won")
	}

	reader, _, err := openUploadObject(ctx, s.storage, asset)
	if err != nil {
		errPayload, _ := json.Marshal(map[string]string{"error": "upload object unavailable"})
		return s.finalizeAnalysisFailure(ctx, job, assetID, model.JobStatusFailed, err, errPayload)
	}
	respBody, callErr := s.executePostureCall(reader, asset.MimeType, input.View, input.ConfigurationID)
	closeErr := reader.Close()
	if callErr == nil && closeErr != nil {
		callErr = closeErr
	}
	if callErr != nil {
		errPayload, _ := json.Marshal(map[string]string{"error": callErr.Error()})
		return s.finalizeAnalysisFailure(ctx, job, assetID, model.JobStatusFailed, callErr, errPayload)
	}

	analysisPayload, err := validatePostureAgentResponse(respBody, input.ConfigurationID)
	if err != nil {
		errPayload, _ := json.Marshal(map[string]string{"error": "posture agent identity validation failed"})
		return s.finalizeAnalysisFailure(ctx, job, assetID, model.JobStatusFailed, err, errPayload)
	}
	s.recordPostureGovernance(ctx, job, analysisPayload)
	_ = s.jobRuntime.UpdateProgress(ctx, job.ID, map[string]any{"stage": "posture_completed", "percent": 100})
	if err := s.jobRuntime.TransitionTo(ctx, job.ID, model.JobStatusCompleted, json.RawMessage(respBody), nil); err != nil {
		return fmt.Errorf("complete posture job lifecycle: %w", err)
	}
	completed, err := s.postureRuns.CompleteRun(ctx, run.ID, job.UserID, input.ConfigurationID, analysisPayload)
	if err != nil {
		return fmt.Errorf("complete posture run lifecycle: %w", err)
	}
	if !completed {
		return fmt.Errorf("complete posture run lifecycle: competing terminal transition already won")
	}
	return nil
}

// finalizeAnalysisFailure terminates the job and moves the derived posture
// run of the asset to failed. Terminal results are monotonic.
func (s *UploadService) finalizeAnalysisFailure(
	ctx context.Context,
	job model.Job,
	assetID uuid.UUID,
	jobStatus model.JobStatus,
	cause error,
	payload json.RawMessage,
) error {
	message := "posture analysis failed"
	if cause != nil {
		message = cause.Error()
	} else if jobStatus == model.JobStatusTimedOut {
		message = "stale posture job timed out"
	}
	if err := s.jobRuntime.TransitionTo(ctx, job.ID, jobStatus, nil, map[string]string{"error": message}); err != nil {
		return fmt.Errorf("finalize posture job lifecycle: %w", err)
	}
	if s.postureRuns != nil {
		run, err := s.postureRuns.GetOwnedByAsset(ctx, assetID, job.UserID)
		if err != nil {
			return fmt.Errorf("fail posture run lifecycle: %w", err)
		}
		if run != nil {
			updated, err := s.postureRuns.FailRun(ctx, run.ID, job.UserID, payload)
			if err != nil {
				return fmt.Errorf("fail posture run lifecycle: %w", err)
			}
			if !updated {
				return fmt.Errorf("fail posture run lifecycle: competing terminal transition already won")
			}
		}
	}
	return cause
}

func (s *UploadService) timeoutPostureJob(ctx context.Context, job model.Job) error {
	input, err := parsePostureJobInput(job)
	if err != nil {
		return s.jobRuntime.TransitionTo(ctx, job.ID, model.JobStatusTimedOut, nil, map[string]string{"error": err.Error()})
	}
	assetID, parseErr := uuid.Parse(input.AssetID)
	if parseErr != nil {
		return s.jobRuntime.TransitionTo(ctx, job.ID, model.JobStatusTimedOut, nil, map[string]string{"error": "stale posture job timed out"})
	}
	return s.finalizeAnalysisFailure(ctx, job, assetID, model.JobStatusTimedOut, nil, json.RawMessage(`{"error":"stale posture job timed out"}`))
}

func parsePostureJobInput(job model.Job) (postureJobInput, error) {
	var input postureJobInput
	if err := json.Unmarshal(job.Input, &input); err != nil {
		return input, fmt.Errorf("parse posture job input: %w", err)
	}
	if input.AssetID == "" || input.View == "" || input.ConfigurationID == "" {
		return input, fmt.Errorf("posture job input missing required fields")
	}
	return input, nil
}

// validatePostureAgentResponse verifies the posture agent answered under the
// exact registered configuration and provenance, then pins the Go decision
// trace into the persisted payload.
func validatePostureAgentResponse(respBody []byte, expectedConfigurationID string) ([]byte, error) {
	registration, ok := knownPostureConfigurations[strings.TrimSpace(expectedConfigurationID)]
	if !ok {
		return nil, fmt.Errorf("unknown Posture Agent configuration id %q", expectedConfigurationID)
	}
	var envelope struct {
		Status string         `json:"status"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil, fmt.Errorf("decode posture agent response: %w", err)
	}
	if envelope.Status != "completed" || envelope.Result == nil {
		return nil, fmt.Errorf("posture agent returned incomplete response")
	}
	configuration, ok := envelope.Result["agent_configuration"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("posture agent response missing agent_configuration")
	}
	if id, _ := configuration["id"].(string); id != expectedConfigurationID {
		return nil, fmt.Errorf("posture agent configuration mismatch: got %q want %q", id, expectedConfigurationID)
	}
	if role, _ := configuration["role"].(string); role != "posture" {
		return nil, fmt.Errorf("posture agent role mismatch: %q", role)
	}
	if policy, _ := configuration["decision_policy_revision"].(string); policy != registration.DecisionPolicyRevision {
		return nil, fmt.Errorf("posture decision policy mismatch: %q", policy)
	}
	if logicalModel, _ := configuration["logical_model"].(string); logicalModel != registration.LogicalModel {
		return nil, fmt.Errorf("posture logical model mismatch: %q", logicalModel)
	}
	if registration.MechanismRevision != "" {
		if revision, _ := configuration["geometry_mechanism_revision"].(string); revision != registration.MechanismRevision {
			return nil, fmt.Errorf("posture configuration mechanism mismatch: %q", revision)
		}
	}
	execution, ok := envelope.Result["execution_provenance"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("posture agent response missing execution_provenance")
	}
	if logicalModel, _ := execution["logical_model"].(string); logicalModel != registration.LogicalModel {
		return nil, fmt.Errorf("posture execution logical model mismatch: %q", logicalModel)
	}
	if err := validatePostureMechanismProvenance(envelope.Result, registration); err != nil {
		return nil, err
	}
	trace := map[string]any{
		"decision":                 "persist",
		"authority":                "go",
		"agent_configuration_id":   expectedConfigurationID,
		"decision_policy_revision": registration.DecisionPolicyRevision,
		"logical_model":            registration.LogicalModel,
	}
	if registration.MechanismRevision != "" {
		trace["mechanism_revision"] = registration.MechanismRevision
		trace["model_sha256"] = registration.ModelSHA256
		trace["threshold_revision"] = registration.ThresholdRevision
		trace["threshold_sha256"] = registration.ThresholdSHA256
	}
	envelope.Result["generation_decision_trace"] = trace
	return json.Marshal(envelope.Result)
}

func validatePostureMechanismProvenance(
	result map[string]any,
	registration postureConfigurationRegistration,
) error {
	if registration.MechanismRevision == "" {
		return nil
	}
	mechanism, ok := result["mechanism_provenance"].(map[string]any)
	if !ok {
		return errors.New("posture agent response missing mechanism_provenance")
	}
	if status, _ := mechanism["status"].(string); status != "verified" {
		return fmt.Errorf("posture mechanism status mismatch: %q", status)
	}
	expected := map[string]string{
		"mechanism_revision": registration.MechanismRevision,
		"engine":             registration.Engine,
		"engine_version":     registration.EngineVersion,
		"model_uri":          registration.ModelURI,
		"model_sha256":       registration.ModelSHA256,
		"threshold_revision": registration.ThresholdRevision,
		"threshold_sha256":   registration.ThresholdSHA256,
	}
	for field, want := range expected {
		got, _ := mechanism[field].(string)
		if got != want {
			return fmt.Errorf("posture mechanism %s mismatch: got %q want %q", field, got, want)
		}
	}
	return nil
}

// recordPostureGovernance audits the P2 gate result for posture analysis jobs.
func (s *UploadService) recordPostureGovernance(ctx context.Context, job model.Job, respBody []byte) {
	if s.outputReviewService == nil || len(respBody) == 0 {
		return
	}

	var parsed map[string]any
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return
	}

	verdict := "unknown"
	issues := datatypes.JSON("[]")
	var validated datatypes.JSON

	if gov, ok := parsed["governance"].(map[string]any); ok {
		if v, ok := gov["verdict"].(string); ok && v != "" {
			verdict = v
		}
		if rawIssues, ok := gov["issues"]; ok {
			if b, err := json.Marshal(rawIssues); err == nil {
				issues = datatypes.JSON(b)
			}
		}
	}

	if verdict == "accepted" || verdict == "degraded" {
		safe := make(map[string]any, len(parsed))
		for k, v := range parsed {
			if k == "governance" || k == "safety_fallback" {
				continue
			}
			safe[k] = v
		}
		if b, err := json.Marshal(safe); err == nil {
			validated = datatypes.JSON(b)
		}
	}

	jobID := job.ID
	userID := job.UserID
	s.outputReviewService.RecordReview(
		ctx,
		"posture",
		verdict,
		&userID,
		nil,
		&jobID,
		nil,
		issues,
		validated,
		datatypes.JSON(respBody),
	)
}

// PostureAnalysisView is one completed single-view analysis, stripped of raw
// file metadata so consultation tools and assessment can consume it safely.
type PostureAnalysisView struct {
	UploadID       string          `json:"upload_id"`
	View           string          `json:"view"`
	FileType       string          `json:"file_type"`
	AnalysisStatus string          `json:"analysis_status"`
	Analysis       json.RawMessage `json:"analysis"`
	CreatedAt      time.Time       `json:"created_at"`
}

// PostureAnalysisSummary is the Agent/assessment-facing aggregate of the
// user's latest completed three-view posture analyses.
type PostureAnalysisSummary struct {
	HasAnalysis bool                  `json:"has_analysis"`
	Views       []PostureAnalysisView `json:"views"`
	// Findings is a flattened list of finding objects across views for quick use.
	Findings []any `json:"findings"`
	// Summaries is per-view summary_markdown text when present.
	Summaries []string `json:"summaries"`
}

// BuildPostureAnalysisSummary collapses completed posture runs into the compact
// shape shared by the read-only HTTP endpoint, consultation business context,
// and assessment reuse. Prefer the newest completed run per view.
func BuildPostureAnalysisSummary(runs []model.PostureAnalysisRun) PostureAnalysisSummary {
	summary := PostureAnalysisSummary{
		HasAnalysis: false,
		Views:       []PostureAnalysisView{},
		Findings:    []any{},
		Summaries:   []string{},
	}
	if len(runs) == 0 {
		return summary
	}

	seenView := map[string]bool{}
	for _, run := range runs {
		if seenView[run.View] {
			continue
		}
		if len(run.ResultPayload) == 0 {
			continue
		}
		seenView[run.View] = true
		summary.HasAnalysis = true
		summary.Views = append(summary.Views, PostureAnalysisView{
			UploadID:       run.AssetID.String(),
			View:           run.View,
			FileType:       run.FileType(),
			AnalysisStatus: string(run.Status),
			Analysis:       json.RawMessage(run.ResultPayload),
			CreatedAt:      run.CreatedAt,
		})

		var parsed map[string]any
		if err := json.Unmarshal(run.ResultPayload, &parsed); err == nil {
			if findings, ok := parsed["findings"].([]any); ok {
				summary.Findings = append(summary.Findings, findings...)
			}
			if text, ok := parsed["summary_markdown"].(string); ok && text != "" {
				summary.Summaries = append(summary.Summaries, text)
			}
		}
	}
	return summary
}

// GetPostureAnalysisSummary loads and collapses the caller's completed posture
// analyses. Empty result (no completed analysis) is not an error.
func (s *UploadService) GetPostureAnalysisSummary(ctx context.Context, userID uuid.UUID) (PostureAnalysisSummary, error) {
	runs, err := s.postureRuns.ListCompletedByUser(ctx, userID)
	if err != nil {
		return PostureAnalysisSummary{}, err
	}
	return BuildPostureAnalysisSummary(runs), nil
}
