package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ---------------------------------------------------------------------------
// Posture analysis pipeline (mirrors the OCR pipeline in upload_jobs_ocr.go)
// ---------------------------------------------------------------------------

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
	respBody, callErr := s.aiClient.AnalyzePosture(reader, asset.MimeType, input.View, input.ConfigurationID)
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

// ---------------------------------------------------------------------------
// Response validation and governance audit
// ---------------------------------------------------------------------------

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
