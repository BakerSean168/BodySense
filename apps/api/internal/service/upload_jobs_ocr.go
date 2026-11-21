package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

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
	respBody, callErr := s.healthDocumentClient.ExtractHealthDocument(io.TeeReader(reader, documentHasher), asset.MimeType, input.ConfigurationID)
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
