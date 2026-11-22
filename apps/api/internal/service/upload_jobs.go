package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Job inputs and worker bootstrap
// ---------------------------------------------------------------------------

type ocrJobInput struct {
	AssetID         string `json:"asset_id"`
	ConfigurationID string `json:"configuration_id,omitempty"`
}

type postureJobInput struct {
	AssetID         string `json:"asset_id"`
	View            string `json:"view"` // "front" | "side" | "back"
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
				slog.Default().Error("upload_job_recovery_failed", "error", err)
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
					slog.Default().Error("process_job_failed", "job_type", jt.name, "job_id", job.ID, "error", err)
				}
				processed++
			case "running":
				if err := jt.timeout(ctx, job); err != nil {
					slog.Default().Error("timeout_stale_job_failed", "job_type", jt.name, "job_id", job.ID, "error", err)
				}
				processed++
			}
		}
	}
	return processed, nil
}

// ---------------------------------------------------------------------------
// Job creation
// ---------------------------------------------------------------------------

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
